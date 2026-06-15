package session

import (
	"testing"

	"matrixops-agent/types"
	"pkgs/db/models"
	"pkgs/db/storage"
	"pkgs/taskqueue"
	"pkgs/testutil"
)

func TestEnsureRuntimeMemoryState_InitializesFromEmptyBase(t *testing.T) {
	runner := &AgentRunner{}
	runtimeConfig := &RuntimeConfig{}
	if err := runner.ensureRuntimeMemoryState(runtimeConfig); err != nil {
		t.Fatalf("ensureRuntimeMemoryState: %v", err)
	}
	if runtimeConfig.MemoryState == nil {
		t.Fatal("expected memory state")
	}
	if runtimeConfig.MemoryState.Snapshot() == nil {
		t.Fatal("expected memory snapshot")
	}
}

func TestDeliverSupplementUserMessage_CreatesMessageAndSyncsMemory(t *testing.T) {
	db := testutil.OpenTaskTestDB(t)
	task := testutil.CreateTask(t, db)
	sessionID := "session-supplement-test"

	runner := &AgentRunner{
		db:      db,
		task:    task,
		session: &types.Info{ID: sessionID, ProjectID: "1", Directory: t.TempDir()},
		emitter: NewEmitter(db, sessionID),
	}
	runtimeConfig := &RuntimeConfig{
		Worker:        &models.Worker{Name: "chat"},
		ModelSettings: &models.ModelSettings{Name: "default_model_config"},
		LLMConfig:     &models.LLMConfig{Name: "test-llm"},
		Assistant: &MessageInfo{
			ID:        "assistant-1",
			SessionID: sessionID,
			Role:      RoleAssistant,
			Time:      MessageTime{Created: 1},
		},
		MemoryState: NewProcessV2MemoryState(nil),
	}

	item := models.TaskMessageQueueItem{
		ID:      "sup-1",
		Content: "补充说明",
	}
	if err := runner.deliverSupplementUserMessage(runtimeConfig, item); err != nil {
		t.Fatalf("deliverSupplementUserMessage: %v", err)
	}

	messages, err := storage.GetMessageWithPartsBySessionIDLight(db, sessionID)
	if err != nil {
		t.Fatalf("GetMessageWithPartsBySessionIDLight: %v", err)
	}
	userCount := 0
	for _, msg := range messages {
		if msg != nil && msg.Info != nil && msg.Info.Role == RoleUser {
			userCount++
		}
	}
	if userCount != 1 {
		t.Fatalf("expected 1 user message, got %d", userCount)
	}
	for _, msg := range messages {
		if msg != nil && msg.Info != nil && msg.Info.Role == RoleUser {
			if msg.Info.MessageKind != MessageKindUser {
				t.Fatalf("expected user messageKind, got %q", msg.Info.MessageKind)
			}
		}
	}

	entries, err := storage.ListMemoryEntriesBySession(db, sessionID)
	if err != nil {
		t.Fatalf("ListMemoryEntriesBySession: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected memory entries after supplement user message")
	}
	if len(runtimeConfig.MemoryState.Snapshot().Entries) == 0 {
		t.Fatal("expected runtime memory entries synced from db")
	}
}

func TestDeliverSupplementUserMessage_SystemQueueItemSetsMessageKind(t *testing.T) {
	db := testutil.OpenTaskTestDB(t)
	task := testutil.CreateTask(t, db)
	sessionID := "session-supplement-system"

	runner := &AgentRunner{
		db:      db,
		task:    task,
		session: &types.Info{ID: sessionID, ProjectID: "1", Directory: t.TempDir()},
		emitter: NewEmitter(db, sessionID),
	}
	runtimeConfig := &RuntimeConfig{
		Worker:        &models.Worker{Name: "chat"},
		ModelSettings: &models.ModelSettings{Name: "default_model_config"},
		LLMConfig:     &models.LLMConfig{Name: "test-llm"},
		Assistant: &MessageInfo{
			ID:        "assistant-1",
			SessionID: sessionID,
			Role:      RoleAssistant,
			Time:      MessageTime{Created: 1},
		},
		MemoryState: NewProcessV2MemoryState(nil),
	}

	item := models.TaskMessageQueueItem{
		ID:         "sup-sys",
		Type:       models.TaskMessageQueueTypeSystem,
		Content:    "⏰ 提醒：测试",
		Source:     models.TaskMessageQueueSourceReminder,
		Supplement: true,
	}
	if err := runner.deliverSupplementUserMessage(runtimeConfig, item); err != nil {
		t.Fatalf("deliverSupplementUserMessage: %v", err)
	}

	messages, err := storage.GetMessageWithPartsBySessionIDLight(db, sessionID)
	if err != nil {
		t.Fatalf("GetMessageWithPartsBySessionIDLight: %v", err)
	}
	var found *MessageInfo
	for _, msg := range messages {
		if msg != nil && msg.Info != nil && msg.Info.Role == RoleUser {
			found = msg.Info
			break
		}
	}
	if found == nil {
		t.Fatal("expected system supplement message")
	}
	if found.MessageKind != MessageKindSystem {
		t.Fatalf("messageKind = %q, want system", found.MessageKind)
	}
	if found.MessageOrigin != models.TaskMessageQueueSourceReminder {
		t.Fatalf("messageOrigin = %q", found.MessageOrigin)
	}

	entries, err := storage.ListMemoryEntriesBySession(db, sessionID)
	if err != nil {
		t.Fatalf("ListMemoryEntriesBySession: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected memory entries after supplement user message")
	}
	if len(runtimeConfig.MemoryState.Snapshot().Entries) == 0 {
		t.Fatal("expected runtime memory entries synced from db")
	}
}

func TestMessageQueue_ConsumeSupplementsDelegatesToHandler(t *testing.T) {
	db := testutil.OpenTaskTestDB(t)
	task := testutil.CreateTask(t, db, func(task *models.Task) {
		task.MessageQueue = []models.TaskMessageQueueItem{
			{ID: "sup-1", Content: "via queue", Supplement: true},
		}
	})
	q := taskqueue.New(db, task.ID, nil)
	var got string
	_, err := q.ConsumeSupplements(func(item models.TaskMessageQueueItem) error {
		got = item.Content
		return nil
	})
	if err != nil {
		t.Fatalf("ConsumeSupplements: %v", err)
	}
	if got != "via queue" {
		t.Fatalf("got content %q", got)
	}
}
