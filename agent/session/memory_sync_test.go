package session

import (
	"strings"
	"testing"
	"time"

	"matrixops-agent/types"
)

func TestBuildMemoryEntriesFromMessage_TextAndTool(t *testing.T) {
	message := &WithParts{
		Info: &MessageInfo{
			ID:        "msg-1",
			SessionID: "sess-1",
			Role:      RoleAssistant,
			Time: MessageTime{
				Created: time.Now().UnixMilli(),
			},
		},
		Parts: []*Part{
			{
				ID:   "part-text",
				Type: "text",
				Text: "hello world",
				Metadata: map[string]interface{}{
					"phase":                     "final_answer",
					"responsesOutputMessageRaw": `{"type":"message","id":"msg_1","role":"assistant","status":"completed","phase":"final_answer","content":[{"type":"output_text","text":"hello world"}]}`,
					"responsesReasoningItemRaws": []string{
						`{"type":"reasoning","id":"rs_1","status":"completed","summary":[{"type":"summary_text","text":"reasoning"}],"encrypted_content":"enc_1"}`,
					},
				},
			},
			{
				ID:   "part-tool",
				Type: "tool",
				Tool: &ToolPart{
					Name:   "read",
					CallID: "call-1",
					State: ToolState{
						Input:  map[string]interface{}{"path": "README.md"},
						Output: "[Tool Output]: file content",
					},
				},
			},
		},
	}

	entries, err := buildMemoryEntriesFromMessage(message)
	if err != nil {
		t.Fatalf("buildMemoryEntriesFromMessage returned error: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 memory entries, got %d", len(entries))
	}

	if entries[0].EntryKind != "text" || entries[0].Role != "assistant" {
		t.Fatalf("unexpected first entry: %#v", entries[0])
	}
	if entries[0].Phase != "final_answer" {
		t.Fatalf("expected first entry phase to be captured, got %#v", entries[0].Phase)
	}
	if entries[0].ResponsesOutputMessageRaw == "" {
		t.Fatalf("expected responses output message raw to be captured")
	}
	if entries[0].ResponsesReasoningItemRawsJSON == "" {
		t.Fatalf("expected responses reasoning raws json to be captured")
	}
	if entries[1].EntryKind != "tool_call" || entries[1].Role != "assistant" {
		t.Fatalf("unexpected second entry: %#v", entries[1])
	}
	if entries[1].ToolCallID != "call-1" || entries[1].ToolName != "read" {
		t.Fatalf("unexpected structured tool fields: %#v", entries[1])
	}
	if entries[1].ToolRequestRawJSON != "" {
		t.Fatalf("expected tool request raw json to stay empty when no raw backend payload exists: %#v", entries[1])
	}
	if entries[1].ToolOutput != "[Tool Output]: file content" {
		t.Fatalf("expected tool output to be preserved in one entry: %#v", entries[1])
	}
}

func TestMemoryEntriesToChatHistory(t *testing.T) {
	entries := []*types.MemoryEntry{
		{
			Role:                           "assistant",
			Content:                        "normalized",
			RawOutput:                      `{"call_tool":"answer","params":"raw"}`,
			Phase:                          "commentary",
			ResponsesOutputMessageRaw:      `{"type":"message","id":"msg_hist","role":"assistant","status":"completed","phase":"commentary","content":[{"type":"output_text","text":"normalized"}]}`,
			ResponsesReasoningItemRawsJSON: `["{\"type\":\"reasoning\",\"id\":\"rs_hist\",\"status\":\"completed\",\"summary\":[{\"type\":\"summary_text\",\"text\":\"reasoning\"}],\"encrypted_content\":\"enc_hist\"}"]`,
		},
		{
			Role:               "assistant",
			EntryKind:          "tool_call",
			Content:            `{"call_tool":"call_tool","params":{"tool_name":"read_file","tool_input":{"path":"README.md"}}}`,
			ToolName:           "read_file",
			ToolStatus:         "completed",
			ToolRequestRawJSON: `{"call_tool":"call_tool","params":{"tool_name":"read_file","tool_input":{"path":"README.md"}}}`,
			ToolOutput:         "[Tool Output]: ok",
		},
	}

	history := memoryEntriesToChatHistory(entries)
	if len(history) != 2 {
		t.Fatalf("expected 2 history items, got %d", len(history))
	}
	if history[0].RenderContent() != `{"call_tool":"answer","params":"raw"}` {
		t.Fatalf("expected first history item to prefer raw output, got %q", history[0].RenderContent())
	}
	if history[0].Phase != "commentary" {
		t.Fatalf("expected first history item phase to round-trip, got %#v", history[0].Phase)
	}
	if history[0].ResponsesOutputMessageRaw == "" {
		t.Fatalf("expected first history item raw message to round-trip")
	}
	if len(history[0].ResponsesReasoningItemRaws) != 1 {
		t.Fatalf("expected first history item reasoning raws to round-trip, got %#v", history[0].ResponsesReasoningItemRaws)
	}
	if history[1].RenderContent() != `{"call_tool":"call_tool","params":{"tool_name":"read_file","tool_input":{"path":"README.md"}}}` {
		t.Fatalf("unexpected second history item content: %q", history[1].RenderContent())
	}
	if history[1].CallToolInfo == "" {
		t.Fatalf("expected structured tool info to be carried into chat history: %#v", history[1])
	}
}

func TestMemoryEntriesToChatHistory_MergesTextAssistantBeforeNativeToolBatch(t *testing.T) {
	entries := []*types.MemoryEntry{
		{
			EntryKind:        "text",
			Role:             "assistant",
			SourceMessageID:  "msg-round-1",
			Content:          "visible text",
			ReasoningContent: "reasoning blob",
			Phase:            "commentary",
		},
		{
			EntryKind:          "tool_call",
			Role:               "assistant",
			SourceMessageID:    "msg-round-1",
			ToolCallID:         "tc-a",
			ToolName:           "read",
			ToolInputJSON:      `{"path":"a"}`,
			ToolRequestRawJSON: `{"path":"a"}`,
			ToolOutput:         "out-a",
		},
		{
			EntryKind:          "tool_call",
			Role:               "assistant",
			SourceMessageID:    "msg-round-1",
			ToolCallID:         "tc-b",
			ToolName:           "list",
			ToolInputJSON:      `{"path":"."}`,
			ToolRequestRawJSON: `{"path":"."}`,
			ToolOutput:         "out-b",
		},
	}

	history := memoryEntriesToChatHistory(entries)
	if len(history) != 3 {
		t.Fatalf("expected 3 history rows (1 merged assistant + 2 tool), got %d", len(history))
	}
	h0 := history[0]
	if h0.Role != "assistant" || len(h0.NativeToolCalls) != 2 {
		t.Fatalf("expected merged assistant with 2 native calls, got %#v", h0)
	}
	if strings.TrimSpace(h0.Content) != "visible text" {
		t.Fatalf("expected merged visible text, got %q", h0.Content)
	}
	if strings.TrimSpace(h0.ReasoningContent) != "reasoning blob" {
		t.Fatalf("expected merged reasoning, got %q", h0.ReasoningContent)
	}
	if h0.Phase != "commentary" {
		t.Fatalf("expected merged phase, got %q", h0.Phase)
	}
}

func TestMemoryEntriesToChatHistory_NativeToolCallBatch(t *testing.T) {
	entries := []*types.MemoryEntry{
		{
			EntryKind:          "tool_call",
			Role:               "assistant",
			SourceMessageID:    "msg-round-1",
			ToolCallID:         "tc-1",
			ToolName:           "bash",
			ToolInputJSON:      `{"command":"ls"}`,
			ToolRequestRawJSON: `{"command":"ls"}`,
			ToolOutput:         "out1",
		},
		{
			EntryKind:          "tool_call",
			Role:               "assistant",
			SourceMessageID:    "msg-round-1",
			ToolCallID:         "tc-2",
			ToolName:           "glob",
			ToolInputJSON:      `{"pattern":"*.go"}`,
			ToolRequestRawJSON: `{"pattern":"*.go"}`,
			ToolOutput:         "out2",
		},
	}

	history := memoryEntriesToChatHistory(entries)
	if len(history) != 3 {
		t.Fatalf("expected 3 history items (1 assistant + 2 tool), got %d", len(history))
	}
	if history[0].Role != "assistant" || len(history[0].NativeToolCalls) != 2 {
		t.Fatalf("expected assistant with 2 native tool calls, got %#v", history[0])
	}
	if history[0].NativeToolCalls[0].ID != "tc-1" || history[0].NativeToolCalls[1].ID != "tc-2" {
		t.Fatalf("unexpected native call ids: %#v", history[0].NativeToolCalls)
	}
	if history[1].Role != "tool" || history[1].ToolCallID != "tc-1" || history[1].Content != "out1" {
		t.Fatalf("unexpected first tool row: %#v", history[1])
	}
	if history[2].Role != "tool" || history[2].ToolCallID != "tc-2" || history[2].Content != "out2" {
		t.Fatalf("unexpected second tool row: %#v", history[2])
	}
}

func TestMemoryEntriesToChatHistory_DoesNotMergeToolCallsFromDifferentSourceMessages(t *testing.T) {
	entries := []*types.MemoryEntry{
		{
			EntryKind:          "tool_call",
			Role:               "assistant",
			SourceMessageID:    "msg-round-1",
			ToolCallID:         "tc-1",
			ToolName:           "rg",
			ToolInputJSON:      `{"pattern":"AgentRunner"}`,
			ToolRequestRawJSON: `{"pattern":"AgentRunner"}`,
			ToolOutput:         "out-1",
		},
		{
			EntryKind:          "tool_call",
			Role:               "assistant",
			SourceMessageID:    "msg-round-2",
			ToolCallID:         "tc-2",
			ToolName:           "read",
			ToolInputJSON:      `{"path":"a.go"}`,
			ToolRequestRawJSON: `{"path":"a.go"}`,
			ToolOutput:         "out-2",
		},
		{
			EntryKind:          "tool_call",
			Role:               "assistant",
			SourceMessageID:    "msg-round-2",
			ToolCallID:         "tc-3",
			ToolName:           "list",
			ToolInputJSON:      `{}`,
			ToolRequestRawJSON: `{}`,
			ToolOutput:         "out-3",
		},
	}

	history := memoryEntriesToChatHistory(entries)
	if len(history) != 5 {
		t.Fatalf("expected 5 history rows (2 assistant rounds + 3 tool results), got %d: %#v", len(history), history)
	}
	if len(history[0].NativeToolCalls) != 1 || history[0].NativeToolCalls[0].ID != "tc-1" {
		t.Fatalf("expected first round assistant with one tool call, got %#v", history[0])
	}
	if history[0].SourceMessageID != "msg-round-1" {
		t.Fatalf("expected first round source message id, got %q", history[0].SourceMessageID)
	}
	if len(history[2].NativeToolCalls) != 2 || history[2].NativeToolCalls[0].ID != "tc-2" || history[2].NativeToolCalls[1].ID != "tc-3" {
		t.Fatalf("expected second round assistant with two tool calls, got %#v", history[2])
	}
	if history[2].SourceMessageID != "msg-round-2" {
		t.Fatalf("expected second round source message id, got %q", history[2].SourceMessageID)
	}
}

func TestMemoryEntriesToChatHistory_DoesNotMergeSyntheticSummaryWithFollowingTools(t *testing.T) {
	entries := []*types.MemoryEntry{
		{
			EntryKind: "summary_user",
			Role:      "user",
			Content:   "总结一下之前都做了什么。",
			Synthetic: true,
		},
		{
			EntryKind: "summary_assistant",
			Role:      "assistant",
			Content:   "<current_focus>\n压缩后的摘要\n</current_focus>",
			Synthetic: true,
		},
		{
			EntryKind:          "tool_call",
			Role:               "assistant",
			SourceMessageID:    "msg-old",
			ToolCallID:         "tc-old",
			ToolName:           "write",
			ToolInputJSON:      `{"path":"store/log.go"}`,
			ToolRequestRawJSON: `{"path":"store/log.go"}`,
			ToolOutput:         "written",
		},
	}

	history := memoryEntriesToChatHistory(entries)
	if len(history) != 4 {
		t.Fatalf("expected 4 history rows (user + summary assistant + tool assistant + tool result), got %d: %#v", len(history), history)
	}
	if history[1].Content != "<current_focus>\n压缩后的摘要\n</current_focus>" || len(history[1].NativeToolCalls) != 0 {
		t.Fatalf("expected summary assistant to stay separate, got %#v", history[1])
	}
	if len(history[2].NativeToolCalls) != 1 || history[2].NativeToolCalls[0].ID != "tc-old" {
		t.Fatalf("expected following tool batch to stay separate, got %#v", history[2])
	}
	if strings.Contains(strings.TrimSpace(history[2].Content), "<current_focus>") {
		t.Fatalf("expected tool assistant message not to contain summary text, got %#v", history[2])
	}
}
