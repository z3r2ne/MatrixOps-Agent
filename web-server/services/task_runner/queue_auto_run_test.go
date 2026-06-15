package task_runner

import (
	"testing"

	database "pkgs/db"
	"pkgs/db/models"
	"pkgs/testutil"
)

func TestTryAutoRunTaskQueue_NoOpWhenRunning(t *testing.T) {
	db := testutil.OpenTaskTestDB(t)
	task := testutil.CreateTask(t, db, func(task *models.Task) {
		task.Status = string(models.TaskStatusDone)
		task.MessageQueue = []models.TaskMessageQueueItem{{ID: "q1", Content: "hello"}}
	})
	MarkTaskRunningForTest(task.ID)
	defer UnmarkTaskRunningForTest(task.ID)

	if err := TryAutoRunTaskQueue(task.ID, WithDB(db), WithWSHub(NewRecordingWSHub())); err != nil {
		t.Fatalf("TryAutoRunTaskQueue: %v", err)
	}
	queue, err := database.GetTaskQueue(db, task.ID)
	if err != nil {
		t.Fatalf("GetTaskQueue: %v", err)
	}
	if len(queue) != 1 || queue[0].ID != "q1" {
		t.Fatalf("queue should be unchanged: %#v", queue)
	}
}

func TestTryAutoRunTaskQueue_NoOpWhenAutoSendDisabled(t *testing.T) {
	db := testutil.OpenTaskTestDB(t)
	task := testutil.CreateTask(t, db, func(task *models.Task) {
		task.Status = string(models.TaskStatusDone)
		task.MessageQueue = []models.TaskMessageQueueItem{{ID: "q1", Content: "hello"}}
	})
	if err := database.SetTaskMessageQueueAutoSend(db, task.ID, false); err != nil {
		t.Fatalf("SetTaskMessageQueueAutoSend: %v", err)
	}

	if err := TryAutoRunTaskQueue(task.ID, WithDB(db), WithWSHub(NewRecordingWSHub())); err != nil {
		t.Fatalf("TryAutoRunTaskQueue: %v", err)
	}
	queue, err := database.GetTaskQueue(db, task.ID)
	if err != nil {
		t.Fatalf("GetTaskQueue: %v", err)
	}
	if len(queue) != 1 {
		t.Fatalf("queue len = %d, want 1", len(queue))
	}
}

func TestTryAutoRunTaskQueue_DequeuesHeadWhenEligible(t *testing.T) {
	db := testutil.OpenTaskTestDB(t)
	task := testutil.CreateTask(t, db, func(task *models.Task) {
		task.Status = string(models.TaskStatusDone)
		task.MessageQueue = []models.TaskMessageQueueItem{
			{ID: "empty", Content: "   "},
			{ID: "next", Content: "run me"},
		}
	})
	hub := NewRecordingWSHub()

	if err := TryAutoRunTaskQueue(task.ID, WithDB(db), WithWSHub(hub)); err != nil {
		t.Fatalf("TryAutoRunTaskQueue: %v", err)
	}

	queue, err := database.GetTaskQueue(db, task.ID)
	if err != nil {
		t.Fatalf("GetTaskQueue: %v", err)
	}
	if len(queue) != 0 {
		t.Fatalf("queue after auto run = %#v, want empty (head dequeued for launch)", queue)
	}
}

func TestTaskStatusAllowsQueueAutoRun(t *testing.T) {
	if !taskStatusAllowsQueueAutoRun(string(models.TaskStatusDone)) {
		t.Fatal("done should allow auto run")
	}
	if taskStatusAllowsQueueAutoRun(string(models.TaskStatusFailed)) {
		t.Fatal("failed should not allow auto run")
	}
}
