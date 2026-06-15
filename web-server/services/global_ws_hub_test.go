package services

import (
	"encoding/json"
	"testing"
	"time"

	agentsession "matrixops-agent/session"
	"matrixops/services/task_runner"
	"matrixops/types"
	database "pkgs/db"
	"pkgs/db/models"
	"pkgs/testutil"
)

func TestParseWSUserParts(t *testing.T) {
	absPath, err := agentsession.SaveTempUserInputFile("a.png", []byte("x"))
	if err != nil {
		t.Fatalf("SaveTempUserInputFile: %v", err)
	}
	parts, text, err := ParseWSUserParts("", 1, json.RawMessage(`[
		{"type":"text","text":"hello"},
		{"type":"file","path":"`+absPath+`","mime":"image/png","filename":"a.png","source":"picker"}
	]`))
	if err != nil {
		t.Fatalf("ParseWSUserParts: %v", err)
	}
	if text != "hello" {
		t.Fatalf("text = %q", text)
	}
	if len(parts) != 2 {
		t.Fatalf("parts len = %d, want 2", len(parts))
	}
	if parts[1].Path != absPath {
		t.Fatalf("path = %q, want %q", parts[1].Path, absPath)
	}
	if parts[1].InputSource != "picker" {
		t.Fatalf("inputSource = %q", parts[1].InputSource)
	}
}

func TestHandleSendMessage_EnqueuesWhenTaskRunning(t *testing.T) {
	db := testutil.OpenTaskTestDB(t)
	task := testutil.CreateTask(t, db)
	hub := NewGlobalWSHub(db)

	task_runner.MarkTaskRunningForTest(task.ID)
	defer task_runner.UnmarkTaskRunningForTest(task.ID)

	err := hub.handleSendMessage(task.ID, "queued while running", nil)
	if err != nil {
		t.Fatalf("handleSendMessage: %v", err)
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
}

func TestHandleSendMessage_RejectsEmptyPayload(t *testing.T) {
	db := testutil.OpenTaskTestDB(t)
	task := testutil.CreateTask(t, db)
	hub := NewGlobalWSHub(db)

	err := hub.handleSendMessage(task.ID, "   ", nil)
	if err == nil {
		t.Fatal("expected error for empty content")
	}
}

func TestHandleSubscribe_SendsTaskQueueSnapshot(t *testing.T) {
	db := testutil.OpenTaskTestDB(t)
	task := testutil.CreateTask(t, db, func(task *models.Task) {
		task.MessageQueue = []models.TaskMessageQueueItem{
			{ID: "q1", Content: "hello", CreatedAt: time.Now().UnixMilli()},
		}
	})
	if err := database.SetTaskMessageQueueAutoSend(db, task.ID, false); err != nil {
		t.Fatalf("SetTaskMessageQueueAutoSend: %v", err)
	}

	hub := NewGlobalWSHub(db)
	client := NewGlobalWSClient("test-client")
	hub.Register(client)

	if err := hub.handleSubscribe(client, task.ID); err != nil {
		t.Fatalf("handleSubscribe: %v", err)
	}

	var snapshot *WSOutgoingMessage
	deadline := time.After(2 * time.Second)
	for snapshot == nil {
		select {
		case raw := <-client.Send:
			var msg WSOutgoingMessage
			if err := json.Unmarshal(raw, &msg); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if msg.Type == types.WSTypeTaskQueue {
				snapshot = &msg
			}
		case <-deadline:
			t.Fatal("timeout waiting for task queue snapshot")
		}
	}

	data, ok := snapshot.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("snapshot data type = %T", snapshot.Data)
	}
	if autoSend, _ := data["autoSend"].(bool); autoSend {
		t.Fatal("expected autoSend=false in snapshot")
	}
	queueRaw, ok := data["queue"].([]interface{})
	if !ok || len(queueRaw) != 1 {
		t.Fatalf("unexpected queue in snapshot: %#v", data["queue"])
	}
}

func TestTaskQueueWSData_ReflectsAutoSendFlag(t *testing.T) {
	db := testutil.OpenTaskTestDB(t)
	task := testutil.CreateTask(t, db)
	if err := database.SetTaskMessageQueueAutoSend(db, task.ID, false); err != nil {
		t.Fatalf("SetTaskMessageQueueAutoSend: %v", err)
	}
	hub := NewGlobalWSHub(db)

	data := hub.taskQueueWSData(task.ID, nil)
	if data["autoSend"] != false {
		t.Fatalf("autoSend = %v, want false", data["autoSend"])
	}
}
