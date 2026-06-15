package task_runner

import (
	"context"
	"time"

	database "pkgs/db"
	"pkgs/db/models"
)

// buildSubtaskRunContext 合并父任务与子工具调用的 context：任一取消时子任务也应停止。
func buildSubtaskRunContext(parentTaskCtx, parentToolCtx context.Context) (context.Context, context.CancelFunc) {
	parents := make([]context.Context, 0, 2)
	if parentTaskCtx != nil {
		parents = append(parents, parentTaskCtx)
	}
	if parentToolCtx != nil && parentToolCtx != parentTaskCtx {
		parents = append(parents, parentToolCtx)
	}
	if len(parents) == 0 {
		return context.Background(), func() {}
	}
	if len(parents) == 1 {
		return context.WithCancel(parents[0])
	}
	return mergeContexts(parents...)
}

// waitSubtaskResult 等待子任务结束；若 runCtx 先取消则主动 CancelTask 并返回取消原因。
func (r *TaskRuntime) waitSubtaskResult(runCtx context.Context, taskID uint) (map[string]interface{}, error) {
	if runCtx == nil {
		runCtx = context.Background()
	}

	done := make(chan error, 1)
	go func() {
		done <- WaitTask(taskID)
	}()

	select {
	case err := <-done:
		if err != nil {
			return nil, err
		}
		return r.loadSubtaskResultPayload(taskID)
	case <-runCtx.Done():
		_ = CancelTask(taskID)
		select {
		case err := <-done:
			if err != nil {
				return nil, context.Cause(runCtx)
			}
			payload, loadErr := r.loadSubtaskResultPayload(taskID)
			if loadErr != nil {
				return nil, context.Cause(runCtx)
			}
			if status, _ := payload["status"].(string); status == "" {
				payload["status"] = string(models.TaskStatusCancelled)
			}
			return payload, context.Cause(runCtx)
		case <-time.After(30 * time.Second):
			return nil, context.Cause(runCtx)
		}
	}
}

func (r *TaskRuntime) loadSubtaskResultPayload(taskID uint) (map[string]interface{}, error) {
	freshTask, err := database.GetTaskByID(r.db, taskID)
	if err != nil {
		return nil, err
	}
	completion, err := getSubtaskCompletionResult(r.db, taskID)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"taskId":        freshTask.ID,
		"sessionId":     completion.SessionID,
		"status":        freshTask.Status,
		"answer":        completion.Answer,
		"summary":       completion.Summary,
		"error":         completion.Error,
		"durationMs":    completion.DurationMs,
		"workDir":       completion.WorkDir,
		"branch":        completion.Branch,
		"baseBranch":    completion.BaseBranch,
		"modifiedFiles": completion.ModifiedFiles,
		"createdFiles":  completion.CreatedFiles,
	}, nil
}
