package tool

import (
	"encoding/json"
	"errors"
	"fmt"
)

type WorkerTaskProgressMessage struct {
	ID         string                   `json:"id"`
	Role       string                   `json:"role"`
	WorkerName string                   `json:"workerName,omitempty"`
	Parts      []map[string]interface{} `json:"parts,omitempty"`
}

type WorkerTaskProgressResult struct {
	TaskID     uint                       `json:"taskId"`
	SessionID  string                     `json:"sessionId"`
	Status     string                     `json:"status"`
	WorkerName string                     `json:"workerName"`
	TaskName   string                     `json:"taskName,omitempty"`
	Content    string                     `json:"content,omitempty"`
	Answer     string                     `json:"answer,omitempty"`
	Messages   []WorkerTaskProgressMessage `json:"messages"`
}

type GetWorkerTaskProgressFunc func(ctx Context, taskID uint, limit int) (*WorkerTaskProgressResult, error)

type GetWorkerTaskProgressTool struct {
	getProgress GetWorkerTaskProgressFunc
}

func NewGetWorkerTaskProgressTool(getProgress GetWorkerTaskProgressFunc) *GetWorkerTaskProgressTool {
	return &GetWorkerTaskProgressTool{getProgress: getProgress}
}

func (GetWorkerTaskProgressTool) Name() string { return "get_worker_task_progress" }

func (GetWorkerTaskProgressTool) VerbosName() string { return "获取子任务进度" }

func (GetWorkerTaskProgressTool) Description() string {
	return "获取指定 worker 子任务的当前进度，包括任务状态和近期对话消息记录。默认返回最近 10 条消息，可通过 limit 参数调整数量（1-50）。"
}

func (GetWorkerTaskProgressTool) Schema() map[string]interface{} {
	return ObjectParamSchema(map[string]interface{}{
		"task_id": map[string]interface{}{
			"type":        "integer",
			"description": "子任务 ID，取自 run_worker_task 返回的 metadata 中的 subtaskTaskId。",
		},
		"limit": map[string]interface{}{
			"type":        "integer",
			"description": "可选。返回的消息数量上限，默认 10，最大 50。",
		},
	}, []string{"task_id"})
}

func (t *GetWorkerTaskProgressTool) Execute(ctx Context, input map[string]interface{}) (Result, error) {
	if t == nil || t.getProgress == nil {
		return Result{IsError: true, Name: "get_worker_task_progress"}, errors.New("get_worker_task_progress: missing getter")
	}

	taskID, err := parseOptionalUintID(input["task_id"])
	if err != nil {
		return Result{IsError: true, Name: "get_worker_task_progress"}, fmt.Errorf("get_worker_task_progress: invalid task_id: %w", err)
	}
	if taskID == 0 {
		return Result{IsError: true, Name: "get_worker_task_progress"}, errors.New("get_worker_task_progress: missing task_id")
	}

	limit := 10
	if limitVal, ok := input["limit"]; ok {
		if parsedLimit, parseErr := parseOptionalUintID(limitVal); parseErr == nil && parsedLimit > 0 {
			limit = int(parsedLimit)
		}
	}
	if limit > 50 {
		limit = 50
	}

	progress, err := t.getProgress(ctx, taskID, limit)
	if err != nil {
		return Result{IsError: true, Name: "get_worker_task_progress"}, fmt.Errorf("get_worker_task_progress: %w", err)
	}

	resultJSON, err := json.Marshal(progress)
	if err != nil {
		return Result{IsError: true, Name: "get_worker_task_progress"}, fmt.Errorf("get_worker_task_progress: marshal result: %w", err)
	}

	return Result{
		Name:    "get_worker_task_progress",
		Content: string(resultJSON),
		Metadata: map[string]interface{}{
			"progressTaskId": progress.TaskID,
			"progressStatus": progress.Status,
		},
	}, nil
}
