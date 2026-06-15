package coreagent

import (
	"strings"
	"testing"

	agentmemory "matrixops.local/memory"
	agentprovider "matrixops-agent/provider"
	"matrixops.local/core_agent/streamtypes"
)

func TestBuildMemoryMessagesHistoryCoalescesSplitAssistantNativeToolRound(t *testing.T) {
	messages := BuildMemoryMessages(&agentmemory.Memory{
		History: []*agentmemory.ChatHistoryItem{
			{
				Role:             "assistant",
				Content:          "planning visible text",
				ReasoningContent: "internal reasoning",
				SourceMessageID:  "msg-round-1",
			},
			{
				Role:            "assistant",
				SourceMessageID: "msg-round-1",
				NativeToolCalls: []agentmemory.ChatHistoryNativeToolCall{
					{ID: "id-1", Name: "read", Arguments: `{"path":"x"}`},
				},
			},
			{
				Role:       "tool",
				ToolCallID: "id-1",
				Content:    "tool output 1",
			},
			{
				Role:            "assistant",
				SourceMessageID: "msg-round-1",
				NativeToolCalls: []agentmemory.ChatHistoryNativeToolCall{
					{ID: "id-2", Name: "list", Arguments: `{}`},
				},
			},
			{
				Role:       "tool",
				ToolCallID: "id-2",
				Content:    "tool output 2",
			},
		},
	})
	if len(messages) != 3 {
		t.Fatalf("expected 3 messages (1 merged assistant + 2 tool), got %d %#v", len(messages), messages)
	}
	if messages[0].Role != "assistant" || len(messages[0].ToolCalls) != 2 {
		t.Fatalf("expected merged assistant with 2 tool_calls, got %#v", messages[0])
	}
	if got, ok := messages[0].Content.(string); !ok || got != "planning visible text" {
		t.Fatalf("expected merged content, got %#v", messages[0].Content)
	}
	if messages[0].ReasoningContent != "internal reasoning" {
		t.Fatalf("expected merged reasoning, got %q", messages[0].ReasoningContent)
	}
}

func TestBuildMemoryMessagesEntriesCoalesceTextReasoningBeforeNativeTools(t *testing.T) {
	messages := BuildMemoryMessages(&agentmemory.Memory{
		Entries: []*agentmemory.MemoryEntry{
			{
				EntryKind:        "text",
				Role:             "assistant",
				SourceMessageID:  "msg-round-1",
				Content:          "visible reply",
				ReasoningContent: "internal reasoning",
			},
			{
				EntryKind:          "tool_call",
				Role:               "assistant",
				SourceMessageID:    "msg-round-1",
				ToolCallID:         "call-a",
				ToolName:           "read",
				ToolInputJSON:      `{"path":"a"}`,
				ToolRequestRawJSON: `{"path":"a"}`,
				ToolOutput:         "out-a",
			},
			{
				EntryKind:          "tool_call",
				Role:               "assistant",
				SourceMessageID:    "msg-round-1",
				ToolCallID:         "call-b",
				ToolName:           "list",
				ToolInputJSON:      `{"path":"."}`,
				ToolRequestRawJSON: `{"path":"."}`,
				ToolOutput:         "out-b",
			},
		},
	})

	if len(messages) != 3 {
		t.Fatalf("expected 3 messages (1 merged assistant + 2 tool), got %d: %#v", len(messages), messages)
	}
	if messages[0].Role != "assistant" || len(messages[0].ToolCalls) != 2 {
		t.Fatalf("expected one assistant with 2 tool_calls, got %#v", messages[0])
	}
	if got := messages[0].Content.(string); got != "visible reply" {
		t.Fatalf("expected merged content, got %q", got)
	}
	if messages[0].ReasoningContent != "internal reasoning" {
		t.Fatalf("expected merged reasoning, got %q", messages[0].ReasoningContent)
	}
	if messages[1].Role != "tool" || messages[1].ToolCallID != "call-a" {
		t.Fatalf("unexpected first tool message: %#v", messages[1])
	}
	if messages[2].Role != "tool" || messages[2].ToolCallID != "call-b" {
		t.Fatalf("unexpected second tool message: %#v", messages[2])
	}
}

func TestBuildMemoryMessagesEntriesDoesNotCoalesceDifferentSourceMessageToolCalls(t *testing.T) {
	messages := BuildMemoryMessages(&agentmemory.Memory{
		Entries: []*agentmemory.MemoryEntry{
			{
				EntryKind:          "tool_call",
				Role:               "assistant",
				SourceMessageID:    "msg-round-1",
				ToolCallID:         "call-a",
				ToolName:           "rg",
				ToolInputJSON:      `{"pattern":"a"}`,
				ToolRequestRawJSON: `{"pattern":"a"}`,
				ToolOutput:         "out-a",
			},
			{
				EntryKind:          "tool_call",
				Role:               "assistant",
				SourceMessageID:    "msg-round-2",
				ToolCallID:         "call-b",
				ToolName:           "read",
				ToolInputJSON:      `{"path":"b"}`,
				ToolRequestRawJSON: `{"path":"b"}`,
				ToolOutput:         "out-b",
			},
			{
				EntryKind:          "tool_call",
				Role:               "assistant",
				SourceMessageID:    "msg-round-2",
				ToolCallID:         "call-c",
				ToolName:           "list",
				ToolInputJSON:      `{}`,
				ToolRequestRawJSON: `{}`,
				ToolOutput:         "out-c",
			},
		},
	})

	if len(messages) != 5 {
		t.Fatalf("expected 5 messages (2 assistant rounds + 3 tool results), got %d: %#v", len(messages), messages)
	}
	if len(messages[0].ToolCalls) != 1 || messages[0].ToolCalls[0].ID != "call-a" {
		t.Fatalf("expected first round assistant with one tool call, got %#v", messages[0])
	}
	if len(messages[2].ToolCalls) != 2 || messages[2].ToolCalls[0].ID != "call-b" || messages[2].ToolCalls[1].ID != "call-c" {
		t.Fatalf("expected second round assistant with two tool calls, got %#v", messages[2])
	}
}

func TestBuildMemoryMessagesHistoryDoesNotCoalesceDifferentSourceMessageToolCalls(t *testing.T) {
	messages := BuildMemoryMessages(&agentmemory.Memory{
		History: []*agentmemory.ChatHistoryItem{
			{
				Role:             "assistant",
				ReasoningContent: "search AgentRunner.Process",
				ThinkingSignature: "sig-1",
				SourceMessageID:  "msg-round-1",
				NativeToolCalls: []agentmemory.ChatHistoryNativeToolCall{
					{ID: "tool-1", Name: "rg", Arguments: `{"pattern":"AgentRunner\\.Process"}`},
				},
			},
			{Role: "tool", ToolCallID: "tool-1", Content: "[Tool Output]: 工具输出为空"},
			{
				Role:            "assistant",
				SourceMessageID: "msg-round-2",
				NativeToolCalls: []agentmemory.ChatHistoryNativeToolCall{
					{ID: "tool-2", Name: "rg", Arguments: `{"pattern":"\\.Process\\("}`},
				},
			},
			{Role: "tool", ToolCallID: "tool-2", Content: "[Tool Output]: 工具输出为空"},
			{
				Role:            "assistant",
				SourceMessageID: "msg-round-3",
				NativeToolCalls: []agentmemory.ChatHistoryNativeToolCall{
					{ID: "tool-3", Name: "rg", Arguments: `{"pattern":"AgentRunner"}`},
				},
			},
			{Role: "tool", ToolCallID: "tool-3", Content: "many matches"},
			{
				Role:            "assistant",
				SourceMessageID: "msg-round-4",
				NativeToolCalls: []agentmemory.ChatHistoryNativeToolCall{
					{ID: "tool-4", Name: "rg", Arguments: `{"pattern":"Process"}`},
				},
			},
			{Role: "tool", ToolCallID: "tool-4", Content: "huge output"},
		},
	})

	if len(messages) != 8 {
		t.Fatalf("expected 8 messages (4 assistant rounds + 4 tool results), got %d: %#v", len(messages), messages)
	}
	for i := 0; i < 4; i++ {
		asst := messages[i*2]
		tool := messages[i*2+1]
		if asst.Role != "assistant" || len(asst.ToolCalls) != 1 {
			t.Fatalf("expected assistant round %d with one tool call, got %#v", i+1, asst)
		}
		if tool.Role != "tool" {
			t.Fatalf("expected tool result after round %d, got %#v", i+1, tool)
		}
	}
}

func TestBuildMemoryMessagesBuildsToolCallAndToolResult(t *testing.T) {
	messages := BuildMemoryMessages(&agentmemory.Memory{
		Entries: []*agentmemory.MemoryEntry{
			{
				EntryKind:          "tool_call",
				Role:               "assistant",
				ToolCallID:         "call-1",
				ToolName:           "read_file",
				ToolInputJSON:      `{"path":"README.md"}`,
				ToolRequestRawJSON: `{"path":"README.md"}`,
				ToolOutput:         "file content",
			},
		},
	})

	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}
	if messages[0].Role != "assistant" {
		t.Fatalf("expected first role assistant, got %q", messages[0].Role)
	}
	if len(messages[0].ToolCalls) != 1 {
		t.Fatalf("expected assistant tool call history")
	}
	if messages[0].ToolCalls[0].Name != "read_file" {
		t.Fatalf("unexpected tool name: %q", messages[0].ToolCalls[0].Name)
	}
	if messages[1].Role != "tool" {
		t.Fatalf("expected second role tool, got %q", messages[1].Role)
	}
	if messages[1].ToolCallID != "call-1" {
		t.Fatalf("unexpected tool call id: %q", messages[1].ToolCallID)
	}
}

func TestBuildMemoryMessagesHistoryNativeToolCalls(t *testing.T) {
	messages := BuildMemoryMessages(&agentmemory.Memory{
		History: []*agentmemory.ChatHistoryItem{
			{
				Role: "assistant",
				NativeToolCalls: []agentmemory.ChatHistoryNativeToolCall{
					{ID: "x1", Name: "read_file", Arguments: `{"path":"README.md"}`},
				},
			},
			{
				Role:              "tool",
				ToolCallID:        "x1",
				Content:           "     1\tfile content",
				ToolSystemMessage: "1 lines read from file starting from line 1. Total lines in file: 1. End of file reached.",
			},
		},
	})
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}
	if messages[0].Role != "assistant" || len(messages[0].ToolCalls) != 1 {
		t.Fatalf("expected assistant with one tool call, got %#v", messages[0])
	}
	if messages[0].ToolCalls[0].ID != "x1" || messages[0].ToolCalls[0].Name != "read_file" {
		t.Fatalf("unexpected tool call: %#v", messages[0].ToolCalls[0])
	}
	if messages[1].Role != "tool" || messages[1].ToolCallID != "x1" {
		t.Fatalf("unexpected tool message: %#v", messages[1])
	}
	parts, ok := messages[1].Content.([]agentprovider.CommonContentPart)
	if !ok || len(parts) != 2 {
		t.Fatalf("expected 2-part tool content, got %#v", messages[1].Content)
	}
	if !strings.Contains(parts[0].Text, "<system>") || !strings.Contains(parts[0].Text, "1 lines read") {
		t.Fatalf("expected system summary in first part, got %q", parts[0].Text)
	}
}

func TestBuildMemoryMessagesBuildsPlainUserAssistantHistory(t *testing.T) {
	messages := BuildMemoryMessages(&agentmemory.Memory{
		Entries: []*agentmemory.MemoryEntry{
			{EntryKind: "text", Role: "user", Content: "用户消息"},
			{EntryKind: "text", Role: "assistant", Content: "助手消息"},
		},
	})

	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}
	if messages[0].Role != "user" {
		t.Fatalf("expected first role user, got %q", messages[0].Role)
	}
	if messages[1].Role != "assistant" {
		t.Fatalf("expected second role assistant, got %q", messages[1].Role)
	}
}

func TestBuildMemoryMessagesRestoresResponsesReplayFields(t *testing.T) {
	messages := BuildMemoryMessages(&agentmemory.Memory{
		Entries: []*agentmemory.MemoryEntry{
			{
				EntryKind:                      "text",
				Role:                           "assistant",
				Content:                        "最终回答",
				Phase:                          "final_answer",
				ResponsesOutputMessageRaw:      `{"type":"message","id":"msg_1","role":"assistant","status":"completed","phase":"final_answer","content":[{"type":"output_text","text":"最终回答"}]}`,
				ResponsesReasoningItemRawsJSON: `["{\"type\":\"reasoning\",\"id\":\"rs_1\",\"status\":\"completed\",\"summary\":[{\"type\":\"summary_text\",\"text\":\"summary\"}],\"encrypted_content\":\"enc_1\"}"]`,
			},
		},
	})

	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}
	if messages[0].Phase != "final_answer" {
		t.Fatalf("expected phase to be restored, got %#v", messages[0].Phase)
	}
	if messages[0].ResponsesOutputMessageRaw == "" {
		t.Fatalf("expected responses output message raw to be restored")
	}
	if len(messages[0].ResponsesReasoningItemRaws) != 1 {
		t.Fatalf("expected reasoning replay items to be restored, got %#v", messages[0].ResponsesReasoningItemRaws)
	}
}

func TestBuildMemoryMessagesHistoryUserIncludesImageParts(t *testing.T) {
	messages := BuildMemoryMessages(&agentmemory.Memory{
		History: []*agentmemory.ChatHistoryItem{
			{
				Role:    "user",
				Content: "请分析图片",
				LLMContentParts: []agentmemory.ChatHistoryContentPart{
					{Type: "text", Text: "请分析图片"},
					{
						Type:     "image_url",
						ImageURL: &agentmemory.ChatHistoryImageURL{URL: "https://example.com/a.png"},
					},
				},
			},
		},
	})
	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}
	parts, ok := messages[0].Content.([]agentprovider.CommonContentPart)
	if !ok {
		t.Fatalf("expected multimodal content parts, got %#v", messages[0].Content)
	}
	if len(parts) != 2 || parts[0].Text != "请分析图片" || parts[1].ImageURL.URL != "https://example.com/a.png" {
		t.Fatalf("unexpected content parts: %#v", parts)
	}
}

func TestBuildMemoryMessagesEntryToolResultDoesNotUseCompleted(t *testing.T) {
	messages := BuildMemoryMessages(&agentmemory.Memory{
		Entries: []*agentmemory.MemoryEntry{
			{
				ToolName:   "bash",
				ToolCallID: "call-1",
				ToolStatus: "completed",
			},
		},
	})
	if len(messages) != 2 {
		t.Fatalf("expected assistant+tool messages, got %d", len(messages))
	}
	if messages[1].Role != "tool" || messages[1].Content != agentmemory.EmptyToolResultMessage {
		t.Fatalf("unexpected tool message: %#v", messages[1])
	}
}

func TestBuildMemoryMessages_DoesNotMergeSyntheticSummaryWithFollowingTools(t *testing.T) {
	messages := BuildMemoryMessages(&agentmemory.Memory{
		History: []*agentmemory.ChatHistoryItem{
			{Role: "user", Content: "总结一下之前都做了什么。", Synthetic: true},
			{Role: "assistant", Content: "<current_focus>\n压缩后的摘要\n</current_focus>", Synthetic: true},
			{
				Role:            "assistant",
				SourceMessageID: "msg-old",
				NativeToolCalls: []agentmemory.ChatHistoryNativeToolCall{{ID: "tc-old", Name: "write", Arguments: `{"path":"store/log.go"}`}},
			},
			{Role: "tool", ToolCallID: "tc-old", Content: "written"},
		},
	})
	if len(messages) != 4 {
		t.Fatalf("expected 4 LLM messages, got %d: %#v", len(messages), messages)
	}
	if got := strings.TrimSpace(streamtypes.RenderMessageTextContent(messages[1].Content)); got != "<current_focus>\n压缩后的摘要\n</current_focus>" {
		t.Fatalf("expected summary text in its own assistant message, got %q", got)
	}
	if len(messages[2].ToolCalls) != 1 || messages[2].ToolCalls[0].ID != "tc-old" {
		t.Fatalf("expected separate tool-call assistant message, got %#v", messages[2])
	}
	if strings.Contains(streamtypes.RenderMessageTextContent(messages[2].Content), "<current_focus>") {
		t.Fatalf("expected tool assistant message not to contain summary text, got %#v", messages[2])
	}
}
