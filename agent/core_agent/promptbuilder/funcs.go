package promptbuilder

import (
	"encoding/json"
	"fmt"
	"strings"
	"text/template"
)

// standardTemplateFuncMap 供多轮任务类模板复用（v2_task、frontend_engineer 等）。
func standardTemplateFuncMap() template.FuncMap {
	return template.FuncMap{
		"add": func(a, b int) int { return a + b },
		"marshalJSON": func(v interface{}) string {
			bytes, err := json.MarshalIndent(v, "   ", "  ")
			if err != nil {
				return fmt.Sprintf("%v", v)
			}
			return string(bytes)
		},
		"memoryContent": func(memory interface{ PromptContent() string }) string {
			if memory == nil {
				return ""
			}
			return memory.PromptContent()
		},
		"contextUsagePercent": func(info *ContextInfo) int {
			if info == nil || info.LimitTokens <= 0 || info.CurrentTokens <= 0 {
				return 0
			}
			percent := info.CurrentTokens * 100 / info.LimitTokens
			if percent < 0 {
				return 0
			}
			if percent > 100 {
				return 100
			}
			return percent
		},
		"historyContent": func(item interface{ RenderContent() string }) string {
			if item == nil {
				return ""
			}
			return item.RenderContent()
		},
		"historyRoleLabel": func(role string) string {
			switch strings.TrimSpace(role) {
			case "user":
				return "user"
			case "assistant":
				return "assistant"
			case "tool", "tool_call", "tool_calls":
				return "tool"
			case "":
				return "message"
			default:
				return strings.TrimSpace(role)
			}
		},
		"hasToolName": func(tools []ToolDefinition, name string) bool {
			name = strings.TrimSpace(name)
			if name == "" {
				return false
			}
			for _, tool := range tools {
				if strings.TrimSpace(tool.Name) == name {
					return true
				}
			}
			return false
		},
	}
}
