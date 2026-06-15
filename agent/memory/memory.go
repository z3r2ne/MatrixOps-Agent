package memory

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type Memory struct {
	GlobalPrompt          string             `json:"globalPrompt,omitempty"`
	OccupationPrompt      string             `json:"occupationPrompt,omitempty"`
	ProjectPrompt         string             `json:"projectPrompt,omitempty"`
	ModelPrompt           string             `json:"modelPrompt,omitempty"`
	WorkerPrompt          string             `json:"workerPrompt,omitempty"`
	SessionGuidancePrompt string             `json:"sessionGuidancePrompt,omitempty"`
	OutputStylePrompt     string             `json:"outputStylePrompt,omitempty"`
	ToolPriorityPrompt    string             `json:"toolPriorityPrompt,omitempty"`
	ProjectFilePrompt     []FilePrompt       `json:"projectFilePrompt,omitempty"`
	InstalledSkillsPrompt string             `json:"installedSkillsPrompt,omitempty"`
	SkillPrompts          []FilePrompt       `json:"skillPrompts,omitempty"`
	EnvPrompt             string             `json:"envPrompt,omitempty"`
	Entries               []*MemoryEntry     `json:"entries,omitempty"`
	History               []*ChatHistoryItem `json:"history,omitempty"`
	LatestToolCall        *LatestToolCall    `json:"latestToolCall,omitempty"`
}

type MemoryEntry struct {
	ID                             uint   `json:"id"`
	SessionID                      string `json:"sessionID"`
	SourceMessageID                string `json:"sourceMessageID,omitempty"`
	SourcePartID                   string `json:"sourcePartID,omitempty"`
	EntryKind                      string `json:"entryKind"`
	Role                           string `json:"role"`
	Content                        string `json:"content,omitempty"`
	RawOutput                      string `json:"rawOutput,omitempty"`
	Phase                          string `json:"phase,omitempty"`
	ResponsesOutputMessageRaw      string `json:"responsesOutputMessageRaw,omitempty"`
	ResponsesReasoningItemRawsJSON string `json:"responsesReasoningItemRawsJSON,omitempty"`
	ReasoningContent               string `json:"reasoningContent,omitempty"`
	ThinkingSignature              string `json:"thinkingSignature,omitempty"`
	CallToolInfo                   string `json:"callToolInfo,omitempty"`
	ToolCallID                     string `json:"toolCallID,omitempty"`
	ToolName                       string `json:"toolName,omitempty"`
	ToolStatus                     string `json:"toolStatus,omitempty"`
	ToolReason                     string `json:"toolReason,omitempty"`
	ToolRequestRawJSON             string `json:"toolRequestRawJSON,omitempty"`
	ToolInputJSON                  string `json:"toolInputJSON,omitempty"`
	ToolOutput                     string `json:"toolOutput,omitempty"`
	ToolSystemMessage              string `json:"toolSystemMessage,omitempty"`
	ToolError                      string `json:"toolError,omitempty"`
	ToolTitle                      string `json:"toolTitle,omitempty"`
	ToolMetadataJSON               string `json:"toolMetadataJSON,omitempty"`
	Synthetic                      bool   `json:"synthetic,omitempty"`
	SearchArchived                 bool   `json:"searchArchived,omitempty"`
	CompressionLevel               int    `json:"compressionLevel,omitempty"`
	Sequence                       int64  `json:"sequence"`
	TokenCount                     int    `json:"tokenCount"`
	Created                        int64  `json:"created"`
	Updated                        int64  `json:"updated"`
}

func (e *MemoryEntry) RenderContent() string {
	if e == nil {
		return ""
	}

	if output := strings.TrimSpace(e.ToolOutput); output != "" {
		return output
	}

	if errText := strings.TrimSpace(e.ToolError); errText != "" {
		return errText
	}

	if rawOutput := strings.TrimSpace(e.RawOutput); rawOutput != "" {
		return rawOutput
	}

	if content := strings.TrimSpace(e.Content); content != "" {
		return content
	}

	if requestRaw := strings.TrimSpace(e.ToolRequestRawJSON); requestRaw != "" {
		return requestRaw
	}

	if inputJSON := strings.TrimSpace(e.ToolInputJSON); inputJSON != "" {
		return inputJSON
	}

	if callToolInfo := strings.TrimSpace(e.CallToolInfo); callToolInfo != "" {
		return callToolInfo
	}

	return ""
}

func (e *MemoryEntry) HasToolCall() bool {
	if e == nil {
		return false
	}

	return strings.TrimSpace(e.ToolCallID) != "" ||
		strings.TrimSpace(e.ToolName) != "" ||
		strings.TrimSpace(e.ToolStatus) != "" ||
		strings.TrimSpace(e.ToolRequestRawJSON) != "" ||
		strings.TrimSpace(e.ToolInputJSON) != "" ||
		strings.TrimSpace(e.ToolOutput) != "" ||
		strings.TrimSpace(e.ToolError) != ""
}

// ShouldMergeAssistantTextWithNativeToolBatch reports whether a text assistant entry
// should be coalesced into the following native tool_call batch when rebuilding chat history.
func ShouldMergeAssistantTextWithNativeToolBatch(textEntry *MemoryEntry, toolBatch []*MemoryEntry) bool {
	if textEntry == nil || len(toolBatch) == 0 || toolBatch[0] == nil {
		return false
	}
	if textEntry.Synthetic {
		return false
	}
	switch strings.TrimSpace(textEntry.EntryKind) {
	case "summary_assistant", "summary_user":
		return false
	}
	textMessageID := strings.TrimSpace(textEntry.SourceMessageID)
	toolMessageID := strings.TrimSpace(toolBatch[0].SourceMessageID)
	if textMessageID != "" && toolMessageID != "" {
		return textMessageID == toolMessageID
	}
	if textMessageID != "" || toolMessageID != "" {
		return false
	}
	return strings.TrimSpace(textEntry.EntryKind) == "text"
}

// ShouldBatchNativeToolCallEntries reports whether next belongs in the same native
// tool_call batch as first when rebuilding chat history. Entries from different
// assistant messages (different SourceMessageID) must stay separate even when stored
// as consecutive tool_call rows.
func ShouldBatchNativeToolCallEntries(first, next *MemoryEntry) bool {
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

func (e *MemoryEntry) SerializeMap() map[string]interface{} {
	if e == nil {
		return map[string]interface{}{}
	}

	payload := map[string]interface{}{
		"role":       strings.TrimSpace(e.Role),
		"entry_kind": strings.TrimSpace(e.EntryKind),
	}

	if value := strings.TrimSpace(e.Content); value != "" {
		payload["content"] = value
	}
	if value := strings.TrimSpace(e.RawOutput); value != "" {
		payload["raw_output"] = value
	}
	if value := strings.TrimSpace(e.Phase); value != "" {
		payload["phase"] = value
	}
	if value := strings.TrimSpace(e.ResponsesOutputMessageRaw); value != "" {
		payload["responses_output_message_raw"] = value
	}
	if value := strings.TrimSpace(e.ResponsesReasoningItemRawsJSON); value != "" {
		payload["responses_reasoning_item_raws_json"] = value
	}
	if value := strings.TrimSpace(e.ReasoningContent); value != "" {
		payload["reasoning_content"] = value
	}
	if value := strings.TrimSpace(e.CallToolInfo); value != "" {
		payload["call_tool_info"] = value
	}
	if value := strings.TrimSpace(e.SourceMessageID); value != "" {
		payload["source_message_id"] = value
	}
	if value := strings.TrimSpace(e.SourcePartID); value != "" {
		payload["source_part_id"] = value
	}
	if e.Sequence > 0 {
		payload["sequence"] = e.Sequence
	}
	if e.TokenCount > 0 {
		payload["token_count"] = e.TokenCount
	}
	if e.Created > 0 {
		payload["created"] = e.Created
	}
	if e.Updated > 0 {
		payload["updated"] = e.Updated
	}
	if e.Synthetic {
		payload["synthetic"] = true
	}
	if e.SearchArchived {
		payload["search_archived"] = true
	}

	if e.HasToolCall() {
		toolCall := map[string]interface{}{}
		if value := strings.TrimSpace(e.ToolCallID); value != "" {
			toolCall["id"] = value
		}
		if value := strings.TrimSpace(e.ToolName); value != "" {
			toolCall["name"] = value
		}
		if value := strings.TrimSpace(e.ToolStatus); value != "" {
			toolCall["status"] = value
		}
		if value := strings.TrimSpace(e.ToolReason); value != "" {
			toolCall["reason"] = value
		}
		if value := strings.TrimSpace(e.ToolRequestRawJSON); value != "" {
			toolCall["request_raw_json"] = value
		}
		if value := strings.TrimSpace(e.ToolInputJSON); value != "" {
			toolCall["input_json"] = value
		}
		if value := strings.TrimSpace(e.ToolOutput); value != "" {
			toolCall["output"] = value
		}
		if value := strings.TrimSpace(e.ToolError); value != "" {
			toolCall["error"] = value
		}
		if value := strings.TrimSpace(e.ToolTitle); value != "" {
			toolCall["title"] = value
		}
		if value := strings.TrimSpace(e.ToolMetadataJSON); value != "" {
			toolCall["metadata_json"] = value
		}
		payload["tool_call"] = toolCall
	}

	return payload
}

func (e *MemoryEntry) SerializeText() string {
	if e == nil {
		return ""
	}

	return renderMemoryTranscript(memoryTranscriptItemsFromEntry(e))
}

const SerializeMemoryEntriesJSONMaxLen = 200000

// SerializeMemoryEntriesJSON 将会话 memory entries 序列化为 JSON，供记忆压缩日志使用。
func SerializeMemoryEntriesJSON(entries []*MemoryEntry) string {
	if len(entries) == 0 {
		return "[]"
	}
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		fallback, marshalErr := json.Marshal(map[string]string{"error": err.Error()})
		if marshalErr != nil {
			return `{"error":"marshal memory entries failed"}`
		}
		return string(fallback)
	}
	text := string(data)
	if len(text) > SerializeMemoryEntriesJSONMaxLen {
		return text[:SerializeMemoryEntriesJSONMaxLen] + "\n... [truncated]"
	}
	return text
}

// TranscriptSourceEntries 返回与 SerializeEntries、MsgIDsToEntryIndexes 一致的条目来源：
// 优先持久化的 Entries；若为空则用 History 推导（与 Prompt 里 <memory> 的 MsgID 编号一致）。
func (m *Memory) TranscriptSourceEntries() []*MemoryEntry {
	if m == nil {
		return nil
	}
	if len(m.Entries) > 0 {
		return m.Entries
	}
	if len(m.History) > 0 {
		return memoryEntriesFromChatHistoryForTranscript(m.History)
	}
	return nil
}

func (m *Memory) SerializeEntries() string {
	if m == nil {
		return "[]"
	}

	entries := m.TranscriptSourceEntries()
	if len(entries) == 0 {
		return "[]"
	}

	items := memoryTranscriptItemsFromEntries(entries)

	rendered := renderMemoryTranscript(items)
	if rendered == "" {
		return "[]"
	}

	return rendered
}

func collectFollowingToolRows(history []*ChatHistoryItem, start int, max int) []*ChatHistoryItem {
	if max <= 0 || start >= len(history) {
		return nil
	}
	out := make([]*ChatHistoryItem, 0, max)
	for i := start; i < len(history) && len(out) < max; i++ {
		it := history[i]
		if it == nil || strings.TrimSpace(it.Role) != "tool" {
			return out
		}
		out = append(out, it)
	}
	return out
}

func memoryEntriesFromChatHistoryForTranscript(history []*ChatHistoryItem) []*MemoryEntry {
	out := make([]*MemoryEntry, 0, len(history))
	i := 0
	for i < len(history) {
		item := history[i]
		if item == nil {
			i++
			continue
		}
		role := strings.TrimSpace(item.Role)

		if role == "assistant" && len(item.NativeToolCalls) > 0 {
			if text := strings.TrimSpace(item.Content); text != "" && text != "call_tool" {
				out = append(out, &MemoryEntry{
					EntryKind:                      "text",
					Role:                           "assistant",
					Content:                        text,
					RawOutput:                      strings.TrimSpace(item.RawOutput),
					Phase:                          strings.TrimSpace(item.Phase),
					ResponsesOutputMessageRaw:      strings.TrimSpace(item.ResponsesOutputMessageRaw),
					ResponsesReasoningItemRawsJSON: mustMarshalStringSlice(item.ResponsesReasoningItemRaws),
					ReasoningContent:               strings.TrimSpace(item.ReasoningContent),
					Created:                        item.Created,
				})
			} else if ro := strings.TrimSpace(item.RawOutput); ro != "" {
				out = append(out, &MemoryEntry{
					EntryKind:                      "text",
					Role:                           "assistant",
					RawOutput:                      ro,
					Phase:                          strings.TrimSpace(item.Phase),
					ResponsesOutputMessageRaw:      strings.TrimSpace(item.ResponsesOutputMessageRaw),
					ResponsesReasoningItemRawsJSON: mustMarshalStringSlice(item.ResponsesReasoningItemRaws),
					ReasoningContent:               strings.TrimSpace(item.ReasoningContent),
					Created:                        item.Created,
				})
			}

			toolRows := collectFollowingToolRows(history, i+1, len(item.NativeToolCalls))
			for j, c := range item.NativeToolCalls {
				entry := &MemoryEntry{
					EntryKind:                      "tool_call",
					Role:                           "assistant",
					ToolCallID:                     strings.TrimSpace(c.ID),
					ToolName:                       strings.TrimSpace(c.Name),
					ToolInputJSON:                  strings.TrimSpace(c.Arguments),
					ToolRequestRawJSON:             strings.TrimSpace(c.Arguments),
					Phase:                          strings.TrimSpace(item.Phase),
					ResponsesOutputMessageRaw:      strings.TrimSpace(item.ResponsesOutputMessageRaw),
					ResponsesReasoningItemRawsJSON: mustMarshalStringSlice(item.ResponsesReasoningItemRaws),
					ReasoningContent:               strings.TrimSpace(item.ReasoningContent),
					Created:                        item.Created,
				}
				if j < len(toolRows) {
					tr := toolRows[j]
					if tr != nil {
						if id := strings.TrimSpace(tr.ToolCallID); id == "" || id == entry.ToolCallID {
							entry.ToolOutput = strings.TrimSpace(tr.Content)
							entry.ToolSystemMessage = strings.TrimSpace(tr.ToolSystemMessage)
							entry.ToolError = strings.TrimSpace(tr.ToolError)
							entry.ToolStatus = strings.TrimSpace(tr.ToolStatus)
						}
					}
				}
				out = append(out, entry)
			}
			i += 1 + len(toolRows)
			continue
		}

		if role == "tool" {
			out = append(out, &MemoryEntry{
				EntryKind:         "tool_call",
				Role:              "assistant",
				ToolCallID:        strings.TrimSpace(item.ToolCallID),
				ToolName:          strings.TrimSpace(item.ToolName),
				ToolOutput:        strings.TrimSpace(item.Content),
				ToolSystemMessage: strings.TrimSpace(item.ToolSystemMessage),
				ToolError:         strings.TrimSpace(item.ToolError),
				ToolStatus:        strings.TrimSpace(item.ToolStatus),
				Created:           item.Created,
			})
			i++
			continue
		}

		if strings.HasPrefix(role, "call_tool_") {
			toolName := strings.TrimPrefix(role, "call_tool_")
			out = append(out, &MemoryEntry{
				EntryKind:  "tool_call",
				Role:       "assistant",
				ToolName:   toolName,
				ToolOutput: strings.TrimSpace(item.Content),
				Created:    item.Created,
			})
			i++
			continue
		}

		raw := strings.TrimSpace(item.RawOutput)
		if raw == "" {
			raw = strings.TrimSpace(item.Content)
		}
		out = append(out, &MemoryEntry{
			EntryKind:                      "text",
			Role:                           role,
			RawOutput:                      raw,
			Content:                        strings.TrimSpace(item.Content),
			Phase:                          strings.TrimSpace(item.Phase),
			ResponsesOutputMessageRaw:      strings.TrimSpace(item.ResponsesOutputMessageRaw),
			ResponsesReasoningItemRawsJSON: mustMarshalStringSlice(item.ResponsesReasoningItemRaws),
			ReasoningContent:               strings.TrimSpace(item.ReasoningContent),
			Created:                        item.Created,
		})
		i++
	}
	return out
}

func mustMarshalStringSlice(values []string) string {
	if len(values) == 0 {
		return ""
	}
	encoded, err := json.Marshal(values)
	if err != nil {
		return ""
	}
	return string(encoded)
}

type memoryTranscriptItem struct {
	Created    int64
	MsgID      int64
	EntryIndex int // index into Entries slice
	Bytes      int
	Role       string
	Content    string
}

func appendTranscriptLine(items *[]memoryTranscriptItem, nextMsgID *int64, entryIndex int, created int64, bytes int, role, content string) {
	content = strings.TrimSpace(content)
	if content == "" {
		return
	}
	*nextMsgID++
	*items = append(*items, memoryTranscriptItem{
		Created:    created,
		MsgID:      *nextMsgID,
		EntryIndex: entryIndex,
		Bytes:      bytes,
		Role:       role,
		Content:    content,
	})
}

func memoryTranscriptItemsFromEntries(entries []*MemoryEntry) []memoryTranscriptItem {
	items := make([]memoryTranscriptItem, 0, len(entries)*2)
	var nextMsgID int64
	suppressToolRequestBySource := make(map[string]bool)

	for index, entry := range entries {
		if entry == nil {
			continue
		}

		sourceKey := memoryEntrySourceKey(entry, index)
		rawOutput := strings.TrimSpace(entry.RawOutput)
		entryBytes := MemoryEntryByteSize(entry)
		if rawOutput != "" {
			appendTranscriptLine(&items, &nextMsgID, index, entry.Created, entryBytes, "assistant", rawOutput)
			if entry.HasToolCall() {
				suppressToolRequestBySource[sourceKey] = true
			} else {
				delete(suppressToolRequestBySource, sourceKey)
			}
		}

		if entry.HasToolCall() {
			if rawOutput == "" && !suppressToolRequestBySource[sourceKey] {
				request := firstNonEmptyTrimmed(entry.ToolRequestRawJSON, entry.Content)
				appendTranscriptLine(&items, &nextMsgID, index, entry.Created, entryBytes, transcriptRoleForEntry(entry), request)
			}

			result := RenderToolResultContentFromEntry(entry)
			appendTranscriptLine(&items, &nextMsgID, index, entry.Created, entryBytes, toolTranscriptRole(entry.ToolName), result)
			continue
		}

		delete(suppressToolRequestBySource, sourceKey)

		if rawOutput != "" {
			continue
		}

		content := strings.TrimSpace(entry.RenderContent())
		appendTranscriptLine(&items, &nextMsgID, index, entry.Created, entryBytes, transcriptRoleForEntry(entry), content)
	}

	return items
}

// MsgIDsToEntryIndexes 将提示词里展示的 MsgID（与 SerializeEntries 转写一致）解析为去重后的 entry 下标，顺序为在 msgIDs 中首次出现的顺序。
func MsgIDsToEntryIndexes(entries []*MemoryEntry, msgIDs []int64) ([]int, error) {
	items := memoryTranscriptItemsFromEntries(entries)
	if len(items) == 0 {
		return nil, fmt.Errorf("no transcript lines built from entries")
	}
	idToEntry := make(map[int64]int, len(items))
	for _, it := range items {
		if it.MsgID <= 0 {
			continue
		}
		if _, exists := idToEntry[it.MsgID]; exists {
			continue
		}
		idToEntry[it.MsgID] = it.EntryIndex
	}
	var result []int
	seenIdx := map[int]struct{}{}
	for _, id := range msgIDs {
		idx, ok := idToEntry[id]
		if !ok {
			return nil, fmt.Errorf("unknown MsgID %d", id)
		}
		if _, dup := seenIdx[idx]; dup {
			continue
		}
		seenIdx[idx] = struct{}{}
		result = append(result, idx)
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("no entries selected")
	}
	return result, nil
}

func memoryEntrySourceKey(entry *MemoryEntry, index int) string {
	if entry == nil {
		return fmt.Sprintf("entry:%d", index)
	}
	if value := strings.TrimSpace(entry.SourceMessageID); value != "" {
		return "message:" + value
	}
	if entry.Sequence > 0 {
		return fmt.Sprintf("sequence:%d", entry.Sequence)
	}
	return fmt.Sprintf("entry:%d", index)
}

func memoryTranscriptItemsFromEntry(entry *MemoryEntry) []memoryTranscriptItem {
	if entry == nil {
		return nil
	}

	role := transcriptRoleForEntry(entry)
	created := entry.Created
	entryBytes := MemoryEntryByteSize(entry)
	items := make([]memoryTranscriptItem, 0, 2)
	var nextMsgID int64
	const singleEntryIndex = 0

	if entry.HasToolCall() {
		request := firstNonEmptyTrimmed(entry.ToolRequestRawJSON, entry.Content, entry.RawOutput)
		appendTranscriptLine(&items, &nextMsgID, singleEntryIndex, created, entryBytes, role, request)

		result := RenderToolResultContentFromEntry(entry)
		appendTranscriptLine(&items, &nextMsgID, singleEntryIndex, created, entryBytes, toolTranscriptRole(entry.ToolName), result)

		if len(items) > 0 {
			return items
		}
	}

	content := strings.TrimSpace(entry.RenderContent())
	appendTranscriptLine(&items, &nextMsgID, singleEntryIndex, created, entryBytes, role, content)
	if len(items) == 0 {
		return nil
	}
	return items
}

func renderMemoryTranscript(items []memoryTranscriptItem) string {
	builder := strings.Builder{}

	for _, item := range items {
		content := strings.TrimSpace(item.Content)
		if content == "" {
			continue
		}

		if builder.Len() > 0 {
			builder.WriteString("\n")
		}

		builder.WriteString("======\n")
		builder.WriteString(formatMemoryTimestamp(item.Created))
		if item.MsgID > 0 {
			builder.WriteString("\nMsgID: ")
			builder.WriteString(strconv.FormatInt(item.MsgID, 10))
		}
		if item.Bytes > 0 {
			builder.WriteString(" · ")
			builder.WriteString(strconv.Itoa(item.Bytes))
			builder.WriteString(" bytes")
		}
		builder.WriteString("\n[")
		builder.WriteString(normalizeTranscriptRole(item.Role))
		builder.WriteString("]: ")
		builder.WriteString(content)
	}

	return builder.String()
}

func transcriptRoleForEntry(entry *MemoryEntry) string {
	if entry == nil {
		return "assistant"
	}

	if strings.EqualFold(strings.TrimSpace(entry.EntryKind), "tool_result") {
		return "call_tool"
	}

	return normalizeTranscriptRole(entry.Role)
}

func normalizeTranscriptRole(role string) string {
	trimmed := strings.TrimSpace(role)
	switch {
	case strings.HasPrefix(trimmed, "call_tool_"):
		return trimmed
	}

	switch trimmed {
	case "", "assistant":
		return "assistant"
	case "tool", "tool_call", "tool_calls", "call_tool", "tool_result":
		return "call_tool"
	default:
		return trimmed
	}
}

func toolTranscriptRole(toolName string) string {
	normalizedName := strings.TrimSpace(toolName)
	if normalizedName == "" {
		return "call_tool"
	}
	return "call_tool_" + normalizedName
}

func formatMemoryTimestamp(created int64) string {
	if created <= 0 {
		return "未知时间"
	}

	var timestamp time.Time
	switch {
	case created >= 1_000_000_000_000:
		timestamp = time.UnixMilli(created)
	default:
		timestamp = time.Unix(created, 0)
	}

	return timestamp.Format("2006年01月02日15时04分05秒")
}

func firstNonEmptyTrimmed(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func MemoryEntryByteSize(entry *MemoryEntry) int {
	if entry == nil {
		return 0
	}

	total := 0
	values := []string{
		entry.Content,
		entry.RawOutput,
		entry.CallToolInfo,
		entry.ToolRequestRawJSON,
		entry.ToolInputJSON,
		entry.ToolOutput,
		entry.ToolError,
		entry.ToolTitle,
		entry.ToolMetadataJSON,
	}
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		total += len([]byte(trimmed))
	}
	if total > 0 {
		return total
	}
	return len([]byte(strings.TrimSpace(entry.RenderContent())))
}

func looksLikeToolCallPayload(content string) bool {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return false
	}

	return strings.Contains(trimmed, `"call_tool":"call_tool"`) ||
		strings.Contains(trimmed, `"call_tool": "call_tool"`) ||
		strings.Contains(trimmed, `"action":"call_tool"`) ||
		strings.Contains(trimmed, `"action": "call_tool"`) ||
		strings.Contains(trimmed, `"@action":"call_tool"`) ||
		strings.Contains(trimmed, `"@action": "call_tool"`)
}

func (m *Memory) PromptContent() string {
	if m == nil {
		return ""
	}
	if len(m.Entries) > 0 || len(m.History) > 0 {
		return m.SerializeEntries()
	}
	return ""
}

func (m *Memory) legacyHistoryString() string {
	builder := strings.Builder{}
	builder.WriteString("<history>\n")
	builder.WriteString("以下是对话历史:\n")
	for i, item := range m.History {
		builder.WriteString("<item>\n")
		builder.WriteString("index: " + strconv.Itoa(i+1) + "\n")
		builder.WriteString("role: " + item.Role + "\n")
		if content := item.RenderContent(); content != "" {
			builder.WriteString("content: " + content + "\n")
		}
		if item.CallToolInfo != "" {
			builder.WriteString("callToolInfo: \n")
			builder.WriteString("<callToolInfo>\n")
			builder.WriteString(item.CallToolInfo + "\n")
			builder.WriteString("</callToolInfo>\n")
		}
		builder.WriteString("</item>\n")
	}
	builder.WriteString("</history>\n")

	if m.LatestToolCall != nil {
		builder.WriteString("<latest_message>\n")
		builder.WriteString("instruction: " + m.LatestToolCall.Instruction + "\n")
		if m.LatestToolCall.Content != "" {
			builder.WriteString("content: " + m.LatestToolCall.Content + "\n")
		}
		builder.WriteString("</latest_message>\n")
	}

	return builder.String()
}

func (m *Memory) String() string {
	builder := strings.Builder{}
	builder.WriteString("<system>\n")
	builder.WriteString(m.GlobalPrompt + "\n")
	builder.WriteString(m.OccupationPrompt + "\n")
	builder.WriteString(m.ProjectPrompt + "\n")
	builder.WriteString(m.ModelPrompt + "\n")
	builder.WriteString(m.WorkerPrompt + "\n")
	builder.WriteString(m.SessionGuidancePrompt + "\n")
	builder.WriteString(m.OutputStylePrompt + "\n")
	builder.WriteString(m.ToolPriorityPrompt + "\n")
	builder.WriteString(m.EnvPrompt + "\n")
	builder.WriteString("</system>\n")

	builder.WriteString("<user_prompt>\n")
	for _, item := range m.ProjectFilePrompt {
		builder.WriteString(item.Path + ":\n" + item.Prompt + "\n")
	}
	for _, item := range m.SkillPrompts {
		builder.WriteString(item.Path + ":\n" + item.Prompt + "\n")
	}
	builder.WriteString("</user_prompt>\n")

	builder.WriteString("<memory>\n")
	if promptContent := m.PromptContent(); promptContent != "" {
		builder.WriteString(promptContent + "\n")
	} else {
		builder.WriteString(m.legacyHistoryString())
	}
	builder.WriteString("</memory>\n")

	return builder.String()
}

// ChatHistoryNativeToolCall 表示与 OpenAI function_call 对齐的一条工具调用（参数为 JSON 对象字符串）。
type ChatHistoryNativeToolCall struct {
	ID        string `json:"id,omitempty"`
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

type ChatHistoryItem struct {
	Role                       string                      `json:"role"`
	Content                    string                      `json:"content"`
	LLMContentParts            []ChatHistoryContentPart    `json:"llmContentParts,omitempty"`
	RawOutput                  string                      `json:"rawOutput,omitempty"`
	CallToolInfo               string                      `json:"callToolInfo,omitempty"`
	ToolName                   string                      `json:"toolName,omitempty"`
	ToolCallID                 string                      `json:"toolCallID,omitempty"`
	ToolSystemMessage          string                      `json:"toolSystemMessage,omitempty"`
	ToolError                  string                      `json:"toolError,omitempty"`
	ToolStatus                 string                      `json:"toolStatus,omitempty"`
	NativeToolCalls            []ChatHistoryNativeToolCall `json:"nativeToolCalls,omitempty"`
	Phase                      string                      `json:"phase,omitempty"`
	ResponsesOutputMessageRaw  string                      `json:"responsesOutputMessageRaw,omitempty"`
	ResponsesReasoningItemRaws []string                    `json:"responsesReasoningItemRaws,omitempty"`
	ReasoningContent           string                      `json:"reasoningContent,omitempty"`
	ThinkingSignature            string                      `json:"thinkingSignature,omitempty"`
	SourceMessageID              string                      `json:"sourceMessageID,omitempty"`
	Synthetic                    bool                        `json:"synthetic,omitempty"`
	Created                    int64                       `json:"created,omitempty"`
}

func (i *ChatHistoryItem) RenderContent() string {
	if i == nil {
		return ""
	}

	if rawOutput := strings.TrimSpace(i.RawOutput); rawOutput != "" {
		return rawOutput
	}

	return strings.TrimSpace(i.Content)
}

type LatestToolCall struct {
	Role        string `json:"role,omitempty"`
	ToolName    string `json:"toolName,omitempty"`
	Content     string `json:"content,omitempty"`
	Instruction string `json:"instruction,omitempty"`
}

type FilePrompt struct {
	Path   string `json:"path"`
	Prompt string `json:"prompt"`
}

func FormatMemoryTimestamp(created int64) string {
	return formatMemoryTimestamp(created)
}

func FirstNonEmptyTrimmed(values ...string) string {
	return firstNonEmptyTrimmed(values...)
}

const EmptyToolResultMessage = "(command succeeded with no output)"

func isEmptySuccessToolStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "completed", "success", "done":
		return true
	default:
		return false
	}
}

// RenderToolResultContent 渲染工具结果正文，避免把 ToolStatus（如 completed）误当作输出内容。
func RenderToolResultContent(output, toolError, callToolInfo, toolStatus, toolTitle string) string {
	if result := firstNonEmptyTrimmed(output, toolError, callToolInfo, toolTitle); result != "" {
		return result
	}
	status := strings.TrimSpace(toolStatus)
	if status != "" && !isEmptySuccessToolStatus(status) {
		return status
	}
	if isEmptySuccessToolStatus(status) {
		return EmptyToolResultMessage
	}
	return ""
}

// RenderToolResultContentFromEntry 从 MemoryEntry 渲染工具结果正文。
func RenderToolResultContentFromEntry(entry *MemoryEntry) string {
	if entry == nil {
		return ""
	}
	return RenderToolResultContent(entry.ToolOutput, entry.ToolError, entry.CallToolInfo, entry.ToolStatus, entry.ToolTitle)
}

type ChatHistoryImageURL struct {
	URL string `json:"url,omitempty"`
}

type ChatHistoryContentPart struct {
	Type     string               `json:"type,omitempty"`
	Text     string               `json:"text,omitempty"`
	ImageURL *ChatHistoryImageURL `json:"image_url,omitempty"`
}

func CloneChatHistoryContentParts(parts []ChatHistoryContentPart) []ChatHistoryContentPart {
	if len(parts) == 0 {
		return nil
	}
	out := make([]ChatHistoryContentPart, len(parts))
	copy(out, parts)
	return out
}
