package coreagent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"matrixops.local/core_agent/streamtypes"
	actioncompatible "matrixops.local/core_agent/action_providers/compatible"
	"pkgs/db/models"
	"pkgs/taskqueue"
)

const assistantFooterPreviewMaxRunes = 120

// DefaultMaxRunSteps 在 RunState.MaxSteps 与 RunnerConfig.MaxSteps 均未设置时的回退值。
// 应与 pkgs/db/models.DefaultAgentMaxSteps 保持一致。
const DefaultMaxRunSteps = 1000

type PromptBuilder func(state *RunState) (string, error)

type RunnerHooks struct {
	AppendUserText func(state *RunState, text string) error
	PrepareMemory  func(state *RunState) error
	BuildMemory    func(state *RunState) (any, error)
	PersistTokens  func(state *RunState, tokens *MessageTokens) error
	RecordAction   func(state *RunState, rawOutput string, parts []*Part) error
	AfterStep      func(state *RunState) error
	OnAnswer       func(state *RunState) error
	AfterLLMCall   func(state *RunState, info *LLMCallInfo) error
}

type ToolExecutor func(actionCtx *ActionContext, tool Tool, call ToolCall, toolCtx ToolContext) (ToolResult, error)

type ToolContextBuilder func(state *RunState) ToolContext

type RunnerConfig struct {
	Emitter               Emitter
	LLMClient             ChatClient
	ProviderName          string
	PromptBuilder         PromptBuilder
	Hooks                 RunnerHooks
	ToolContextBuilder    ToolContextBuilder
	ToolExecutor          ToolExecutor
	Tools                 *ToolRegistry
	Model                 string
	Temperature           float64
	TopP                  float64
	MaxOutputTokens       int
	ProviderOptions       any
	MaxSteps              int
	IDGenerator           IDGenerator
	Now                   func() time.Time
	SystemPromptPlacement string
	AnswerActionType      ActionDataType
	// NativeOpenAIToolCalls 为 true 时，使用 github.com/openai/openai-go 流式 Chat Completions，
	// 以原生 tools/function 调用接口并从响应 message.tool_calls 解析工具（需 ProviderOptions 提供 API Key 等）。
	// 为 false（默认）时保持兼容模式：提示词约定 JSON 动作，经 StreamV2 从模型文本流解析。
	NativeOpenAIToolCalls    bool
	ReasoningEffort          string
	TextVerbosity            string
	EnableEncryptedReasoning bool
	ParallelToolCalls        bool
	PromptCacheKey           string
	ThinkingType             string
	EnableThinking           *bool
	BudgetTokens             *int
	// ActionProvider 覆盖默认的流式 action 提供者。为空时，NewRunner 使用
	// newDefaultActionProvider 根据 NativeOpenAIToolCalls 自动创建实现。
	ActionProvider ActionProvider
	// StallWatchdogTimeout 为工具 stall watchdog 触发超时。≤0 表示禁用。
	StallWatchdogTimeout time.Duration
	// RepeatedToolCallThreshold 为连续相同工具调用触发 repeat watchdog 的次数；≤0 时使用 DefaultRepeatedToolCallThreshold。
	RepeatedToolCallThreshold int
	// OnRepeatedToolCall 在连续相同工具调用达到阈值时回调（例如向任务消息队列注入系统警告）。
	OnRepeatedToolCall func(state *RunState, toolName string, args map[string]interface{}, count int) error
	// SilentToolCallThreshold 为连续无思考/文本输出的工具调用触发看门狗的次数；≤0 时使用 DefaultSilentToolCallThreshold。
	SilentToolCallThreshold int
	// OnSilentToolStreak 在连续无文字输出的工具调用达到阈值时回调（例如注入补充系统提示）。
	OnSilentToolStreak func(state *RunState, count int) error
	// OnStallWatchdogToolCancelled 在看门狗取消工具执行时回调（例如向任务消息队列注入系统提示）。
	OnStallWatchdogToolCancelled func(state *RunState, toolName, callID, reason string, elapsed time.Duration) error
	// MessageQueue 任务消息队列；非空时在每步结束后消费 supplement 并入对话。
	MessageQueue *taskqueue.Queue
	// ConsumeSupplement 消费一条 supplement 队列项（创建用户消息、同步 memory 等）。
	ConsumeSupplement func(state *RunState, item models.TaskMessageQueueItem) error
	// AuthorizeToolCall 在工具真正执行前、看门狗计时开始前调用。返回 (nil, nil) 表示放行；
	// 非 nil *ToolResult 表示拦截（如权限拒绝），不再执行工具。
	AuthorizeToolCall func(actionCtx *ActionContext, call ToolCall) (*ToolResult, error)
}

type LLMCallInfo struct {
	Prompt         string
	RawRequest     string
	RawResponse    string
	RawOutput      string
	Actions        []string
	ParsedResponse string
	Usage          *Usage
	Error          error
}

type RunState struct {
	Context   context.Context
	SessionID string
	Assistant *Message
	UserInput string
	Tools     []ToolDefinition
	Memory    any
	Prompt    string
	Step      int
	MaxSteps  int
	// SingleStep, when true, runs at most one model iteration (tools may still run inside that step). MaxSteps is not rewritten.
	SingleStep bool
	// MaxStepsExhaustedFinalPass selects the answer-only summary prompt after the main loop hits MaxSteps without answer.
	MaxStepsExhaustedFinalPass bool
	// OmitAnswerInPromptMerge 为 true 时，PromptBuilder 在 Tools 为空时用 MergePromptToolDefinitions 且不并入 answer（原生 OpenAI 模式由 runner 在每轮请求前设置）。
	OmitAnswerInPromptMerge bool
	// NativeOpenAIToolCalls 为 true 时，提示词走「原生工具调用」版式（不要求正文 JSON、不在提示中重复各工具的 Schema）。
	NativeOpenAIToolCalls bool
	Data                  map[string]any
	LastLLMCall           *LLMCallInfo
	HTTPClient            *http.Client
	OnRawRequest          func(raw string)
	OnRawResponse         func(raw string)
	OnRetryError          func(err error, retryAttempt int, maxRetries int, nextDelay time.Duration, attemptDuration time.Duration, rawResponse string)
	// EnableCallToolReason 兼容模式下 call_tool action 是否要求 reason 字段。
	EnableCallToolReason bool
	// CompatibleActionSchemas 兼容模式 prompt 注入的 action 列表；为空时由 ActionProvider 使用默认 SessionActionSchemas。
	CompatibleActionSchemas []ActionSchema
	// ToolRepeatTracker 跟踪连续相同工具调用，供 repeat watchdog 使用。
	ToolRepeatTracker *ToolRepeatTracker
	// SilentToolTracker 跟踪连续无思考/文本输出的工具调用，供 silent tool watchdog 使用。
	SilentToolTracker *SilentToolTracker
	// NextToolStallWatchdogTimeout 为下一次工具调用的看门狗超时 override；消费后清零。
	NextToolStallWatchdogTimeout time.Duration
}

type Runner struct {
	emitter      Emitter
	llmClient    ChatClient
	prompt       PromptBuilder
	hooks        RunnerHooks
	toolCtx      ToolContextBuilder
	toolExec     ToolExecutor
	tools        *ToolRegistry
	actions      map[string]ActionHandler
	actionOrder  []string
	cfg          RunnerConfig
	messageQueue *taskqueue.Queue
}

func cloneMessage(message *Message) *Message {
	if message == nil {
		return nil
	}
	cloned := *message
	if message.Tools != nil {
		cloned.Tools = make(map[string]bool, len(message.Tools))
		for key, value := range message.Tools {
			cloned.Tools[key] = value
		}
	}
	if message.Tokens != nil {
		tokenCopy := *message.Tokens
		cloned.Tokens = &tokenCopy
	}
	if message.Error != nil {
		errorCopy := *message.Error
		if message.Error.ResponseHeaders != nil {
			errorCopy.ResponseHeaders = make(map[string]string, len(message.Error.ResponseHeaders))
			for key, value := range message.Error.ResponseHeaders {
				errorCopy.ResponseHeaders[key] = value
			}
		}
		cloned.Error = &errorCopy
	}
	if message.Path != nil {
		pathCopy := *message.Path
		cloned.Path = &pathCopy
	}
	cloned.Summary = message.Summary
	cloned.Memory = message.Memory
	if len(message.ResponsesReasoningItemRaws) > 0 {
		cloned.ResponsesReasoningItemRaws = append([]string(nil), message.ResponsesReasoningItemRaws...)
	}
	return &cloned
}

func (r *Runner) prepareAssistantMessageForStep(state *RunState, stepNumber int) {
	if state == nil || state.Assistant == nil {
		return
	}
	if stepNumber <= 1 {
		return
	}
	next := cloneMessage(state.Assistant)
	if next == nil {
		return
	}
	next.ID = r.nextID("message")
	next.ParentID = ""
	next.Finish = ""
	next.State = "loading"
	next.Error = nil
	next.Cost = 0
	next.Memory = nil
	next.Tokens = nil
	next.Summary = nil
	next.ResponsesOutputMessageRaw = ""
	next.ResponsesReasoningItemRaws = nil
	now := r.now().UnixMilli()
	next.Time = MessageTime{Created: now}
	state.Assistant = next
}

func NewRunner(cfg RunnerConfig) (*Runner, error) {
	if cfg.PromptBuilder == nil {
		cfg.PromptBuilder = MustCreatePromptBuilder(DefaultPromptBuilderName, PromptBuilderOptions{})
	}
	if cfg.IDGenerator == nil {
		cfg.IDGenerator = DefaultIDGenerator
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	cfg.SystemPromptPlacement = NormalizeSystemPromptPlacement(cfg.SystemPromptPlacement)
	if cfg.AnswerActionType == "" {
		cfg.AnswerActionType = ActionDataTypeText
	}
	if cfg.Emitter == nil {
		cfg.Emitter = NoEmitter{}
	}
	if cfg.LLMClient == nil {
		cfg.LLMClient = MustCreateProviderClient(cfg.ProviderName)
	}
	if cfg.Tools == nil {
		cfg.Tools = NewToolRegistry()
	}
	if cfg.ActionProvider == nil {
		cfg.ActionProvider = newDefaultActionProvider(cfg.NativeOpenAIToolCalls, cfg.LLMClient)
	}

	runner := &Runner{
		emitter:      cfg.Emitter,
		llmClient:    cfg.LLMClient,
		prompt:       cfg.PromptBuilder,
		hooks:        cfg.Hooks,
		toolCtx:      cfg.ToolContextBuilder,
		toolExec:     cfg.ToolExecutor,
		tools:        cfg.Tools,
		actions:      map[string]ActionHandler{},
		actionOrder:  make([]string, 0, 8),
		cfg:          cfg,
		messageQueue: cfg.MessageQueue,
	}
	runner.RegisterDefaultActions()
	return runner, nil
}

func (r *Runner) RegisterAction(handler ActionHandler) {
	if handler == nil {
		return
	}
	schema := handler.Schema()
	name := strings.TrimSpace(schema.ActionName)
	if name == "" {
		return
	}
	if _, exists := r.actions[name]; !exists {
		r.actionOrder = append(r.actionOrder, name)
	}
	r.actions[name] = handler
}

func (r *Runner) RegisterTool(tool Tool) {
	r.tools.Register(tool)
}

func (r *Runner) HandleAction(state *RunState, action *ActionOutput) ([]*Part, error) {
	if state == nil {
		return nil, fmt.Errorf("run state is required")
	}
	if action == nil {
		return nil, fmt.Errorf("action is nil")
	}
	if strings.TrimSpace(action.Action) == "call_tool" {
		return nil, fmt.Errorf("call_tool must be dispatched via ToolCalls channel")
	}
	if !r.isActionAllowed(state, action.Action) {
		return nil, fmt.Errorf("current mode does not allow action type: %s", action.Action)
	}
	name := strings.TrimSpace(action.Action)
	ctx := &ActionContext{
		Context: state.Context,
		Runner:  r,
		State:   state,
		Prompt:  state.Prompt,
		Memory:  state.Memory,
	}
	handler, ok := r.actions[name]
	if !ok {
		return nil, fmt.Errorf("unknown compatible control action: %s", name)
	}
	return handler.Handle(ctx, action)
}

// ExecuteToolCallRequest runs a normalized tool invocation from StreamOutput.ToolCalls.
func (r *Runner) ExecuteToolCallRequest(state *RunState, req *CallToolRequest) ([]*Part, error) {
	return r.executeToolCallRequest(state, req)
}

func (r *Runner) executeToolCallRequest(state *RunState, req *CallToolRequest) ([]*Part, error) {
	if state == nil {
		return nil, fmt.Errorf("run state is required")
	}
	if req == nil || strings.TrimSpace(req.Name) == "" {
		return nil, fmt.Errorf("tool call request is empty")
	}
	name := strings.TrimSpace(req.Name)
	ctx := &ActionContext{
		Context: state.Context,
		Runner:  r,
		State:   state,
		Prompt:  state.Prompt,
		Memory:  state.Memory,
	}
	if name == "call_tool" {
		action := &ActionOutput{
			Index:   req.Index,
			Action:  name,
			Data:    req.Arguments,
			RawJSON: strings.TrimSpace(req.RawJSON),
		}
		return r.handleCallTool(ctx, action)
	}
	if !r.isActionAllowed(state, name) {
		return nil, fmt.Errorf("current mode does not allow tool: %s", name)
	}
	action := &ActionOutput{
		Index:   req.Index,
		Action:  name,
		Data:    req.Arguments,
		RawJSON: strings.TrimSpace(req.RawJSON),
	}
	if handler, ok := r.actions[name]; ok {
		return handler.Handle(ctx, action)
	}
	return r.handleDirectRegistryTool(ctx, name, action)
}

// executeAgentIteration runs one model round (prepare memory → prompt → stream → actions).
// When the model ends with answer, it finalizes the assistant message and returns answerSeen=true.
func (r *Runner) executeAgentIteration(state *RunState, stepNumber int, updatePart func(*Part) error) (answerSeen bool, err error) {
	state.NativeOpenAIToolCalls = r.cfg.NativeOpenAIToolCalls
	state.Step = stepNumber
	select {
	case <-state.Context.Done():
		return false, state.Context.Err()
	default:
	}

	startPart := &Part{
		ID:        r.nextID("part"),
		MessageID: state.Assistant.ID,
		SessionID: state.SessionID,
		Type:      PartTypeStartStep,
		Time:      &PartTime{Start: r.now().UnixMilli(), End: r.now().UnixMilli()},
	}
	if err := updatePart(startPart); err != nil {
		return false, err
	}

	if r.hooks.PrepareMemory != nil {
		if err := r.hooks.PrepareMemory(state); err != nil {
			return false, fmt.Errorf("prepare memory: %w", err)
		}
	}
	if r.hooks.BuildMemory != nil {
		memory, err := r.hooks.BuildMemory(state)
		if err != nil {
			return false, fmt.Errorf("build memory: %w", err)
		}
		state.Memory = memory
	}

	var promptTools []ToolDefinition
	var streamTools []ToolDefinition
	if r.cfg.NativeOpenAIToolCalls {
		streamTools = OmitAnswerToolDefinitions(state.Tools)
		if state.MaxStepsExhaustedFinalPass {
			promptTools = state.Tools
		} else {
			state.OmitAnswerInPromptMerge = true
			promptTools = OmitAnswerToolDefinitions(state.Tools)
			if len(promptTools) == 0 {
				promptTools = MergePromptToolDefinitions(nil, PromptToolMergeOptions{
					ExcludeAnswer: true,
				})
			}
		}
	} else {
		promptTools = state.Tools
		streamTools = state.Tools
	}

	savedTools := state.Tools
	state.Tools = promptTools
	promptText, err := r.prompt(state)
	state.Tools = savedTools
	state.OmitAnswerInPromptMerge = false
	if err != nil {
		return false, fmt.Errorf("build prompt: %w", err)
	}
	state.Prompt = promptText
	useV3Prompt := state.NativeOpenAIToolCalls && !state.MaxStepsExhaustedFinalPass
	requestUserInput := currentIterationUserInput(state, stepNumber)
	var promptPayload PromptPayload
	if useV3Prompt {
		promptPayload = PrepareFullPromptPayload(promptText, r.cfg.SystemPromptPlacement, requestUserInput)
	} else {
		promptPayload = PreparePromptPayload(promptText, r.cfg.SystemPromptPlacement)
	}
	historyMessages := BuildMemoryMessages(state.Memory)
	if useV3Prompt {
		historyMessages = removeLastMatchingUserMessage(historyMessages, requestUserInput)
	}

	state.Assistant.State = "thinking"
	_, _ = r.emitter.UpdateMessage(state.Assistant)
	r.emitAssistantFooterStatus(state.Assistant.ID, "正在请求大模型…", true)

	var rawRequestPayload string
	var rawResponsePayload string
	var response *StreamOutput
	var actionSchemas []streamtypes.ActionPromptSchema
	if !state.NativeOpenAIToolCalls {
		var override []actioncompatible.ActionSchema
		for _, schema := range state.CompatibleActionSchemas {
			override = append(override, actioncompatible.ActionSchema{
				ActionName:  schema.ActionName,
				Description: schema.Description,
				DataSchema:  schema.DataSchema,
			})
		}
		schemas := actioncompatible.ResolveSessionActionSchemas(state.EnableCallToolReason, override)
		actionSchemas = actioncompatible.ToActionPromptSchemas(schemas)
	}

	actionCount := 0
	sawNonAnswerAction := false
	var actionErr error
	actionRawOutputs := make([]string, 0, 8)
	var dispatchMu sync.Mutex
	var (
		contentPart   *Part
		reasoningPart *Part
	)

	applyControlAction := func(action *ActionOutput) error {
		if action == nil || strings.TrimSpace(action.Action) == "" {
			dispatchMu.Lock()
			if actionErr == nil {
				actionErr = fmt.Errorf("received empty control action")
			}
			dispatchMu.Unlock()
			return nil
		}
		dispatchMu.Lock()
		actionCount++
		if answerSeen {
			if actionErr == nil {
				actionErr = fmt.Errorf("answer must be the last action, but got %s afterwards", action.Action)
			}
			dispatchMu.Unlock()
			return nil
		}
		if actionErr != nil {
			dispatchMu.Unlock()
			return nil
		}
		dispatchMu.Unlock()

		select {
		case <-state.Context.Done():
			return state.Context.Err()
		default:
		}

		parts, err := r.HandleAction(state, action)
		if err != nil {
			if part, handled := r.buildDisallowedActionToolErrorPart(state, action, err); handled {
				actionRawOutput := strings.TrimSpace(action.RawJSON)
				dispatchMu.Lock()
				if actionRawOutput != "" {
					actionRawOutputs = append(actionRawOutputs, actionRawOutput)
					if part.Metadata == nil {
						part.Metadata = map[string]interface{}{}
					}
					part.Metadata["rawOutput"] = actionRawOutput
				}
				dispatchMu.Unlock()
				if err := updatePart(part); err != nil {
					dispatchMu.Lock()
					actionErr = err
					dispatchMu.Unlock()
					return nil
				}
				r.emitAssistantFooterStatusToolPart(state.Assistant.ID, part)
				if r.hooks.RecordAction != nil {
					if err := r.hooks.RecordAction(state, actionRawOutput, prependReasoningPartForMemoryRecord(response, reasoningPart, []*Part{part})); err != nil {
						r.emitAssistantFooterStatus(state.Assistant.ID, "", false)
						return err
					}
				}
				dispatchMu.Lock()
				sawNonAnswerAction = true
				dispatchMu.Unlock()
				return nil
			}
			dispatchMu.Lock()
			actionErr = err
			dispatchMu.Unlock()
			return nil
		}

		actionRawOutput := strings.TrimSpace(action.RawJSON)
		dispatchMu.Lock()
		if actionRawOutput != "" {
			actionRawOutputs = append(actionRawOutputs, actionRawOutput)
		}
		dispatchMu.Unlock()

		firstPartWithRaw := false
		for _, part := range parts {
			if part == nil {
				continue
			}
			if actionRawOutput != "" && !firstPartWithRaw {
				if part.Metadata == nil {
					part.Metadata = map[string]interface{}{}
				}
				part.Metadata["rawOutput"] = actionRawOutput
				firstPartWithRaw = true
			}
			if err := updatePart(part); err != nil {
				dispatchMu.Lock()
				actionErr = err
				dispatchMu.Unlock()
				break
			}
			r.emitAssistantFooterStatusToolPart(state.Assistant.ID, part)
		}
		dispatchMu.Lock()
		if actionErr != nil {
			dispatchMu.Unlock()
			return nil
		}
		if action.Action == "answer" {
			answerSeen = true
		} else {
			sawNonAnswerAction = true
		}
		dispatchMu.Unlock()

		r.markSilentToolTextOutputFromParts(state, parts...)

		if r.hooks.RecordAction != nil {
			if err := r.hooks.RecordAction(state, actionRawOutput, prependReasoningPartForMemoryRecord(response, reasoningPart, parts)); err != nil {
				r.emitAssistantFooterStatus(state.Assistant.ID, "", false)
				return err
			}
		}
		return nil
	}

	streamIn := StreamInput{
		Context:          state.Context,
		Model:            r.cfg.Model,
		Prompt:           promptPayload.UserPrompt,
		SystemPrompt:     promptPayload.SystemPrompt,
		Instruction:      promptPayload.Instruction,
		HistoryMessages:  historyMessages,
		UserContentParts: nil,
		Abort:            state.Context,
		Temperature:      r.cfg.Temperature,
		TopP:             r.cfg.TopP,
		MaxOutputTokens:  r.cfg.MaxOutputTokens,
		ProviderOptions:  r.cfg.ProviderOptions,
		Tools:            streamTools,
		ActionSchemas:    actionSchemas,
		HTTPClient:       state.HTTPClient,
		OnRawRequest: func(raw string) {
			rawRequestPayload = raw
			if state.OnRawRequest != nil {
				state.OnRawRequest(raw)
			}
		},
		OnRawResponse: func(raw string) {
			rawResponsePayload = raw
			if state.OnRawResponse != nil {
				state.OnRawResponse(raw)
			}
		},
		OnRetryError:             state.OnRetryError,
		ReasoningEffort:          r.cfg.ReasoningEffort,
		TextVerbosity:            r.cfg.TextVerbosity,
		EnableEncryptedReasoning: r.cfg.EnableEncryptedReasoning,
		ParallelToolCalls:        r.cfg.ParallelToolCalls,
		PromptCacheKey:           r.cfg.PromptCacheKey,
		ThinkingType:             r.cfg.ThinkingType,
		EnableThinking:           r.cfg.EnableThinking,
		BudgetTokens:             r.cfg.BudgetTokens,
	}
	if !state.NativeOpenAIToolCalls {
		streamIn.CompatibleControlHandler = func(action *streamtypes.ActionOutput) error {
			return applyControlAction(action)
		}
	}
	response, err = r.cfg.ActionProvider.Stream(streamIn)
	if err != nil {
		r.emitAssistantFooterStatus(state.Assistant.ID, "", false)
		if llmErr := r.afterLLMCall(state, &LLMCallInfo{
			Prompt:      promptText,
			RawRequest:  rawRequestPayload,
			RawResponse: rawResponsePayload,
			Error:       err,
		}); llmErr != nil {
			return false, llmErr
		}
		return false, err
	}

	r.emitAssistantFooterStatus(state.Assistant.ID, "正在处理模型输出…", true)

	reasonCallback := func(snapshot []byte) error {
		content := string(snapshot)
		if content == "" {
			return nil
		}
		if reasoningPart == nil {
			now := r.now().UnixMilli()
			reasoningPart = &Part{
				ID:        r.nextID("part"),
				MessageID: state.Assistant.ID,
				SessionID: state.SessionID,
				Type:      PartTypeReasoning,
				Reasoning: "",
				Time:      &PartTime{Start: now, Created: now},
			}
		}
		reasoningPart.Reasoning = content
		r.markSilentToolTextOutput(state)
		return updatePart(reasoningPart)
	}

	contentCallback := func(snapshot []byte) error {
		content := string(snapshot)
		if content == "" {
			return nil
		}
		if contentPart == nil {
			now := r.now().UnixMilli()
			contentPart = &Part{
				ID:        r.nextID("part"),
				MessageID: state.Assistant.ID,
				SessionID: state.SessionID,
				Type:      PartTypeText,
				Text:      "",
				Time:      &PartTime{Start: now, Created: now},
			}
		}
		contentPart.Text = content
		r.markSilentToolTextOutput(state)
		return updatePart(contentPart)
	}

	toolCallCallback := func(req *streamtypes.CallToolRequest) error {
		if req == nil || strings.TrimSpace(req.Name) == "" {
			dispatchMu.Lock()
			if actionErr == nil {
				actionErr = fmt.Errorf("received empty tool call request")
			}
			dispatchMu.Unlock()
			return nil
		}
		dispatchMu.Lock()
		actionCount++
		if answerSeen {
			if actionErr == nil {
				actionErr = fmt.Errorf("answer must be the last action, but got tool %s afterwards", req.Name)
			}
			dispatchMu.Unlock()
			return nil
		}
		if actionErr != nil {
			dispatchMu.Unlock()
			return nil
		}
		dispatchMu.Unlock()

		select {
		case <-state.Context.Done():
			return state.Context.Err()
		default:
		}

		parts, err := r.executeToolCallRequest(state, req)

		dispatchMu.Lock()
		defer dispatchMu.Unlock()

		if err != nil {
			action := &ActionOutput{Index: req.Index, Action: req.Name, Data: req.Arguments, RawJSON: req.RawJSON}
			if part, handled := r.buildDisallowedActionToolErrorPart(state, action, err); handled {
				actionRawOutput := strings.TrimSpace(req.RawJSON)
				if actionRawOutput != "" {
					actionRawOutputs = append(actionRawOutputs, actionRawOutput)
					if part.Metadata == nil {
						part.Metadata = map[string]interface{}{}
					}
					part.Metadata["rawOutput"] = actionRawOutput
				}
				if err := updatePart(part); err != nil {
					actionErr = err
					return nil
				}
				r.emitAssistantFooterStatusToolPart(state.Assistant.ID, part)
				if r.hooks.RecordAction != nil {
					if err := r.hooks.RecordAction(state, actionRawOutput, prependReasoningPartForMemoryRecord(response, reasoningPart, []*Part{part})); err != nil {
						r.emitAssistantFooterStatus(state.Assistant.ID, "", false)
						return err
					}
				}
				sawNonAnswerAction = true
				return nil
			}
			actionErr = err
			return nil
		}

		actionRawOutput := strings.TrimSpace(req.RawJSON)
		if actionRawOutput != "" {
			actionRawOutputs = append(actionRawOutputs, actionRawOutput)
		}

		firstPartWithRaw := false
		for _, part := range parts {
			if part == nil {
				continue
			}
			if actionRawOutput != "" && !firstPartWithRaw {
				if part.Metadata == nil {
					part.Metadata = map[string]interface{}{}
				}
				part.Metadata["rawOutput"] = actionRawOutput
				firstPartWithRaw = true
			}
			if err := updatePart(part); err != nil {
				actionErr = err
				return nil
			}
			r.emitAssistantFooterStatusToolPart(state.Assistant.ID, part)
		}
		if actionErr != nil {
			return nil
		}
		sawNonAnswerAction = true

		if r.hooks.RecordAction != nil {
			if err := r.hooks.RecordAction(state, actionRawOutput, prependReasoningPartForMemoryRecord(response, reasoningPart, parts)); err != nil {
				r.emitAssistantFooterStatus(state.Assistant.ID, "", false)
				return err
			}
		}
		return nil
	}

	readErr := response.Read(streamtypes.StreamReadOptions{
		Interval:   DefaultStreamReaderInterval,
		OnReason:   reasonCallback,
		OnContent:  contentCallback,
		OnToolCall: toolCallCallback,
	})
	if readErr != nil && actionErr == nil {
		actionErr = readErr
	}
	if reasoningPart != nil {
		now := r.now().UnixMilli()
		if reasoningPart.Time != nil {
			reasoningPart.Time.End = now
		}
		_ = updatePart(reasoningPart)
	}
	if contentPart != nil {
		now := r.now().UnixMilli()
		if contentPart.Time != nil {
			contentPart.Time.End = now
		}
		_ = updatePart(contentPart)
	}
	if answerSeen && sawNonAnswerAction {
		answerSeen = false
	}
	nativeTextAnswerSeen := response != nil && response.NativeAssistantTextFinishesTurn

	if readErr != nil {
		// 与成功路径一致：累积的「模型 JSON 文本」在 RawTextReader；解析失败时也要写入，便于日志与 LastLLMCall 排查
		var rawOutput string
		if response.RawTextReader != nil {
			rawOutputBytes, readErr := io.ReadAll(response.RawTextReader)
			if readErr != nil {
				log.Printf("[core_agent.runner] read RawTextReader after parse failure: %v", readErr)
			} else if len(rawOutputBytes) > 0 {
				rawOutput = string(rawOutputBytes)
			}
		}
		rawResp := rawResponsePayload
		if strings.TrimSpace(rawResp) == "" {
			rawResp = strings.TrimSpace(rawOutput)
		}
		logStreamV2ParseFailure(readErr, bytes.NewBufferString(rawOutput), rawResponsePayload)
		r.emitAssistantFooterStatus(state.Assistant.ID, "", false)
		if llmErr := r.afterLLMCall(state, &LLMCallInfo{
			Prompt:         promptText,
			RawRequest:     rawRequestPayload,
			RawResponse:    rawResp,
			RawOutput:      rawOutput,
			Actions:        append([]string(nil), actionRawOutputs...),
			ParsedResponse: buildParsedResponseSummary(actionRawOutputs),
			Error:          readErr,
		}); llmErr != nil {
			return false, llmErr
		}
		return false, readErr
	}
	if reasoningPart != nil {
		sig := strings.TrimSpace(response.AnthropicThinkingSignature())
		if sig != "" {
			if reasoningPart.Metadata == nil {
				reasoningPart.Metadata = map[string]interface{}{}
			}
			reasoningPart.Metadata["anthropicThinkingSignature"] = sig
			_ = updatePart(reasoningPart)
		}
	}
	if phase := strings.TrimSpace(response.Phase); phase != "" {
		state.Assistant.Phase = phase
	}
	if raw := strings.TrimSpace(response.ResponsesOutputMessageRaw); raw != "" {
		state.Assistant.ResponsesOutputMessageRaw = raw
	}
	if len(response.ResponsesReasoningItemRaws) > 0 {
		state.Assistant.ResponsesReasoningItemRaws = append([]string(nil), response.ResponsesReasoningItemRaws...)
	}

	var stepTokens *MessageTokens
	if response.Usage != nil {
		tokens := MessageTokensFromUsage(response.Usage)
		stepTokens = &tokens
		state.Assistant.Tokens = stepTokens
		if r.hooks.PersistTokens != nil {
			if err := r.hooks.PersistTokens(state, stepTokens); err != nil {
				return false, err
			}
		}
	}
	if _, err := r.emitter.UpdateMessage(state.Assistant); err != nil {
		return false, err
	}

	var rawOutput string
	if rawOutputBytes, err := io.ReadAll(response.RawTextReader); err == nil && len(rawOutputBytes) > 0 {
		rawOutput = string(rawOutputBytes)
	}
	if strings.TrimSpace(rawResponsePayload) == "" {
		rawResponsePayload = strings.TrimSpace(rawOutput)
	}

	llmCallInfo := &LLMCallInfo{
		Prompt:         promptText,
		RawRequest:     rawRequestPayload,
		RawResponse:    rawResponsePayload,
		RawOutput:      rawOutput,
		Actions:        append([]string(nil), actionRawOutputs...),
		ParsedResponse: buildParsedResponseSummary(actionRawOutputs),
		Usage:          response.Usage,
	}

	hasNativeTextAnswer := false
	if nativeTextAnswerSeen && contentPart != nil && strings.TrimSpace(contentPart.Text) != "" {
		hasNativeTextAnswer = true
	}
	if !hasNativeTextAnswer && strings.TrimSpace(rawOutput) != "" && nativeTextAnswerSeen {
		hasNativeTextAnswer = true
	}

	if actionCount == 0 && !hasNativeTextAnswer {
		r.emitAssistantFooterStatus(state.Assistant.ID, "", false)
		llmCallInfo.Error = missingModelActionError(rawOutput, rawResponsePayload)
		if err := r.afterLLMCall(state, llmCallInfo); err != nil {
			return false, err
		}
		log.Printf("[core_agent.runner] %v", llmCallInfo.Error)
		if strings.TrimSpace(rawOutput) == "" {
			return false, llmCallInfo.Error
		}
		return false, llmCallInfo.Error
	}
	if actionErr != nil {
		r.emitAssistantFooterStatus(state.Assistant.ID, "", false)
		llmCallInfo.Error = actionErr
		if err := r.afterLLMCall(state, llmCallInfo); err != nil {
			return false, err
		}
		return false, actionErr
	}
	if err := r.afterLLMCall(state, llmCallInfo); err != nil {
		r.emitAssistantFooterStatus(state.Assistant.ID, "", false)
		return false, err
	}

	r.emitAssistantFooterStatus(state.Assistant.ID, "", false)
	r.markSilentToolTextOutputFromParts(state, reasoningPart, contentPart)

	finishReason := "step-complete"
	switch {
	case answerSeen:
		finishReason = "answer-action"
	case hasNativeTextAnswer:
		finishReason = "native-text-answer"
	case state.MaxStepsExhaustedFinalPass:
		finishReason = "max-steps-final-pass"
	}
	finishPart := &Part{
		ID:        r.nextID("part"),
		MessageID: state.Assistant.ID,
		SessionID: state.SessionID,
		Type:      PartTypeFinishStep,
		Tokens:    stepTokens,
		Metadata: map[string]interface{}{
			"finishReason":               finishReason,
			"answerSeen":                 answerSeen,
			"hasNativeTextAnswer":        hasNativeTextAnswer,
			"nativeTextFinishesTurn":     nativeTextAnswerSeen,
			"maxStepsExhaustedFinalPass": state.MaxStepsExhaustedFinalPass,
			"actionCount":                actionCount,
		},
		Time: &PartTime{Start: r.now().UnixMilli(), End: r.now().UnixMilli()},
	}
	if err := updatePart(finishPart); err != nil {
		return false, err
	}

	state.Assistant.Finish = "step-finish"
	_, _ = r.emitter.UpdateMessage(state.Assistant)

	if r.hooks.AfterStep != nil {
		if err := r.hooks.AfterStep(state); err != nil {
			return false, err
		}
	}

	if answerSeen || hasNativeTextAnswer {
		if answerSeen && contentPart == nil {
			log.Printf("[runner] assistant message %s finished=answer without text part; reason=%s nativeTextFinishesTurn=%t actions=%d rawOutputBytes=%d",
				strings.TrimSpace(state.Assistant.ID),
				finishReason,
				nativeTextAnswerSeen,
				actionCount,
				len(strings.TrimSpace(llmCallInfo.RawOutput)),
			)
		}
		if r.hooks.OnAnswer != nil {
			if err := r.hooks.OnAnswer(state); err != nil {
				return false, err
			}
		}
		state.Assistant.Finish = "answer"
		state.Assistant.Time.Completed = r.now().UnixMilli()
		_, _ = r.emitter.UpdateMessage(state.Assistant)
		r.emitAssistantFooterStatus(state.Assistant.ID, "", false)
		return true, nil
	}

	return false, nil
}

func previewActionDataText(action *ActionOutput) (string, error) {
	if action == nil || action.Data == nil {
		return "", nil
	}
	payload, err := io.ReadAll(action.Data)
	if err != nil {
		return "", err
	}
	action.Data = bytes.NewReader(payload)
	var text string
	if err := json.Unmarshal(payload, &text); err == nil {
		return text, nil
	}
	return strings.TrimSpace(string(payload)), nil
}

func missingModelActionError(rawOutput string, rawResponse string) error {
	output := strings.TrimSpace(rawOutput)
	if output != "" {
		return fmt.Errorf("model output missing @action/data, raw output: %s", streamtypes.TruncateStringForLog(output, 4000))
	}
	response := strings.TrimSpace(rawResponse)
	if response != "" {
		return fmt.Errorf("model output missing @action/data and raw output is empty; raw response: %s", streamtypes.TruncateStringForLog(response, 4000))
	}
	return fmt.Errorf("model output missing @action/data and raw output is empty; raw response is empty")
}

func removeLastMatchingUserMessage(messages []*ModelMessage, userInput string) []*ModelMessage {
	userInput = strings.TrimSpace(userInput)
	if userInput == "" || len(messages) == 0 {
		return messages
	}
	for i := len(messages) - 1; i >= 0; i-- {
		message := messages[i]
		if message == nil || strings.TrimSpace(message.Role) != "user" {
			continue
		}
		content, ok := message.Content.(string)
		if !ok || strings.TrimSpace(content) != userInput {
			continue
		}
		out := make([]*ModelMessage, 0, len(messages)-1)
		out = append(out, messages[:i]...)
		out = append(out, messages[i+1:]...)
		return out
	}
	return messages
}

func currentIterationUserInput(state *RunState, stepNumber int) string {
	if state == nil {
		return ""
	}
	if state.NativeOpenAIToolCalls && !state.MaxStepsExhaustedFinalPass && stepNumber > 1 {
		return ""
	}
	return state.UserInput
}

func (r *Runner) Run(state *RunState) error {
	if state == nil {
		return fmt.Errorf("run state is required")
	}
	if state.Assistant == nil {
		return fmt.Errorf("assistant message is required")
	}
	if strings.TrimSpace(state.SessionID) == "" {
		state.SessionID = state.Assistant.SessionID
	}
	if state.Assistant.SessionID == "" {
		state.Assistant.SessionID = state.SessionID
	}
	if state.Context == nil {
		state.Context = context.Background()
	}
	if len(state.Tools) == 0 && r.tools != nil {
		state.Tools = r.tools.Definitions()
	}

	if r.hooks.AppendUserText != nil {
		if err := r.hooks.AppendUserText(state, state.UserInput); err != nil {
			return err
		}
	}
	if _, err := r.emitter.UpdateMessage(state.Assistant); err != nil {
		return fmt.Errorf("init assistant message: %w", err)
	}

	maxIterations := state.MaxSteps
	if maxIterations <= 0 {
		maxIterations = r.cfg.MaxSteps
	}
	if maxIterations <= 0 {
		maxIterations = DefaultMaxRunSteps
	}
	if state.SingleStep {
		maxIterations = 1
	}

	updatePart := func(part *Part) error {
		_, err := r.emitter.UpdatePart(part)
		return err
	}

	if err := r.consumeSupplementsAfterStep(state); err != nil {
		return err
	}

	for step := 0; step < maxIterations; step++ {
		if step > 0 {
			r.prepareAssistantMessageForStep(state, step+1)
			if _, err := r.emitter.UpdateMessage(state.Assistant); err != nil {
				return fmt.Errorf("init assistant message: %w", err)
			}
		}
		answerSeen, err := r.executeAgentIteration(state, step+1, updatePart)
		if err != nil {
			return err
		}
		if err := r.consumeSupplementsAfterStep(state); err != nil {
			return err
		}
		if answerSeen {
			return nil
		}
	}

	if state.SingleStep {
		state.Assistant.Time.Completed = r.now().UnixMilli()
		_, _ = r.emitter.UpdateMessage(state.Assistant)
		return nil
	}

	origTools := state.Tools
	state.MaxStepsExhaustedFinalPass = true
	state.Tools = []ToolDefinition{AnswerToolDefinition()}
	r.prepareAssistantMessageForStep(state, maxIterations+1)
	if _, err := r.emitter.UpdateMessage(state.Assistant); err != nil {
		return fmt.Errorf("init assistant message: %w", err)
	}
	answerSeen, err := r.executeAgentIteration(state, maxIterations+1, updatePart)
	state.MaxStepsExhaustedFinalPass = false
	state.Tools = origTools
	if err != nil {
		return err
	}
	if answerSeen {
		return nil
	}

	state.Assistant.Time.Completed = r.now().UnixMilli()
	_, _ = r.emitter.UpdateMessage(state.Assistant)
	return nil
}

func (r *Runner) consumeSupplementsAfterStep(state *RunState) error {
	if r == nil || r.messageQueue == nil || r.cfg.ConsumeSupplement == nil {
		return nil
	}
	_, err := r.messageQueue.ConsumeSupplements(func(item models.TaskMessageQueueItem) error {
		return r.cfg.ConsumeSupplement(state, item)
	})
	return err
}

func buildParsedResponseSummary(actionRawOutputs []string) string {
	if len(actionRawOutputs) == 0 {
		return ""
	}
	summaries := make([]map[string]any, 0, len(actionRawOutputs))
	for _, raw := range actionRawOutputs {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		var envelope map[string]any
		if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
			summaries = append(summaries, map[string]any{
				"raw": raw,
			})
			continue
		}
		actionName, _ := envelope["@action"].(string)
		if strings.TrimSpace(actionName) == "" {
			actionName, _ = envelope["call_tool"].(string)
		}
		actionName = strings.TrimSpace(actionName)
		item := map[string]any{
			"action": actionName,
		}
		data := envelope["data"]
		if data == nil {
			data = envelope["params"]
		}
		switch actionName {
		case "message":
			if dataMap, ok := data.(map[string]any); ok {
				if message, ok := dataMap["message"]; ok {
					item["message"] = message
				}
				if nextStep, ok := dataMap["next_step"]; ok {
					item["next_step"] = nextStep
				}
			} else {
				item["data"] = data
			}
		case "answer":
			if dataMap, ok := data.(map[string]any); ok {
				if c, ok := dataMap["content"]; ok {
					item["message"] = c
				} else {
					item["data"] = data
				}
			} else {
				item["message"] = data
			}
		case "call_tool":
			item["arguments"] = data
		default:
			item["tool"] = actionName
			item["arguments"] = data
		}
		summaries = append(summaries, item)
	}
	if len(summaries) == 0 {
		return ""
	}
	data, err := json.MarshalIndent(summaries, "", "  ")
	if err != nil {
		return ""
	}
	return string(data)
}

// ExecuteOnce runs exactly one agent step. Any tool calls within that step will
// still execute, but the runner will not start a second model loop afterwards.
func (r *Runner) ExecuteOnce(state *RunState) error {
	if state == nil {
		return fmt.Errorf("run state is required")
	}
	prev := state.SingleStep
	state.SingleStep = true
	defer func() {
		state.SingleStep = prev
	}()
	return r.Run(state)
}

func (r *Runner) isActionAllowed(state *RunState, actionName string) bool {
	name := strings.TrimSpace(actionName)
	if name == "" {
		return false
	}
	if _, ok := r.actions[name]; ok {
		return true
	}
	for _, def := range state.Tools {
		if strings.TrimSpace(def.Name) == name {
			return true
		}
	}
	_, err := r.tools.Get(name)
	return err == nil
}

func (r *Runner) emitAssistantFooterStatus(assistantID, text string, loading bool) {
	if assistantID == "" {
		return
	}
	r.emitter.Emit(EventAssistantFooterStatus, AssistantFooterStatusPayload{
		MessageID: assistantID,
		Text:      text,
		Loading:   loading,
	})
}

func trimAssistantFooterPreview(text string) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return ""
	}
	runes := []rune(trimmed)
	if len(runes) <= assistantFooterPreviewMaxRunes {
		return trimmed
	}
	return string(runes[len(runes)-assistantFooterPreviewMaxRunes:])
}

func (r *Runner) emitAssistantFooterStatusTextDelta(assistantID, text string) {
	preview := trimAssistantFooterPreview(text)
	if preview == "" {
		return
	}
	r.emitAssistantFooterStatus(assistantID, preview, true)
}

func (r *Runner) emitAssistantFooterStatusToolPart(assistantID string, part *Part) {
	if assistantID == "" || part == nil || part.Tool == nil {
		return
	}
	status := strings.TrimSpace(part.Tool.State.Status)
	toolName := strings.TrimSpace(part.Tool.Name)
	if toolName == "" {
		toolName = "tool"
	}
	pieces := make([]string, 0, 3)
	if toolName != "" {
		pieces = append(pieces, toolName)
	}
	if title := strings.TrimSpace(part.Tool.State.Title); title != "" {
		pieces = append(pieces, title)
	}
	if inputPreview := trimAssistantFooterPreview(toolStateInputPreview(part.Tool.State)); inputPreview != "" {
		pieces = append(pieces, inputPreview)
	} else if output := trimAssistantFooterPreview(part.Tool.State.Output); output != "" {
		pieces = append(pieces, output)
	} else if raw := trimAssistantFooterPreview(part.Tool.State.Raw); raw != "" {
		pieces = append(pieces, raw)
	} else if errText := trimAssistantFooterPreview(part.Tool.State.Error); errText != "" {
		pieces = append(pieces, errText)
	}
	text := strings.Join(pieces, " · ")
	if text == "" {
		text = toolName
	}
	loading := status == "" || status == "preparing" || status == "input-streaming" || status == "running"
	r.emitAssistantFooterStatus(assistantID, text, loading)
}

func toolStateInputPreview(state ToolState) string {
	if len(state.Metadata) == 0 {
		return ""
	}
	preview, _ := state.Metadata["inputPreview"].(string)
	return strings.TrimSpace(preview)
}

func (r *Runner) buildDisallowedActionToolErrorPart(state *RunState, action *ActionOutput, handleErr error) (*Part, bool) {
	if state == nil || action == nil || handleErr == nil {
		return nil, false
	}
	errText := strings.TrimSpace(handleErr.Error())
	if !strings.Contains(errText, "current mode does not allow action type:") {
		return nil, false
	}
	toolName := strings.TrimSpace(action.Action)
	if toolName == "" {
		return nil, false
	}
	now := r.now().UnixMilli()
	return &Part{
		ID:        r.nextID("part"),
		MessageID: state.Assistant.ID,
		SessionID: state.SessionID,
		Type:      PartTypeTool,
		Metadata: map[string]interface{}{
			"generatedByRunner": true,
		},
		Tool: &ToolPart{
			Name:   toolName,
			CallID: r.nextID("tool"),
			State: ToolState{
				Status:        "error",
				Raw:           strings.TrimSpace(action.RawJSON),
				Error:         errText,
				SystemMessage: "ERROR: call tool failed, reason: " + errText,
				Output:        "",
				Time: PartTime{
					Start: now,
					End:   now,
				},
				Metadata: map[string]interface{}{
					"toolError":        errText,
					"permissionDenied": true,
					"disallowedAction": toolName,
					"inputStreaming":   false,
				},
			},
		},
	}, true
}

func (r *Runner) nextID(prefix string) string {
	return r.cfg.IDGenerator(prefix)
}

func (r *Runner) now() time.Time {
	return r.cfg.Now()
}

func (r *Runner) buildToolContext(state *RunState) ToolContext {
	ctx := ToolContext{SessionID: state.SessionID}
	if r.toolCtx != nil {
		ctx = r.toolCtx(state)
	}
	if ctx.SessionID == "" {
		ctx.SessionID = state.SessionID
	}
	if ctx.Context == nil {
		ctx.Context = state.Context
	}
	return ctx
}

func (r *Runner) executeToolCall(actionCtx *ActionContext, call ToolCall) (ToolResult, error) {
	return r.executeToolCallWithContext(actionCtx, call, nil)
}

func (r *Runner) executeToolCallWithContext(actionCtx *ActionContext, call ToolCall, ctxOverride context.Context) (ToolResult, error) {
	if actionCtx != nil {
		r.observeRepeatedToolCall(actionCtx.State, call)
		r.observeSilentToolCall(actionCtx.State, call)
	}
	toolInstance, err := r.tools.Get(call.Name)
	if err != nil {
		return ToolResult{IsError: true, Name: call.Name}, err
	}
	toolCtx := r.buildToolContext(actionCtx.State)
	if ctxOverride != nil {
		toolCtx.Context = ctxOverride
	}
	if r.toolExec != nil {
		return r.toolExec(actionCtx, toolInstance, call, toolCtx)
	}
	result, err := toolInstance.Execute(toolCtx, call.Arguments)
	if err != nil {
		return ToolResult{IsError: true, Name: call.Name}, err
	}
	if result.Name == "" {
		result.Name = toolInstance.Name()
	}
	return result, nil
}

func (r *Runner) resolveStall(state *RunState, toolName string, toolArgs map[string]interface{}, toolOutput string, elapsed time.Duration) (decision string, reason string, continueWait time.Duration, err error) {
	if r.cfg.ActionProvider == nil {
		return "", "", 0, fmt.Errorf("no action provider available")
	}

	argsJSON, _ := json.Marshal(toolArgs)
	output := strings.TrimSpace(toolOutput)
	if output == "" {
		output = "（暂无输出）"
	}
	prompt := fmt.Sprintf(
		"你正在监控一个工具的执行。该工具已经运行了 %v，超出了预期的执行时间。\n\n"+
			"工具信息：\n- 名称: %s\n- 参数: %s\n\n"+
			"当前已收集的工具输出：\n%s\n\n"+
			"请决定下一步操作。你可以选择：\n"+
			"1. \"continue\" - 继续等待工具完成执行，并给出预估还需要等待多少秒\n"+
			"2. \"cancel\" - 终止工具执行，返回错误\n\n"+
			"请按以下 JSON 格式输出你的决定（不要包含其他文本，只输出 JSON）：\n"+
			`{"decision":"continue"|"cancel","reason":"你的决策理由","wait_seconds":继续等待的秒数(仅当 decision=continue 时必填正整数)}`,
		elapsed, toolName, string(argsJSON), output,
	)

	messages := BuildMemoryMessages(state.Memory)
	messages = append(messages, &ModelMessage{
		Role:    "user",
		Content: prompt,
	})

	streamIn := StreamInput{
		Context:         state.Context,
		Model:           r.cfg.Model,
		SystemPrompt:    "你是一个工具执行监控器。你的任务是决定一个长时间运行的工具是否应该继续等待或被取消。请只输出 JSON，不要输出其他文本。",
		HistoryMessages: messages,
		Temperature:     r.cfg.Temperature,
		TopP:            r.cfg.TopP,
		MaxOutputTokens: 512,
		ProviderOptions: r.cfg.ProviderOptions,
		HTTPClient:      state.HTTPClient,
		Tools:           nil,
	}

	response, err := r.cfg.ActionProvider.Stream(streamIn)
	if err != nil {
		return "", "", 0, fmt.Errorf("stall watchdog llm call failed: %w", err)
	}

	if readErr := response.Read(streamtypes.StreamReadOptions{
		Interval: DefaultStreamReaderInterval,
	}); readErr != nil {
		return "", "", 0, fmt.Errorf("stall watchdog read failed: %w", readErr)
	}

	var rawOutput string
	if response.RawTextReader != nil {
		rawOutputBytes, readErr := io.ReadAll(response.RawTextReader)
		if readErr != nil {
			return "", "", 0, fmt.Errorf("stall watchdog read raw output failed: %w", readErr)
		}
		rawOutput = string(rawOutputBytes)
	}
	rawOutput = strings.TrimSpace(rawOutput)
	if rawOutput == "" {
		return "", "", 0, fmt.Errorf("stall watchdog received empty response")
	}

	var decisionPayload struct {
		Decision    string `json:"decision"`
		Reason      string `json:"reason"`
		WaitSeconds int    `json:"wait_seconds"`
	}
	if err := json.Unmarshal([]byte(rawOutput), &decisionPayload); err != nil {
		// Try extracting JSON from code blocks or braces.
		jsonBlockRegex := regexp.MustCompile("(?s)```(?:json)?\n?(.*?)\n?```")
		matches := jsonBlockRegex.FindStringSubmatch(rawOutput)
		if len(matches) > 1 {
			jsonText := strings.TrimSpace(matches[1])
			if unmarshalErr := json.Unmarshal([]byte(jsonText), &decisionPayload); unmarshalErr == nil {
				err = nil
			}
		}
		if err != nil {
			braceStart := strings.Index(rawOutput, "{")
			braceEnd := strings.LastIndex(rawOutput, "}")
			if braceStart >= 0 && braceEnd > braceStart {
				jsonText := rawOutput[braceStart : braceEnd+1]
				if unmarshalErr := json.Unmarshal([]byte(jsonText), &decisionPayload); unmarshalErr == nil {
					err = nil
				}
			}
		}
		if err != nil {
			return "", "", 0, fmt.Errorf("stall watchdog failed to parse response: %w", err)
		}
	}

	decisionPayload.Decision = strings.ToLower(strings.TrimSpace(decisionPayload.Decision))
	if decisionPayload.Decision != "continue" && decisionPayload.Decision != "cancel" {
		return "", "", 0, fmt.Errorf("stall watchdog returned invalid decision: %q", decisionPayload.Decision)
	}
	if decisionPayload.Decision == "continue" {
		if decisionPayload.WaitSeconds <= 0 {
			return "", "", 0, fmt.Errorf("stall watchdog returned invalid wait_seconds: %d", decisionPayload.WaitSeconds)
		}
		continueWait = time.Duration(decisionPayload.WaitSeconds) * time.Second
	}

	return decisionPayload.Decision, decisionPayload.Reason, continueWait, nil
}

func (r *Runner) afterLLMCall(state *RunState, info *LLMCallInfo) error {
	if state != nil {
		state.LastLLMCall = info
		if state.Data == nil {
			state.Data = map[string]any{}
		}
		state.Data["last_llm_call"] = info
	}
	if r.hooks.AfterLLMCall != nil {
		return r.hooks.AfterLLMCall(state, info)
	}
	return nil
}

func MessageTokensFromUsage(usage *Usage) MessageTokens {
	if usage == nil {
		return MessageTokens{}
	}
	return MessageTokens{
		Input:     usage.InputTokens,
		Output:    usage.OutputTokens,
		Reasoning: usage.ReasoningTokens,
		Cache: TokenCache{
			Read: usage.CachedInputTokens,
		},
	}
}

const streamParseLogHeadBytes = 12000
const streamParseLogTailBytes = 8000

// prependReasoningPartForMemoryRecord 在落库 parts 前附上 reasoning part（并写入 Anthropic thinking 签名），
// 以便 session memory 能重建 Messages API 所需的 thinking 块。
func prependReasoningPartForMemoryRecord(response *StreamOutput, reasoningPart *Part, parts []*Part) []*Part {
	sig := ""
	if response != nil {
		sig = strings.TrimSpace(response.AnthropicThinkingSignature())
	}
	if reasoningPart == nil {
		return parts
	}
	rp := *reasoningPart
	if rp.Metadata == nil {
		rp.Metadata = map[string]interface{}{}
	}
	if sig != "" {
		rp.Metadata["anthropicThinkingSignature"] = sig
	}
	if strings.TrimSpace(rp.Reasoning) == "" && sig == "" {
		return parts
	}
	return append([]*Part{&rp}, parts...)
}

// logStreamV2ParseFailure 在 Wait() 解析失败时打出：错误链、送入 parseActionStream 的累积文本头尾、以及 OnRawResponse 收到的原始 HTTP片段（若存在）。
func logStreamV2ParseFailure(parseErr error, rawTextReader io.Reader, httpRawResponse string) {
	log.Printf("[core_agent.runner] StreamV2 JSON/action stream parse failed: %v", parseErr)

	var b []byte
	switch r := rawTextReader.(type) {
	case *bytes.Buffer:
		b = r.Bytes()
	case nil:
	default:
		got, readErr := io.ReadAll(rawTextReader)
		if readErr != nil {
			log.Printf("[core_agent.runner] read accumulated model text for logging: %v", readErr)
		} else {
			b = got
		}
	}
	if len(b) > 0 {
		head := b
		if len(head) > streamParseLogHeadBytes {
			head = head[:streamParseLogHeadBytes]
		}
		tail := b
		if len(b) > streamParseLogTailBytes {
			tail = b[len(b)-streamParseLogTailBytes:]
		}
		log.Printf("[core_agent.runner] accumulated model text (bytes fed to action stream parser): total=%d | head (%d bytes):\n%s\n| tail (%d bytes):\n%s",
			len(b), len(head), string(head), len(tail), string(tail))
	} else {
		log.Printf("[core_agent.runner] accumulated model text is empty (no text-delta/content before failure, or read error)")
	}

	httpRaw := strings.TrimSpace(httpRawResponse)
	if httpRaw != "" {
		h := httpRaw
		if len(h) > streamParseLogHeadBytes {
			h = h[:streamParseLogHeadBytes]
		}
		t := httpRaw
		if len(httpRaw) > streamParseLogTailBytes {
			t = httpRaw[len(httpRaw)-streamParseLogTailBytes:]
		}
		log.Printf("[core_agent.runner] OnRawResponse HTTP/raw snippet (may be transport framing, not identical to model text): total=%d | head:\n%s\n| tail:\n%s",
			len(httpRaw), h, t)
	}
}
