package coreagent

import (
	"encoding/json"
	"strings"

	agentmemory "matrixops.local/memory"
	agentprovider "matrixops-agent/provider"
	"matrixops.local/core_agent/streamtypes"
)

func BuildMemoryMessages(value any) []*ModelMessage {
	memory := resolveAgentMemory(value)
	if memory == nil {
		return nil
	}

	if len(memory.History) > 0 {
		out := make([]*ModelMessage, 0, len(memory.History))
		for _, item := range memory.History {
			out = append(out, buildMemoryMessagesFromHistoryItem(item)...)
		}
		// 与 Entries 路径一致：多次 RecordAction 会把「正文 assistant」与「单工具 assistant」拆成多行 History，
		// 合并后再交给 Chat/Responses 重放，避免一次模型回合被拆成多条 assistant。
		return coalesceAssistantNativeToolMessages(out)
	}
	if len(memory.Entries) > 0 {
		out := make([]*ModelMessage, 0, len(memory.Entries)*2)
		for _, entry := range memory.Entries {
			out = append(out, buildMemoryMessagesFromEntry(entry)...)
		}
		return coalesceAssistantNativeToolMessages(out)
	}
	return nil
}

// coalesceAssistantNativeToolMessages 将「仅正文的 assistant」与紧随的「(单工具 assistant, tool)」重复序列合并为一条带 tool_calls 的 assistant，
// 与 Chat Completions 多轮格式一致（content + reasoning_content + tool_calls 同一条）。
func coalesceAssistantNativeToolMessages(messages []*ModelMessage) []*ModelMessage {
	if len(messages) < 2 {
		return messages
	}
	out := make([]*ModelMessage, 0, len(messages))
	i := 0
	for i < len(messages) {
		cur := messages[i]
		if cur == nil {
			i++
			continue
		}
		if isAssistantTextOnlyModelMessage(cur) && i+1 < len(messages) && isAssistantSingleToolModelMessage(messages[i+1]) &&
			shouldMergeAssistantModelMessages(cur, messages[i+1]) {
			merged := cloneModelMessageForMerge(messages[i+1])
			mergeAssistantPreludeInto(merged, cur)
			i += 2
			var tailTools []*ModelMessage
			for i < len(messages) {
				msg := messages[i]
				if msg == nil {
					i++
					continue
				}
				if isAssistantSingleToolModelMessage(msg) {
					if !shouldCoalesceAssistantToolMessages(merged, msg) {
						break
					}
					merged.ToolCalls = append(merged.ToolCalls, msg.ToolCalls[0])
					i++
					if i < len(messages) && messages[i] != nil && strings.TrimSpace(messages[i].Role) == "tool" {
						tailTools = append(tailTools, messages[i])
						i++
					}
					continue
				}
				if strings.TrimSpace(msg.Role) == "tool" {
					tailTools = append(tailTools, msg)
					i++
					continue
				}
				break
			}
			out = append(out, merged)
			out = append(out, tailTools...)
			continue
		}
		if isAssistantSingleToolModelMessage(cur) {
			merged := cloneModelMessageForMerge(cur)
			i++
			var tailTools []*ModelMessage
			for i < len(messages) {
				msg := messages[i]
				if msg == nil {
					i++
					continue
				}
				if isAssistantSingleToolModelMessage(msg) {
					if !shouldCoalesceAssistantToolMessages(merged, msg) {
						break
					}
					merged.ToolCalls = append(merged.ToolCalls, msg.ToolCalls[0])
					i++
					if i < len(messages) && messages[i] != nil && strings.TrimSpace(messages[i].Role) == "tool" {
						tailTools = append(tailTools, messages[i])
						i++
					}
					continue
				}
				if strings.TrimSpace(msg.Role) == "tool" {
					tailTools = append(tailTools, msg)
					i++
					continue
				}
				break
			}
			out = append(out, merged)
			out = append(out, tailTools...)
			continue
		}
		out = append(out, cur)
		i++
	}
	return out
}

func isAssistantTextOnlyModelMessage(m *ModelMessage) bool {
	return m != nil && strings.TrimSpace(m.Role) == "assistant" && len(m.ToolCalls) == 0
}

func isAssistantSingleToolModelMessage(m *ModelMessage) bool {
	return m != nil && strings.TrimSpace(m.Role) == "assistant" && len(m.ToolCalls) == 1
}

func shouldMergeAssistantModelMessages(textMsg, toolMsg *ModelMessage) bool {
	if textMsg == nil || toolMsg == nil {
		return false
	}
	if textMsg.Synthetic {
		return false
	}
	textMessageID := strings.TrimSpace(textMsg.SourceMessageID)
	toolMessageID := strings.TrimSpace(toolMsg.SourceMessageID)
	if textMessageID != "" && toolMessageID != "" {
		return textMessageID == toolMessageID
	}
	if textMessageID != "" || toolMessageID != "" {
		return false
	}
	return true
}

func shouldCoalesceAssistantToolMessages(first, next *ModelMessage) bool {
	if first == nil || next == nil {
		return false
	}
	firstID := strings.TrimSpace(first.SourceMessageID)
	nextID := strings.TrimSpace(next.SourceMessageID)
	if firstID == "" || nextID == "" {
		return false
	}
	return firstID == nextID
}

func cloneModelMessageForMerge(m *ModelMessage) *ModelMessage {
	if m == nil {
		return nil
	}
	c := *m
	if len(m.ToolCalls) > 0 {
		c.ToolCalls = append([]ToolCall(nil), m.ToolCalls...)
	}
	if len(m.ResponsesReasoningItemRaws) > 0 {
		c.ResponsesReasoningItemRaws = append([]string(nil), m.ResponsesReasoningItemRaws...)
	}
	return &c
}

func mergeAssistantPreludeInto(dst, prelude *ModelMessage) {
	if dst == nil || prelude == nil {
		return
	}
	if pc := strings.TrimSpace(streamtypes.RenderMessageTextContent(prelude.Content)); pc != "" {
		if strings.TrimSpace(streamtypes.RenderMessageTextContent(dst.Content)) == "" {
			dst.Content = pc
		}
	}
	if p := strings.TrimSpace(prelude.ReasoningContent); p != "" {
		dst.ReasoningContent = p
	}
	if p := strings.TrimSpace(prelude.ThinkingSignature); p != "" {
		dst.ThinkingSignature = p
	}
	if strings.TrimSpace(dst.Phase) == "" {
		dst.Phase = strings.TrimSpace(prelude.Phase)
	}
	if strings.TrimSpace(dst.ResponsesOutputMessageRaw) == "" {
		dst.ResponsesOutputMessageRaw = strings.TrimSpace(prelude.ResponsesOutputMessageRaw)
	}
	if len(dst.ResponsesReasoningItemRaws) == 0 && len(prelude.ResponsesReasoningItemRaws) > 0 {
		dst.ResponsesReasoningItemRaws = append([]string(nil), prelude.ResponsesReasoningItemRaws...)
	}
}

func resolveAgentMemory(value any) *agentmemory.Memory {
	switch typed := value.(type) {
	case *agentmemory.Memory:
		if typed == nil {
			return nil
		}
		return typed
	case agentmemory.Memory:
		copied := typed
		return &copied
	case interface {
		TranscriptSourceEntries() []*agentmemory.MemoryEntry
	}:
		return &agentmemory.Memory{Entries: typed.TranscriptSourceEntries()}
	default:
		return nil
	}
}

func buildMemoryMessagesFromEntry(entry *agentmemory.MemoryEntry) []*ModelMessage {
	if entry == nil {
		return nil
	}

	if entry.HasToolCall() {
		return buildToolMemoryMessages(entry)
	}

	content := strings.TrimSpace(entry.RenderContent())
	reasoningRaws := parseResponsesReasoningItemRawsJSON(entry.ResponsesReasoningItemRawsJSON)
	if content == "" &&
		strings.TrimSpace(entry.ResponsesOutputMessageRaw) == "" &&
		len(reasoningRaws) == 0 &&
		strings.TrimSpace(entry.ReasoningContent) == "" {
		return nil
	}

	role := normalizeMemoryMessageRole(entry.Role)
	return []*ModelMessage{{
		Role:                       role,
		Content:                    content,
		SourceMessageID:            strings.TrimSpace(entry.SourceMessageID),
		Synthetic:                  entry.Synthetic,
		Phase:                      strings.TrimSpace(entry.Phase),
		ResponsesOutputMessageRaw:  strings.TrimSpace(entry.ResponsesOutputMessageRaw),
		ResponsesReasoningItemRaws: reasoningRaws,
		ReasoningContent:           strings.TrimSpace(entry.ReasoningContent),
		ThinkingSignature:          strings.TrimSpace(entry.ThinkingSignature),
	}}
}

func buildMemoryMessagesFromHistoryItem(item *agentmemory.ChatHistoryItem) []*ModelMessage {
	if item == nil {
		return nil
	}

	role := strings.TrimSpace(item.Role)
	content := strings.TrimSpace(item.Content)
	if content == "" {
		content = strings.TrimSpace(item.RenderContent())
	}

	if len(item.NativeToolCalls) > 0 {
		toolCalls := make([]ToolCall, 0, len(item.NativeToolCalls))
		for _, c := range item.NativeToolCalls {
			toolCalls = append(toolCalls, ToolCall{
				ID:        strings.TrimSpace(c.ID),
				Name:      strings.TrimSpace(c.Name),
				Arguments: parseMemoryToolArgumentsFromJSONString(strings.TrimSpace(c.Arguments)),
			})
		}
		msg := &ModelMessage{
			Role:                       "assistant",
			ToolCalls:                  toolCalls,
			SourceMessageID:            strings.TrimSpace(item.SourceMessageID),
			Synthetic:                  item.Synthetic,
			Phase:                      strings.TrimSpace(item.Phase),
			ResponsesOutputMessageRaw:  strings.TrimSpace(item.ResponsesOutputMessageRaw),
			ResponsesReasoningItemRaws: append([]string(nil), item.ResponsesReasoningItemRaws...),
			ReasoningContent:           strings.TrimSpace(item.ReasoningContent),
			ThinkingSignature:          strings.TrimSpace(item.ThinkingSignature),
		}
		if text := strings.TrimSpace(item.Content); text != "" && text != "call_tool" {
			msg.Content = text
		}
		return []*ModelMessage{msg}
	}

	switch {
	case strings.HasPrefix(role, "call_tool_"):
		if content == "" {
			return nil
		}
		return []*ModelMessage{{
			Role:    "assistant",
			Content: content,
		}}
	case role == "tool":
		toolID := strings.TrimSpace(item.ToolCallID)
		if toolID == "" {
			return nil
		}
		resultContent := BuildToolLLMContentFromHistoryToolItem(item)
		if streamtypes.RenderMessageTextContent(resultContent) == "" {
			return nil
		}
		return []*ModelMessage{{
			Role:       "tool",
			ToolCallID: toolID,
			Content:    resultContent,
		}}
	default:
		if content == "" && len(item.LLMContentParts) == 0 && strings.TrimSpace(item.ResponsesOutputMessageRaw) == "" && len(item.ResponsesReasoningItemRaws) == 0 && strings.TrimSpace(item.ReasoningContent) == "" && strings.TrimSpace(item.ThinkingSignature) == "" {
			return nil
		}
		msg := &ModelMessage{
			Role:                       normalizeMemoryMessageRole(role),
			SourceMessageID:            strings.TrimSpace(item.SourceMessageID),
			Synthetic:                  item.Synthetic,
			Phase:                      strings.TrimSpace(item.Phase),
			ResponsesOutputMessageRaw:  strings.TrimSpace(item.ResponsesOutputMessageRaw),
			ResponsesReasoningItemRaws: append([]string(nil), item.ResponsesReasoningItemRaws...),
			ReasoningContent:           strings.TrimSpace(item.ReasoningContent),
			ThinkingSignature:          strings.TrimSpace(item.ThinkingSignature),
		}
		if len(item.LLMContentParts) > 0 {
			msg.Content = agentprovider.SimplifyTextOnlyContent(chatHistoryContentPartsToProvider(item.LLMContentParts))
		} else {
			msg.Content = content
		}
		return []*ModelMessage{msg}
	}
}

func chatHistoryContentPartsToProvider(parts []agentmemory.ChatHistoryContentPart) []agentprovider.CommonContentPart {
	if len(parts) == 0 {
		return nil
	}
	out := make([]agentprovider.CommonContentPart, 0, len(parts))
	for _, part := range parts {
		switch strings.TrimSpace(part.Type) {
		case "text":
			if strings.TrimSpace(part.Text) == "" {
				continue
			}
			out = append(out, agentprovider.CommonContentPart{Type: "text", Text: strings.TrimSpace(part.Text)})
		case "image_url":
			if part.ImageURL == nil || strings.TrimSpace(part.ImageURL.URL) == "" {
				continue
			}
			out = append(out, agentprovider.CommonContentPart{
				Type:     "image_url",
				ImageURL: &agentprovider.CommonImageURL{URL: strings.TrimSpace(part.ImageURL.URL)},
			})
		}
	}
	return out
}

func buildToolMemoryMessages(entry *agentmemory.MemoryEntry) []*ModelMessage {
	messages := make([]*ModelMessage, 0, 2)

	toolName := strings.TrimSpace(entry.ToolName)
	toolCallID := strings.TrimSpace(entry.ToolCallID)
	toolArgs := parseMemoryToolArguments(entry)

	if toolName != "" && toolCallID != "" {
		assistantMessage := &ModelMessage{
			Role: "assistant",
			ToolCalls: []ToolCall{{
				ID:        toolCallID,
				Name:      toolName,
				Arguments: toolArgs,
			}},
			SourceMessageID:   strings.TrimSpace(entry.SourceMessageID),
			Synthetic:         entry.Synthetic,
			ReasoningContent:  strings.TrimSpace(entry.ReasoningContent),
			ThinkingSignature: strings.TrimSpace(entry.ThinkingSignature),
		}
		if content := strings.TrimSpace(entry.RawOutput); content != "" && content != strings.TrimSpace(entry.ToolRequestRawJSON) {
			assistantMessage.Content = content
		}
		messages = append(messages, assistantMessage)
	} else if request := strings.TrimSpace(agentmemory.FirstNonEmptyTrimmed(entry.ToolRequestRawJSON, entry.Content, entry.RawOutput, entry.CallToolInfo)); request != "" {
		messages = append(messages, &ModelMessage{
			Role:    "assistant",
			Content: request,
		})
	}

	resultContent := BuildToolLLMContentFromEntry(entry)
	if streamtypes.RenderMessageTextContent(resultContent) == "" {
		return messages
	}

	if toolCallID != "" {
		messages = append(messages, &ModelMessage{
			Role:       "tool",
			ToolCallID: toolCallID,
			Content:    resultContent,
		})
		return messages
	}

	messages = append(messages, &ModelMessage{
		Role:    "assistant",
		Content: resultContent,
	})
	return messages
}

func parseMemoryToolArgumentsFromJSONString(candidate string) map[string]interface{} {
	candidate = strings.TrimSpace(candidate)
	if candidate == "" {
		return map[string]interface{}{}
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(candidate), &parsed); err == nil && parsed != nil {
		return parsed
	}
	return map[string]interface{}{}
}

func parseMemoryToolArguments(entry *agentmemory.MemoryEntry) map[string]interface{} {
	candidates := []string{
		strings.TrimSpace(entry.ToolInputJSON),
		strings.TrimSpace(entry.ToolRequestRawJSON),
		strings.TrimSpace(entry.Content),
	}
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(candidate), &parsed); err == nil && parsed != nil {
			return parsed
		}
	}
	return map[string]interface{}{}
}

func normalizeMemoryMessageRole(role string) string {
	switch strings.TrimSpace(role) {
	case "system":
		return "system"
	case "user":
		return "user"
	default:
		return "assistant"
	}
}

func parseResponsesReasoningItemRawsJSON(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var values []string
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}
