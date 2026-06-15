package session

import (
	"testing"

	"matrixops.local/core_agent/streamtypes"
	database "pkgs/db"
	"pkgs/db/models"
	"pkgs/taskqueue"
	"pkgs/testutil"
)

func TestHandleEmptyStreamRetry_PrependsQueueHead(t *testing.T) {
	db := testutil.OpenTaskTestDB(t)
	task := testutil.CreateTask(t, db)
	broadcasts := 0
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
			broadcasts++
		}),
	}

	err := runner.handleEmptyStreamRetry(streamtypes.NewRetryableEmptyStreamOutputError("end_turn", `stop_reason":"end_turn"`))
	if err != nil {
		t.Fatalf("handleEmptyStreamRetry: %v", err)
	}
	if broadcasts != 1 {
		t.Fatalf("broadcasts = %d, want 1", broadcasts)
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
	if queue[0].Source != taskMessageQueueMetadataSourceEmptyStreamRetry {
		t.Fatalf("source = %q, want %q", queue[0].Source, taskMessageQueueMetadataSourceEmptyStreamRetry)
	}
	if !queue[0].Supplement {
		t.Fatal("expected supplement=true")
	}
	if queue[0].Content != emptyStreamRetryContinueContent {
		t.Fatalf("content = %q, want %q", queue[0].Content, emptyStreamRetryContinueContent)
	}
}

func TestHandleEmptyStreamRetry_DoesNotConsumeSupplementImmediately(t *testing.T) {
	db := testutil.OpenTaskTestDB(t)
	task := testutil.CreateTask(t, db)
	memory := NewProcessV2MemoryState(nil)
	runner := &AgentRunner{
		db:           db,
		task:         task,
		messageQueue: taskqueue.New(db, task.ID, nil),
	}
	runtimeConfig := &RuntimeConfig{MemoryState: memory}

	err := runner.handleEmptyStreamRetry(streamtypes.NewRetryableEmptyStreamOutputError("end_turn", ""))
	if err != nil {
		t.Fatalf("handleEmptyStreamRetry: %v", err)
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
	_ = runtimeConfig
}

func TestHandleEmptyStreamRetry_SkipsDuplicateHeadSupplement(t *testing.T) {
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

	emptyErr := streamtypes.NewRetryableEmptyStreamOutputError("end_turn", "")
	if err := runner.handleEmptyStreamRetry(emptyErr); err != nil {
		t.Fatalf("first handleEmptyStreamRetry: %v", err)
	}
	if err := runner.handleEmptyStreamRetry(emptyErr); err != nil {
		t.Fatalf("second handleEmptyStreamRetry: %v", err)
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

func TestHandleEmptyStreamRetry_IgnoresNonEmptyStreamError(t *testing.T) {
	db := testutil.OpenTaskTestDB(t)
	task := testutil.CreateTask(t, db)
	runner := &AgentRunner{
		db:           db,
		task:         task,
		messageQueue: taskqueue.New(db, task.ID, nil),
	}

	if err := runner.handleEmptyStreamRetry(nil); err != nil {
		t.Fatalf("handleEmptyStreamRetry(nil): %v", err)
	}
	queue, err := database.GetTaskQueue(db, task.ID)
	if err != nil {
		t.Fatalf("GetTaskQueue: %v", err)
	}
	if len(queue) != 0 {
		t.Fatalf("queue len = %d, want 0", len(queue))
	}
}
