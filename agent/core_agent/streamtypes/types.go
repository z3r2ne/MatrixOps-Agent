package streamtypes

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	agentprovider "matrixops-agent/provider"
)

// ModelMessage represents a single message in the chat history.
type ModelMessage struct {
	Role                       string
	Content                    interface{}
	Name                       string
	ToolCallID                 string
	ToolCalls                  []ToolCall
	Phase                      string
	ResponsesOutputMessageRaw  string
	ResponsesReasoningItemRaws []string
	ReasoningContent           string
	// ThinkingSignature 用于 Anthropic extended thinking 多轮：与 ReasoningContent 一起回传上一助手轮的 thinking 块。
	ThinkingSignature string
	// SourceMessageID 重建 LLM 消息时用于判断 assistant 正文与 tool_calls 是否属于同一轮。
	SourceMessageID string
	// Synthetic 为 true 时不应与后续 tool_calls 合并（如压缩摘要条目）。
	Synthetic bool
}

// ChatRequest is the unified request passed to a ChatClient.
type ChatRequest struct {
	Messages        []*ModelMessage
	Context         context.Context
	Tools           []ToolDefinition
	ActionSchemas   []ActionPromptSchema
	Temperature     float64
	TopP            float64
	MaxOutputTokens int
	ProviderOptions any
	Model           string
	ExtraOptions    map[string]interface{}
}

// ChatResponse is the unified response from a ChatClient.
type ChatResponse struct {
	Message   ModelMessage
	ToolCalls []ToolCall
	Finish    string
	Usage     *Usage
}

// StreamEvent represents a single event in a streaming chat response.
type StreamEvent struct {
	Type          string
	Text          string
	ToolIndex     int
	ToolCallID    string
	ToolName      string
	ToolArguments string
	Finish        string
	Usage         *Usage
	Error         error
}

// Usage tracks token consumption for a single LLM call.
type Usage struct {
	InputTokens       int
	OutputTokens      int
	ReasoningTokens   int
	CachedInputTokens int
}

// ToolDefinition describes a tool that the model may call.
type ToolDefinition struct {
	Name        string
	Description string
	Schema      map[string]interface{}
}

// ActionPromptSchema describes a control action available in compatible (JSON envelope) mode.
type ActionPromptSchema struct {
	ActionName  string
	Description string
	DataSchema  interface{}
}

// ToolCall represents a single tool invocation from the model.
type ToolCall struct {
	ID        string
	Name      string
	Arguments map[string]interface{}
}

// ToolContext carries contextual data for tool execution.
type ToolContext struct {
	Context   context.Context
	SessionID string
	Directory string
	Worktree  string
	Values    map[string]interface{}
}

// ActionOutput is the parsed result of a compatible-mode {@action,data} envelope.
// Native providers emit CallToolRequest directly instead.
type ActionOutput struct {
	Index   int
	Action  string
	Data    io.Reader
	RawJSON string
}

// CallToolRequest is a normalized tool invocation emitted by StreamOutput (native or compatible).
type CallToolRequest struct {
	Index     int
	CallID    string
	Name      string
	Arguments io.Reader
	RawJSON   string
	Reason    string
}

// CompatibleControlHandler processes non-tool compatible envelopes (message, answer).
type CompatibleControlHandler func(action *ActionOutput) error

// StreamInput carries all parameters for a streaming action request.
type StreamInput struct {
	Context                  context.Context
	Model                    string
	Prompt                   string
	SystemPrompt             string
	Instruction              string
	HistoryMessages          []*ModelMessage
	UserContentParts         []agentprovider.CommonContentPart
	Abort                    context.Context
	Temperature              float64
	TopP                     float64
	MaxOutputTokens          int
	ProviderOptions          any
	Tools                    []ToolDefinition
	ActionSchemas            []ActionPromptSchema
	HTTPClient               *http.Client
	OnRawRequest             func(raw string)
	OnRawResponse            func(raw string)
	OnRetryError             func(err error, retryAttempt int, maxRetries int, nextDelay time.Duration, attemptDuration time.Duration, rawResponse string)
	// CompatibleControlHandler 仅兼容模式：处理 message/answer 等非工具信封。
	CompatibleControlHandler CompatibleControlHandler
	ReasoningEffort          string
	TextVerbosity            string
	EnableEncryptedReasoning bool
	ParallelToolCalls        bool
	PromptCacheKey           string
	ThinkingType             string
	EnableThinking           *bool
	BudgetTokens             *int
}

// StreamOutput is the result of a streaming action request.
type StreamOutput struct {
	ToolCalls <-chan *CallToolRequest
	RawTextReader io.Reader
	// ContentReader 在 StreamInput.Tools 非空且走原生工具流时由 provider 填充：助手可见正文（与 RawTextReader 中正文增量同源），应在 Wait 成功返回后再读取。
	ContentReader io.Reader
	// NativeAssistantTextFinishesTurn 为 true 表示本轮模型以纯文本结束（无原生 tool_calls / tool_use），Runner 据此视为 answer 收尾。
	NativeAssistantTextFinishesTurn bool
	ReasonReader                    io.Reader
	Wait                            func() error
	Usage                           *Usage
	Phase                           string
	ResponsesOutputMessageRaw       string
	ResponsesReasoningItemRaws      []string

	anthropicThinkingSigMu sync.RWMutex
	anthropicThinkingSig   string
}

// SetAnthropicThinkingSignature 记录本轮 Anthropic thinking 块的签名（供下一轮 Messages 回传）。
func (o *StreamOutput) SetAnthropicThinkingSignature(signature string) {
	if o == nil {
		return
	}
	s := strings.TrimSpace(signature)
	if s == "" {
		return
	}
	o.anthropicThinkingSigMu.Lock()
	o.anthropicThinkingSig = s
	o.anthropicThinkingSigMu.Unlock()
}

// AnthropicThinkingSignature 返回上一流式响应中最后一个 thinking 块的签名（可能为空）。
func (o *StreamOutput) AnthropicThinkingSignature() string {
	if o == nil {
		return ""
	}
	o.anthropicThinkingSigMu.RLock()
	defer o.anthropicThinkingSigMu.RUnlock()
	return o.anthropicThinkingSig
}
