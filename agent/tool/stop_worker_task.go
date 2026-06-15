package tool

import (
	"errors"
	"fmt"
)

type StopWorkerTaskFunc func(ctx Context, taskID uint) error

type StopWorkerTaskTool struct {
	stop StopWorkerTaskFunc
}

func NewStopWorkerTaskTool(stop StopWorkerTaskFunc) *StopWorkerTaskTool {
	return &StopWorkerTaskTool{stop: stop}
}

func (StopWorkerTaskTool) Name() string { return "stop_worker_task" }

func (StopWorkerTaskTool) VerbosName() string { return "停止子任务" }

func (StopWorkerTaskTool) Description() string {
	return "停止一个正在运行的 worker 子任务。需要提供子任务的 task_id（从 run_worker_task 返回的 metadata 中的 subtaskTaskId 获取）。"
}

func (StopWorkerTaskTool) Schema() map[string]interface{} {
	return ObjectParamSchema(map[string]interface{}{
		"task_id": map[string]interface{}{
			"type":        "integer",
			"description": "要停止的子任务 ID，取自 run_worker_task 返回的 metadata 中的 subtaskTaskId。",
		},
	}, []string{"task_id"})
}

func (t *StopWorkerTaskTool) Execute(ctx Context, input map[string]interface{}) (Result, error) {
	if t == nil || t.stop == nil {
		return Result{IsError: true, Name: "stop_worker_task"}, errors.New("stop_worker_task: missing stop function")
	}

	taskID, err := parseOptionalUintID(input["task_id"])
	if err != nil {
		return Result{IsError: true, Name: "stop_worker_task"}, fmt.Errorf("stop_worker_task: invalid task_id: %w", err)
	}
	if taskID == 0 {
		return Result{IsError: true, Name: "stop_worker_task"}, errors.New("stop_worker_task: missing task_id")
	}

	if err := t.stop(ctx, taskID); err != nil {
		return Result{IsError: true, Name: "stop_worker_task"}, fmt.Errorf("stop_worker_task: %w", err)
	}

	return Result{
		Name:    "stop_worker_task",
		Content: fmt.Sprintf("已请求停止任务 %d", taskID),
		Metadata: map[string]interface{}{
			"stoppedTaskId": taskID,
		},
	}, nil
}
