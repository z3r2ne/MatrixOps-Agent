package promptbuilder

import agentmemory "matrixops.local/memory"

type ContextInfo struct {
	LimitTokens   int
	CurrentTokens int
	CurrentBytes  int
}

type ToolDefinition struct {
	Name        string
	Description string
	Schema      map[string]interface{}
}

type Input struct {
	Memory      *agentmemory.Memory
	Tools       []ToolDefinition
	UserInput   string
	WorkerExtraPrompt string
	ContextInfo *ContextInfo
	// NativeOpenAITools 为 true 时，模板不要求模型输出 call_tool JSON，也不在 <tools> 中展开 JSON Schema（参数以 API 侧 tools 为准）。
	NativeOpenAITools bool
}

type Builder func(input Input) (string, error)

func boolParam(params map[string]interface{}, key string, fallback bool) bool {
	if len(params) == 0 {
		return fallback
	}
	value, ok := params[key]
	if !ok {
		return fallback
	}
	typed, ok := value.(bool)
	if !ok {
		return fallback
	}
	return typed
}
