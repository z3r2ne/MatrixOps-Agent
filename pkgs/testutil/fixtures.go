package testutil

import (
	"testing"

	"pkgs/db/models"

	"gorm.io/gorm"
)

// CreateTask 在数据库中创建一条最小可用的任务记录。
func CreateTask(t *testing.T, db *gorm.DB, opts ...func(*models.Task)) *models.Task {
	t.Helper()
	task := &models.Task{
		ProjectID:            1,
		Content:              "test task",
		WorkerName:           "chat",
		Status:               string(models.TaskStatusQueue),
		SessionID:            "session-test",
		MessageQueue:         []models.TaskMessageQueueItem{},
		MessageQueueAutoSend: true,
	}
	for _, opt := range opts {
		opt(task)
	}
	if err := db.Create(task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}
	return task
}
