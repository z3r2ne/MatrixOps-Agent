package handlers

import (
	"testing"
	"time"

	database "pkgs/db"
	"pkgs/db/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupHandlerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	if err := db.AutoMigrate(&models.Task{}, &models.TaskExecution{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	return db
}

func TestResolveTaskSessionIDPrefersExistingTaskSession(t *testing.T) {
	db := setupHandlerTestDB(t)

	task := &models.Task{
		ProjectID: 1,
		Content:   "test task",
		SessionID: "task-session",
		Status:    "done",
	}
	if err := db.Create(task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}

	got := resolveTaskSessionID(db, task)
	if got != "task-session" {
		t.Fatalf("sessionID = %q, want %q", got, "task-session")
	}
}

func TestResolveTaskSessionIDFallsBackToLatestExecution(t *testing.T) {
	db := setupHandlerTestDB(t)

	task := &models.Task{
		ProjectID: 1,
		Content:   "test task",
		Status:    "done",
	}
	if err := db.Create(task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}

	olderExecution := &models.TaskExecution{
		TaskID:         task.ID,
		AgentSessionID: "older-session",
		CreatedAt:      time.Now().Add(-time.Minute),
	}
	if err := db.Create(olderExecution).Error; err != nil {
		t.Fatalf("create older execution: %v", err)
	}

	latestExecution := &models.TaskExecution{
		TaskID:         task.ID,
		AgentSessionID: "latest-session",
		CreatedAt:      time.Now(),
	}
	if err := db.Create(latestExecution).Error; err != nil {
		t.Fatalf("create latest execution: %v", err)
	}

	got := resolveTaskSessionID(db, task)
	if got != "latest-session" {
		t.Fatalf("sessionID = %q, want %q", got, "latest-session")
	}

	refreshedTask, err := database.GetTaskByID(db, task.ID)
	if err != nil {
		t.Fatalf("reload task: %v", err)
	}
	if refreshedTask.SessionID != "latest-session" {
		t.Fatalf("persisted sessionID = %q, want %q", refreshedTask.SessionID, "latest-session")
	}
}
