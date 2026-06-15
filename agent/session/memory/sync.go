package sessionmemory

import (
	"encoding/json"
	"strings"
	"time"

	agenttoken "matrixops-agent/token"
	"matrixops-agent/types"
	agentmemory "matrixops.local/memory"
	"pkgs/db/storage"

	"gorm.io/gorm"
)

func BuildEntriesFromMessage(message *types.WithParts) ([]*types.MemoryEntry, error) {
	if message == nil || message.Info == nil {
		return nil, nil
	}

	role := string(message.Info.Role)
	baseSequence := message.Info.Time.Created * 1000
	if baseSequence == 0 {
		baseSequence = time.Now().UnixMilli() * 1000
	}

	entries := make([]*types.MemoryEntry, 0)
	sequenceOffset := int64(0)

	appendEntry := func(entry *types.MemoryEntry) {
		if entry == nil {
			return
		}
		entry.SessionID = message.Info.SessionID
		entry.SourceMessageID = message.Info.ID
		if entry.Sequence == 0 {
			entry.Sequence = baseSequence + sequenceOffset
		}
		if entry.Created == 0 {
			entry.Created = message.Info.Time.Created
		}
		entry.Updated = time.Now().UnixMilli()
		entry.TokenCount = EstimateMemoryEntryTokenCount(entry)
		entries = append(entries, entry)
		sequenceOffset++
	}

	for i, part := range message.Parts {
		if part == nil {
			continue
		}
		if part.Type == "text" && part.Text != "" {
			appendEntry(&types.MemoryEntry{
				SourcePartID:                   part.ID,
				EntryKind:                      "text",
				Role:                           role,
				Content:                        part.Text,
				RawOutput:                      RawOutputFromPart(part),
				Phase:                          PhaseFromPart(part),
				ResponsesOutputMessageRaw:      ResponsesOutputMessageRawFromPart(part),
				ResponsesReasoningItemRawsJSON: ResponsesReasoningItemRawsJSONFromPart(part),
				ReasoningContent:               ReasoningContentForTextMemoryPart(message.Parts, i, part),
				ThinkingSignature:            ThinkingSignatureForTextMemoryPart(message.Parts, i, part),
			})
			continue
		}
		if part.Type == "tool" && part.Tool != nil {
			requestJSON, err := FormatToolCallRequestForHistory(part)
			if err != nil {
				return nil, err
			}
			inputJSON, err := FormatToolCallInputJSON(part)
			if err != nil {
				return nil, err
			}
			metadataJSON, err := FormatToolCallMetadataJSON(part)
			if err != nil {
				return nil, err
			}
			entry := &types.MemoryEntry{
				SourcePartID:       part.ID,
				EntryKind:          "tool_call",
				Role:               role,
				Content:            requestJSON,
				RawOutput:          RawOutputFromPart(part),
				ToolCallID:         strings.TrimSpace(part.Tool.CallID),
				ToolName:           strings.TrimSpace(part.Tool.Name),
				ToolStatus:         strings.TrimSpace(part.Tool.State.Status),
				ToolReason:         strings.TrimSpace(part.Reason),
				ToolRequestRawJSON: requestJSON,
				ToolInputJSON:      inputJSON,
				ToolOutput:         strings.TrimSpace(part.Tool.State.Output),
				ToolSystemMessage:  strings.TrimSpace(part.Tool.State.SystemMessage),
				ToolError:          strings.TrimSpace(part.Tool.State.Error),
				ToolTitle:          strings.TrimSpace(part.Tool.State.Title),
				ToolMetadataJSON:   metadataJSON,
			}
			entry.CallToolInfo = BuildToolCallInfoText(entry)
			appendEntry(entry)
		}
	}
	return entries, nil
}

func BuildEntriesFromPart(messageInfo *types.MessageInfo, part *types.Part) ([]*types.MemoryEntry, error) {
	if messageInfo == nil || part == nil {
		return nil, nil
	}

	role := string(messageInfo.Role)
	baseSequence := partTimeCreated(part, messageInfo.Time.Created) * 1000
	if baseSequence == 0 {
		baseSequence = time.Now().UnixMilli() * 1000
	}

	appendEntry := func(entry *types.MemoryEntry) []*types.MemoryEntry {
		if entry == nil {
			return nil
		}
		entry.SessionID = messageInfo.SessionID
		entry.SourceMessageID = messageInfo.ID
		entry.SourcePartID = part.ID
		if entry.Sequence == 0 {
			entry.Sequence = baseSequence
		}
		if entry.Created == 0 {
			entry.Created = partTimeCreated(part, messageInfo.Time.Created)
		}
		entry.Updated = time.Now().UnixMilli()
		entry.TokenCount = EstimateMemoryEntryTokenCount(entry)
		return []*types.MemoryEntry{entry}
	}

	if part.Type == "text" && part.Text != "" {
		return appendEntry(&types.MemoryEntry{
			EntryKind:                      "text",
			Role:                           role,
			Content:                        part.Text,
			RawOutput:                      RawOutputFromPart(part),
			Phase:                          PhaseFromPart(part),
			ResponsesOutputMessageRaw:      ResponsesOutputMessageRawFromPart(part),
			ResponsesReasoningItemRawsJSON: ResponsesReasoningItemRawsJSONFromPart(part),
			ReasoningContent:               ReasoningContentFromPart(part),
			ThinkingSignature:              anthropicThinkingSignatureFromPart(part),
		}), nil
	}

	if part.Type == "tool" && part.Tool != nil {
		requestJSON, err := FormatToolCallRequestForHistory(part)
		if err != nil {
			return nil, err
		}
		inputJSON, err := FormatToolCallInputJSON(part)
		if err != nil {
			return nil, err
		}
		metadataJSON, err := FormatToolCallMetadataJSON(part)
		if err != nil {
			return nil, err
		}
		entry := &types.MemoryEntry{
			EntryKind:          "tool_call",
			Role:               role,
			Content:            requestJSON,
			RawOutput:          RawOutputFromPart(part),
			ToolCallID:         strings.TrimSpace(part.Tool.CallID),
			ToolName:           strings.TrimSpace(part.Tool.Name),
			ToolStatus:         strings.TrimSpace(part.Tool.State.Status),
			ToolReason:         strings.TrimSpace(part.Reason),
			ToolRequestRawJSON: requestJSON,
			ToolInputJSON:      inputJSON,
			ToolOutput:         strings.TrimSpace(part.Tool.State.Output),
			ToolSystemMessage:  strings.TrimSpace(part.Tool.State.SystemMessage),
			ToolError:          strings.TrimSpace(part.Tool.State.Error),
			ToolTitle:          strings.TrimSpace(part.Tool.State.Title),
			ToolMetadataJSON:   metadataJSON,
		}
		entry.CallToolInfo = BuildToolCallInfoText(entry)
		return appendEntry(entry), nil
	}

	return nil, nil
}

func SyncPartMemory(db *gorm.DB, messageInfo *types.MessageInfo, part *types.Part) error {
	if messageInfo == nil || part == nil {
		return nil
	}
	entries, err := BuildEntriesFromPart(messageInfo, part)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		if sig := anthropicThinkingSignatureFromPart(part); sig != "" {
			return storage.PatchThinkingSignatureForAssistantMessage(db, messageInfo.SessionID, messageInfo.ID, sig)
		}
		return nil
	}
	// Per-part 落库时 BuildEntriesFromPart 只看当前 part，独立 reasoning part 上的正文不会进条目。
	// 这里按整条消息的 part 顺序补齐 ReasoningContent，与 BuildEntriesFromMessage 一致。
	if len(entries) > 0 {
		msg, loadErr := storage.GetMessageWithPartsLight(db, messageInfo.ID)
		if loadErr == nil && msg != nil && len(msg.Parts) > 0 {
			partsForReasoning := make([]*types.Part, len(msg.Parts))
			copy(partsForReasoning, msg.Parts)
			idx := -1
			for i, p := range partsForReasoning {
				if p != nil && p.ID == part.ID {
					partsForReasoning[i] = part
					idx = i
					break
				}
			}
			if idx >= 0 {
				for _, e := range entries {
					if e == nil {
						continue
					}
					switch strings.TrimSpace(e.EntryKind) {
					case "text":
						e.ReasoningContent = ReasoningContentForTextMemoryPart(partsForReasoning, idx, part)
						e.ThinkingSignature = ThinkingSignatureForTextMemoryPart(partsForReasoning, idx, part)
					case "tool_call":
						if prefix := reasoningContentPrefixBeforeIndex(partsForReasoning, idx); prefix != "" {
							e.ReasoningContent = prefix
						}
						if sig := thinkingSignaturePrefixBeforeIndex(partsForReasoning, idx); sig != "" {
							e.ThinkingSignature = sig
						}
					}
				}
			}
		}
	}
	return storage.ReplaceMemoryEntriesForPart(db, messageInfo.SessionID, messageInfo.ID, part.ID, entries)
}

func nativeToolArgumentsJSONFromEntry(entry *types.MemoryEntry) string {
	if entry == nil {
		return "{}"
	}
	if j := strings.TrimSpace(entry.ToolInputJSON); j != "" {
		return j
	}
	raw := strings.TrimSpace(entry.ToolRequestRawJSON)
	if raw == "" {
		return "{}"
	}
	var wrapper struct {
		Params json.RawMessage `json:"params"`
	}
	if err := json.Unmarshal([]byte(raw), &wrapper); err == nil && len(wrapper.Params) > 0 {
		return string(wrapper.Params)
	}
	return raw
}

func isConsecutiveNativeToolCallEntry(entry *types.MemoryEntry) bool {
	if entry == nil {
		return false
	}
	return strings.TrimSpace(entry.EntryKind) == "tool_call" &&
		strings.TrimSpace(entry.ToolCallID) != "" &&
		entry.HasToolCall()
}

func MemoryEntriesToChatHistory(entries []*types.MemoryEntry) []*types.ChatHistoryItem {
	history := make([]*types.ChatHistoryItem, 0, len(entries))
	for i := 0; i < len(entries); i++ {
		entry := entries[i]
		if entry == nil {
			continue
		}
		if isConsecutiveNativeToolCallEntry(entry) {
			batch := []*types.MemoryEntry{entry}
			j := i + 1
			for j < len(entries) && isConsecutiveNativeToolCallEntry(entries[j]) &&
				agentmemory.ShouldBatchNativeToolCallEntries(batch[0], entries[j]) {
				batch = append(batch, entries[j])
				j++
			}
			calls := make([]types.ChatHistoryNativeToolCall, 0, len(batch))
			for _, t := range batch {
				calls = append(calls, types.ChatHistoryNativeToolCall{
					ID:        strings.TrimSpace(t.ToolCallID),
					Name:      strings.TrimSpace(t.ToolName),
					Arguments: nativeToolArgumentsJSONFromEntry(t),
				})
			}
			first := batch[0]
			merged := &types.ChatHistoryItem{
				Role:                       "assistant",
				NativeToolCalls:            calls,
				SourceMessageID:            strings.TrimSpace(first.SourceMessageID),
				Synthetic:                  first.Synthetic,
				Phase:                      strings.TrimSpace(first.Phase),
				ResponsesOutputMessageRaw:  strings.TrimSpace(first.ResponsesOutputMessageRaw),
				ResponsesReasoningItemRaws: ParseResponsesReasoningItemRawsJSON(first.ResponsesReasoningItemRawsJSON),
				ReasoningContent:           strings.TrimSpace(first.ReasoningContent),
				ThinkingSignature:          strings.TrimSpace(first.ThinkingSignature),
				Created:                    first.Created,
			}
			if n := len(history); n > 0 {
				prev := history[n-1]
				prevEntry := (*types.MemoryEntry)(nil)
				if i > 0 {
					prevEntry = entries[i-1]
				}
				if prev != nil && strings.TrimSpace(prev.Role) == "assistant" && len(prev.NativeToolCalls) == 0 &&
					agentmemory.ShouldMergeAssistantTextWithNativeToolBatch(prevEntry, batch) {
					prevContent := strings.TrimSpace(prev.Content)
					if prevContent != "" {
						merged.Content = prevContent
					}
					if p := strings.TrimSpace(prev.ReasoningContent); p != "" {
						merged.ReasoningContent = p
					}
					if p := strings.TrimSpace(prev.ThinkingSignature); p != "" {
						merged.ThinkingSignature = p
					}
					if merged.Phase == "" {
						merged.Phase = strings.TrimSpace(prev.Phase)
					}
					if merged.ResponsesOutputMessageRaw == "" {
						merged.ResponsesOutputMessageRaw = strings.TrimSpace(prev.ResponsesOutputMessageRaw)
					}
					if len(merged.ResponsesReasoningItemRaws) == 0 && len(prev.ResponsesReasoningItemRaws) > 0 {
						merged.ResponsesReasoningItemRaws = append([]string(nil), prev.ResponsesReasoningItemRaws...)
					}
					if strings.TrimSpace(merged.RawOutput) == "" && strings.TrimSpace(prev.RawOutput) != "" {
						merged.RawOutput = strings.TrimSpace(prev.RawOutput)
					}
					history = history[:n-1]
				}
			}
			history = append(history, merged)
			for _, t := range batch {
				body := strings.TrimSpace(t.ToolOutput)
				if body == "" {
					body = strings.TrimSpace(agentmemory.RenderToolResultContent(t.ToolOutput, t.ToolError, "", t.ToolStatus, t.ToolTitle))
				}
				history = append(history, &types.ChatHistoryItem{
					Role:              "tool",
					ToolCallID:        strings.TrimSpace(t.ToolCallID),
					ToolName:          strings.TrimSpace(t.ToolName),
					Content:           body,
					ToolSystemMessage: strings.TrimSpace(t.ToolSystemMessage),
					ToolError:         strings.TrimSpace(t.ToolError),
					ToolStatus:        strings.TrimSpace(t.ToolStatus),
					Created:           t.Created,
				})
			}
			i = j - 1
			continue
		}

		content := entry.Content
		callToolInfo := entry.CallToolInfo
		if entry.HasToolCall() {
			if requestJSON := strings.TrimSpace(entry.ToolRequestRawJSON); requestJSON != "" {
				content = requestJSON
			}
			if strings.TrimSpace(callToolInfo) == "" {
				callToolInfo = BuildToolCallInfoText(entry)
			}
		}
		history = append(history, &types.ChatHistoryItem{
			Role:                       entry.Role,
			Content:                    content,
			RawOutput:                  entry.RawOutput,
			CallToolInfo:               callToolInfo,
			ToolName:                   entry.ToolName,
			ToolCallID:                 strings.TrimSpace(entry.ToolCallID),
			SourceMessageID:            strings.TrimSpace(entry.SourceMessageID),
			Synthetic:                  entry.Synthetic,
			Phase:                      strings.TrimSpace(entry.Phase),
			ResponsesOutputMessageRaw:  strings.TrimSpace(entry.ResponsesOutputMessageRaw),
			ResponsesReasoningItemRaws: ParseResponsesReasoningItemRawsJSON(entry.ResponsesReasoningItemRawsJSON),
			ReasoningContent:           strings.TrimSpace(entry.ReasoningContent),
			ThinkingSignature:          strings.TrimSpace(entry.ThinkingSignature),
			Created:                    entry.Created,
		})
	}
	return history
}

func TotalMemoryTokens(entries []*types.MemoryEntry) int {
	total := 0
	for _, entry := range entries {
		if entry == nil {
			continue
		}
		total += entry.TokenCount
	}
	return total
}

func EstimateMemoryEntryTokenCount(entry *types.MemoryEntry) int {
	if entry == nil {
		return 0
	}
	clone := *entry
	clone.TokenCount = 0
	rendered := clone.SerializeText()
	if strings.TrimSpace(rendered) == "" {
		rendered = clone.RenderContent()
	}
	return agenttoken.Estimate(rendered)
}

func BuildToolCallInfoText(entry *types.MemoryEntry) string {
	if entry == nil {
		return ""
	}
	lines := make([]string, 0, 8)
	if value := strings.TrimSpace(entry.ToolName); value != "" {
		lines = append(lines, "tool_name: "+value)
	}
	if value := strings.TrimSpace(entry.ToolCallID); value != "" {
		lines = append(lines, "tool_call_id: "+value)
	}
	if value := strings.TrimSpace(entry.ToolStatus); value != "" {
		lines = append(lines, "status: "+value)
	}
	if value := strings.TrimSpace(entry.ToolReason); value != "" {
		lines = append(lines, "reason: "+value)
	}
	if value := strings.TrimSpace(entry.ToolInputJSON); value != "" {
		lines = append(lines, "tool_input_json:\n"+value)
	}
	if value := strings.TrimSpace(entry.ToolOutput); value != "" {
		lines = append(lines, "tool_output:\n"+value)
	}
	if value := strings.TrimSpace(entry.ToolError); value != "" {
		lines = append(lines, "tool_error:\n"+value)
	}
	if value := strings.TrimSpace(entry.ToolMetadataJSON); value != "" {
		lines = append(lines, "tool_metadata_json:\n"+value)
	}
	return strings.Join(lines, "\n")
}

func RawOutputFromPart(part *types.Part) string {
	if part == nil || part.Metadata == nil {
		return ""
	}
	rawOutput, _ := part.Metadata["rawOutput"].(string)
	return strings.TrimSpace(rawOutput)
}

func PhaseFromPart(part *types.Part) string {
	if part == nil || part.Metadata == nil {
		return ""
	}
	value, _ := part.Metadata["phase"].(string)
	return strings.TrimSpace(value)
}

func ResponsesOutputMessageRawFromPart(part *types.Part) string {
	if part == nil || part.Metadata == nil {
		return ""
	}
	value, _ := part.Metadata["responsesOutputMessageRaw"].(string)
	return strings.TrimSpace(value)
}

// partReasoningBody returns streaming reasoning payload for reasoning / reasoning-delta parts:
// prefer part.Reasoning, else part.Text (legacy / alternate encodings).
func partReasoningBody(part *types.Part) string {
	if part == nil {
		return ""
	}
	if part.Reasoning != "" {
		return part.Reasoning
	}
	return part.Text
}

// JoinedReasoningFromParts concatenates reasoning bodies from reasoning / reasoning-delta parts in order.
func JoinedReasoningFromParts(parts []*types.Part) string {
	var b strings.Builder
	for _, part := range parts {
		if part == nil {
			continue
		}
		if part.Type == types.PartTypeReasoning || part.Type == types.PartTypeReasoningDelta {
			b.WriteString(partReasoningBody(part))
		}
	}
	return strings.TrimSpace(b.String())
}

func reasoningContentPrefixBeforeIndex(parts []*types.Part, idx int) string {
	if idx <= 0 {
		return ""
	}
	var b strings.Builder
	for j := 0; j < idx && j < len(parts); j++ {
		part := parts[j]
		if part == nil {
			continue
		}
		if part.Type == types.PartTypeReasoning || part.Type == types.PartTypeReasoningDelta {
			b.WriteString(partReasoningBody(part))
		}
	}
	return strings.TrimSpace(b.String())
}

// ReasoningContentForTextMemoryPart resolves reasoning for a text memory row: part.Reasoning on that part,
// else reasoning parts strictly before this index in the message.
func ReasoningContentForTextMemoryPart(parts []*types.Part, idx int, part *types.Part) string {
	if s := ReasoningContentFromPart(part); s != "" {
		return s
	}
	return reasoningContentPrefixBeforeIndex(parts, idx)
}

func ReasoningContentFromPart(part *types.Part) string {
	if part == nil {
		return ""
	}
	if part.Type == types.PartTypeReasoning || part.Type == types.PartTypeReasoningDelta {
		return strings.TrimSpace(partReasoningBody(part))
	}
	if s := strings.TrimSpace(part.Reasoning); s != "" {
		return s
	}
	return ""
}

func anthropicThinkingSignatureFromPart(part *types.Part) string {
	if part == nil || part.Metadata == nil {
		return ""
	}
	v, _ := part.Metadata["anthropicThinkingSignature"].(string)
	return strings.TrimSpace(v)
}

// ThinkingSignatureForTextMemoryPart 将 Anthropic thinking 签名挂到正文 memory 行（与 ReasoningContent 对齐）。
func ThinkingSignatureForTextMemoryPart(parts []*types.Part, idx int, part *types.Part) string {
	if s := anthropicThinkingSignatureFromPart(part); s != "" {
		return s
	}
	return thinkingSignaturePrefixBeforeIndex(parts, idx)
}

func thinkingSignaturePrefixBeforeIndex(parts []*types.Part, idx int) string {
	if idx <= 0 {
		return ""
	}
	for j := idx - 1; j >= 0; j-- {
		part := parts[j]
		if part == nil {
			continue
		}
		if part.Type == types.PartTypeReasoning || part.Type == types.PartTypeReasoningDelta {
			if s := anthropicThinkingSignatureFromPart(part); s != "" {
				return s
			}
		}
	}
	return ""
}

// AnthropicThinkingSignatureFromParts 从本轮 RecordAction 的 parts 中提取 Anthropic thinking 签名（取最后一个非空）。
func AnthropicThinkingSignatureFromParts(parts []*types.Part) string {
	if len(parts) == 0 {
		return ""
	}
	var last string
	for _, part := range parts {
		if s := anthropicThinkingSignatureFromPart(part); s != "" {
			last = s
		}
	}
	return last
}

func ResponsesReasoningItemRawsJSONFromPart(part *types.Part) string {
	if part == nil || part.Metadata == nil {
		return ""
	}
	values := parseResponsesReasoningItemRawsAny(part.Metadata["responsesReasoningItemRaws"])
	if len(values) == 0 {
		return ""
	}
	encoded, err := json.Marshal(values)
	if err != nil {
		return ""
	}
	return string(encoded)
}

func ParseResponsesReasoningItemRawsJSON(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var values []string
	if err := json.Unmarshal([]byte(raw), &values); err == nil {
		out := make([]string, 0, len(values))
		for _, value := range values {
			value = strings.TrimSpace(value)
			if value != "" {
				out = append(out, value)
			}
		}
		return out
	}
	return nil
}

func parseResponsesReasoningItemRawsAny(value interface{}) []string {
	switch typed := value.(type) {
	case nil:
		return nil
	case []string:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			item = strings.TrimSpace(item)
			if item != "" {
				out = append(out, item)
			}
		}
		return out
	case []interface{}:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if text, ok := item.(string); ok {
				text = strings.TrimSpace(text)
				if text != "" {
					out = append(out, text)
				}
			}
		}
		return out
	default:
		return nil
	}
}

func FormatToolCallRequestForHistory(part *types.Part) (string, error) {
	if part == nil || part.Tool == nil {
		return "", nil
	}
	if raw := strings.TrimSpace(part.Tool.State.Raw); raw != "" {
		return raw, nil
	}
	return "", nil
}

func FormatToolCallInputJSON(part *types.Part) (string, error) {
	if part == nil || part.Tool == nil || part.Tool.State.Input == nil {
		return "", nil
	}
	return marshalStructuredJSON(part.Tool.State.Input)
}

func FormatToolCallMetadataJSON(part *types.Part) (string, error) {
	if part == nil || part.Tool == nil {
		return "", nil
	}
	payload := map[string]interface{}{}
	for key, value := range part.Tool.State.MemoryMetadata {
		payload[key] = value
	}
	if strings.TrimSpace(part.Tool.State.FullOutput) != "" {
		payload["fullOutput"] = part.Tool.State.FullOutput
	}
	if len(payload) == 0 {
		return "", nil
	}
	return marshalStructuredJSON(payload)
}

func partTimeCreated(part *types.Part, fallback int64) int64 {
	if part != nil && part.Time != nil && part.Time.Created != 0 {
		return part.Time.Created
	}
	return fallback
}

func marshalStructuredJSON(value interface{}) (string, error) {
	if value == nil {
		return "", nil
	}
	bytes, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}
