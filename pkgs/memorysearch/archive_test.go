package memorysearch

import (
	"testing"

	"matrixops-agent/types"
)

func TestFilterEntriesPendingSearchArchive(t *testing.T) {
	entries := []*types.MemoryEntry{
		{ID: 1, Content: "already archived", SearchArchived: true},
		{ID: 2, Content: "fresh"},
		nil,
		{ID: 3, Content: "also fresh"},
	}

	pending := filterEntriesPendingSearchArchive(entries)
	if len(pending) != 2 {
		t.Fatalf("expected 2 pending entries, got %d", len(pending))
	}
	if pending[0].ID != 2 || pending[1].ID != 3 {
		t.Fatalf("unexpected pending entries: %+v", pending)
	}
}
