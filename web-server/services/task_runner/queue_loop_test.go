package task_runner

import (
	"testing"

	database "pkgs/db"
	"pkgs/db/models"
	"pkgs/taskqueue"
	"pkgs/testutil"
)

func TestDequeueNextMessageQueueItem_RemovesHeadAndBroadcasts(t *testing.T) {
	db := testutil.OpenTaskTestDB(t)
	task := testutil.CreateTask(t, db, func(task *models.Task) {
		task.MessageQueue = []models.TaskMessageQueueItem{
			{ID: "first", Content: "one"},
			{ID: "second", Content: "two"},
		}
	})

	hub := NewRecordingWSHub()
	runtime := &TaskRuntime{
		taskID:       task.ID,
		db:           db,
		wsHub:        hub,
		messageQueue: taskqueue.New(db, task.ID, hub.BroadcastTaskQueue),
	}

	item, err := runtime.dequeueNextMessageQueueItem()
	if err != nil {
		t.Fatalf("dequeueNextMessageQueueItem: %v", err)
	}
	if item == nil || item.ID != "first" {
		t.Fatalf("item = %#v, want first", item)
	}

	queue, err := database.GetTaskQueue(db, task.ID)
	if err != nil {
		t.Fatalf("GetTaskQueue: %v", err)
	}
	if len(queue) != 1 || queue[0].ID != "second" {
		t.Fatalf("remaining queue = %#v", queue)
	}
	if got := hub.LastTaskQueue(task.ID); len(got) != 1 || got[0].ID != "second" {
		t.Fatalf("broadcast queue = %#v", got)
	}
}

func TestTaskQueueItemToInputParts_ConvertsFileParts(t *testing.T) {
	item := &models.TaskMessageQueueItem{
		Content: "with file",
		Parts: []models.TaskInputPart{
			{Type: "file", URL: "data:image/png;base64,abc", Mime: "image/png", Filename: "x.png"},
		},
	}
	parts := taskQueueItemToInputParts(item)
	if len(parts) != 1 || parts[0].URL != "data:image/png;base64,abc" {
		t.Fatalf("parts = %#v", parts)
	}
}
