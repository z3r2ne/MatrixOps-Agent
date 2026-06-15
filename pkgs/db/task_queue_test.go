package database

import (
	"testing"
	"time"

	"pkgs/db/models"
)

func TestPrependTaskQueueItem(t *testing.T) {
	db := openTaskTestDB(t)

	task := &models.Task{
		ProjectID:  1,
		Content:    "queue task",
		WorkerName: "chat",
		Status:     "running",
		SessionID:  "session-prepend",
	}
	if err := db.Create(task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}

	first := models.TaskMessageQueueItem{
		ID:        "queue-1",
		Content:   "first",
		CreatedAt: time.Now().UnixMilli(),
	}
	if err := AppendTaskQueueItem(db, task.ID, first); err != nil {
		t.Fatalf("append queue item: %v", err)
	}

	compaction := models.TaskMessageQueueItem{
		ID:        "queue-compact",
		Type:      models.TaskMessageQueueTypeMemoryCompaction,
		Content:   "请将历史记忆压缩成摘要",
		CreatedAt: time.Now().UnixMilli(),
	}
	if err := PrependTaskQueueItem(db, task.ID, compaction); err != nil {
		t.Fatalf("prepend queue item: %v", err)
	}

	queue, err := GetTaskQueue(db, task.ID)
	if err != nil {
		t.Fatalf("get task queue: %v", err)
	}
	if len(queue) != 2 {
		t.Fatalf("queue length = %d, want 2", len(queue))
	}
	if queue[0].ID != compaction.ID || queue[0].Type != models.TaskMessageQueueTypeMemoryCompaction {
		t.Fatalf("queue[0] = %#v, want compaction item first", queue[0])
	}
	if queue[1].ID != first.ID {
		t.Fatalf("queue[1] = %#v, want original first item second", queue[1])
	}
}
