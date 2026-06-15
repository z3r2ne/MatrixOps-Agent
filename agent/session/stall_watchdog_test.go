package session

import (
	"testing"
	"time"

	coreagent "matrixops.local/core_agent"
	database "pkgs/db"
	"pkgs/db/models"
	"pkgs/taskqueue"
	"pkgs/testutil"
)

func TestHandleStallWatchdogToolCancelled_PrependsQueueHeadAndBroadcasts(t *testing.T) {
	db := testutil.OpenTaskTestDB(t)
	task := testutil.CreateTask(t, db)
	runner := &AgentRunner{
		db:   db,
		task: task,
		messageQueue: taskqueue.New(db, task.ID, func(taskID uint, queue []models.TaskMessageQueueItem) {
			if taskID != task.ID {
				t.Fatalf("unexpected task id %d", taskID)
			}
			if len(queue) != 1 {
				t.Fatalf("expected broadcast queue len 1, got %d", len(queue))
			}
		}),
	}

	err := runner.handleStallWatchdogToolCancelled(nil, "bash", "call-1", "taking too long", 12*time.Second)
	if err != nil {
		t.Fatalf("handleStallWatchdogToolCancelled: %v", err)
	}

	queue, err := database.GetTaskQueue(db, task.ID)
	if err != nil {
		t.Fatalf("GetTaskQueue: %v", err)
	}
	if len(queue) != 1 {
		t.Fatalf("queue len = %d, want 1", len(queue))
	}
	if queue[0].Type != models.TaskMessageQueueTypeSystem {
		t.Fatalf("queue type = %q, want system", queue[0].Type)
	}
	if queue[0].Source != taskMessageQueueMetadataSourceStallWatchdog {
		t.Fatalf("source = %q, want %q", queue[0].Source, taskMessageQueueMetadataSourceStallWatchdog)
	}
	if !queue[0].Supplement {
		t.Fatalf("expected supplement=true")
	}
	expected := coreagent.FormatStallWatchdogToolCancelledWarning("bash", "taking too long", 12*time.Second)
	if queue[0].Content != expected {
		t.Fatalf("content = %q, want %q", queue[0].Content, expected)
	}
}

func TestHandleStallWatchdogToolCancelled_DoesNotConsumeSupplementImmediately(t *testing.T) {
	db := testutil.OpenTaskTestDB(t)
	task := testutil.CreateTask(t, db)
	memory := NewProcessV2MemoryState(nil)
	runner := &AgentRunner{
		db:           db,
		task:         task,
		messageQueue: taskqueue.New(db, task.ID, nil),
	}
	runtimeConfig := &RuntimeConfig{MemoryState: memory}

	err := runner.handleStallWatchdogToolCancelled(runtimeConfig, "bash", "call-1", "taking too long", 12*time.Second)
	if err != nil {
		t.Fatalf("handleStallWatchdogToolCancelled: %v", err)
	}

	queue, err := database.GetTaskQueue(db, task.ID)
	if err != nil {
		t.Fatalf("GetTaskQueue: %v", err)
	}
	if len(queue) != 1 {
		t.Fatalf("queue len = %d, want 1 before runner consumes supplement", len(queue))
	}
	if len(memory.Snapshot().History) != 0 {
		t.Fatal("expected memory unchanged until agent step consumes supplement")
	}
}

func TestHandleStallWatchdogToolCancelled_SkipsDuplicateHeadWarning(t *testing.T) {
	db := testutil.OpenTaskTestDB(t)
	task := testutil.CreateTask(t, db)
	broadcasts := 0
	runner := &AgentRunner{
		db:   db,
		task: task,
		messageQueue: taskqueue.New(db, task.ID, func(uint, []models.TaskMessageQueueItem) {
			broadcasts++
		}),
	}

	if err := runner.handleStallWatchdogToolCancelled(nil, "bash", "call-1", "taking too long", 12*time.Second); err != nil {
		t.Fatalf("first handleStallWatchdogToolCancelled: %v", err)
	}
	if err := runner.handleStallWatchdogToolCancelled(nil, "bash", "call-1", "taking too long", 12*time.Second); err != nil {
		t.Fatalf("second handleStallWatchdogToolCancelled: %v", err)
	}

	queue, err := database.GetTaskQueue(db, task.ID)
	if err != nil {
		t.Fatalf("GetTaskQueue: %v", err)
	}
	if len(queue) != 1 {
		t.Fatalf("queue len = %d, want 1", len(queue))
	}
	if broadcasts != 1 {
		t.Fatalf("broadcasts = %d, want 1", broadcasts)
	}
}
