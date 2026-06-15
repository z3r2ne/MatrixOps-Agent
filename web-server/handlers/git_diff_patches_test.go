package handlers

import (
	"testing"

	"pkgs/db/models"

	"gorm.io/gorm"
)

func createDiffPatchSessionData(t *testing.T, db *gorm.DB, taskID uint, sessionID string, created int64, startSnapshot string, patchHash string, patchSnapshot string) {
	t.Helper()

	session := &models.Session{
		ID:            sessionID,
		ProjectID:     "1",
		Directory:     "/tmp/project",
		Title:         sessionID,
		Version:       "1",
		StartSnapshot: startSnapshot,
		Created:       created,
		Updated:       created,
	}
	if err := db.Create(session).Error; err != nil {
		t.Fatalf("create session %s: %v", sessionID, err)
	}

	execution := &models.TaskExecution{
		TaskID:         taskID,
		AgentSessionID: sessionID,
	}
	if err := db.Create(execution).Error; err != nil {
		t.Fatalf("create execution %s: %v", sessionID, err)
	}

	message := &models.Message{
		ID:        "msg-" + sessionID,
		SessionID: sessionID,
		Role:      "assistant",
		Name:      "assistant",
		Created:   created,
	}
	if err := db.Create(message).Error; err != nil {
		t.Fatalf("create message %s: %v", sessionID, err)
	}

	part := &models.Part{
		ID:        "part-" + sessionID,
		MessageID: message.ID,
		SessionID: sessionID,
		Type:      "patch",
	}
	if err := db.Create(part).Error; err != nil {
		t.Fatalf("create patch part %s: %v", sessionID, err)
	}

	snap := &models.MessageCodeSnapshot{
		ID:          "csnap-" + sessionID,
		SessionID:   sessionID,
		MessageID:   message.ID,
		PartID:      part.ID,
		StartHash:   patchHash,
		EndHash:     patchSnapshot,
		Created:     created,
		Description: "",
	}
	if err := db.Create(snap).Error; err != nil {
		t.Fatalf("create message code snapshot %s: %v", sessionID, err)
	}
}

func TestCollectTaskDiffPatchesIncludesAllTaskSessions(t *testing.T) {
	db := setupSessionHandlerTestDB(t)
	if err := db.AutoMigrate(&models.Task{}, &models.TaskExecution{}, &models.MessageCodeSnapshot{}); err != nil {
		t.Fatalf("auto migrate task tables: %v", err)
	}
	handler := NewGitHandler(db)

	task := &models.Task{
		ProjectID: 1,
		Content:   "task with multiple sessions",
		Status:    "done",
	}
	if err := db.Create(task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}

	createDiffPatchSessionData(t, db, task.ID, "session-1", 100, "start-a", "hash-a", "snap-a")
	createDiffPatchSessionData(t, db, task.ID, "session-2", 200, "start-b", "hash-b", "snap-b")

	firstStartSnapshot, patches, err := handler.collectTaskDiffPatches(task, "")
	if err != nil {
		t.Fatalf("collectTaskDiffPatches returned error: %v", err)
	}

	if firstStartSnapshot != "start-a" {
		t.Fatalf("firstStartSnapshot = %q, want %q", firstStartSnapshot, "start-a")
	}
	if len(patches) != 2 {
		t.Fatalf("expected 2 patches, got %d", len(patches))
	}
	if patches[0].SessionID != "session-1" || patches[1].SessionID != "session-2" {
		t.Fatalf("unexpected patch session order: %+v", patches)
	}
	if patches[0].StartSnapshot != "start-a" || patches[1].StartSnapshot != "start-b" {
		t.Fatalf("expected patches to include matching start snapshots: %+v", patches)
	}
}
