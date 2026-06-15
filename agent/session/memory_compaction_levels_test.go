package session

import (
	"testing"

	"matrixops-agent/types"
)

func TestIsL2CompactionCandidateIncludesUncompressedAndSkipsProtected(t *testing.T) {
	if !isL2CompactionCandidate(&types.MemoryEntry{ID: 1, CompressionLevel: 0, TokenCount: 10}) {
		t.Fatal("expected uncompressed entry to be candidate")
	}
	if isL2CompactionCandidate(&types.MemoryEntry{
		ID:               2,
		EntryKind:        memoryEntryKindCompactionUser,
		CompressionLevel: 2,
		TokenCount:       10,
	}) {
		t.Fatal("expected protected compaction user to be skipped")
	}
}

func TestSelectL2BatchEntriesUsesOldestCompressibleEntries(t *testing.T) {
	entries := []*types.MemoryEntry{
		{ID: 1, CompressionLevel: 0, TokenCount: 100},
		{ID: 2, CompressionLevel: 0, TokenCount: 500},
		{ID: 3, CompressionLevel: 2, TokenCount: 100},
		{ID: 4, CompressionLevel: 3, TokenCount: 50},
	}
	batch := selectL2BatchEntries(entries, 50)
	if len(batch) != 2 || batch[0].ID != 1 || batch[1].ID != 2 {
		t.Fatalf("unexpected batch: %#v", batch)
	}
}

func TestShouldRunGraduatedMemoryCompactionSkipsBelowTrigger(t *testing.T) {
	shouldRun, reason := shouldRunGraduatedMemoryCompaction(GraduatedMemoryCompactionInput{
		Entries:       []*types.MemoryEntry{{ID: 1, Role: "user", Content: "hi", TokenCount: 10}},
		LimitTokens:   1000,
		CurrentTokens: 10,
		Config: GraduatedMemoryCompactionConfig{
			TriggerThresholdPercent: 80,
			TargetPercent:           60,
			L2ScopePercent:          80,
		},
	})
	if shouldRun || reason != "below_trigger_threshold" {
		t.Fatalf("expected skip below trigger, got run=%v reason=%q", shouldRun, reason)
	}
}

func TestShouldRunGraduatedMemoryCompactionIgnoresTargetOnStart(t *testing.T) {
	shouldRun, reason := shouldRunGraduatedMemoryCompaction(GraduatedMemoryCompactionInput{
		Entries:       []*types.MemoryEntry{{ID: 1, Role: "user", Content: "hi", TokenCount: 10}},
		LimitTokens:   1000,
		CurrentTokens: 850,
		Config: GraduatedMemoryCompactionConfig{
			TriggerThresholdPercent: 80,
			TargetPercent:           60,
			L2ScopePercent:          80,
		},
	})
	if !shouldRun || reason != "" {
		t.Fatalf("expected run when usage exceeds trigger even if entries are below target, got run=%v reason=%q", shouldRun, reason)
	}
}

func TestRunGraduatedMemoryCompactionSkipsWhenBelowTrigger(t *testing.T) {
	result, err := RunGraduatedMemoryCompaction(GraduatedMemoryCompactionInput{
		SessionID:   "s1",
		Entries:     []*types.MemoryEntry{{ID: 1, Role: "user", Content: "hi", TokenCount: 10}},
		LimitTokens: 1000,
		Config: GraduatedMemoryCompactionConfig{
			TriggerThresholdPercent: 80,
			TargetPercent:           60,
			L2ScopePercent:          80,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Skipped || result.SkipReason != "below_trigger_threshold" {
		t.Fatalf("expected skip below trigger, got %#v", result)
	}
}
