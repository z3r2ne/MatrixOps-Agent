package session

import (
	"testing"

	coreagent "matrixops.local/core_agent"
	database "pkgs/db"
	"pkgs/db/models"
	"pkgs/taskqueue"
	"pkgs/testutil"
)

func TestHandleRepeatedToolCall_PrependsQueueHeadAndBroadcasts(t *testing.T) {
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
	runtimeConfig := &RuntimeConfig{
		MemoryState: NewProcessV2MemoryState(nil),
	}

	err := runner.handleRepeatedToolCall(runtimeConfig, "grep", map[string]interface{}{"pattern": "foo"}, 5)
	if err != nil {
		t.Fatalf("handleRepeatedToolCall: %v", err)
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
	if queue[0].Source != taskMessageQueueMetadataSourceToolRepeatWatchdog {
		t.Fatalf("source = %q, want %q", queue[0].Source, taskMessageQueueMetadataSourceToolRepeatWatchdog)
	}
	if !queue[0].Supplement {
		t.Fatalf("expected supplement=true")
	}
	expected := coreagent.FormatRepeatedToolCallWarning("grep", 5)
	if queue[0].Content != expected {
		t.Fatalf("content = %q, want %q", queue[0].Content, expected)
	}
	if len(runtimeConfig.MemoryState.Snapshot().History) != 0 {
		t.Fatalf("expected memory unchanged before supplement consumption")
	}
}

func TestHandleRepeatedToolCall_SkipsDuplicateHeadWarning(t *testing.T) {
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
	runtimeConfig := &RuntimeConfig{MemoryState: NewProcessV2MemoryState(nil)}
	args := map[string]interface{}{"pattern": "foo"}

	if err := runner.handleRepeatedToolCall(runtimeConfig, "grep", args, 5); err != nil {
		t.Fatalf("first handleRepeatedToolCall: %v", err)
	}
	if err := runner.handleRepeatedToolCall(runtimeConfig, "grep", args, 5); err != nil {
		t.Fatalf("second handleRepeatedToolCall: %v", err)
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

func TestHandleRepeatedToolCall_AllowsLaterReminderForSameFingerprint(t *testing.T) {
	db := testutil.OpenTaskTestDB(t)
	task := testutil.CreateTask(t, db)
	runner := &AgentRunner{
		db:           db,
		task:         task,
		messageQueue: taskqueue.New(db, task.ID, nil),
	}
	runtimeConfig := &RuntimeConfig{MemoryState: NewProcessV2MemoryState(nil)}
	args := map[string]interface{}{"pattern": "foo"}

	if err := runner.handleRepeatedToolCall(runtimeConfig, "grep", args, 5); err != nil {
		t.Fatalf("first handleRepeatedToolCall: %v", err)
	}
	if err := runner.handleRepeatedToolCall(runtimeConfig, "grep", args, 15); err != nil {
		t.Fatalf("second handleRepeatedToolCall: %v", err)
	}

	queue, err := database.GetTaskQueue(db, task.ID)
	if err != nil {
		t.Fatalf("GetTaskQueue: %v", err)
	}
	if len(queue) != 2 {
		t.Fatalf("queue len = %d, want 2", len(queue))
	}
	if got := metadataInt(queue[0].Metadata["repeatCount"]); got != 15 {
		t.Fatalf("head repeatCount = %d, want 15", got)
	}
	if got := metadataInt(queue[1].Metadata["repeatCount"]); got != 5 {
		t.Fatalf("second repeatCount = %d, want 5", got)
	}
}
