package tool

import (
	"reflect"
	"strings"
	"testing"
)

func TestDefaultRegistryWithQuestionSkipsRunWorkerTaskWithoutRunner(t *testing.T) {
	registry := NewDefaultRegistryWithQuestion(&DefaultRegistryOptions{})
	if _, err := registry.Get("run_worker_task"); err == nil {
		t.Fatal("expected run_worker_task to be absent without runner")
	}
}

func TestDefaultRegistryWithQuestionRegistersRunWorkerTaskWithRunner(t *testing.T) {
	registry := NewDefaultRegistryWithQuestion(&DefaultRegistryOptions{
		AvailableWorkerNames: func() []string {
			return []string{"chat", "leader"}
		},
		RunWorkerTask: func(ctx Context, req RunWorkerTaskRequest, onProgress func(RunWorkerTaskProgress)) (RunWorkerTaskResult, error) {
			return RunWorkerTaskResult{}, nil
		},
	})
	if _, err := registry.Get("run_worker_task"); err != nil {
		t.Fatalf("expected run_worker_task to be present: %v", err)
	}
}

func TestRunWorkerTaskSchemaIncludesWorkerEnum(t *testing.T) {
	tool := NewRunWorkerTaskTool(
		func(ctx Context, req RunWorkerTaskRequest, onProgress func(RunWorkerTaskProgress)) (RunWorkerTaskResult, error) {
			return RunWorkerTaskResult{}, nil
		},
		func() []string { return []string{"leader", "chat", "leader"} },
	)

	schema := tool.Schema()
	properties, ok := schema["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("schema properties missing: %+v", schema)
	}
	workerField, ok := properties["worker"].(map[string]interface{})
	if !ok {
		t.Fatalf("worker field missing: %+v", properties["worker"])
	}
	enumValues, ok := workerField["enum"].([]string)
	if ok {
		if !reflect.DeepEqual(enumValues, []string{"chat", "leader"}) {
			t.Fatalf("unexpected worker enum: %+v", enumValues)
		}
	} else {
		rawEnum, ok := workerField["enum"].([]interface{})
		if !ok {
			t.Fatalf("worker enum missing: %+v", workerField)
		}
		values := make([]string, 0, len(rawEnum))
		for _, item := range rawEnum {
			text, _ := item.(string)
			values = append(values, text)
		}
		if !reflect.DeepEqual(values, []string{"chat", "leader"}) {
			t.Fatalf("unexpected worker enum: %+v", values)
		}
	}

	if _, ok := properties["inheritParentMemory"]; ok {
		t.Fatalf("inheritParentMemory should not be exposed in schema")
	}
	if _, ok := properties["hideParentTools"]; ok {
		t.Fatalf("hideParentTools should not be exposed in schema")
	}
}

func TestRunWorkerTaskRejectsWorkerOutsideWhitelist(t *testing.T) {
	tool := NewRunWorkerTaskTool(
		func(ctx Context, req RunWorkerTaskRequest, onProgress func(RunWorkerTaskProgress)) (RunWorkerTaskResult, error) {
			return RunWorkerTaskResult{}, nil
		},
		func() []string { return []string{"chat", "leader"} },
	)

	result, err := tool.Execute(Context{}, map[string]interface{}{
		"worker":  "frontend_engineer",
		"content": "inspect project",
	})
	if err == nil {
		t.Fatal("expected whitelist error")
	}
	if !result.IsError {
		t.Fatalf("expected result to be error: %+v", result)
	}
	if !strings.Contains(err.Error(), "not allowed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunWorkerTaskIgnoresLegacyMemoryFlags(t *testing.T) {
	var gotReq RunWorkerTaskRequest
	tool := NewRunWorkerTaskTool(
		func(ctx Context, req RunWorkerTaskRequest, onProgress func(RunWorkerTaskProgress)) (RunWorkerTaskResult, error) {
			gotReq = req
			return RunWorkerTaskResult{}, nil
		},
		func() []string { return []string{"chat"} },
	)

	_, err := tool.Execute(Context{}, map[string]interface{}{
		"worker":              "chat",
		"content":             "inspect project",
		"inheritParentMemory": true,
		"hideParentTools":     true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotReq.WorkerName != "chat" || gotReq.Content != "inspect project" {
		t.Fatalf("unexpected request forwarded: %+v", gotReq)
	}
}

func TestRunWorkerTaskPassesTaskID(t *testing.T) {
	var gotReq RunWorkerTaskRequest
	tool := NewRunWorkerTaskTool(
		func(ctx Context, req RunWorkerTaskRequest, onProgress func(RunWorkerTaskProgress)) (RunWorkerTaskResult, error) {
			gotReq = req
			return RunWorkerTaskResult{TaskID: req.TaskID}, nil
		},
		func() []string { return []string{"explore"} },
	)

	_, err := tool.Execute(Context{}, map[string]interface{}{
		"worker":  "explore",
		"content": "continue previous work",
		"task_id": float64(294),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotReq.TaskID != 294 {
		t.Fatalf("TaskID = %d, want 294", gotReq.TaskID)
	}
}

func TestRunWorkerTaskRejectsTaskIDWithName(t *testing.T) {
	tool := NewRunWorkerTaskTool(
		func(ctx Context, req RunWorkerTaskRequest, onProgress func(RunWorkerTaskProgress)) (RunWorkerTaskResult, error) {
			return RunWorkerTaskResult{}, nil
		},
		func() []string { return []string{"explore"} },
	)

	_, err := tool.Execute(Context{}, map[string]interface{}{
		"worker":  "explore",
		"content": "continue",
		"task_id": 12,
		"name":    "new title",
	})
	if err == nil || !strings.Contains(err.Error(), "task_id cannot be used together with name") {
		t.Fatalf("expected task_id/name conflict error, got %v", err)
	}
}
