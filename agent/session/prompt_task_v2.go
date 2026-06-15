package session

import (
	"fmt"
	"strings"
	"time"

	"matrixops-agent/token"
	"matrixops-agent/types"
	coreagent "matrixops.local/core_agent"
	"matrixops.local/core_agent/workersv2/generic"
	database "pkgs/db"
	"pkgs/db/models"
)

// ProcessInputV2 V2版本的处理输入
type ProcessInputV2 struct {
	MaxSteps    int
	ExecuteOnce bool
	UserPrompt  string
}

func (r *AgentRunner) resolveAgentMaxSteps(runtimeConfig *RuntimeConfig) int {
	maxSteps := database.GetAgentMaxSteps(r.db)
	if runtimeConfig != nil && runtimeConfig.Worker != nil && runtimeConfig.Worker.Steps > 0 {
		maxSteps = runtimeConfig.Worker.Steps
	}
	return maxSteps
}

// runTaskV2 V2版本的任务执行（新接口）。
// 默认使用 runtimeConfig 的输入；测试/预览场景可传入 input 覆盖 MaxSteps/ExecuteOnce 等选项。
func (r *AgentRunner) runTaskV2(runtimeConfig *RuntimeConfig, inputOverrides ...ProcessInputV2) error {
	input := ProcessInputV2{
		MaxSteps:    r.resolveAgentMaxSteps(runtimeConfig),
		ExecuteOnce: false,
		UserPrompt:  runtimeConfig.UserInput,
	}
	if len(inputOverrides) > 0 {
		input = inputOverrides[0]
		if input.UserPrompt == "" {
			input.UserPrompt = runtimeConfig.UserInput
		}
	}
	tracingHTTPClient := r.ensureLLMHTTPClient(runtimeConfig)
	latestRawRequest := ""

	ac := r.buildGenericAgentConfig(runtimeConfig, input)
	w, err := generic.New(generic.WithFullAgentConfig(ac))
	if err != nil {
		return err
	}

	coreAssistant := sessionMessageToCore(runtimeConfig.Assistant)
	copyCoreMessageIntoSession(runtimeConfig.Assistant, coreAssistant)

	res, err := w.Chat(runtimeConfig.Ctx, input.UserPrompt, generic.ChatOptions{
		SessionID:   r.GetSessionID(),
		Assistant:   coreAssistant,
		MaxSteps:    input.MaxSteps,
		ExecuteOnce: input.ExecuteOnce,
		Tools:       resolveV2PromptTools(runtimeConfig),
		HTTPClient:          tracingHTTPClient,
		OnRawRequest: func(raw string) {
			latestRawRequest = raw
		},
		OnRetryError: func(callErr error, retryAttempt int, maxRetries int, nextDelay time.Duration, attemptDuration time.Duration, rawResponse string) {
			providerName := ""
			if runtimeConfig.LLMConfig != nil {
				providerName = runtimeConfig.LLMConfig.Name
			}
			_ = r.handleEmptyStreamRetry(callErr)
			r.logLLMAPIAttempt(runtimeConfig, "stream_chat_attempt", latestRawRequest, rawResponse, callErr, retryAttempt, maxRetries, nextDelay, attemptDuration)
			retryMessageError := buildLLMRetryMessageError(callErr, providerName, retryAttempt, maxRetries, nextDelay)
			r.emitAssistantErrorPart(runtimeConfig.Assistant, retryMessageError)
			r.emitter.Emit(EventSessionError, SessionErrorEvent{
				SessionID: runtimeConfig.Assistant.SessionID,
				Error:     retryMessageError,
			})
		},
	})

	if res != nil && res.Result != nil && res.Result.State != nil && res.Result.State.Assistant != nil {
		copyCoreMessageIntoSession(runtimeConfig.Assistant, res.Result.State.Assistant)
	}
	if err == nil {
		return nil
	}

	if isUserCancelledContext(runtimeConfig.Ctx, err) {
		runtimeConfig.Assistant.State = "completed"
		runtimeConfig.Assistant.Finish = userCancelledConversationReason
		runtimeConfig.Assistant.Error = nil
		r.clearAssistantFooterStatus(runtimeConfig.Assistant.ID)
		r.emitAssistantUserCancelledPart(runtimeConfig.Assistant)
		_, _ = r.emitter.UpdateMessage(runtimeConfig.Assistant)
		return err
	}

	rawRequest := ""
	rawResponse := ""
	if res != nil && res.Result != nil && res.Result.State != nil && res.Result.State.LastLLMCall != nil {
		call := res.Result.State.LastLLMCall
		rawRequest = call.RawRequest
		// 解析失败等场景：优先记录模型侧累积文本（与 Runner 成功路径 RawOutput 一致），便于对照错误位置
		if strings.TrimSpace(call.RawOutput) != "" {
			rawResponse = call.RawOutput
		} else {
			rawResponse = call.RawResponse
		}
	}
	r.logProcessV2PromptError(runtimeConfig, buildProcessV2ErrorCommand(err), err.Error(), rawRequest, rawResponse)

	providerName := ""
	if runtimeConfig.LLMConfig != nil {
		providerName = runtimeConfig.LLMConfig.Name
	}
	messageError := FromError(err, providerName)
	runtimeConfig.Assistant.Error = messageError
	r.emitAssistantErrorPart(runtimeConfig.Assistant, messageError)
	r.emitter.Emit(EventSessionError, SessionErrorEvent{
		SessionID: runtimeConfig.Assistant.SessionID,
		Error:     messageError,
	})
	_, _ = r.emitter.UpdateMessage(runtimeConfig.Assistant)
	return fmt.Errorf("run task v2: %w", err)
}

func (r *AgentRunner) logV2PromptError(command string, errorText string, rawRequest string, rawResponse string) {
	if r.db == nil {
		return
	}

	logEntry := &models.CommandLog{
		Source:     "llm_action_error",
		SourceName: fmt.Sprintf("Session %s", r.GetSessionID()),
		Command:    command,
		WorkDir:    r.GetDirectory(),
		StdinData:  rawRequest,
		Stdout:     rawResponse,
		Fields:     models.LegacyCommandLogFields(rawRequest, rawResponse, "", errorText),
		Error:      errorText,
		Status:     "failed",
		CreatedAt:  time.Now(),
		FinishedAt: func() *time.Time { now := time.Now(); return &now }(),
	}
	if r.task != nil {
		logEntry.SourceID = &r.task.ID
	}
	_ = database.CreateCommandLog(r.db, logEntry)
}

// handleCallToolV2 处理工具调用动作
func (r *AgentRunner) handleCallToolV2(runtimeConfig *RuntimeConfig, action *ActionOutput) ([]*Part, error) {
	return r.handleV2ActionWithCore(runtimeConfig, action)
}

// handleMessageV2 处理发送消息动作
func (r *AgentRunner) handleMessageV2(runtimeConfig *RuntimeConfig, action *ActionOutput) (*Part, error) {
	parts, err := r.handleV2ActionWithCore(runtimeConfig, action)
	if err != nil {
		return nil, err
	}
	for _, part := range parts {
		if part != nil && part.Type == types.PartTypeText {
			return part, nil
		}
	}
	return nil, fmt.Errorf("message field is empty")
}

func resolveV2PromptTools(runtimeConfig *RuntimeConfig) []coreagent.ToolDefinition {
	worker := coreToolDefinitionsFromLLM(runtimeConfig.Tools)
	return coreagent.MergePromptToolDefinitions(worker, coreagent.PromptToolMergeOptions{ExcludeAnswer: true})
}

func buildPromptContextInfo(runtimeConfig *RuntimeConfig, memory *types.Memory, userInput string) *coreagent.ContextInfo {
	limit := 0
	if runtimeConfig != nil {
		if runtimeConfig.AutoCompressionLimitTokens > 0 {
			limit = runtimeConfig.AutoCompressionLimitTokens
		} else if runtimeConfig.ModelSettings != nil {
			if runtimeConfig.ModelSettings.ContextLimit > 0 {
				outputLimit := runtimeConfig.ModelSettings.OutputLimit
				if outputLimit < 0 {
					outputLimit = 0
				}
				limit = runtimeConfig.ModelSettings.ContextLimit - outputLimit
				if limit < 0 {
					limit = runtimeConfig.ModelSettings.ContextLimit
				}
			}
		}
	}

	currentBytes := 0
	currentTokens := currentContextTokensFromUsage(runtimeConfig)
	if memory != nil {
		if len(memory.Entries) > 0 {
			currentBytes = totalMemoryBytes(memory.Entries)
		} else {
			promptContent := memory.PromptContent()
			currentBytes = len([]byte(promptContent))
			if currentTokens == 0 {
				currentTokens = token.Estimate(promptContent)
			}
		}
	}
	if currentTokens == 0 {
		currentTokens += token.Estimate(userInput)
	}
	currentBytes += len([]byte(userInput))

	return &coreagent.ContextInfo{
		LimitTokens:   limit,
		CurrentTokens: currentTokens,
		CurrentBytes:  currentBytes,
	}
}

func currentContextTokensFromUsage(runtimeConfig *RuntimeConfig) int {
	if runtimeConfig == nil {
		return 0
	}

	tokens := runtimeConfig.SessionTokens
	if runtimeConfig.Assistant != nil && runtimeConfig.Assistant.Tokens != nil {
		tokens = runtimeConfig.Assistant.Tokens
	}
	if tokens == nil {
		return 0
	}

	currentTokens := tokens.Input
	if tokens.Cache.Read > 0 {
		currentTokens += tokens.Cache.Read
	}
	if currentTokens < 0 {
		return 0
	}
	return currentTokens
}

// captureFinishSnapshot 拍摄结束快照并生成补丁
func (r *AgentRunner) captureFinishSnapshotV2(runtimeConfig *RuntimeConfig) error {
	title := runtimeConfig.UserInput
	if len(title) > 25 {
		runes := []rune(title)
		if len(runes) > 20 {
			title = string(runes[:10]) + "..." + string(runes[len(runes)-10:])
		}
	}
	return r.emitFinishSnapshotPatch(runtimeConfig, title)
}
