package streamtypes

// ChatClient is the minimal interface for non-streaming chat.
type ChatClient interface {
	Chat(request ChatRequest) (ChatResponse, error)
}

// StreamChatClient is the minimal interface for streaming chat.
type StreamChatClient interface {
	StreamChat(request ChatRequest) (<-chan StreamEvent, error)
}

// StreamChatClientWithOptions supports streaming with per-call options.
type StreamChatClientWithOptions interface {
	StreamChatWithOptions(request ChatRequest, opts ...StreamChatOption) (<-chan StreamEvent, error)
}
