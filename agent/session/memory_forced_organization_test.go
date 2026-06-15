package session

import (
	"strings"
	"testing"

	agenttoken "matrixops-agent/token"
	"matrixops-agent/types"
	coreagent "matrixops.local/core_agent"
	"pkgs/db/models"
	"pkgs/db/storage"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupMemoryForcedOrganizationTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := storage.InitStorage(db); err != nil {
		t.Fatalf("init storage: %v", err)
	}
	return db
}

func TestMemoryCompactionTokenLimitPrefersAutoLimit(t *testing.T) {
	limit := memoryCompactionTokenLimit(&RuntimeConfig{
		AutoCompressionLimitTokens: 64000,
		ModelSettings:              &models.ModelSettings{ContextLimit: 128000},
	})
	if limit != 64000 {
		t.Fatalf("expected auto limit, got %d", limit)
	}
}

func TestMemoryCompactionTokenLimitFallsBackToModelContextWindow(t *testing.T) {
	limit := memoryCompactionTokenLimit(&RuntimeConfig{
		ModelSettings: &models.ModelSettings{
			ContextLimit: 128000,
			OutputLimit:  8000,
		},
	})
	if limit != 120000 {
		t.Fatalf("expected model context window fallback, got %d", limit)
	}
}

func TestCurrentContextTokensFromUsagePrefersAssistantTokens(t *testing.T) {
	tokens := currentContextTokensFromUsage(&RuntimeConfig{
		SessionTokens: &MessageTokens{Input: 100, Cache: TokenCache{Read: 20}},
		Assistant: &MessageInfo{
			Tokens: &MessageTokens{Input: 300, Cache: TokenCache{Read: 40}},
		},
	})
	if tokens != 340 {
		t.Fatalf("expected assistant usage tokens, got %d", tokens)
	}
}

func TestCurrentContextTokensFromUsageFallsBackToSessionTokens(t *testing.T) {
	tokens := currentContextTokensFromUsage(&RuntimeConfig{
		SessionTokens: &MessageTokens{Input: 100, Cache: TokenCache{Read: 20}},
	})
	if tokens != 120 {
		t.Fatalf("expected session usage tokens, got %d", tokens)
	}
}

func TestSyncRuntimeMemoryTokensUpdatesAssistantAndSession(t *testing.T) {
	runtimeConfig := &RuntimeConfig{
		SessionTokens: &MessageTokens{Input: 999},
		Assistant:     &MessageInfo{Tokens: &MessageTokens{Input: 888}},
	}
	syncRuntimeMemoryTokens(runtimeConfig, []*types.MemoryEntry{
		{Role: "assistant", Content: "summary", TokenCount: 42},
	})
	if runtimeConfig.SessionTokens == nil || runtimeConfig.SessionTokens.Input != 42 {
		t.Fatalf("expected session tokens synced to 42, got %#v", runtimeConfig.SessionTokens)
	}
	if runtimeConfig.Assistant.Tokens == nil || runtimeConfig.Assistant.Tokens.Input != 42 {
		t.Fatalf("expected assistant tokens synced to 42, got %#v", runtimeConfig.Assistant.Tokens)
	}
}

func TestReloadProcessV2MemoryEntriesFromDBRefreshesStaleMemoryState(t *testing.T) {
	db := setupMemoryForcedOrganizationTestDB(t)
	sessionID := "session-reload-memory"

	dbEntries := []*types.MemoryEntry{
		{SessionID: sessionID, SourceMessageID: "msg-1", SourcePartID: "part-1", EntryKind: "text", Role: "user", Content: "older", Sequence: 1, TokenCount: 5},
		{SessionID: sessionID, SourceMessageID: "msg-2", SourcePartID: "part-2", EntryKind: "text", Role: "user", Content: "yarn dev port question", Sequence: 2, TokenCount: 8},
		{SessionID: sessionID, SourceMessageID: "msg-3", SourcePartID: "part-3", EntryKind: "tool_call", Role: "assistant", ToolName: "read", ToolOutput: "ok", Sequence: 3, TokenCount: 4},
	}
	if err := storage.ReplaceSessionMemoryWithEntries(db, sessionID, dbEntries); err != nil {
		t.Fatalf("seed db memory entries: %v", err)
	}

	runtimeConfig := &RuntimeConfig{
		MemoryState: NewProcessV2MemoryState(&types.Memory{
			Entries: []*types.MemoryEntry{
				{SessionID: sessionID, SourceMessageID: "msg-1", SourcePartID: "part-1", EntryKind: "text", Role: "user", Content: "older", Sequence: 1, TokenCount: 5},
			},
		}),
	}
	runner := &AgentRunner{
		db:      db,
		session: &Info{ID: sessionID},
	}
	if err := runner.reloadProcessV2MemoryEntriesFromDB(runtimeConfig); err != nil {
		t.Fatalf("reloadProcessV2MemoryEntriesFromDB: %v", err)
	}

	snapshot := runtimeConfig.MemoryState.Snapshot()
	got := snapshot.TranscriptSourceEntries()
	if len(got) != 3 {
		t.Fatalf("expected 3 entries after reload, got %d: %#v", len(got), got)
	}
	if got[1].Content != "yarn dev port question" {
		t.Fatalf("unexpected reloaded entry: %#v", got[1])
	}
}

func TestShouldPersistGraduatedCompactionResult(t *testing.T) {
	before := []*types.MemoryEntry{
		{ID: 1, Role: "user", Content: "a", TokenCount: 10},
		{ID: 2, Role: "assistant", Content: "b", TokenCount: 20},
	}

	t.Run("skipped", func(t *testing.T) {
		if shouldPersistGraduatedCompactionResult(before, before, GraduatedMemoryCompactionResult{Skipped: true}) {
			t.Fatal("expected false for skipped compaction")
		}
	})

	t.Run("no_op_same_entries", func(t *testing.T) {
		if shouldPersistGraduatedCompactionResult(before, before, GraduatedMemoryCompactionResult{}) {
			t.Fatal("expected false when compaction made no changes")
		}
	})

	t.Run("level_executed", func(t *testing.T) {
		if !shouldPersistGraduatedCompactionResult(before, before, GraduatedMemoryCompactionResult{LevelsExecuted: []int{1}}) {
			t.Fatal("expected true when a compression level ran")
		}
	})

	t.Run("entry_count_changed", func(t *testing.T) {
		after := append([]*types.MemoryEntry(nil), before[:1]...)
		if !shouldPersistGraduatedCompactionResult(before, after, GraduatedMemoryCompactionResult{}) {
			t.Fatal("expected true when entry count changed")
		}
	})
}

func TestForceOrganizeMemoryFallsBackToEntryTokensWhenUsageMissing(t *testing.T) {
	entries := []*types.MemoryEntry{
		{ID: 1, Role: "assistant", EntryKind: "tool_call", ToolName: "read", ToolOutput: strings.Repeat("x", 4000), TokenCount: 90000},
		{ID: 2, Role: "assistant", EntryKind: "tool_call", ToolName: "read", ToolOutput: strings.Repeat("y", 4000), TokenCount: 90000},
	}
	runtimeConfig := &RuntimeConfig{
		UserInput: "explore",
		ModelSettings: &models.ModelSettings{
			ContextLimit: 128000,
			OutputLimit:  8000,
		},
	}

	currentTokens := currentContextTokensFromUsage(runtimeConfig)
	if currentTokens != 0 {
		t.Fatalf("expected no usage tokens before first LLM call, got %d", currentTokens)
	}

	fallback := totalMemoryTokens(entries) + agenttoken.Estimate(strings.TrimSpace(runtimeConfig.UserInput))
	if fallback <= 0 {
		t.Fatal("expected positive fallback token estimate")
	}

	limit := memoryCompactionTokenLimit(runtimeConfig)
	shouldRun, _ := shouldRunGraduatedMemoryCompaction(GraduatedMemoryCompactionInput{
		Entries:       entries,
		LimitTokens:   limit,
		CurrentTokens: fallback,
		Config: GraduatedMemoryCompactionConfig{
			TriggerThresholdPercent: 80,
			TargetPercent:           60,
			L2ScopePercent:          80,
		},
	})
	if !shouldRun {
		t.Fatalf("expected compaction to trigger with fallback tokens=%d limit=%d", fallback, limit)
	}
}

func TestResolveV2PromptToolsOmitsDedicatedMemoryOrganizationTool(t *testing.T) {
	tools := resolveV2PromptTools(&RuntimeConfig{
		ActionSchemas: []coreagent.ActionSchema{{ActionName: "memory_organization"}},
	})
	for _, tool := range tools {
		if tool.Name == "memory_organization" {
			t.Fatalf("memory organization tool should not be registered")
		}
	}
}
