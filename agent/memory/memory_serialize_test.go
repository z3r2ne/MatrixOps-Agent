package memory

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSerializeMemoryEntriesJSON(t *testing.T) {
	t.Parallel()

	payload := SerializeMemoryEntriesJSON([]*MemoryEntry{
		{
			ID:        1,
			SessionID: "sess-1",
			Role:      "user",
			EntryKind: "message",
			Content:   "hello",
		},
	})
	if !strings.Contains(payload, `"content": "hello"`) {
		t.Fatalf("expected serialized content, got %s", payload)
	}

	var entries []MemoryEntry
	if err := json.Unmarshal([]byte(payload), &entries); err != nil {
		t.Fatalf("unmarshal serialized payload: %v", err)
	}
	if len(entries) != 1 || entries[0].Content != "hello" {
		t.Fatalf("unexpected entries: %+v", entries)
	}
}
