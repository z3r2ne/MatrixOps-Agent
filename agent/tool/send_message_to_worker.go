package tool

import (
	"errors"
	"fmt"
	"strings"
)

type SendMessageToWorkerTool struct {
	send               RunWorkerTaskFunc
	availableWorkersFn func() []string
}

func NewSendMessageToWorkerTool(send RunWorkerTaskFunc, availableWorkersFn func() []string) *SendMessageToWorkerTool {
	return &SendMessageToWorkerTool{
		send:               send,
		availableWorkersFn: availableWorkersFn,
	}
}

func (SendMessageToWorkerTool) Name() string { return "send_message_to_worker" }

func (SendMessageToWorkerTool) VerbosName() string { return "发送消息给子任务" }

func (SendMessageToWorkerTool) Description() string {
	return "向已有的 worker 子任务发送新消息。需要提供 task_id（从 run_worker_task 返回的 metadata 中的 subtaskTaskId 获取）和要发送的内容。子 worker 会看到此前对话并继续回复。"
}

func (SendMessageToWorkerTool) Schema() map[string]interface{} {
	return ObjectParamSchema(map[string]interface{}{
		"task_id": map[string]interface{}{
			"type":        "integer",
			"description": "目标子任务 ID，取自 run_worker_task 返回的 metadata 中的 subtaskTaskId。",
		},
		"content": map[string]interface{}{
			"type":        "string",
			"description": "要发送给子 worker 的消息内容。",
		},
	}, []string{"task_id", "content"})
}

func (t *SendMessageToWorkerTool) Execute(ctx Context, input map[string]interface{}) (Result, error) {
	if t == nil || t.send == nil {
		return Result{IsError: true, Name: "send_message_to_worker"}, errors.New("send_message_to_worker: missing sender")
	}

	taskID, err := parseOptionalUintID(input["task_id"])
	if err != nil {
		return Result{IsError: true, Name: "send_message_to_worker"}, fmt.Errorf("send_message_to_worker: invalid task_id: %w", err)
	}
	if taskID == 0 {
		return Result{IsError: true, Name: "send_message_to_worker"}, errors.New("send_message_to_worker: missing task_id")
	}

	content, _ := input["content"].(string)
	content = strings.TrimSpace(content)
	if content == "" {
		return Result{IsError: true, Name: "send_message_to_worker"}, errors.New("send_message_to_worker: missing content")
	}

	progressReporter := func(progress RunWorkerTaskProgress) {
		metadata := map[string]interface{}{
			"subtaskTaskId":       progress.TaskID,
			"subtaskSessionId":    progress.SessionID,
			"subtaskParentTaskId": progress.ParentTaskID,
			"subtaskWorkerName":   progress.WorkerName,
			"subtaskTaskName":     progress.TaskName,
			"subtaskContent":      progress.Content,
			"subtaskStatus":       progress.Status,
			"subtaskAnswer":       progress.Answer,
		}
		ctx.EmitEvent(StreamEvent{
			Status:   "running",
			Title:    fmt.Sprintf("发送消息给子任务 · %s", progress.WorkerName),
			Metadata: metadata,
		})
	}

	result, err := t.send(ctx, RunWorkerTaskRequest{
		Content: content,
		TaskID:  taskID,
	}, progressReporter)

	metadata := map[string]interface{}{
		"subtaskTaskId":        result.TaskID,
		"subtaskSessionId":     result.SessionID,
		"subtaskParentTaskId":  result.ParentTaskID,
		"subtaskWorkerName":    result.WorkerName,
		"subtaskTaskName":      result.TaskName,
		"subtaskContent":       result.Content,
		"subtaskStatus":        result.Status,
		"subtaskAnswer":        result.Answer,
		"subtaskSummary":       result.Summary,
		"subtaskError":         result.Error,
		"subtaskDurationMs":    result.DurationMs,
		"subtaskWorkDir":       result.WorkDir,
		"subtaskBranch":        result.Branch,
		"subtaskBaseBranch":    result.BaseBranch,
		"subtaskModifiedFiles": result.ModifiedFiles,
		"subtaskCreatedFiles":  result.CreatedFiles,
	}

	contentSummary := strings.TrimSpace(result.Summary)
	if contentSummary == "" {
		contentSummary = strings.TrimSpace(result.Answer)
	}

	toolResult := Result{
		Name:     "send_message_to_worker",
		Content:  contentSummary,
		Metadata: metadata,
	}
	if contentSummary != "" {
		toolResult.FullContent = contentSummary
	}
	if err != nil {
		toolResult.IsError = true
		return toolResult, err
	}
	if strings.TrimSpace(result.Status) == "cancelled" {
		toolResult.Metadata["subtaskCancelled"] = true
	}
	return toolResult, nil
}
