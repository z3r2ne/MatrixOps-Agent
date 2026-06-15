package session

import (
	"testing"

	"pkgs/db/models"
	"pkgs/db/storage"
	"pkgs/taskqueue"
	"pkgs/testutil"
)

func TestDeliverTaskQueueAppendItem_WritesMessageAndMemory(t *testing.T) {
	db := testutil.OpenTaskTestDB(t)
	task := testutil.CreateTask(t, db, func(task *models.Task) {
		task.SessionID = "session-append-idle"
	})
	item := models.TaskMessageQueueItem{
		ID:      "append-1",
		Type:    models.TaskMessageQueueTypeAppend,
		Content: "用户已切换工作目录为 /tmp/worktree",
		Source:  models.TaskMessageQueueSourceFrontend,
	}
	if err := DeliverTaskQueueAppendItem(db, task, item); err != nil {
		t.Fatalf("DeliverTaskQueueAppendItem: %v", err)
	}

	messages, err := storage.GetMessageWithPartsBySessionIDLight(db, task.SessionID)
	if err != nil {
		t.Fatalf("GetMessageWithPartsBySessionIDLight: %v", err)
	}
	var found *MessageInfo
	for _, msg := range messages {
		if msg != nil && msg.Info != nil && msg.Info.Role == RoleUser {
			found = msg.Info
			break
		}
	}
	if found == nil {
		t.Fatal("expected append message in session")
	}
	if found.MessageKind != MessageKindSystem {
		t.Fatalf("messageKind = %q, want system", found.MessageKind)
	}

	entries, err := storage.ListMemoryEntriesBySession(db, task.SessionID)
	if err != nil {
		t.Fatalf("ListMemoryEntriesBySession: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected memory entries after append delivery")
	}
}

func TestMessageQueue_ConsumeSupplementsSkipsAppend(t *testing.T) {
	db := testutil.OpenTaskTestDB(t)
	task := testutil.CreateTask(t, db, func(task *models.Task) {
		task.MessageQueue = []models.TaskMessageQueueItem{
			{ID: "append-1", Type: models.TaskMessageQueueTypeAppend, Content: "via append"},
		}
	})
	q := taskqueue.New(db, task.ID, nil)
	var got string
	consumed, err := q.ConsumeSupplements(func(item models.TaskMessageQueueItem) error {
		got = item.Content
		return nil
	})
	if err != nil {
		t.Fatalf("ConsumeSupplements: %v", err)
	}
	if consumed {
		t.Fatal("append should not be consumed by ConsumeSupplements")
	}
	if got != "" {
		t.Fatalf("got content %q, want empty", got)
	}
}
