package session

import (
	"context"
	"errors"
	"testing"

	"matrixops-agent/tool"
	"matrixops-agent/types"
	"pkgs/db/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestBuildRunWorkerTaskFuncDoesNotForwardParentMemorySnapshot(t *testing.T) {
	sentinel := errors.New("stop-after-capture")
	var capturedArgs map[string]interface{}

	run := buildRunWorkerTaskFunc(
		nil,
		42,
		func(args map[string]interface{}) (map[string]interface{}, error) {
			capturedArgs = args
			return nil, sentinel
		},
		func() *types.Memory {
			return &types.Memory{
				Entries: []*types.MemoryEntry{
					{EntryKind: "text", Role: "user", Content: "parent context"},
				},
			}
		},
	)

	if run == nil {
		t.Fatal("expected run worker task func")
	}

	_, err := run(tool.Context{}, tool.RunWorkerTaskRequest{
		WorkerName: "frontend_engineer",
		Content:    "implement feature",
		TaskName:   "child task",
	}, nil)
	if !errors.Is(err, sentinel) {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedArgs == nil {
		t.Fatal("expected args to be captured")
	}
	if _, ok := capturedArgs["parentMemorySnapshot"]; ok {
		t.Fatalf("parentMemorySnapshot should not be forwarded: %+v", capturedArgs)
	}
}

func TestBuildRunWorkerTaskFuncPassesParentContext(t *testing.T) {
	sentinel := errors.New("stop-after-capture")
	var capturedArgs map[string]interface{}
	parentCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	run := buildRunWorkerTaskFunc(
		nil,
		42,
		func(args map[string]interface{}) (map[string]interface{}, error) {
			capturedArgs = args
			return nil, sentinel
		},
		nil,
	)

	_, err := run(tool.Context{Context: parentCtx}, tool.RunWorkerTaskRequest{
		WorkerName: "explore",
		Content:    "inspect repo",
	}, nil)
	if !errors.Is(err, sentinel) {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := capturedArgs["parentContext"].(context.Context)
	if !ok || got != parentCtx {
		t.Fatalf("parentContext = %#v, want %p", capturedArgs["parentContext"], parentCtx)
	}
}

func TestBuildRunWorkerTaskFuncPassesParentTaskIDAsValue(t *testing.T) {
	sentinel := errors.New("stop-after-capture")
	var capturedArgs map[string]interface{}

	run := buildRunWorkerTaskFunc(
		nil,
		42,
		func(args map[string]interface{}) (map[string]interface{}, error) {
			capturedArgs = args
			return nil, sentinel
		},
		nil,
	)

	if run == nil {
		t.Fatal("expected run worker task func")
	}

	_, err := run(tool.Context{}, tool.RunWorkerTaskRequest{
		WorkerName: "explore",
		Content:    "inspect repo",
	}, nil)
	if !errors.Is(err, sentinel) {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedArgs == nil {
		t.Fatal("expected args to be captured")
	}

	parentTaskID, ok := capturedArgs["parentTaskId"].(uint)
	if !ok {
		t.Fatalf("expected parentTaskId to be uint, got %#v", capturedArgs["parentTaskId"])
	}
	if parentTaskID != 42 {
		t.Fatalf("parentTaskId = %d, want 42", parentTaskID)
	}
}

func openRunWorkerTaskCallbackTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&models.Task{}); err != nil {
		t.Fatalf("migrate task tables: %v", err)
	}
	return db
}

func TestBuildRunWorkerTaskFuncPassesTaskID(t *testing.T) {
	db := openRunWorkerTaskCallbackTestDB(t)
	parent := &models.Task{ProjectID: 1, Content: "parent", WorkerName: "chat", Status: "running", SessionID: "session-parent"}
	if err := db.Create(parent).Error; err != nil {
		t.Fatalf("create parent task: %v", err)
	}
	child := &models.Task{
		ProjectID:    1,
		ParentTaskID: &parent.ID,
		Content:      "child",
		WorkerName:   "explore",
		Status:       "done",
		SessionID:    "session-child",
	}
	if err := db.Create(child).Error; err != nil {
		t.Fatalf("create child task: %v", err)
	}

	sentinel := errors.New("stop-after-capture")
	var capturedArgs map[string]interface{}

	run := buildRunWorkerTaskFunc(
		db,
		parent.ID,
		func(args map[string]interface{}) (map[string]interface{}, error) {
			capturedArgs = args
			return nil, sentinel
		},
		nil,
	)

	_, err := run(tool.Context{}, tool.RunWorkerTaskRequest{
		WorkerName: "explore",
		Content:    "continue",
		TaskID:     child.ID,
	}, nil)
	if !errors.Is(err, sentinel) {
		t.Fatalf("unexpected error: %v", err)
	}

	taskID, ok := capturedArgs["taskId"].(uint)
	if !ok {
		t.Fatalf("expected taskId to be uint, got %#v", capturedArgs["taskId"])
	}
	if taskID != child.ID {
		t.Fatalf("taskId = %d, want %d", taskID, child.ID)
	}
}

func TestValidateRunWorkerTaskContinuationRejectsForeignSubtask(t *testing.T) {
	db := openRunWorkerTaskCallbackTestDB(t)
	parent := &models.Task{ProjectID: 1, Content: "parent", WorkerName: "chat", Status: "running", SessionID: "session-parent"}
	if err := db.Create(parent).Error; err != nil {
		t.Fatalf("create parent task: %v", err)
	}
	otherParentID := uint(999)
	child := &models.Task{
		ProjectID:    1,
		ParentTaskID: &otherParentID,
		Content:      "child",
		WorkerName:   "explore",
		Status:       "done",
		SessionID:    "session-child",
	}
	if err := db.Create(child).Error; err != nil {
		t.Fatalf("create child task: %v", err)
	}

	err := validateRunWorkerTaskContinuation(db, parent.ID, tool.RunWorkerTaskRequest{
		WorkerName: "explore",
		TaskID:     child.ID,
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
}
