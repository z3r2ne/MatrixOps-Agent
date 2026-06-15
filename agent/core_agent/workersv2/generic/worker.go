package generic

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	coreagent "matrixops.local/core_agent"
)

// PromptSection 命名子提示构建器（可依赖 RunState，例如注入项目上下文）。
type PromptSection struct {
	Name    string
	Builder coreagent.PromptBuilder
}

// StaticPromptSection 静态命名片段。
type StaticPromptSection struct {
	Name    string
	Content string
}

// MemorySystem 与 core RunnerHooks 中记忆相关钩子对应，由上层提供具体实现（DB / 内存快照等）。
type MemorySystem struct {
	AppendUserText func(state *coreagent.RunState, text string) error
	Prepare        func(state *coreagent.RunState) error
	Build          func(state *coreagent.RunState) (any, error)
	PersistTokens  func(state *coreagent.RunState, tokens *coreagent.MessageTokens) error
	RecordAction   func(state *coreagent.RunState, rawOutput string, parts []*coreagent.Part) error
	AfterStep      func(state *coreagent.RunState) error
	OnAnswer       func(state *coreagent.RunState) error
	AfterLLMCall   func(state *coreagent.RunState, info *coreagent.LLMCallInfo) error
}

// Worker 表示一个完整 Agent：提示词分层、循环模板、记忆、工具与 LLM。
type Worker struct {
	cfg runtimeConfig

	mu     sync.Mutex
	queue  []RunInput
	notify chan struct{}
	closed bool
	running bool

	resultMu   sync.Mutex
	lastResult *RunResult
	lastErr    error
}

// NewFromLegacy 使用旧版扁平 Config 构造，便于渐进迁移。
func NewFromLegacy(c Config) (*Worker, error) {
	return New(WithLegacyConfig(c))
}

// Name Worker 逻辑名称（如 DB worker.name）。
func (w *Worker) Name() string {
	if w == nil {
		return ""
	}
	return w.cfg.name
}

// RunTask 等价于 Chat(ctx, input, ChatOptions{}) 仅取 Answer 字符串。
func (w *Worker) RunTask(ctx context.Context, input string) (string, error) {
	res, err := w.Chat(ctx, input, ChatOptions{})
	if err != nil {
		return "", err
	}
	if res == nil {
		return "", nil
	}
	return res.Answer, nil
}

// Run 底层多步执行，返回完整 RunResult（含 Parts）。多数场景请用 Chat。
func (w *Worker) Run(input RunInput) (*RunResult, error) {
	return w.run(input, false, w.cfg.emitter)
}

// ExecuteOnce 仅执行一轮模型循环（内部 SingleStep）。
func (w *Worker) ExecuteOnce(input RunInput) (*RunResult, error) {
	return w.run(input, true, w.cfg.emitter)
}

// RunInput 单次运行输入（与 core RunState 字段对齐）。
type RunInput struct {
	Context   context.Context
	SessionID string
	Assistant *coreagent.Message
	UserInput string
	Tools               []coreagent.ToolDefinition
	MaxSteps            int
	HTTPClient          *http.Client
	OnRawRequest        func(raw string)
	OnRawResponse       func(raw string)
	OnRetryError        func(err error, retryAttempt int, maxRetries int, nextDelay time.Duration, attemptDuration time.Duration, rawResponse string)
	ExecuteOnce         bool
	Callbacks           ChatCallbacks
}

// RunResult 运行结果摘要。
type RunResult struct {
	State      *coreagent.RunState
	Assistant  *coreagent.Message
	Parts      []*coreagent.Part
	OutputText string
}

func (w *Worker) run(input RunInput, once bool, emitter coreagent.Emitter) (*RunResult, error) {
	if w == nil {
		return nil, fmt.Errorf("worker is nil")
	}
	if emitter == nil {
		emitter = coreagent.NoEmitter{}
	}

	capture := newCaptureEmitter(emitter)
	runner, err := coreagent.NewRunner(coreagent.RunnerConfig{
		Emitter:                  capture,
		LLMClient:                w.cfg.llmClient,
		ProviderName:             w.cfg.providerName,
		PromptBuilder:            w.buildPromptBuilder(),
		Hooks:                    w.runnerHooks(),
		ToolContextBuilder:       w.cfg.toolContextBuilder,
		ToolExecutor:             w.cfg.toolExecutor,
		Tools:                    w.cfg.tools,
		Model:                    w.cfg.model,
		Temperature:              w.cfg.temperature,
		TopP:                     w.cfg.topP,
		MaxOutputTokens:          w.cfg.maxOutputTokens,
		ProviderOptions:          w.cfg.providerOptions,
		MaxSteps:                 w.resolveMaxSteps(input.MaxSteps, once),
		IDGenerator:              w.cfg.idGenerator,
		Now:                      w.cfg.now,
		SystemPromptPlacement:    w.cfg.systemPromptPlacement,
		NativeOpenAIToolCalls:    w.cfg.nativeOpenAIToolCalls,
		ReasoningEffort:          w.cfg.reasoningEffort,
		TextVerbosity:            w.cfg.textVerbosity,
		EnableEncryptedReasoning: w.cfg.enableEncryptedReasoning,
		ParallelToolCalls:        w.cfg.parallelToolCalls,
		PromptCacheKey:           w.cfg.promptCacheKey,
		ThinkingType:             w.cfg.thinkingType,
		EnableThinking:           w.cfg.enableThinking,
		BudgetTokens:             w.cfg.budgetTokens,
		StallWatchdogTimeout:     w.cfg.stallWatchdogTimeout,
		RepeatedToolCallThreshold: w.cfg.repeatedToolCallThreshold,
		OnRepeatedToolCall:       w.cfg.onRepeatedToolCall,
		SilentToolCallThreshold:  w.cfg.silentToolCallThreshold,
		OnSilentToolStreak:       w.cfg.onSilentToolStreak,
		OnStallWatchdogToolCancelled: w.cfg.onStallWatchdogToolCancelled,
		MessageQueue:             w.cfg.messageQueue,
		ConsumeSupplement:        w.cfg.consumeSupplement,
	})
	if err != nil {
		return nil, err
	}
	if w.cfg.configureRunner != nil {
		w.cfg.configureRunner(runner)
	}

	state := w.newRunState(input, once)
	var runErr error
	if once {
		runErr = runner.ExecuteOnce(state)
	} else {
		runErr = runner.Run(state)
	}
	parts := capture.partsForMessage(state.Assistant.ID)
	out := &RunResult{
		State:      state,
		Assistant:  state.Assistant,
		Parts:      parts,
		OutputText: collectTextOutput(parts),
	}
	if runErr != nil {
		return out, runErr
	}
	return out, nil
}

func (w *Worker) buildPromptBuilder() coreagent.PromptBuilder {
	return func(state *coreagent.RunState) (string, error) {
		sections := make([]string, 0, 8)
		sections = appendIfContent(sections, renderPromptSection("worker_prompt", w.cfg.workerPrompt))
		sections = appendIfContent(sections, renderPromptSection("model_prompt", w.cfg.modelPrompt))
		sections = appendIfContent(sections, renderPromptSection("occupation_prompt", w.cfg.occupationPrompt))
		for _, section := range w.cfg.staticPrompts {
			sections = appendIfContent(sections, renderPromptSection(section.Name, section.Content))
		}
		for _, builder := range w.cfg.subPromptBuilders {
			if builder.Builder == nil {
				continue
			}
			content, err := builder.Builder(state)
			if err != nil {
				return "", fmt.Errorf("build sub prompt %q: %w", builder.Name, err)
			}
			sections = appendIfContent(sections, renderPromptSection(builder.Name, content))
		}
		mainPrompt, err := w.cfg.mainPromptBuilder(state)
		if err != nil {
			return "", fmt.Errorf("build main prompt: %w", err)
		}
		sections = appendIfContent(sections, strings.TrimSpace(mainPrompt))
		return strings.Join(sections, "\n\n"), nil
	}
}

func (w *Worker) runnerHooks() coreagent.RunnerHooks {
	return coreagent.RunnerHooks{
		AppendUserText: w.cfg.memory.AppendUserText,
		PrepareMemory:  w.cfg.memory.Prepare,
		BuildMemory:    w.cfg.memory.Build,
		PersistTokens:  w.cfg.memory.PersistTokens,
		RecordAction:   w.cfg.memory.RecordAction,
		AfterStep:      w.cfg.memory.AfterStep,
		OnAnswer:       w.cfg.memory.OnAnswer,
		AfterLLMCall:   w.cfg.memory.AfterLLMCall,
	}
}

func (w *Worker) newRunState(input RunInput, once bool) *coreagent.RunState {
	sessionID := strings.TrimSpace(input.SessionID)
	if sessionID == "" && input.Assistant != nil {
		sessionID = strings.TrimSpace(input.Assistant.SessionID)
	}
	if sessionID == "" {
		sessionID = w.cfg.idGenerator("session")
	}

	assistant := input.Assistant
	if assistant == nil {
		assistant = &coreagent.Message{
			ID:        w.cfg.idGenerator("message"),
			SessionID: sessionID,
			Role:      coreagent.RoleAssistant,
			Time:      coreagent.MessageTime{Created: w.cfg.now().UnixMilli()},
			State:     "loading",
		}
	} else {
		copied := *assistant
		if strings.TrimSpace(copied.SessionID) == "" {
			copied.SessionID = sessionID
		}
		if copied.ID == "" {
			copied.ID = w.cfg.idGenerator("message")
		}
		if copied.Time.Created == 0 {
			copied.Time.Created = w.cfg.now().UnixMilli()
		}
		assistant = &copied
	}

	state := &coreagent.RunState{
		Context:                 input.Context,
		SessionID:               sessionID,
		Assistant:               assistant,
		UserInput:               input.UserInput,
		Tools:                   append([]coreagent.ToolDefinition(nil), input.Tools...),
		MaxSteps:                w.resolveMaxSteps(input.MaxSteps, once),
		CompatibleActionSchemas: append([]coreagent.ActionSchema(nil), w.cfg.compatibleActionSchemas...),
		HTTPClient:              input.HTTPClient,
		OnRawRequest:            input.OnRawRequest,
		OnRawResponse:           input.OnRawResponse,
		OnRetryError:            input.OnRetryError,
	}
	if state.Context == nil {
		state.Context = context.Background()
	}
	return state
}

func (w *Worker) resolveMaxSteps(inputMaxSteps int, once bool) int {
	if once {
		return 1
	}
	if inputMaxSteps > 0 {
		return inputMaxSteps
	}
	if w.cfg.maxSteps > 0 {
		return w.cfg.maxSteps
	}
	return coreagent.DefaultMaxRunSteps
}

func renderPromptSection(name string, content string) string {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return ""
	}
	tag := strings.TrimSpace(name)
	if tag == "" {
		return trimmed
	}
	return fmt.Sprintf("<%s>\n%s\n</%s>", tag, trimmed, tag)
}

func appendIfContent(items []string, value string) []string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return items
	}
	return append(items, trimmed)
}

// SendMessage 向 Worker 内部消息队列追加一条消息。
// 可在 Start 之前或之后调用；消息会按 FIFO 顺序被消费。
func (w *Worker) SendMessage(input RunInput) error {
	if w == nil {
		return fmt.Errorf("worker is nil")
	}

	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return fmt.Errorf("worker is closed")
	}
	w.queue = append(w.queue, input)
	w.mu.Unlock()

	select {
	case w.notify <- struct{}{}:
	default:
	}
	return nil
}

// Start 启动 Worker 的消息消费循环。
//
// 执行流程：
//  1. 先消费内部队列中已缓存的所有消息（FIFO）；
//  2. 队列为空后阻塞等待新消息（通过 SendMessage 唤醒），若超过空闲超时仍无消息则退出；
//  3. 当 Close 被调用且队列消费完毕后，循环退出并返回 nil。
//
// 典型用法（常驻模式）：
//
//	go w.Start(ctx)
//	w.SendMessage(RunInput{UserInput: "msg1"})
//	w.SendMessage(RunInput{UserInput: "msg2"})
//	w.Close()
//
// 也可作为同步批处理使用：先 SendMessage 再调用 Start，处理完即返回。
func (w *Worker) Start(ctx context.Context) error {
	if w == nil {
		return fmt.Errorf("worker is nil")
	}

	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return fmt.Errorf("worker is already running")
	}
	w.running = true
	w.mu.Unlock()

	defer func() {
		w.mu.Lock()
		w.running = false
		w.mu.Unlock()
	}()

	idleTimer := time.NewTimer(0)
	if !idleTimer.Stop() {
		select {
		case <-idleTimer.C:
		default:
		}
	}
	defer idleTimer.Stop()

	const idleTimeout = 100 * time.Millisecond

	for {
		w.mu.Lock()
		if len(w.queue) > 0 {
			input := w.queue[0]
			w.queue = w.queue[1:]
			w.mu.Unlock()

			if !idleTimer.Stop() {
				select {
				case <-idleTimer.C:
				default:
				}
			}

			if err := w.process(ctx, input); err != nil {
				return err
			}
			continue
		}
		if w.closed {
			w.mu.Unlock()
			return nil
		}
		w.mu.Unlock()

		idleTimer.Reset(idleTimeout)
		select {
		case <-ctx.Done():
			if !idleTimer.Stop() {
				select {
				case <-idleTimer.C:
				default:
				}
			}
			return ctx.Err()
		case <-w.notify:
			if !idleTimer.Stop() {
				select {
				case <-idleTimer.C:
				default:
				}
			}
			// 被 SendMessage 或 Close 唤醒，继续循环
		case <-idleTimer.C:
			// 空闲超时，检查队列是否仍为空
			w.mu.Lock()
			empty := len(w.queue) == 0
			w.mu.Unlock()
			if empty {
				return nil
			}
		}
	}
}

// Close 关闭 Worker 的消息接收通道。
// 关闭后 SendMessage 将返回错误；已启动的 Start 会在处理完剩余消息后退出。
func (w *Worker) Close() error {
	if w == nil {
		return fmt.Errorf("worker is nil")
	}

	w.mu.Lock()
	w.closed = true
	w.mu.Unlock()

	select {
	case w.notify <- struct{}{}:
	default:
	}
	return nil
}

func (w *Worker) process(ctx context.Context, input RunInput) error {
	if input.Context == nil {
		input.Context = ctx
	}

	emitter := w.cfg.emitter
	if emitter == nil {
		emitter = coreagent.NoEmitter{}
	}
	emitter = chainEmitter(emitter, input.Callbacks)

	var result *RunResult
	var err error
	if input.ExecuteOnce {
		result, err = w.run(input, true, emitter)
	} else {
		result, err = w.run(input, false, emitter)
	}

	w.resultMu.Lock()
	w.lastResult = result
	w.lastErr = err
	w.resultMu.Unlock()

	return err
}
