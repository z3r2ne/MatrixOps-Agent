package storage

import (
	"testing"

	"matrixops-agent/types"
	"pkgs/db/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openSessionRetryTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := InitStorage(db); err != nil {
		t.Fatalf("init storage: %v", err)
	}
	if err := ensureMemoryEntriesSchema(db); err != nil {
		t.Fatalf("ensure memory schema: %v", err)
	}
	if err := ensureMessageCodeSnapshotSchema(db); err != nil {
		t.Fatalf("ensure code snapshot schema: %v", err)
	}
	return db
}

func TestRetryFromUserMessageRemovesMessageTail(t *testing.T) {
	db := openSessionRetryTestDB(t)
	sessionID := "session-retry"

	session := &models.Session{
		ID:        sessionID,
		ProjectID: "project-1",
		Title:     "retry session",
		Version:   "1",
		Created:   1,
		Updated:   1,
	}
	if err := db.Create(session).Error; err != nil {
		t.Fatalf("create session: %v", err)
	}

	user1 := &types.MessageInfo{ID: "m-user-1", SessionID: sessionID, Role: types.RoleUser, Time: types.MessageTime{Created: 100}}
	assistant1 := &types.MessageInfo{ID: "m-assistant-1", SessionID: sessionID, Role: types.RoleAssistant, Time: types.MessageTime{Created: 101}}
	user2 := &types.MessageInfo{ID: "m-user-2", SessionID: sessionID, Role: types.RoleUser, Time: types.MessageTime{Created: 200}}
	assistant2 := &types.MessageInfo{ID: "m-assistant-2", SessionID: sessionID, Role: types.RoleAssistant, Time: types.MessageTime{Created: 201}}

	for _, msg := range []*types.MessageInfo{user1, assistant1, user2, assistant2} {
		if err := UpdateMessageInfo(db, msg); err != nil {
			t.Fatalf("update message %s: %v", msg.ID, err)
		}
	}

	if _, err := UpdatePart(db, &types.Part{ID: "p-user-1", MessageID: user1.ID, SessionID: sessionID, Type: types.PartTypeText, Text: "first"}); err != nil {
		t.Fatalf("part user1: %v", err)
	}
	if _, err := UpdatePart(db, &types.Part{ID: "p-user-2-text", MessageID: user2.ID, SessionID: sessionID, Type: types.PartTypeText, Text: "retry me"}); err != nil {
		t.Fatalf("part user2 text: %v", err)
	}
	if _, err := UpdatePart(db, &types.Part{ID: "p-user-2-file", MessageID: user2.ID, SessionID: sessionID, Type: "file", URL: "data:text/plain;base64,Zm9v", Filename: "note.txt", Mime: "text/plain"}); err != nil {
		t.Fatalf("part user2 file: %v", err)
	}
	if _, err := UpdatePart(db, &types.Part{ID: "p-assistant-2", MessageID: assistant2.ID, SessionID: sessionID, Type: types.PartTypeText, Text: "later"}); err != nil {
		t.Fatalf("part assistant2: %v", err)
	}

	if err := CreateMemoryEntry(db, &types.MemoryEntry{SessionID: sessionID, SourceMessageID: user2.ID, EntryKind: "history", Role: "user", Content: "retry me", Sequence: 1, Created: 200, Updated: 200}); err != nil {
		t.Fatalf("create memory: %v", err)
	}
	if err := UpsertMessagePromptSnapshot(db, assistant2.ID, sessionID, "prompt", "response"); err != nil {
		t.Fatalf("create prompt snapshot: %v", err)
	}
	if err := CreateMessageCodeSnapshot(db, &models.MessageCodeSnapshot{ID: "code-1", SessionID: sessionID, MessageID: assistant2.ID, PartID: "p-assistant-2", StartHash: "a", EndHash: "b", Created: 201}); err != nil {
		t.Fatalf("create code snapshot: %v", err)
	}

	result, err := RetryFromUserMessage(db, sessionID, user2.ID)
	if err != nil {
		t.Fatalf("RetryFromUserMessage: %v", err)
	}
	if result.Text != "retry me" {
		t.Fatalf("retry text = %q", result.Text)
	}
	if len(result.Parts) != 1 || result.Parts[0].Type != "file" {
		t.Fatalf("expected preserved non-text user parts, got %+v", result.Parts)
	}

	messages, err := GetMessageWithPartsBySessionIDLight(db, sessionID)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("message count = %d, want 2", len(messages))
	}
	if messages[0].Info.ID != user1.ID || messages[1].Info.ID != assistant1.ID {
		t.Fatalf("unexpected remaining messages: %+v", messages)
	}

	memoryEntries, err := ListMemoryEntriesBySession(db, sessionID)
	if err != nil {
		t.Fatalf("list memory: %v", err)
	}
	if len(memoryEntries) != 0 {
		t.Fatalf("expected tail memory entries to be removed, got %+v", memoryEntries)
	}

	if _, err := GetPromptSnapshotByMessageID(db, assistant2.ID); err == nil {
		t.Fatal("expected prompt snapshot for removed tail to be deleted")
	}
	if _, err := GetMessageCodeSnapshotByID(db, "code-1"); err == nil {
		t.Fatal("expected code snapshot for removed tail to be deleted")
	}
}
