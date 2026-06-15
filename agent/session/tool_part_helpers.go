package session

import (
	"encoding/json"
	"strings"
)

func normalizeActionName(atAction string, action string) string {
	if value := strings.TrimSpace(atAction); value != "" {
		return value
	}
	return strings.TrimSpace(action)
}

// toolNameFromCallToolParams 从 call_tool 内置动作的 params（单工具或 tool_calls）解析工具名。
func toolNameFromCallToolParams(params json.RawMessage) string {
	if len(params) == 0 {
		return ""
	}
	var payload struct {
		Name      string `json:"name"`
		ToolName  string `json:"tool_name"`
		ToolCalls []struct {
			Name     string `json:"name"`
			ToolName string `json:"tool_name"`
		} `json:"tool_calls"`
	}
	if err := json.Unmarshal(params, &payload); err != nil {
		return ""
	}
	if n := strings.TrimSpace(payload.Name); n != "" {
		return n
	}
	if n := strings.TrimSpace(payload.ToolName); n != "" {
		return n
	}
	if len(payload.ToolCalls) > 0 {
		if n := strings.TrimSpace(payload.ToolCalls[0].Name); n != "" {
			return n
		}
		return strings.TrimSpace(payload.ToolCalls[0].ToolName)
	}
	return ""
}
