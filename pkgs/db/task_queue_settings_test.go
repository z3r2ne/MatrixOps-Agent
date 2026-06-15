package database

import (
	"testing"

	"pkgs/db/models"
	"pkgs/testutil"
)

func TestGetTaskQueueSettings_ReturnsAutoSendFlag(t *testing.T) {
	db := testutil.OpenTaskTestDB(t)
	task := testutil.CreateTask(t, db, func(task *models.Task) {
		task.MessageQueue = []models.TaskMessageQueueItem{
			{ID: "q1", Content: "one"},
		}
	})
	if err := SetTaskMessageQueueAutoSend(db, task.ID, false); err != nil {
		t.Fatalf("SetTaskMessageQueueAutoSend: %v", err)
	}

	queue, autoSend, err := GetTaskQueueSettings(db, task.ID)
	if err != nil {
		t.Fatalf("GetTaskQueueSettings: %v", err)
	}
	if autoSend {
		t.Fatal("expected autoSend=false")
	}
	if len(queue) != 1 || queue[0].Content != "one" {
		t.Fatalf("queue = %#v", queue)
	}
}

func TestSetTaskMessageQueueAutoSend_Persists(t *testing.T) {
	db := testutil.OpenTaskTestDB(t)
	task := testutil.CreateTask(t, db)

	if err := SetTaskMessageQueueAutoSend(db, task.ID, false); err != nil {
		t.Fatalf("SetTaskMessageQueueAutoSend: %v", err)
	}
	_, autoSend, err := GetTaskQueueSettings(db, task.ID)
	if err != nil {
		t.Fatalf("GetTaskQueueSettings: %v", err)
	}
	if autoSend {
		t.Fatal("expected autoSend=false after update")
	}
}
