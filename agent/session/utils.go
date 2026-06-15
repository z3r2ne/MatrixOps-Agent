package session

import (
	"matrixops-agent/llm"
	"matrixops-agent/tool"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

func resolveTools(overrides map[string]bool, registry *tool.Registry) []llm.ToolDefinition {
	if registry == nil {
		return nil
	}
	names := registry.Names()
	out := []llm.ToolDefinition{}
	for _, name := range prioritizeToolNames(names) {
		if overrides != nil {
			if enabled, ok := overrides[name]; ok && !enabled {
				continue
			}
		}
		tool, err := registry.Get(name)
		if err != nil {
			continue
		}
		out = append(out, llm.ToolDefinition{
			Name:        name,
			Description: tool.Description(),
			Schema:      tool.Schema(),
		})
	}
	return out
}

func prioritizeToolNames(names []string) []string {
	if len(names) == 0 {
		return nil
	}
	priority := map[string]int{
		"run_worker_task": 0,
		"read":            10,
		"rg":              20,
		"glob":            30,
		"list":            40,
		"tree":            50,
	}

	out := append([]string(nil), names...)
	sort.SliceStable(out, func(i, j int) bool {
		left := priority[out[i]]
		right := priority[out[j]]
		leftKnown := left != 0 || out[i] == "run_worker_task"
		rightKnown := right != 0 || out[j] == "run_worker_task"
		switch {
		case leftKnown && rightKnown:
			if left != right {
				return left < right
			}
		case leftKnown:
			return true
		case rightKnown:
			return false
		}
		return out[i] < out[j]
	})
	return out
}

func messagesToContextPrompt(messages []*llm.ModelMessage, role string) string {

	var contextBuilder strings.Builder
	contextBuilder.WriteString("以下是之前的对话历史，请基于这些上下文回答新的问题，你的身份是 [" + role + "]：\n\n")
	contextBuilder.WriteString("<对话历史>\n")

	for i, msg := range messages {
		// 跳过空消息
		if msg == nil {
			continue
		}

		// 格式化角色名
		role := msg.Role
		switch role {
		case "user":
			role = "用户"
		case "assistant":
			role = "助手"
		case "system":
			role = "系统"
		case "tool":
			role = "工具"
		}

		// 提取消息内容
		var content string
		switch v := msg.Content.(type) {
		case string:
			content = v
		case []interface{}:
			// 处理多部分内容（如文本+图片）
			var parts []string
			for _, part := range v {
				if partMap, ok := part.(map[string]interface{}); ok {
					if text, ok := partMap["text"].(string); ok && text != "" {
						parts = append(parts, text)
					}
				}
			}
			content = strings.Join(parts, "\n")
		default:
			// 尝试将其他类型转换为字符串
			content = fmt.Sprint(v)
		}

		// 跳过空内容
		if strings.TrimSpace(content) == "" && len(msg.ToolCalls) == 0 {
			continue
		}

		// 添加消息到上下文
		contextBuilder.WriteString(fmt.Sprintf("[%s]\n", role))

		if content != "" {
			contextBuilder.WriteString(content)
			contextBuilder.WriteString("\n")
		}

		// 添加工具调用信息
		if len(msg.ToolCalls) > 0 {
			contextBuilder.WriteString("【调用工具】\n")
			for _, tc := range msg.ToolCalls {
				contextBuilder.WriteString(fmt.Sprintf("- %s", tc.Name))
				if len(tc.Arguments) > 0 {
					argsJSON, _ := json.Marshal(tc.Arguments)
					contextBuilder.WriteString(fmt.Sprintf(": %s", string(argsJSON)))
				}
				contextBuilder.WriteString("\n")
			}
		}

		// 在消息之间添加分隔
		if i < len(messages)-1 {
			contextBuilder.WriteString("\n")
		}
	}

	contextBuilder.WriteString("</对话历史>\n\n")
	// contextBuilder.WriteString("<当前问题>\n")
	// contextBuilder.WriteString(input)
	// contextBuilder.WriteString("\n</当前问题>\n\n")
	contextBuilder.WriteString("请基于上述对话历史基于你的身份和系统提示回答, 你的身份是 [" + role + "]：")

	return contextBuilder.String()
}
