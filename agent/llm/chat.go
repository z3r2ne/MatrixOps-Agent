package llm

import (
	"context"
	"pkgs/db/models"
)

type ToolDefinition struct {
	Name        string
	Description string
	Schema      map[string]interface{}
}

type ToolCall struct {
	ID        string
	Name      string
	Arguments map[string]interface{}
}

type ChatRequest struct {
	Messages        []*ModelMessage
	Context         context.Context
	Tools           []ToolDefinition
	ModelSettings   *models.ModelSettings
	Temperature     float64
	TopP            float64
	MaxOutputTokens int
	ProviderOptions *models.LLMConfig
	Model           string
	ExtraOptions    map[string]interface{}
}

type ChatResponse struct {
	Message   ModelMessage
	ToolCalls []ToolCall
	Finish    string
	Usage     *Usage
}

type ChatClient interface {
	Chat(request ChatRequest) (ChatResponse, error)
}

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

type StreamChatClient interface {
	StreamChat(request ChatRequest) (<-chan StreamEvent, error)
}

type StreamChatClientWithOptions interface {
	StreamChatWithOptions(request ChatRequest, opts ...StreamChatOption) (<-chan StreamEvent, error)
}

type Usage struct {
	InputTokens       int
	OutputTokens      int
	ReasoningTokens   int
	CachedInputTokens int
}
