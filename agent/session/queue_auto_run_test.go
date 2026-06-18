package session

import (
	"sync/atomic"
	"testing"

	database "pkgs/db"
	"pkgs/db/models"
	"pkgs/taskqueue"
	"pkgs/testutil"
)

func TestPrependSupplementQueueItem_TriggersAutoRunAndEnablesAutoSend(t *testing.T) {
	db := testutil.OpenTaskTestDB(t)
	task := testutil.CreateTask(t, db)
	if err := database.SetTaskMessageQueueAutoSend(db, task.ID, false); err != nil {
		t.Fatalf("SetTaskMessageQueueAutoSend: %v", err)
	}

	var autoRunCount int32
	runner := &AgentRunner{
		db:   db,
		task: task,
		messageQueue: taskqueue.New(db, task.ID, func(uint, []models.TaskMessageQueueItem) {
		}),
		queueAutoRun: func() {
			atomic.AddInt32(&autoRunCount, 1)
		},
	}

	item := models.TaskMessageQueueItem{
		ID:         "async-tool-result-test",
		Type:       models.TaskMessageQueueTypeSystem,
		Content:    "<async_tool_result>done</async_tool_result>",
		Source:     models.TaskMessageQueueSourceAsyncToolResult,
		Supplement: true,
	}
	if err := runner.prependSupplementQueueItem(item); err != nil {
		t.Fatalf("prependSupplementQueueItem: %v", err)
	}
	if atomic.LoadInt32(&autoRunCount) != 1 {
		t.Fatalf("autoRunCount = %d, want 1", autoRunCount)
	}
	_, autoSend, err := database.GetTaskQueueSettings(db, task.ID)
	if err != nil {
		t.Fatalf("GetTaskQueueSettings: %v", err)
	}
	if !autoSend {
		t.Fatal("expected messageQueueAutoSend=true after supplement enqueue")
	}
	queue, err := runner.messageQueue.Load()
	if err != nil {
		t.Fatalf("Load queue: %v", err)
	}
	if len(queue) != 1 || queue[0].ID != item.ID {
		t.Fatalf("unexpected queue: %#v", queue)
	}
}
