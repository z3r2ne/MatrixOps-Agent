package database

import (
	"testing"
	"time"

	"pkgs/db/models"
	"pkgs/testutil"

	"gorm.io/gorm"
)

func openTaskTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	return testutil.OpenTaskTestDB(t)
}

func TestBackfillSubtaskParentTaskIDs(t *testing.T) {
	db := openTaskTestDB(t)

	parent := &models.Task{
		ProjectID:  1,
		Content:    "parent task",
		WorkerName: "chat",
		Status:     "done",
		SessionID:  "session-parent",
	}
	if err := db.Create(parent).Error; err != nil {
		t.Fatalf("create parent task: %v", err)
	}

	child := &models.Task{
		ProjectID:  1,
		Content:    "child task",
		WorkerName: "explore",
		Status:     "done",
		SessionID:  "session-child",
	}
	if err := db.Create(child).Error; err != nil {
		t.Fatalf("create child task: %v", err)
	}

	root := &models.Task{
		ProjectID:  1,
		Content:    "root task",
		WorkerName: "chat",
		Status:     "done",
		SessionID:  "session-root",
	}
	if err := db.Create(root).Error; err != nil {
		t.Fatalf("create root task: %v", err)
	}

	part := &models.Part{
		ID:        "part-1",
		MessageID: "message-1",
		SessionID: parent.SessionID,
		Type:      "tool",
		Tool: models.JSONField{Data: map[string]interface{}{
			"tool": "run_worker_task",
			"state": map[string]interface{}{
				"metadata": map[string]interface{}{
					"subtaskTaskId": child.ID,
				},
			},
		}},
	}
	if err := db.Create(part).Error; err != nil {
		t.Fatalf("create tool part: %v", err)
	}

	if err := BackfillSubtaskParentTaskIDs(db); err != nil {
		t.Fatalf("backfill parent task ids: %v", err)
	}

	var freshChild models.Task
	if err := db.First(&freshChild, child.ID).Error; err != nil {
		t.Fatalf("reload child task: %v", err)
	}
	if freshChild.ParentTaskID == nil || *freshChild.ParentTaskID != parent.ID {
		t.Fatalf("child parentTaskID = %#v, want %d", freshChild.ParentTaskID, parent.ID)
	}

	var freshRoot models.Task
	if err := db.First(&freshRoot, root.ID).Error; err != nil {
		t.Fatalf("reload root task: %v", err)
	}
	if freshRoot.ParentTaskID != nil {
		t.Fatalf("expected unrelated root task to remain root, got %#v", freshRoot.ParentTaskID)
	}
}

func TestTaskMessageQueueRoundTrip(t *testing.T) {
	db := openTaskTestDB(t)

	task := &models.Task{
		ProjectID:  1,
		Content:    "queue task",
		WorkerName: "chat",
		Status:     "running",
		SessionID:  "session-queue",
	}
	if err := db.Create(task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}

	first := models.TaskMessageQueueItem{
		ID:        "queue-1",
		Content:   "first queued message",
		Parts:     []models.TaskInputPart{{Type: "image", URL: "https://example.com/a.png", Mime: "image/png", Filename: "a.png"}},
		CreatedAt: time.Now().UnixMilli(),
	}
	if err := AppendTaskQueueItem(db, task.ID, first); err != nil {
		t.Fatalf("append queue item: %v", err)
	}

	queue, err := GetTaskQueue(db, task.ID)
	if err != nil {
		t.Fatalf("get task queue after append: %v", err)
	}
	if len(queue) != 1 {
		t.Fatalf("queue length after append = %d, want 1", len(queue))
	}
	if queue[0].ID != first.ID || queue[0].Content != first.Content {
		t.Fatalf("queue[0] = %#v, want %#v", queue[0], first)
	}
	if len(queue[0].Parts) != 1 || queue[0].Parts[0].URL != first.Parts[0].URL {
		t.Fatalf("queue[0].Parts = %#v, want %#v", queue[0].Parts, first.Parts)
	}

	replacement := []models.TaskMessageQueueItem{
		{
			ID:        "queue-2",
			Content:   "replacement message",
			CreatedAt: time.Now().UnixMilli(),
		},
	}
	if err := UpdateTaskQueue(db, task.ID, replacement); err != nil {
		t.Fatalf("update task queue: %v", err)
	}

	queue, err = GetTaskQueue(db, task.ID)
	if err != nil {
		t.Fatalf("get task queue after update: %v", err)
	}
	if len(queue) != 1 {
		t.Fatalf("queue length after update = %d, want 1", len(queue))
	}
	if queue[0].ID != replacement[0].ID || queue[0].Content != replacement[0].Content {
		t.Fatalf("queue[0] after update = %#v, want %#v", queue[0], replacement[0])
	}
}
