package sessionmemory

import (
	"encoding/json"
	"strings"
	"sync"
	"time"

	agentmemory "matrixops.local/memory"
	"matrixops-agent/types"
)

type ProcessV2MemoryState struct {
	mu     sync.Mutex
	memory *types.Memory
}

func NewProcessV2MemoryState(base *types.Memory) *ProcessV2MemoryState {
	return &ProcessV2MemoryState{memory: CloneProcessV2Memory(base)}
}

func (s *ProcessV2MemoryState) Snapshot() *types.Memory {
	if s == nil {
		return &types.Memory{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return CloneProcessV2Memory(s.memory)
}

func (s *ProcessV2MemoryState) AppendUserText(text string, llmContentParts []types.ChatHistoryContentPart) {
	if s == nil || (strings.TrimSpace(text) == "" && len(llmContentParts) == 0) {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.memory == nil {
		s.memory = &types.Memory{}
	}
	s.memory.History = append(s.memory.History, &types.ChatHistoryItem{
		Role:            "user",
		Content:         text,
		LLMContentParts: types.CloneChatHistoryContentParts(llmContentParts),
		Created:         time.Now().UnixMilli(),
	})
	s.memory.LatestToolCall = BuildLatestToolCall(s.memory.History)
}

func (s *ProcessV2MemoryState) RecordAction(rawOutput string, parts []*types.Part, sourceMessageID string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.memory == nil {
		s.memory = &types.Memory{}
	}
	var assistantText []string
	var toolParts []*types.Part
	phase := ""
	responsesOutputMessageRaw := ""
	var responsesReasoningItemRaws []string
	for _, part := range parts {
		if part == nil {
			continue
		}
		if part.Metadata != nil {
			if phase == "" {
				if value, ok := part.Metadata["phase"].(string); ok {
					phase = strings.TrimSpace(value)
				}
			}
			if responsesOutputMessageRaw == "" {
				if value, ok := part.Metadata["responsesOutputMessageRaw"].(string); ok {
					responsesOutputMessageRaw = strings.TrimSpace(value)
				}
			}
			if len(responsesReasoningItemRaws) == 0 {
				switch typed := part.Metadata["responsesReasoningItemRaws"].(type) {
				case []string:
					responsesReasoningItemRaws = append([]string(nil), typed...)
				case []interface{}:
					for _, item := range typed {
						if text, ok := item.(string); ok && strings.TrimSpace(text) != "" {
							responsesReasoningItemRaws = append(responsesReasoningItemRaws, strings.TrimSpace(text))
						}
					}
				}
			}
		}
		if part.Type == types.PartTypeText && strings.TrimSpace(part.Text) != "" {
			assistantText = append(assistantText, strings.TrimSpace(part.Text))
		}
		if part.Type == types.PartTypeTool && part.Tool != nil {
			toolParts = append(toolParts, part)
		}
	}
	reasoningContent := JoinedReasoningFromParts(parts)
	allToolCallIDs := true
	for _, part := range toolParts {
		if strings.TrimSpace(part.Tool.CallID) == "" || strings.TrimSpace(part.Tool.Name) == "" {
			allToolCallIDs = false
			break
		}
	}
	var nativeCalls []types.ChatHistoryNativeToolCall
	if allToolCallIDs {
		nativeCalls = make([]types.ChatHistoryNativeToolCall, 0, len(toolParts))
		for _, part := range toolParts {
			args := "{}"
			if j, err := FormatToolCallInputJSON(part); err == nil && strings.TrimSpace(j) != "" {
				args = strings.TrimSpace(j)
			}
			nativeCalls = append(nativeCalls, types.ChatHistoryNativeToolCall{
				ID:        strings.TrimSpace(part.Tool.CallID),
				Name:      strings.TrimSpace(part.Tool.Name),
				Arguments: args,
			})
		}
	}
	hasToolPart := len(toolParts) > 0
	if strings.TrimSpace(rawOutput) != "" || len(assistantText) > 0 || hasToolPart {
		content := strings.Join(assistantText, "\n")
		if content == "" && hasToolPart && !allToolCallIDs {
			content = "call_tool"
		}
		s.memory.History = append(s.memory.History, &types.ChatHistoryItem{
			Role:                       "assistant",
			Content:                    content,
			RawOutput:                  strings.TrimSpace(rawOutput),
			NativeToolCalls:            nativeCalls,
			SourceMessageID:            strings.TrimSpace(sourceMessageID),
			Phase:                      phase,
			ResponsesOutputMessageRaw:  responsesOutputMessageRaw,
			ResponsesReasoningItemRaws: append([]string(nil), responsesReasoningItemRaws...),
			ReasoningContent:           reasoningContent,
			ThinkingSignature:          AnthropicThinkingSignatureFromParts(parts),
			Created:                    time.Now().UnixMilli(),
		})
	}
	for _, part := range toolParts {
		entry := &types.ChatHistoryItem{
			ToolName: strings.TrimSpace(part.Tool.Name),
			Created:  time.Now().UnixMilli(),
		}
		if allToolCallIDs {
			entry.Role = "tool"
			entry.ToolCallID = strings.TrimSpace(part.Tool.CallID)
		} else {
			entry.Role = toolTranscriptRole(part.Tool.Name)
		}
		entry.ToolSystemMessage = strings.TrimSpace(part.Tool.State.SystemMessage)
		entry.ToolError = strings.TrimSpace(part.Tool.State.Error)
		entry.ToolStatus = strings.TrimSpace(part.Tool.State.Status)
		switch {
		case strings.TrimSpace(part.Tool.State.Output) != "":
			entry.Content = part.Tool.State.Output
		case strings.TrimSpace(part.Tool.State.Error) != "":
			entry.Content = strings.TrimSpace(part.Tool.State.Error)
		default:
			entry.Content = agentmemory.RenderToolResultContent(
				part.Tool.State.Output,
				part.Tool.State.Error,
				"",
				part.Tool.State.Status,
				"",
			)
		}
		s.memory.History = append(s.memory.History, entry)
	}
	s.memory.LatestToolCall = BuildLatestToolCall(s.memory.History)
}

func (s *ProcessV2MemoryState) ReplaceEntries(entries []*types.MemoryEntry) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.memory == nil {
		s.memory = &types.Memory{}
	}
	clonedEntries := CloneMemoryEntries(entries)
	s.memory.Entries = clonedEntries
	s.memory.History = MemoryEntriesToChatHistory(clonedEntries)
	s.memory.LatestToolCall = BuildLatestToolCall(s.memory.History)
}

func CloneProcessV2Memory(memory *types.Memory) *types.Memory {
	if memory == nil {
		return &types.Memory{}
	}
	cloned := *memory
	cloned.ProjectFilePrompt = append([]types.FilePrompt(nil), memory.ProjectFilePrompt...)
	cloned.SkillPrompts = append([]types.FilePrompt(nil), memory.SkillPrompts...)
	if len(memory.History) > 0 {
		cloned.History = make([]*types.ChatHistoryItem, 0, len(memory.History))
		for _, item := range memory.History {
			if item != nil {
				copied := *item
				if len(item.NativeToolCalls) > 0 {
					copied.NativeToolCalls = append([]types.ChatHistoryNativeToolCall(nil), item.NativeToolCalls...)
				}
				cloned.History = append(cloned.History, &copied)
			}
		}
	}
	if len(memory.Entries) > 0 {
		cloned.Entries = CloneMemoryEntries(memory.Entries)
	}
	if memory.LatestToolCall != nil {
		latest := *memory.LatestToolCall
		cloned.LatestToolCall = &latest
	}
	return &cloned
}

func BuildLatestToolCall(chatHistory []*types.ChatHistoryItem) *types.LatestToolCall {
	if len(chatHistory) == 0 {
		return nil
	}
	last := chatHistory[len(chatHistory)-1]
	if last == nil {
		return nil
	}
	role := strings.TrimSpace(last.Role)
	content := last.RenderContent()
	switch role {
	case "assistant":
		if len(last.NativeToolCalls) > 0 {
			toolName := strings.TrimSpace(last.NativeToolCalls[len(last.NativeToolCalls)-1].Name)
			instruction := "你刚刚申请了调用工具，工具已经执行成功。请基于工具结果继续选择你的动作。"
			if toolName != "" {
				instruction = "你刚刚申请了调用工具 " + toolName + "，工具已经执行成功。请基于工具结果继续选择你的动作。"
			}
			return &types.LatestToolCall{Role: role, ToolName: toolName, Instruction: instruction}
		}
		if content == "" {
			return nil
		}
		return &types.LatestToolCall{Role: role, Content: content, Instruction: "你刚刚输出了以下内容，现在继续。"}
	case "user":
		if content == "" {
			return nil
		}
		return &types.LatestToolCall{Role: role, Content: content, Instruction: "用户刚刚说了以下内容，需要你作答。"}
	case "tool":
		toolName := latestToolNameFromHistoryItem(last, chatHistory)
		instruction := "你刚刚申请了调用工具，工具已经执行成功。请基于工具结果继续选择你的动作。"
		if toolName != "" {
			instruction = "你刚刚申请了调用工具 " + toolName + "，工具已经执行成功。请基于工具结果继续选择你的动作。"
		}
		return &types.LatestToolCall{Role: role, ToolName: toolName, Content: content, Instruction: instruction}
	default:
		if role == "tool_call" || strings.HasPrefix(role, "call_tool") {
			toolName := latestToolNameFromHistoryItem(last, chatHistory)
			instruction := "你刚刚申请了调用工具，工具已经执行成功。请基于工具结果继续选择你的动作。"
			if toolName != "" {
				instruction = "你刚刚申请了调用工具 " + toolName + "，工具已经执行成功。请基于工具结果继续选择你的动作。"
			}
			return &types.LatestToolCall{Role: role, ToolName: toolName, Instruction: instruction}
		}
		if content == "" {
			return nil
		}
		return &types.LatestToolCall{Role: role, Content: content, Instruction: "以下是最后一条消息，请继续。"}
	}
}

func latestToolNameFromHistoryItem(item *types.ChatHistoryItem, chatHistory []*types.ChatHistoryItem) string {
	if item != nil {
		if value := strings.TrimSpace(item.ToolName); value != "" {
			return value
		}
		if len(item.NativeToolCalls) > 0 {
			return strings.TrimSpace(item.NativeToolCalls[len(item.NativeToolCalls)-1].Name)
		}
		if role := strings.TrimSpace(item.Role); strings.HasPrefix(role, "call_tool_") {
			return strings.TrimPrefix(role, "call_tool_")
		}
	}
	return latestToolName(chatHistory)
}

func latestToolName(chatHistory []*types.ChatHistoryItem) string {
	if len(chatHistory) < 2 {
		return ""
	}
	prev := chatHistory[len(chatHistory)-2]
	if prev == nil {
		return ""
	}
	if len(prev.NativeToolCalls) > 0 {
		return strings.TrimSpace(prev.NativeToolCalls[len(prev.NativeToolCalls)-1].Name)
	}
	content := prev.RenderContent()
	if content == "" {
		return ""
	}
	var flat struct {
		CallTool string          `json:"call_tool"`
		Params   json.RawMessage `json:"params"`
	}
	if err := json.Unmarshal([]byte(content), &flat); err != nil {
		return ""
	}
	if strings.TrimSpace(flat.CallTool) != "call_tool" {
		return ""
	}
	return toolNameFromCallToolParams(flat.Params)
}

func toolTranscriptRole(toolName string) string {
	name := strings.TrimSpace(toolName)
	if name == "" {
		return "call_tool"
	}
	return "call_tool_" + name
}

func toolNameFromCallToolParams(params json.RawMessage) string {
	if len(params) == 0 {
		return ""
	}
	var payload struct {
		ToolName  string `json:"tool_name"`
		ToolCalls []struct {
			ToolName string `json:"tool_name"`
		} `json:"tool_calls"`
	}
	if err := json.Unmarshal(params, &payload); err != nil {
		return ""
	}
	if n := strings.TrimSpace(payload.ToolName); n != "" {
		return n
	}
	if len(payload.ToolCalls) > 0 {
		return strings.TrimSpace(payload.ToolCalls[0].ToolName)
	}
	return ""
}
