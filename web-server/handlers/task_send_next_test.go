package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"matrixops/services"
	"matrixops/services/task_runner"
	database "pkgs/db"
	"pkgs/db/models"
	"pkgs/testutil"

	"github.com/gin-gonic/gin"
)

func TestSendNextTaskQueueItem_ReordersWhenTaskRunning(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := testutil.OpenTaskTestDB(t)
	task := testutil.CreateTask(t, db, func(task *models.Task) {
		task.MessageQueue = []models.TaskMessageQueueItem{
			{ID: "a", Content: "first"},
			{ID: "b", Content: "second"},
		}
	})

	hub := services.NewGlobalWSHub(db)
	services.SetGlobalWSHubForTest(hub)

	task_runner.MarkTaskRunningForTest(task.ID)
	defer task_runner.UnmarkTaskRunningForTest(task.ID)

	handler := NewTaskHandler(db)
	resp := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(resp)
	ctx.Params = gin.Params{
		{Key: "id", Value: strconv.FormatUint(uint64(task.ID), 10)},
		{Key: "itemId", Value: "b"},
	}
	ctx.Request = httptest.NewRequest(http.MethodPost, "/tasks/1/queue/b/send-next", nil)

	handler.SendNextTaskQueueItem(ctx)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.Code, resp.Body.String())
	}

	var body map[string]interface{}
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if body["message"] != "已加入本轮补充" {
		t.Fatalf("message = %v", body["message"])
	}

	queueRaw, ok := body["queue"].([]interface{})
	if !ok || len(queueRaw) != 2 {
		t.Fatalf("response queue = %#v", body["queue"])
	}
	first, _ := queueRaw[0].(map[string]interface{})
	if first["id"] != "b" {
		t.Fatalf("queue[0].id = %v, want b", first["id"])
	}

	queue, err := database.GetTaskQueue(db, task.ID)
	if err != nil {
		t.Fatalf("get queue: %v", err)
	}
	if len(queue) != 2 || queue[0].ID != "b" || !queue[0].Supplement {
		t.Fatalf("db queue = %#v", queue)
	}
}
