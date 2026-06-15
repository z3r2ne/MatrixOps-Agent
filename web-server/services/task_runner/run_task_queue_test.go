package task_runner

import (
	"testing"

	database "pkgs/db"
	"pkgs/db/models"
	"pkgs/testutil"

	"matrixops-agent/types"
)

func TestRunTask_EnqueuesWhenTaskRunning(t *testing.T) {
	db := testutil.OpenTaskTestDB(t)
	task := testutil.CreateTask(t, db)
	hub := NewRecordingWSHub()

	MarkTaskRunningForTest(task.ID)
	defer UnmarkTaskRunningForTest(task.ID)

	err := RunTask(task.ID,
		WithDB(db),
		WithWSHub(hub),
		WithContent("queued while running"),
		WithInputSource(TaskInputSourceFrontend),
	)
	if err != nil {
		t.Fatalf("RunTask: %v", err)
	}

	queue, err := database.GetTaskQueue(db, task.ID)
	if err != nil {
		t.Fatalf("GetTaskQueue: %v", err)
	}
	if len(queue) != 1 {
		t.Fatalf("queue len = %d, want 1", len(queue))
	}
	if queue[0].Content != "queued while running" {
		t.Fatalf("queue content = %q", queue[0].Content)
	}
	if queue[0].Type != models.TaskMessageQueueTypeUser {
		t.Fatalf("queue type = %q, want user", queue[0].Type)
	}
	if queue[0].Source != TaskInputSourceFrontend {
		t.Fatalf("queue source = %q, want frontend", queue[0].Source)
	}
	if len(hub.LastTaskQueue(task.ID)) != 1 {
		t.Fatalf("expected queue broadcast")
	}
}

func TestRunTask_EnqueuesWithAttachmentsWhenTaskRunning(t *testing.T) {
	db := testutil.OpenTaskTestDB(t)
	task := testutil.CreateTask(t, db)
	hub := NewRecordingWSHub()

	MarkTaskRunningForTest(task.ID)
	defer UnmarkTaskRunningForTest(task.ID)

	err := RunTask(task.ID,
		WithDB(db),
		WithWSHub(hub),
		WithInputParts([]*types.Part{{
			Type:     "file",
			URL:      "data:image/png;base64,x",
			Mime:     "image/png",
			Filename: "a.png",
		}}),
	)
	if err != nil {
		t.Fatalf("RunTask: %v", err)
	}

	queue, err := database.GetTaskQueue(db, task.ID)
	if err != nil {
		t.Fatalf("GetTaskQueue: %v", err)
	}
	if len(queue) != 1 || len(queue[0].Parts) != 1 {
		t.Fatalf("unexpected queue: %#v", queue)
	}
}

func TestRunTask_UsesCustomQueueItemID(t *testing.T) {
	db := testutil.OpenTaskTestDB(t)
	task := testutil.CreateTask(t, db)
	hub := NewRecordingWSHub()

	MarkTaskRunningForTest(task.ID)
	defer UnmarkTaskRunningForTest(task.ID)

	const customID = "wechat-bot-123"
	err := RunTask(task.ID,
		WithDB(db),
		WithWSHub(hub),
		WithContent("from ilink"),
		WithInputSource(TaskInputSourceWeChat),
		WithQueueItemID(customID),
	)
	if err != nil {
		t.Fatalf("RunTask: %v", err)
	}

	queue, err := database.GetTaskQueue(db, task.ID)
	if err != nil {
		t.Fatalf("GetTaskQueue: %v", err)
	}
	if len(queue) != 1 || queue[0].ID != customID {
		t.Fatalf("queue item id = %q, want %q", queue[0].ID, customID)
	}
}
