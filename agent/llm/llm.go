package llm

type Telemetry struct {
	Enabled bool
	UserID  string
}

type ModelMessage struct {
	Role       string
	Content    interface{}
	Name       string
	ToolCallID string
	ToolCalls  []ToolCall
}

type GenerateRequest struct {
	Telemetry       Telemetry
	Temperature     float64
	Messages        []ModelMessage
	Model           interface{}
	Schema          interface{}
	ProviderOptions map[string]interface{}
}

type GenerateResult struct {
	Identifier   string
	WhenToUse    string
	SystemPrompt string
}

type Generator interface {
	GenerateObject(request GenerateRequest) (GenerateResult, error)
	StreamObject(request GenerateRequest) (GenerateResult, error)
}

type GeneratorMessageType string

const (
	GeneratorMessageTypeTextDelta      GeneratorMessageType = "text-delta"
	GeneratorMessageTypeToolDelta      GeneratorMessageType = "tool-delta"
	GeneratorMessageTypeReasoningDelta GeneratorMessageType = "reasoning-delta"
	GeneratorMessageTypeFinish         GeneratorMessageType = "finish"
	GeneratorMessageTypeError          GeneratorMessageType = "error"
	GeneratorMessageTypeStart          GeneratorMessageType = "start"
	GeneratorMessageTypeStep           GeneratorMessageType = "step"
	GeneratorMessageTypeEnd            GeneratorMessageType = "end"
	GeneratorMessageTypeStartStep      GeneratorMessageType = "start-step"
	GeneratorMessageTypeFinishStep     GeneratorMessageType = "finish-step"
	GeneratorMessageTypeTextStart      GeneratorMessageType = "text-start"
	GeneratorMessageTypeTextEnd        GeneratorMessageType = "text-end"
	GeneratorMessageTypeTextFinish     GeneratorMessageType = "text-finish"
)
