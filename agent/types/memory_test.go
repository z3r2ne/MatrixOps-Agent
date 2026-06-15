package types

import (
	"strings"
	"testing"
	"time"
)

func TestMemorySerializeEntries_FormatsTranscript(t *testing.T) {
	userCreated := time.Date(2026, 4, 9, 10, 0, 1, 0, time.UTC).UnixMilli()
	assistantCreated := time.Date(2026, 4, 9, 10, 0, 2, 0, time.UTC).UnixMilli()
	toolCreated := time.Date(2026, 4, 9, 10, 0, 3, 0, time.UTC).UnixMilli()

	memory := &Memory{
		Entries: []*MemoryEntry{
			{
				EntryKind: "text",
				Role:      "user",
				Content:   "你好",
				Sequence:  1,
				Created:   userCreated,
			},
			{
				EntryKind: "text",
				Role:      "assistant",
				Content:   "你好",
				Sequence:  2,
				Created:   assistantCreated,
			},
			{
				EntryKind:          "tool_call",
				Role:               "assistant",
				ToolRequestRawJSON: `{"call_tool":"call_tool","params":{"tool_name":"read_file","tool_input":{"path":"README.md"}}}`,
				ToolOutput:         "README 内容",
				Sequence:           3,
				Created:            toolCreated,
			},
		},
	}

	serialized := memory.SerializeEntries()

	if strings.Count(serialized, "======") != 4 {
		t.Fatalf("expected 4 transcript blocks, got:\n%s", serialized)
	}
	if !strings.Contains(serialized, formatMemoryTimestamp(userCreated)) {
		t.Fatalf("expected user timestamp in transcript, got:\n%s", serialized)
	}
	if !strings.Contains(serialized, "MsgID: 1") {
		t.Fatalf("expected MsgID 1 in transcript, got:\n%s", serialized)
	}
	if !strings.Contains(serialized, "[user]: 你好") {
		t.Fatalf("expected user line in transcript, got:\n%s", serialized)
	}
	if !strings.Contains(serialized, "[assistant]: 你好") {
		t.Fatalf("expected assistant line in transcript, got:\n%s", serialized)
	}
	if !strings.Contains(serialized, `[assistant]: {"call_tool":"call_tool","params":{"tool_name":"read_file","tool_input":{"path":"README.md"}}}`) {
		t.Fatalf("expected tool request line in transcript, got:\n%s", serialized)
	}
	if !strings.Contains(serialized, "[call_tool]: README 内容") {
		t.Fatalf("expected tool output line in transcript, got:\n%s", serialized)
	}
}

func TestMemorySerializeEntries_MultiActionEntriesEmitOneAssistantBlockPerAction(t *testing.T) {
	created := time.Date(2026, 4, 9, 10, 2, 0, 0, time.UTC).UnixMilli()
	rawFirst := `{"call_tool":"call_tool","params":{"tool_name":"glob","tool_input":{"pattern":"**/AGENTS.md"}}}`
	rawSecond := `{"call_tool":"call_tool","params":{"tool_name":"tree","tool_input":{"path":"/repo","depth":2}}}`

	memory := &Memory{
		Entries: []*MemoryEntry{
			{
				SourceMessageID:    "msg-1",
				EntryKind:          "tool_call",
				Role:               "assistant",
				RawOutput:          rawFirst,
				ToolRequestRawJSON: `{"call_tool":"call_tool","params":{"tool_name":"glob","tool_input":{"pattern":"**/AGENTS.md"}}}`,
				ToolName:           "glob",
				ToolOutput:         "[Tool Output]: 工具输出为空",
				Sequence:           1,
				Created:            created,
			},
			{
				SourceMessageID:    "msg-1",
				EntryKind:          "tool_call",
				Role:               "assistant",
				RawOutput:          rawSecond,
				ToolRequestRawJSON: `{"call_tool":"call_tool","params":{"tool_name":"tree","tool_input":{"path":"/repo","depth":2}}}`,
				ToolName:           "tree",
				ToolOutput:         "[Tool Output]: tree output",
				Sequence:           2,
				Created:            created,
			},
		},
	}

	serialized := memory.SerializeEntries()

	if strings.Count(serialized, "[assistant]: "+rawFirst) != 1 {
		t.Fatalf("expected one assistant raw block for first action, got:\n%s", serialized)
	}
	if strings.Count(serialized, "[assistant]: "+rawSecond) != 1 {
		t.Fatalf("expected one assistant raw block for second action, got:\n%s", serialized)
	}
	if !strings.Contains(serialized, "[call_tool_glob]: [Tool Output]: 工具输出为空") {
		t.Fatalf("expected glob tool block, got:\n%s", serialized)
	}
	if !strings.Contains(serialized, "[call_tool_tree]: [Tool Output]: tree output") {
		t.Fatalf("expected tree tool block, got:\n%s", serialized)
	}
}

func TestMemorySerializeEntries_AssignsSequentialMsgIDsIgnoringInternalSequence(t *testing.T) {
	created := time.Date(2026, 4, 9, 10, 3, 0, 0, time.UTC).UnixMilli()
	memory := &Memory{
		Entries: []*MemoryEntry{
			{
				EntryKind: "text",
				Role:      "user",
				Content:   "first",
				Sequence:  1712912345000,
				Created:   created,
			},
			{
				EntryKind: "text",
				Role:      "assistant",
				Content:   "second",
				Sequence:  1712912345001,
				Created:   created + 1,
			},
		},
	}

	serialized := memory.SerializeEntries()

	if !strings.Contains(serialized, "MsgID: 1") || !strings.Contains(serialized, "MsgID: 2") {
		t.Fatalf("expected MsgID 1 and 2 in transcript, got:\n%s", serialized)
	}
	if strings.Contains(serialized, "1712912345000") || strings.Contains(serialized, "1712912345001") {
		t.Fatalf("expected internal sequence not to appear in transcript, got:\n%s", serialized)
	}
}
