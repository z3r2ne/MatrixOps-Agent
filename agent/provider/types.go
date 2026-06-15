package provider

type UsageInfo struct {
	InputTokens        int
	OutputTokens       int
	ReasoningTokens    int
	CacheReadTokens    int
	CacheWrite5mTokens int
	CacheWrite1hTokens int
}

type CommonMessage struct {
	Type string `json:"type,omitempty"`

	// 工具调用
	CallID string `json:"call_id,omitempty"`

	// 消息内容
	Role    string      `json:"role,omitempty"`
	Content interface{} `json:"content,omitempty"` // string or []CommonContentPart
	// ToolCallID string           `json:"tool_call_id,omitempty"`
	ToolCalls []CommonToolCall `json:"tool_calls,omitempty"`
}

type CommonContentPart struct {
	Type     string          `json:"type"`
	Text     string          `json:"text,omitempty"`
	ImageURL *CommonImageURL `json:"image_url,omitempty"`
}

type CommonImageURL struct {
	URL string `json:"url"`
}

type CommonToolCall struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"`
	Function CommonToolCallFunction `json:"function"`
}

type CommonToolCallFunction struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

type CommonTool struct {
	Type     string             `json:"type"`
	Function CommonToolFunction `json:"function"`
}

type CommonToolFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
	Strict      bool                   `json:"strict,omitempty"`
}

type CommonRequest struct {
	Model        string          `json:"model"`
	MaxTokens    int             `json:"max_tokens,omitempty"`
	Temperature  float64         `json:"temperature,omitempty"`
	TopP         float64         `json:"top_p,omitempty"`
	Stop         interface{}     `json:"stop,omitempty"` // string or []string
	Instructions string          `json:"instructions,omitempty"`
	Messages     []CommonMessage `json:"messages"`
	Stream       bool            `json:"stream,omitempty"`
	Tools        []CommonTool    `json:"tools,omitempty"`
	ToolChoice   interface{}     `json:"tool_choice,omitempty"` // string or object

	// Extra fields from OpenAI Request
	Include    []string               `json:"include,omitempty"`
	Truncation interface{}            `json:"truncation,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	Store      bool                   `json:"store,omitempty"`
	User       string                 `json:"user,omitempty"`
	Text       interface{}            `json:"text,omitempty"`
	Reasoning  interface{}            `json:"reasoning,omitempty"`
}

type CommonResponse struct {
	ID      string                 `json:"id"`
	Object  string                 `json:"object"`
	Created int64                  `json:"created"`
	Model   string                 `json:"model"`
	Choices []CommonResponseChoice `json:"choices"`
	Usage   *CommonUsage           `json:"usage,omitempty"`
}

type CommonResponseChoice struct {
	Index        int           `json:"index"`
	Message      CommonMessage `json:"message"`
	FinishReason string        `json:"finish_reason"`
}

type CommonUsage struct {
	PromptTokens        int `json:"prompt_tokens,omitempty"`
	CompletionTokens    int `json:"completion_tokens,omitempty"`
	TotalTokens         int `json:"total_tokens,omitempty"`
	PromptTokensDetails *struct {
		CachedTokens int `json:"cached_tokens,omitempty"`
	} `json:"prompt_tokens_details,omitempty"`
}

type CommonChunk struct {
	ID      string              `json:"id"`
	Object  string              `json:"object"`
	Created int64               `json:"created"`
	Model   string              `json:"model"`
	Choices []CommonChunkChoice `json:"choices"`
	Usage   *CommonUsage        `json:"usage,omitempty"`
}

type CommonChunkChoice struct {
	Index        int              `json:"index"`
	Delta        CommonChunkDelta `json:"delta"`
	FinishReason string           `json:"finish_reason"`
}

type CommonChunkDelta struct {
	Role             string                `json:"role,omitempty"`
	Content          string                `json:"content,omitempty"`
	ReasoningContent string                `json:"reasoning_content,omitempty"`
	ToolCalls        []CommonChunkToolCall `json:"tool_calls,omitempty"`
}

type CommonChunkToolCall struct {
	Index    int                     `json:"index"`
	ID       string                  `json:"id,omitempty"`
	Type     string                  `json:"type,omitempty"`
	Function *CommonToolCallFunction `json:"function,omitempty"`
}
