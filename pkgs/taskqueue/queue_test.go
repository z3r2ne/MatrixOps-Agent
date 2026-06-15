package taskqueue

import (
	"testing"

	"pkgs/db/models"
	"pkgs/testutil"
)

type recordingHandler struct {
	items []models.TaskMessageQueueItem
}

func (h *recordingHandler) handle(item models.TaskMessageQueueItem) error {
	h.items = append(h.items, item)
	return nil
}

func TestConsumeSupplements_AppendsAndRemoves(t *testing.T) {
	db := testutil.OpenTaskTestDB(t)
	task := testutil.CreateTask(t, db, func(task *models.Task) {
		task.MessageQueue = []models.TaskMessageQueueItem{
			{
				ID:         "sup-1",
				Type:       models.TaskMessageQueueTypeSystem,
				Content:    "watchdog warning",
				Source:     models.TaskMessageQueueSourceToolRepeatWatchdog,
				Supplement: true,
			},
		}
	})

	var broadcasted [][]models.TaskMessageQueueItem
	q := New(db, task.ID, func(_ uint, queue []models.TaskMessageQueueItem) {
		broadcasted = append(broadcasted, append([]models.TaskMessageQueueItem(nil), queue...))
	})

	handler := &recordingHandler{}
	consumed, err := q.ConsumeSupplements(handler.handle)
	if err != nil {
		t.Fatalf("ConsumeSupplements: %v", err)
	}
	if !consumed {
		t.Fatal("expected supplement to be consumed")
	}
	if len(handler.items) != 1 || handler.items[0].Content != "watchdog warning" {
		t.Fatalf("items = %#v", handler.items)
	}
	queue, err := q.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(queue) != 0 {
		t.Fatalf("queue = %#v", queue)
	}
	if len(broadcasted) == 0 {
		t.Fatal("expected broadcast")
	}
}

func TestConsumeSupplements_SkipsNonSupplementHead(t *testing.T) {
	db := testutil.OpenTaskTestDB(t)
	task := testutil.CreateTask(t, db, func(task *models.Task) {
		task.MessageQueue = []models.TaskMessageQueueItem{
			{ID: "user-1", Content: "hello"},
		}
	})
	q := New(db, task.ID, nil)
	handler := &recordingHandler{}
	consumed, err := q.ConsumeSupplements(handler.handle)
	if err != nil {
		t.Fatalf("ConsumeSupplements: %v", err)
	}
	if consumed {
		t.Fatal("expected no supplement consumption")
	}
}

func TestDequeueNext_RemovesHead(t *testing.T) {
	db := testutil.OpenTaskTestDB(t)
	task := testutil.CreateTask(t, db, func(task *models.Task) {
		task.MessageQueue = []models.TaskMessageQueueItem{
			{ID: "a", Content: "one"},
			{ID: "b", Content: "two"},
		}
	})
	q := New(db, task.ID, nil)
	item, err := q.DequeueNext()
	if err != nil {
		t.Fatalf("DequeueNext: %v", err)
	}
	if item == nil || item.ID != "a" {
		t.Fatalf("item = %#v", item)
	}
	queue, err := q.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(queue) != 1 || queue[0].ID != "b" {
		t.Fatalf("queue = %#v", queue)
	}
}
