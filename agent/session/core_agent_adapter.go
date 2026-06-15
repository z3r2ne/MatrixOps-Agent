package session

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"matrixops-agent/llm"
	"matrixops-agent/plugin"
	"matrixops-agent/tool"
	"matrixops-agent/types"
	coreagent "matrixops.local/core_agent"
	actioncompatible "matrixops.local/core_agent/action_providers/compatible"
	"matrixops.local/core_agent/workersv2/generic"
	database "pkgs/db"
	"pkgs/db/models"

	"gorm.io/gorm"
)

type coreEmitterAdapter struct {
	mu        sync.Mutex
	inner     *Emitter
	onMessage func(*coreagent.Message)
}

func (a *coreEmitterAdapter) sessionPartsForRecord(parts []*coreagent.Part) []*Part {
	a.mu.Lock()
	defer a.mu.Unlock()
	return corePartsToSession(parts)
}

func (a *coreEmitterAdapter) UpdateMessage(info *coreagent.Message) (*coreagent.Message, error) {
	a.mu.Lock()
	sessionInfo := coreMessageToSession(info)
	a.mu.Unlock()
	updated, err := a.inner.UpdateMessage(sessionInfo)
	if err != nil {
		return nil, err
	}
	coreInfo := sessionMessageToCore(updated)
	if a.onMessage != nil {
		a.onMessage(coreInfo)
	}
	return coreInfo, nil
}

func (a *coreEmitterAdapter) UpdatePart(part *coreagent.Part) (*coreagent.Part, error) {
	a.mu.Lock()
	sessionPart := corePartToSession(part)
	a.mu.Unlock()
	updated, err := a.inner.UpdatePart(sessionPart)
	if err != nil {
		return nil, err
	}
	return sessionPartToCore(updated), nil
}

func (a *coreEmitterAdapter) Emit(name string, payload interface{}) {
	if name == coreagent.EventAssistantFooterStatus {
		if p, ok := payload.(coreagent.AssistantFooterStatusPayload); ok {
			a.inner.Emit(EventAssistantFooterStatus, AssistantFooterStatusEvent{
				MessageID: p.MessageID,
				Text:      p.Text,
				Loading:   p.Loading,
			})
			return
		}
	}
	a.inner.Emit(name, payload)
}

type coreLLMClientAdapter struct {
	inner         llm.ChatClient
	pluginManager *plugin.Manager
}

type coreNoopLLMClient struct{}

func (c *coreNoopLLMClient) Chat(request llm.ChatRequest) (llm.ChatResponse, error) {
	return llm.ChatResponse{}, fmt.Errorf("llm client is not configured")
}

func (a *coreLLMClientAdapter) Chat(request coreagent.ChatRequest) (coreagent.ChatResponse, error) {
	llmRequest, err := a.toLLMRequest(request)
	if err != nil {
		return coreagent.ChatResponse{}, err
	}
	if err := a.applyPluginRequest(&llmRequest); err != nil {
		return coreagent.ChatResponse{}, err
	}
	response, err := a.inner.Chat(llmRequest)
	if err != nil {
		return coreagent.ChatResponse{}, err
	}
	return coreChatResponseFromLLM(response), nil
}

func (a *coreLLMClientAdapter) StreamChatWithOptions(request coreagent.ChatRequest, opts ...coreagent.StreamChatOption) (<-chan coreagent.StreamEvent, error) {
	options := coreagent.NewStreamChatOptions(opts...)
	if options.OnRequest != nil {
		if err := options.OnRequest(&request); err != nil {
			return nil, err
		}
	}

	llmRequest, err := a.toLLMRequest(request)
	if err != nil {
		return nil, err
	}
	if err := a.applyPluginRequest(&llmRequest); err != nil {
		return nil, err
	}

	streamer, ok := a.inner.(llm.StreamChatClientWithOptions)
	if !ok {
		return nil, fmt.Errorf("llm client does not support stream chat with options")
	}
	stream, err := streamer.StreamChatWithOptions(
		llmRequest,
		llm.WithHTTPClient(options.HTTPClient),
		llm.WithOnRawRequest(options.OnRawRequest),
		llm.WithOnRawResponse(options.OnRawResponse),
		llm.WithOnRetryError(options.OnRetryError),
	)
	if err != nil {
		return nil, err
	}

	// Buffered so forwarding from the LLM client does not synchronously block on each event
	// while core_agent.StreamV2 is busy writing to the parse pipe.
	out := make(chan coreagent.StreamEvent, 64)
	go func() {
		defer close(out)
		for event := range stream {
			out <- coreStreamEventFromLLM(event)
		}
	}()
	return out, nil
}

func (a *coreLLMClientAdapter) toLLMRequest(request coreagent.ChatRequest) (llm.ChatRequest, error) {
	providerOptions, _ := request.ProviderOptions.(*models.LLMConfig)
	messages := make([]*llm.ModelMessage, 0, len(request.Messages))
	for _, message := range request.Messages {
		if message == nil {
			continue
		}
		messages = append(messages, &llm.ModelMessage{
			Role:       message.Role,
			Content:    message.Content,
			Name:       message.Name,
			ToolCallID: message.ToolCallID,
			ToolCalls:  coreToolCallsToLLM(message.ToolCalls),
		})
	}
	return llm.ChatRequest{
		Messages:        messages,
		Context:         request.Context,
		Tools:           coreToolDefinitionsToLLM(request.Tools),
		Temperature:     request.Temperature,
		TopP:            request.TopP,
		MaxOutputTokens: request.MaxOutputTokens,
		ProviderOptions: providerOptions,
		Model:           request.Model,
		ExtraOptions:    request.ExtraOptions,
	}, nil
}

func (a *coreLLMClientAdapter) applyPluginRequest(request *llm.ChatRequest) error {
	if a.pluginManager == nil {
		return nil
	}
	return a.pluginManager.ApplyLLMRequest(&plugin.LLMRequest{Kind: plugin.LLMRequestKindChat, Chat: request})
}

type coreSessionToolAdapter struct {
	runner        *AgentRunner
	runtimeConfig *RuntimeConfig
	tool          tool.Tool
}

func (a *coreSessionToolAdapter) Name() string {
	return a.tool.Name()
}

func (a *coreSessionToolAdapter) Description() string {
	return a.tool.Description()
}

func (a *coreSessionToolAdapter) Schema() map[string]interface{} {
	return a.tool.Schema()
}

func (a *coreSessionToolAdapter) Execute(ctx coreagent.ToolContext, input map[string]interface{}) (coreagent.ToolResult, error) {
	call := llm.ToolCall{Name: a.tool.Name(), Arguments: input}
	sessionToolCtx := tool.Context{
		Context:   ctx.Context,
		SessionID: ctx.SessionID,
		Directory: ctx.Directory,
		Worktree:  ctx.Worktree,
		Values:    ctx.Values,
	}
	if updateHandler, ok := ctx.Values["tool_event_handler"].(func(tool.StreamEvent)); ok {
		sessionToolCtx.OnEvent = updateHandler
	}
	plan := a.runner.prepareToolCallPlanWithContext(a.runtimeConfig, a.runner.session, a.runtimeConfig.Worker, call, sessionToolCtx)
	result, err := plan.Run()
	if err != nil {
		return coreToolResultFromSession(result), err
	}
	return coreToolResultFromSession(result), nil
}

func (r *AgentRunner) effectiveMaxOutputTokens(runtimeConfig *RuntimeConfig) int {
	limit := 0
	if runtimeConfig != nil && runtimeConfig.ModelSettings != nil {
		limit = runtimeConfig.ModelSettings.OutputLimit
	}
	return database.EffectiveLLMMaxOutputTokens(r.db, limit)
}

func (r *AgentRunner) buildCoreRunner(runtimeConfig *RuntimeConfig, input ProcessInputV2) (*coreagent.Runner, *coreagent.RunState, error) {
	tracingHTTPClient := r.ensureLLMHTTPClient(runtimeConfig)
	latestRawRequest := ""
	llmClient := runtimeConfig.LLMClient
	if llmClient == nil {
		llmClient = &coreNoopLLMClient{}
	}

	model := runtimeConfig.Model
	temperature := 0.0
	topP := 0.0
	if runtimeConfig.Worker != nil {
		model = runtimeConfig.Worker.Model
		if runtimeConfig.Worker.Temperature != nil {
			temperature = *runtimeConfig.Worker.Temperature
		}
		topP = runtimeConfig.Worker.TopP
	}

	coreEmitter := &coreEmitterAdapter{inner: r.emitter, onMessage: func(info *coreagent.Message) { copyCoreMessageIntoSession(runtimeConfig.Assistant, info) }}
	silentToolThreshold, onSilentToolStreak := r.resolveSilentToolWatchdogRunnerOpts(runtimeConfig)

	runner, err := coreagent.NewRunner(coreagent.RunnerConfig{
		Emitter:       coreEmitter,
		LLMClient:     &coreLLMClientAdapter{inner: llmClient, pluginManager: r.pluginManager},
		PromptBuilder: r.buildCorePromptBuilder(runtimeConfig, input.UserPrompt),
		ToolExecutor: func(actionCtx *coreagent.ActionContext, toolInstance coreagent.Tool, call coreagent.ToolCall, toolCtx coreagent.ToolContext) (coreagent.ToolResult, error) {
			if actionCtx != nil {
				part := actionCtx.GetToolPart(call.ID)
				if part != nil {
					if toolCtx.Values == nil {
						toolCtx.Values = map[string]interface{}{}
					}
					toolCtx.Values["tool_event_handler"] = newCoreToolProgressReporter(actionCtx, part)
				}
			}
			result, execErr := toolInstance.Execute(toolCtx, call.Arguments)
			if execErr != nil {
				return result, execErr
			}
			return result, nil
		},
		Hooks: coreagent.RunnerHooks{
			AppendUserText: func(state *coreagent.RunState, text string) error {
				if runtimeConfig.MemoryState != nil {
					llmParts, err := BuildUnifiedLLMContentParts(runtimeConfig.Parts, r.GetDirectory())
					if err != nil {
						return err
					}
					runtimeConfig.MemoryState.AppendUserText(text, llmParts)
				}
				return nil
			},
			PrepareMemory: func(state *coreagent.RunState) error {
				return r.prepareProcessV2Memory(runtimeConfig)
			},
			BuildMemory: func(state *coreagent.RunState) (any, error) {
				return r.buildProcessV2Memory(runtimeConfig)
			},
			PersistTokens: func(state *coreagent.RunState, tokens *coreagent.MessageTokens) error {
				sessionTokens := coreMessageTokensToSession(tokens)
				runtimeConfig.Assistant.Tokens = &sessionTokens
				return r.persistProcessV2Tokens(runtimeConfig, &sessionTokens)
			},
			RecordAction: func(state *coreagent.RunState, rawOutput string, parts []*coreagent.Part) error {
				if runtimeConfig.MemoryState != nil {
					sourceMessageID := ""
					if state != nil && state.Assistant != nil {
						sourceMessageID = strings.TrimSpace(state.Assistant.ID)
					}
					runtimeConfig.MemoryState.RecordAction(rawOutput, coreEmitter.sessionPartsForRecord(parts), sourceMessageID)
				}
				return nil
			},
			OnAnswer: func(state *coreagent.RunState) error {
				if runtimeConfig.Assistant.Snapshot == "" {
					return nil
				}
				return r.captureProcessV2Finish(runtimeConfig)
			},
			AfterLLMCall: func(state *coreagent.RunState, info *coreagent.LLMCallInfo) error {
				return r.persistPromptSnapshot(state, info)
			},
		},
		ToolContextBuilder: func(state *coreagent.RunState) coreagent.ToolContext {
			return r.buildSessionCoreToolContext(state)
		},
		Tools:                    r.buildCoreToolRegistry(runtimeConfig),
		Model:                    model,
		Temperature:              temperature,
		TopP:                     topP,
		MaxOutputTokens:          r.effectiveMaxOutputTokens(runtimeConfig),
		ProviderOptions:          runtimeConfig.LLMConfig,
		MaxSteps:                 input.MaxSteps,
		SystemPromptPlacement:    resolveSystemPromptPlacement(runtimeConfig),
		NativeOpenAIToolCalls:    runtimeConfig.LLMConfig != nil && runtimeConfig.LLMConfig.NativeOpenAIToolCalls,
		ReasoningEffort:          runtimeConfig.ReasoningEffort,
		TextVerbosity:            runtimeConfig.TextVerbosity,
		EnableEncryptedReasoning: runtimeConfig.EnableEncryptedReasoning,
		ParallelToolCalls:        runtimeConfig.ParallelToolCalls,
		PromptCacheKey:           runtimeConfig.PromptCacheKey,
		ThinkingType:             runtimeConfig.ThinkingType,
		EnableThinking:           runtimeConfig.EnableThinking,
		BudgetTokens:             runtimeConfig.BudgetTokens,
		StallWatchdogTimeout:     runtimeConfig.StallWatchdogTimeout,
		RepeatedToolCallThreshold: coreagent.DefaultRepeatedToolCallThreshold,
		OnRepeatedToolCall:       r.buildRepeatedToolCallHandler(runtimeConfig),
		SilentToolCallThreshold:  silentToolThreshold,
		OnSilentToolStreak:       onSilentToolStreak,
		OnStallWatchdogToolCancelled: r.buildStallWatchdogToolCancelledHandler(runtimeConfig),
		MessageQueue:             r.messageQueue,
		ConsumeSupplement: func(_ *coreagent.RunState, item models.TaskMessageQueueItem) error {
			return r.deliverSupplementUserMessage(runtimeConfig, item)
		},
	})
	if err != nil {
		return nil, nil, err
	}

	state := &coreagent.RunState{
		Context:             runtimeConfig.Ctx,
		SessionID:           r.GetSessionID(),
		Assistant:           sessionMessageToCore(runtimeConfig.Assistant),
		UserInput:           input.UserPrompt,
		Tools:               resolveV2PromptTools(runtimeConfig),
		MaxSteps:            input.MaxSteps,
		SingleStep:          input.ExecuteOnce,
		EnableCallToolReason: runtimeConfig.EnableCallToolReason,
		CompatibleActionSchemas: append([]coreagent.ActionSchema(nil), runtimeConfig.ActionSchemas...),
		HTTPClient:          tracingHTTPClient,
		OnRawRequest: func(raw string) {
			latestRawRequest = raw
		},
		OnRetryError: func(err error, retryAttempt int, maxRetries int, nextDelay time.Duration, attemptDuration time.Duration, rawResponse string) {
			providerName := ""
			if runtimeConfig.LLMConfig != nil {
				providerName = runtimeConfig.LLMConfig.Name
			}
			_ = r.handleEmptyStreamRetry(err)
			r.logLLMAPIAttempt(runtimeConfig, "stream_chat_attempt", latestRawRequest, rawResponse, err, retryAttempt, maxRetries, nextDelay, attemptDuration)
			retryMessageError := buildLLMRetryMessageError(err, providerName, retryAttempt, maxRetries, nextDelay)
			r.emitAssistantErrorPart(runtimeConfig.Assistant, retryMessageError)
			r.emitter.Emit(EventSessionError, SessionErrorEvent{
				SessionID: runtimeConfig.Assistant.SessionID,
				Error:     retryMessageError,
			})
		},
	}
	copyCoreMessageIntoSession(runtimeConfig.Assistant, state.Assistant)
	return runner, state, nil
}

func (r *AgentRunner) buildGenericAgentConfig(runtimeConfig *RuntimeConfig, input ProcessInputV2) generic.AgentConfig {
	llmClient := runtimeConfig.LLMClient
	if llmClient == nil {
		llmClient = &coreNoopLLMClient{}
	}

	model := runtimeConfig.Model
	temperature := 0.0
	topP := 0.0
	workerName := ""
	extJSON := ""
	if runtimeConfig.Worker != nil {
		model = runtimeConfig.Worker.Model
		if runtimeConfig.Worker.Temperature != nil {
			temperature = *runtimeConfig.Worker.Temperature
		}
		topP = runtimeConfig.Worker.TopP
		workerName = runtimeConfig.Worker.Name
		extJSON = runtimeConfig.Worker.ExtConfig
	}

	coreEmitter := &coreEmitterAdapter{inner: r.emitter, onMessage: func(info *coreagent.Message) { copyCoreMessageIntoSession(runtimeConfig.Assistant, info) }}
	silentToolThreshold, onSilentToolStreak := r.resolveSilentToolWatchdogRunnerOpts(runtimeConfig)

	return generic.AgentConfig{
		Name: workerName,
		Loop: generic.LoopPromptSettings{
			Builder: r.buildCorePromptBuilder(runtimeConfig, input.UserPrompt),
		},
		Memory: generic.MemorySystem{
			AppendUserText: func(state *coreagent.RunState, text string) error {
				if runtimeConfig.MemoryState != nil {
					llmParts, err := BuildUnifiedLLMContentParts(runtimeConfig.Parts, r.GetDirectory())
					if err != nil {
						return err
					}
					runtimeConfig.MemoryState.AppendUserText(text, llmParts)
				}
				return nil
			},
			Prepare: func(state *coreagent.RunState) error {
				return r.prepareProcessV2Memory(runtimeConfig)
			},
			Build: func(state *coreagent.RunState) (any, error) {
				return r.buildProcessV2Memory(runtimeConfig)
			},
			PersistTokens: func(state *coreagent.RunState, tokens *coreagent.MessageTokens) error {
				sessionTokens := coreMessageTokensToSession(tokens)
				runtimeConfig.Assistant.Tokens = &sessionTokens
				return r.persistProcessV2Tokens(runtimeConfig, &sessionTokens)
			},
			RecordAction: func(state *coreagent.RunState, rawOutput string, parts []*coreagent.Part) error {
				if runtimeConfig.MemoryState != nil {
					sourceMessageID := ""
					if state != nil && state.Assistant != nil {
						sourceMessageID = strings.TrimSpace(state.Assistant.ID)
					}
					runtimeConfig.MemoryState.RecordAction(rawOutput, coreEmitter.sessionPartsForRecord(parts), sourceMessageID)
				}
				return nil
			},
			OnAnswer: func(state *coreagent.RunState) error {
				if runtimeConfig.Assistant.Snapshot == "" {
					return nil
				}
				return r.captureProcessV2Finish(runtimeConfig)
			},
			AfterLLMCall: func(state *coreagent.RunState, info *coreagent.LLMCallInfo) error {
				return r.persistPromptSnapshot(state, info)
			},
		},
		LLM: generic.LLMSettings{
			Client:          &coreLLMClientAdapter{inner: llmClient, pluginManager: r.pluginManager},
			Model:           model,
			Temperature:     temperature,
			TopP:            topP,
			MaxOutputTokens: r.effectiveMaxOutputTokens(runtimeConfig),
			ProviderOptions: runtimeConfig.LLMConfig,
		},
		Runtime: generic.RuntimeSettings{
			Emitter: coreEmitter,
			Tools:   r.buildCoreToolRegistry(runtimeConfig),
			ToolExecutor: func(actionCtx *coreagent.ActionContext, toolInstance coreagent.Tool, call coreagent.ToolCall, toolCtx coreagent.ToolContext) (coreagent.ToolResult, error) {
				if actionCtx != nil {
					part := actionCtx.GetToolPart(call.ID)
					if part != nil {
						if toolCtx.Values == nil {
							toolCtx.Values = map[string]interface{}{}
						}
						toolCtx.Values["tool_event_handler"] = newCoreToolProgressReporter(actionCtx, part)
					}
				}
				result, execErr := toolInstance.Execute(toolCtx, call.Arguments)
				if execErr != nil {
					return result, execErr
				}
				return result, nil
			},
			ToolContextBuilder: func(state *coreagent.RunState) coreagent.ToolContext {
				return r.buildSessionCoreToolContext(state)
			},
			MaxSteps:                 input.MaxSteps,
			SystemPromptPlacement:    resolveSystemPromptPlacement(runtimeConfig),
			NativeOpenAIToolCalls:    runtimeConfig.LLMConfig != nil && runtimeConfig.LLMConfig.NativeOpenAIToolCalls,
			ReasoningEffort:          runtimeConfig.ReasoningEffort,
			TextVerbosity:            runtimeConfig.TextVerbosity,
			EnableEncryptedReasoning: runtimeConfig.EnableEncryptedReasoning,
			ParallelToolCalls:        runtimeConfig.ParallelToolCalls,
			PromptCacheKey:           runtimeConfig.PromptCacheKey,
			ThinkingType:             runtimeConfig.ThinkingType,
			EnableThinking:           runtimeConfig.EnableThinking,
			BudgetTokens:             runtimeConfig.BudgetTokens,
			StallWatchdogTimeout:     runtimeConfig.StallWatchdogTimeout,
			RepeatedToolCallThreshold: coreagent.DefaultRepeatedToolCallThreshold,
			OnRepeatedToolCall:       r.buildRepeatedToolCallHandler(runtimeConfig),
			SilentToolCallThreshold:  silentToolThreshold,
			OnSilentToolStreak:       onSilentToolStreak,
			OnStallWatchdogToolCancelled: r.buildStallWatchdogToolCancelledHandler(runtimeConfig),
			MessageQueue:             r.messageQueue,
			ConsumeSupplement: func(_ *coreagent.RunState, item models.TaskMessageQueueItem) error {
				return r.deliverSupplementUserMessage(runtimeConfig, item)
			},
		},
		Ext: generic.ExtConfigFromJSON(extJSON),
	}
}

func (r *AgentRunner) resolveSilentToolWatchdogRunnerOpts(runtimeConfig *RuntimeConfig) (threshold int, handler func(state *coreagent.RunState, count int) error) {
	if r == nil || runtimeConfig == nil || !runtimeConfig.SilentToolWatchdogEnabled {
		return 0, nil
	}
	return coreagent.DefaultSilentToolCallThreshold, r.buildSilentToolStreakHandler(runtimeConfig)
}

func resolveSystemPromptPlacement(runtimeConfig *RuntimeConfig) string {
	if runtimeConfig == nil || runtimeConfig.ModelSettings == nil {
		return coreagent.SystemPromptPlacementUserInput
	}
	return coreagent.NormalizeSystemPromptPlacement(runtimeConfig.ModelSettings.SystemPromptPlacement)
}

// BuildGenericAgentConfigFromDB 根据 workerName 从数据库走与 buildRuntimeConfig 相同的路径得到 RuntimeConfig，再生成 generic.AgentConfig。
//
// db 非空时会通过 WithDB 写入 AgentRunnerConfig，并在 buildRuntimeConfig 中优先于 r.db 使用（便于测试库或读写分离连接）。
// db 为空时回退为 r.db。仅 (db, workerName) 仍不足以构造可执行配置：Emitter、会话目录等仍依赖 AgentRunner。
// input.UserPrompt 会写入运行配置中的用户输入；input.MaxSteps 等参与 AgentConfig。
func (r *AgentRunner) BuildGenericAgentConfigFromDB(ctx context.Context, db *gorm.DB, workerName string, input ProcessInputV2) (generic.AgentConfig, error) {
	if r == nil {
		return generic.AgentConfig{}, fmt.Errorf("agent runner is required")
	}
	if db == nil {
		db = r.db
	}
	if db == nil {
		return generic.AgentConfig{}, fmt.Errorf("db is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	cfg := NewAgentRunnerConfig(
		WithCtx(ctx),
		WithWorker(workerName),
		WithInputText(input.UserPrompt),
		WithSkipCreateUserMessage(true),
		WithDB(db),
	)
	rc, err := r.buildRuntimeConfig(cfg)
	if err != nil {
		return generic.AgentConfig{}, err
	}
	return r.buildGenericAgentConfig(rc, input), nil
}

func (r *AgentRunner) buildSessionCoreToolContext(state *coreagent.RunState) coreagent.ToolContext {
	if state == nil {
		return coreagent.ToolContext{}
	}
	return coreagent.ToolContext{
		Context:   state.Context,
		SessionID: r.GetSessionID(),
		Directory: r.GetDirectory(),
		Worktree:  r.GetDirectory(),
		Values:    coreagent.EnrichToolContextValues(state, nil),
	}
}

func (r *AgentRunner) buildCoreToolRegistry(runtimeConfig *RuntimeConfig) *coreagent.ToolRegistry {
	registry := coreagent.NewToolRegistry()
	if runtimeConfig == nil || runtimeConfig.ToolRegistry == nil {
		return registry
	}
	allowed := map[string]struct{}{}
	for _, definition := range runtimeConfig.Tools {
		allowed[definition.Name] = struct{}{}
	}
	for _, name := range runtimeConfig.ToolRegistry.Names() {
		if len(allowed) > 0 {
			if _, ok := allowed[name]; !ok {
				continue
			}
		}
		instance, err := runtimeConfig.ToolRegistry.Get(name)
		if err != nil {
			continue
		}
		registry.Register(&coreSessionToolAdapter{runner: r, runtimeConfig: runtimeConfig, tool: instance})
	}
	return registry
}

func (r *AgentRunner) buildCorePromptBuilder(runtimeConfig *RuntimeConfig, userPrompt string) coreagent.PromptBuilder {
	builderName := coreagent.DefaultPromptBuilderName
	if runtimeConfig != nil && runtimeConfig.Worker != nil {
		switch strings.TrimSpace(runtimeConfig.Worker.Name) {
		case "leader":
			builderName = coreagent.LeaderPromptBuilderName
		case "frontend_engineer":
			builderName = coreagent.FrontendEngineerPromptBuilderName
		}
	}
	return coreagent.MustCreatePromptBuilder(builderName, coreagent.PromptBuilderOptions{
		ContextInfoBuilder: func(state *coreagent.RunState) *coreagent.ContextInfo {
			memory, _ := state.Memory.(*types.Memory)
			if memory == nil {
				memory = &types.Memory{}
			}
			info := buildPromptContextInfo(runtimeConfig, memory, state.UserInput)
			if info == nil {
				return nil
			}
			return &coreagent.ContextInfo{
				LimitTokens:   info.LimitTokens,
				CurrentTokens: info.CurrentTokens,
				CurrentBytes:  info.CurrentBytes,
			}
		},
	})
}

func buildProcessV2ErrorCommand(err error) string {
	message := err.Error()
	switch {
	case strings.Contains(message, "missing @action"), strings.Contains(message, "missing call_tool"), strings.Contains(message, "missing call_tool/params"):
		return "stream_v2_missing_action"
	case strings.Contains(message, "unknown action type"), strings.Contains(message, "current mode does not allow action type"), strings.Contains(message, "answer must be the last action"):
		return "stream_v2_action_error"
	default:
		return "stream_v2_error"
	}
}

func sessionMessageToCore(info *MessageInfo) *coreagent.Message {
	if info == nil {
		return nil
	}
	message := &coreagent.Message{
		ID:         info.ID,
		SessionID:  info.SessionID,
		Role:       coreagent.Role(info.Role),
		ParentID:   info.ParentID,
		Occupation: info.Occupation,
		Worker:     info.Worker,
		Provider:   info.Provider,
		Model:      info.Model,
		ProviderID: info.ProviderID,
		ModelID:    info.ModelID,
		System:     info.System,
		Tools:      cloneBoolMap(info.Tools),
		Variant:    info.Variant,
		Summary:    info.Summary,
		Finish:     info.Finish,
		Cost:       info.Cost,
		Memory:     info.Memory,
		Tokens:     sessionMessageTokensToCore(info.Tokens),
		Error:      sessionMessageErrorToCore(info.Error),
		State:      info.State,
		Snapshot:   info.Snapshot,
		Time: coreagent.MessageTime{
			Created:   info.Time.Created,
			Completed: info.Time.Completed,
		},
	}
	if info.Path != nil {
		message.Path = &coreagent.MessagePath{Cwd: info.Path.Cwd, Root: info.Path.Root}
	}
	return message
}

func coreMessageToSession(info *coreagent.Message) *MessageInfo {
	if info == nil {
		return nil
	}
	var memory *types.Memory
	if typedMemory, ok := info.Memory.(*types.Memory); ok {
		memory = typedMemory
	}
	message := &MessageInfo{
		ID:         info.ID,
		SessionID:  info.SessionID,
		Role:       Role(info.Role),
		ParentID:   info.ParentID,
		Occupation: info.Occupation,
		Worker:     info.Worker,
		Provider:   info.Provider,
		Model:      info.Model,
		ProviderID: info.ProviderID,
		ModelID:    info.ModelID,
		System:     info.System,
		Tools:      cloneBoolMap(info.Tools),
		Variant:    info.Variant,
		Summary:    info.Summary,
		Finish:     info.Finish,
		Cost:       info.Cost,
		Memory:     memory,
		Tokens:     coreMessageTokensToSessionPtr(info.Tokens),
		Error:      coreMessageErrorToSession(info.Error),
		State:      info.State,
		Snapshot:   info.Snapshot,
		Time: MessageTime{
			Created:   info.Time.Created,
			Completed: info.Time.Completed,
		},
	}
	if info.Path != nil {
		message.Path = &MessagePath{Cwd: info.Path.Cwd, Root: info.Path.Root}
	}
	return message
}

func copyCoreMessageIntoSession(target *MessageInfo, source *coreagent.Message) {
	if target == nil || source == nil {
		return
	}
	copied := coreMessageToSession(source)
	*target = *copied
}

func sessionPartToCore(part *Part) *coreagent.Part {
	if part == nil {
		return nil
	}
	converted := &coreagent.Part{
		ID:          part.ID,
		MessageID:   part.MessageID,
		SessionID:   part.SessionID,
		Type:        part.Type,
		Text:        part.Text,
		Reasoning:   part.Reasoning,
		Synthetic:   part.Synthetic,
		Ignored:     part.Ignored,
		Metadata:    cloneAnyMap(part.Metadata),
		Snapshot:    part.Snapshot,
		Hash:        part.Hash,
		Files:       append([]string(nil), part.Files...),
		Mime:        part.Mime,
		Filename:    part.Filename,
		Path:        part.Path,
		InputSource: part.InputSource,
		URL:         part.URL,
		Source:      part.Source,
		AgentName:   part.AgentName,
		Auto:        part.Auto,
		Description: part.Description,
		Subagent:    part.Subagent,
		Command:     part.Command,
		Attempt:     part.Attempt,
		Reason:      part.Reason,
		Cost:        part.Cost,
		Tokens:      sessionMessageTokensToCore(part.Tokens),
		Error:       sessionMessageErrorToCore(part.Error),
	}
	if part.Time != nil {
		converted.Time = &coreagent.PartTime{Start: part.Time.Start, End: part.Time.End, Created: part.Time.Created, Compacted: part.Time.Compacted}
	}
	if part.Tool != nil {
		converted.Tool = &coreagent.ToolPart{Name: part.Tool.Name, CallID: part.Tool.CallID, State: sessionToolStateToCore(part.Tool.State), Metadata: cloneAnyMap(part.Tool.Metadata)}
	}
	return converted
}

func corePartToSession(part *coreagent.Part) *Part {
	if part == nil {
		return nil
	}
	converted := &Part{
		ID:          part.ID,
		MessageID:   part.MessageID,
		SessionID:   part.SessionID,
		Type:        part.Type,
		Text:        part.Text,
		Reasoning:   part.Reasoning,
		Synthetic:   part.Synthetic,
		Ignored:     part.Ignored,
		Metadata:    cloneAnyMap(part.Metadata),
		Snapshot:    part.Snapshot,
		Hash:        part.Hash,
		Files:       append([]string(nil), part.Files...),
		Mime:        part.Mime,
		Filename:    part.Filename,
		Path:        part.Path,
		InputSource: part.InputSource,
		URL:         part.URL,
		Source:      part.Source,
		AgentName:   part.AgentName,
		Auto:        part.Auto,
		Description: part.Description,
		Subagent:    part.Subagent,
		Command:     part.Command,
		Attempt:     part.Attempt,
		Reason:      part.Reason,
		Cost:        part.Cost,
		Tokens:      coreMessageTokensToSessionPtr(part.Tokens),
		Error:       coreMessageErrorToSession(part.Error),
	}
	if part.Time != nil {
		converted.Time = &PartTime{Start: part.Time.Start, End: part.Time.End, Created: part.Time.Created, Compacted: part.Time.Compacted}
	}
	if part.Tool != nil {
		converted.Tool = &ToolPart{Name: part.Tool.Name, CallID: part.Tool.CallID, State: coreToolStateToSession(part.Tool.State), Metadata: cloneAnyMap(part.Tool.Metadata)}
	}
	return converted
}

func corePartsToSession(parts []*coreagent.Part) []*Part {
	out := make([]*Part, 0, len(parts))
	for _, part := range parts {
		out = append(out, corePartToSession(part))
	}
	return out
}

func cloneBoolMap(input map[string]bool) map[string]bool {
	if input == nil {
		return nil
	}
	out := make(map[string]bool, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func cloneAnyMap(input map[string]interface{}) map[string]interface{} {
	if input == nil {
		return nil
	}
	out := make(map[string]interface{}, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func sessionToolStateToCore(state ToolState) coreagent.ToolState {
	attachments := make([]coreagent.Part, 0, len(state.Attachments))
	for _, attachment := range state.Attachments {
		if converted := sessionPartToCore(&attachment); converted != nil {
			attachments = append(attachments, *converted)
		}
	}
	return coreagent.ToolState{
		Status:         state.Status,
		Input:          state.Input,
		Raw:            state.Raw,
		Title:          state.Title,
		SystemMessage:  state.SystemMessage,
		Output:         state.Output,
		Error:          state.Error,
		Metadata:       cloneAnyMap(state.Metadata),
		MemoryMetadata: cloneAnyMap(state.MemoryMetadata),
		FullOutput:     state.FullOutput,
		Attachments:    attachments,
		Time:           coreagent.PartTime{Start: state.Time.Start, End: state.Time.End, Created: state.Time.Created, Compacted: state.Time.Compacted},
	}
}

func coreToolStateToSession(state coreagent.ToolState) ToolState {
	attachments := make([]Part, 0, len(state.Attachments))
	for _, attachment := range state.Attachments {
		if converted := corePartToSession(&attachment); converted != nil {
			attachments = append(attachments, *converted)
		}
	}
	return ToolState{
		Status:         state.Status,
		Input:          state.Input,
		Raw:            state.Raw,
		Title:          state.Title,
		SystemMessage:  state.SystemMessage,
		Output:         state.Output,
		Error:          state.Error,
		Metadata:       cloneAnyMap(state.Metadata),
		MemoryMetadata: cloneAnyMap(state.MemoryMetadata),
		FullOutput:     state.FullOutput,
		Attachments:    attachments,
		Time:           PartTime{Start: state.Time.Start, End: state.Time.End, Created: state.Time.Created, Compacted: state.Time.Compacted},
	}
}

func sessionMessageTokensToCore(tokens *MessageTokens) *coreagent.MessageTokens {
	if tokens == nil {
		return nil
	}
	return &coreagent.MessageTokens{Input: tokens.Input, Output: tokens.Output, Reasoning: tokens.Reasoning, Cache: coreagent.TokenCache{Read: tokens.Cache.Read, Write: tokens.Cache.Write}}
}

func coreMessageTokensToSession(tokens *coreagent.MessageTokens) MessageTokens {
	if tokens == nil {
		return MessageTokens{}
	}
	return MessageTokens{Input: tokens.Input, Output: tokens.Output, Reasoning: tokens.Reasoning, Cache: TokenCache{Read: tokens.Cache.Read, Write: tokens.Cache.Write}}
}

func coreMessageTokensToSessionPtr(tokens *coreagent.MessageTokens) *MessageTokens {
	if tokens == nil {
		return nil
	}
	converted := coreMessageTokensToSession(tokens)
	return &converted
}

func sessionMessageErrorToCore(err *MessageError) *coreagent.MessageError {
	if err == nil {
		return nil
	}
	return &coreagent.MessageError{Name: err.Name, Message: err.Message, ProviderID: err.ProviderID, StatusCode: err.StatusCode, IsRetryable: err.IsRetryable, ResponseBody: err.ResponseBody, ResponseHeaders: err.ResponseHeaders, Metadata: err.Metadata}
}

func coreMessageErrorToSession(err *coreagent.MessageError) *MessageError {
	if err == nil {
		return nil
	}
	return &MessageError{Name: err.Name, Message: err.Message, ProviderID: err.ProviderID, StatusCode: err.StatusCode, IsRetryable: err.IsRetryable, ResponseBody: err.ResponseBody, ResponseHeaders: err.ResponseHeaders, Metadata: err.Metadata}
}

func coreToolDefinitionsFromLLM(defs []llm.ToolDefinition) []coreagent.ToolDefinition {
	out := make([]coreagent.ToolDefinition, 0, len(defs))
	for _, def := range defs {
		out = append(out, coreagent.ToolDefinition{Name: def.Name, Description: def.Description, Schema: def.Schema})
	}
	return out
}

func coreToolDefinitionsToLLM(defs []coreagent.ToolDefinition) []llm.ToolDefinition {
	out := make([]llm.ToolDefinition, 0, len(defs))
	for _, def := range defs {
		out = append(out, llm.ToolDefinition{Name: def.Name, Description: def.Description, Schema: def.Schema})
	}
	return out
}

func coreToolCallsToLLM(calls []coreagent.ToolCall) []llm.ToolCall {
	out := make([]llm.ToolCall, 0, len(calls))
	for _, call := range calls {
		out = append(out, llm.ToolCall{ID: call.ID, Name: call.Name, Arguments: call.Arguments})
	}
	return out
}

func coreChatResponseFromLLM(response llm.ChatResponse) coreagent.ChatResponse {
	return coreagent.ChatResponse{Message: coreagent.ModelMessage{Role: response.Message.Role, Content: response.Message.Content, Name: response.Message.Name, ToolCallID: response.Message.ToolCallID, ToolCalls: llmToolCallsToCore(response.Message.ToolCalls)}, ToolCalls: llmToolCallsToCore(response.ToolCalls), Finish: response.Finish, Usage: llmUsageToCore(response.Usage)}
}

func llmToolCallsToCore(calls []llm.ToolCall) []coreagent.ToolCall {
	out := make([]coreagent.ToolCall, 0, len(calls))
	for _, call := range calls {
		out = append(out, coreagent.ToolCall{ID: call.ID, Name: call.Name, Arguments: call.Arguments})
	}
	return out
}

func coreStreamEventFromLLM(event llm.StreamEvent) coreagent.StreamEvent {
	return coreagent.StreamEvent{Type: event.Type, Text: event.Text, ToolIndex: event.ToolIndex, ToolCallID: event.ToolCallID, ToolName: event.ToolName, ToolArguments: event.ToolArguments, Finish: event.Finish, Usage: llmUsageToCore(event.Usage), Error: event.Error}
}

func llmUsageToCore(usage *llm.Usage) *coreagent.Usage {
	if usage == nil {
		return nil
	}
	return &coreagent.Usage{InputTokens: usage.InputTokens, OutputTokens: usage.OutputTokens, ReasoningTokens: usage.ReasoningTokens, CachedInputTokens: usage.CachedInputTokens}
}

func coreToolResultFromSession(result tool.Result) coreagent.ToolResult {
	return coreagent.ToolResult{
		Content:            result.Content,
		Message:            result.Message,
		FullContent:        result.FullContent,
		IsError:            result.IsError,
		Truncated:          result.Truncated,
		PreserveFullOutput: result.PreserveFullOutput,
		OutputPath:         result.OutputPath,
		Title:          result.Title,
		Metadata:       cloneAnyMap(result.Metadata),
		MemoryMetadata: cloneAnyMap(result.MemoryMetadata),
		Vars:           cloneAnyMap(result.Vars),
		Name:           result.Name,
	}
}

func (r *AgentRunner) handleV2ActionWithCore(runtimeConfig *RuntimeConfig, action *ActionOutput) ([]*Part, error) {
	coreRunner, state, err := r.buildCoreRunner(runtimeConfig, ProcessInputV2{
		MaxSteps:    r.resolveAgentMaxSteps(runtimeConfig),
		ExecuteOnce: false,
		UserPrompt:  runtimeConfig.UserInput,
	})
	if err != nil {
		return nil, err
	}
	coreAction := &coreagent.ActionOutput{
		Index:   action.Index,
		Action:  action.Action,
		Data:    action.Data,
		RawJSON: action.RawJSON,
	}
	if actioncompatible.IsControlAction(coreAction.Action) {
		parts, err := coreRunner.HandleAction(state, coreAction)
		if err != nil {
			return nil, err
		}
		return corePartsToSession(parts), nil
	}
	name := strings.TrimSpace(coreAction.Action)
	if name == "" {
		return nil, fmt.Errorf("tool call action is empty")
	}
	parts, err := coreRunner.ExecuteToolCallRequest(state, &coreagent.CallToolRequest{
		Index:     coreAction.Index,
		Name:      name,
		Arguments: coreAction.Data,
		RawJSON:   coreAction.RawJSON,
	})
	if err != nil {
		return nil, err
	}
	return corePartsToSession(parts), nil
}
