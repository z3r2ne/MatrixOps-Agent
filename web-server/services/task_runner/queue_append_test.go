package task_runner

import (
	"testing"

	database "pkgs/db"
	"pkgs/db/models"
	"pkgs/db/storage"
	"pkgs/testutil"
)

func TestTryConsumeAppendQueue_WritesMemoryWithoutLaunching(t *testing.T) {
	db := testutil.OpenTaskTestDB(t)
	task := testutil.CreateTask(t, db, func(task *models.Task) {
		task.Status = string(models.TaskStatusDone)
		task.SessionID = "session-append-queue"
		task.MessageQueue = []models.TaskMessageQueueItem{
			{
				ID:      "append-1",
				Type:    models.TaskMessageQueueTypeAppend,
				Content: "用户已切换工作目录为 /data/wt",
			},
		}
	})
	hub := NewRecordingWSHub()

	if err := TryConsumeAppendQueue(task.ID, WithDB(db), WithWSHub(hub)); err != nil {
		t.Fatalf("TryConsumeAppendQueue: %v", err)
	}

	queue, err := database.GetTaskQueue(db, task.ID)
	if err != nil {
		t.Fatalf("GetTaskQueue: %v", err)
	}
	if len(queue) != 0 {
		t.Fatalf("queue = %#v, want empty", queue)
	}

	entries, err := storage.ListMemoryEntriesBySession(db, task.SessionID)
	if err != nil {
		t.Fatalf("ListMemoryEntriesBySession: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected memory entries after append consumption")
	}
}

func TestTryConsumeAppendQueue_NoOpWhenRunning(t *testing.T) {
	db := testutil.OpenTaskTestDB(t)
	task := testutil.CreateTask(t, db, func(task *models.Task) {
		task.MessageQueue = []models.TaskMessageQueueItem{
			{ID: "append-1", Type: models.TaskMessageQueueTypeAppend, Content: "keep"},
		}
	})
	MarkTaskRunningForTest(task.ID)
	defer UnmarkTaskRunningForTest(task.ID)

	if err := TryConsumeAppendQueue(task.ID, WithDB(db), WithWSHub(NewRecordingWSHub())); err != nil {
		t.Fatalf("TryConsumeAppendQueue: %v", err)
	}
	queue, err := database.GetTaskQueue(db, task.ID)
	if err != nil {
		t.Fatalf("GetTaskQueue: %v", err)
	}
	if len(queue) != 1 {
		t.Fatalf("queue len = %d, want 1 while running", len(queue))
	}
}
