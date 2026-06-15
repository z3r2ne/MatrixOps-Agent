package coreagent

import (
	"strings"

	agentmemory "matrixops.local/memory"
	agentprovider "matrixops-agent/provider"
)

const (
	emptyToolSystemMessage    = "Tool output is empty."
	nonTextToolSystemMessage  = "Tool returned non-text content."
)

// FormatToolSystemTag 将摘要文案包在 <system> 标签内。
func FormatToolSystemTag(message string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return ""
	}
	return "<system>" + message + "</system>"
}

// BuildToolLLMContent 构建发给大模型的 tool 消息 content（system 摘要 + 正文，对齐 Kimi CLI）。
func BuildToolLLMContent(systemMessage, body string, isError bool, toolError string) interface{} {
	systemMessage = strings.TrimSpace(systemMessage)
	toolError = strings.TrimSpace(toolError)
	hasBody := strings.TrimSpace(body) != ""

	if isError {
		msg := systemMessage
		if msg == "" {
			msg = toolError
		}
		if msg == "" {
			msg = "Tool returned an error."
		}
		parts := []agentprovider.CommonContentPart{{
			Type: "text",
			Text: FormatToolSystemTag("ERROR: " + msg),
		}}
		if hasBody {
			parts = append(parts, agentprovider.CommonContentPart{Type: "text", Text: body})
		}
		return parts
	}

	parts := make([]agentprovider.CommonContentPart, 0, 2)
	if systemMessage != "" {
		parts = append(parts, agentprovider.CommonContentPart{
			Type: "text",
			Text: FormatToolSystemTag(systemMessage),
		})
	}
	if hasBody {
		parts = append(parts, agentprovider.CommonContentPart{Type: "text", Text: body})
	}
	if len(parts) == 0 {
		return []agentprovider.CommonContentPart{{
			Type: "text",
			Text: FormatToolSystemTag(emptyToolSystemMessage),
		}}
	}
	if !hasTextContentPart(parts) {
		parts = append([]agentprovider.CommonContentPart{{
			Type: "text",
			Text: FormatToolSystemTag(nonTextToolSystemMessage),
		}}, parts...)
	}
	return parts
}

func hasTextContentPart(parts []agentprovider.CommonContentPart) bool {
	for _, part := range parts {
		if strings.TrimSpace(part.Type) == "text" && strings.TrimSpace(part.Text) != "" {
			return true
		}
	}
	return false
}

// BuildToolLLMContentFromHistoryToolItem 从对话历史中的 tool 行构建 tool 消息 content。
func BuildToolLLMContentFromHistoryToolItem(item *agentmemory.ChatHistoryItem) interface{} {
	if item == nil {
		return BuildToolLLMContent("", "", false, "")
	}
	body := item.Content
	if strings.TrimSpace(body) == "" {
		body = strings.TrimSpace(item.RenderContent())
	}
	isError := strings.TrimSpace(item.ToolError) != "" ||
		strings.EqualFold(strings.TrimSpace(item.ToolStatus), "error") ||
		strings.EqualFold(strings.TrimSpace(item.ToolStatus), "cancelled")
	toolError := strings.TrimSpace(item.ToolError)
	if toolError == "" {
		toolError = strings.TrimSpace(item.ToolStatus)
	}
	systemMessage := resolveToolSystemMessageFromHistoryItem(item)
	return BuildToolLLMContent(systemMessage, body, isError, toolError)
}

// BuildToolLLMContentFromEntry 从记忆条目构建 tool 消息 content。
func BuildToolLLMContentFromEntry(entry *agentmemory.MemoryEntry) interface{} {
	if entry == nil {
		return BuildToolLLMContent("", "", false, "")
	}
	body := entry.ToolOutput
	systemMessage := resolveToolSystemMessageFromEntry(entry)
	isError := strings.TrimSpace(entry.ToolError) != "" ||
		strings.EqualFold(strings.TrimSpace(entry.ToolStatus), "error") ||
		strings.EqualFold(strings.TrimSpace(entry.ToolStatus), "cancelled")

	if systemMessage == "" && body == "" && !isError {
		legacy := agentmemory.RenderToolResultContent(entry.ToolOutput, entry.ToolError, entry.CallToolInfo, entry.ToolStatus, entry.ToolTitle)
		if strings.TrimSpace(legacy) != "" {
			return legacy
		}
	}

	toolError := strings.TrimSpace(entry.ToolError)
	if toolError == "" {
		toolError = strings.TrimSpace(entry.ToolStatus)
	}
	return BuildToolLLMContent(systemMessage, body, isError, toolError)
}
