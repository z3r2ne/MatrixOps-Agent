package session

import (
	"fmt"
	"strings"
	"time"

	coregit "matrixops.local/core_agent/git"
	database "pkgs/db"
	"pkgs/db/models"
)

func (r *AgentRunner) handleNewWorktreeCommand(runtimeConfig *RuntimeConfig) error {
	project := runtimeConfig.Project
	if project == nil {
		return fmt.Errorf("未关联项目")
	}

	baseBranch := r.task.BaseBranch
	if baseBranch == "" {
		var err error
		baseBranch, err = coregit.CurrentBranch(project.Path)
		if err != nil {
			return fmt.Errorf("获取当前分支失败: %w", err)
		}
	}

	newBranch := strings.TrimSpace(runtimeConfig.NewWorktreeBranch)

	worktreePath, err := coregit.CreateWorktree(project.Path, project.WorktreePath, newBranch, baseBranch)
	if err != nil {
		return fmt.Errorf("创建 worktree 失败: %w", err)
	}

	// 更新 task
	r.task.WorkDir = worktreePath
	r.task.Branch = newBranch
	if err := database.UpdateTask(r.db, r.task); err != nil {
		return fmt.Errorf("更新任务失败: %w", err)
	}

	appendItem := models.TaskMessageQueueItem{
		ID:        fmt.Sprintf("append-new-worktree-%d", time.Now().UnixNano()),
		Type:      models.TaskMessageQueueTypeAppend,
		Content:   fmt.Sprintf("用户已切换工作目录为 %s", worktreePath),
		Source:    models.TaskMessageQueueSourceFrontend,
		CreatedAt: time.Now().UnixMilli(),
	}
	if err := r.deliverImmediateQueueItem(runtimeConfig, appendItem); err != nil {
		return fmt.Errorf("写入 worktree 提示: %w", err)
	}

	// 更新运行时工作目录
	if r.session != nil {
		r.session.Directory = worktreePath
	}
	runtimeConfig.NewWorktreeBranch = "" // 重置

	// 广播任务更新，让前端知道 workDir 变了
	if r.emitter != nil {
		r.emitter.Emit("task.workdir.changed", worktreePath)
	}

	return nil
}

func (r *AgentRunner) emitCommandFailure(runtimeConfig *RuntimeConfig, command string, err error) {
	if r == nil || runtimeConfig == nil || runtimeConfig.Assistant == nil || err == nil {
		return
	}
	messageError := &MessageError{
		Name:    command + "Error",
		Message: err.Error(),
	}
	runtimeConfig.Assistant.State = "completed"
	runtimeConfig.Assistant.Time.Completed = time.Now().UnixMilli()
	runtimeConfig.Assistant.Error = messageError
	r.emitAssistantErrorPart(runtimeConfig.Assistant, messageError)
	if r.emitter != nil {
		r.emitter.Emit(EventSessionError, SessionErrorEvent{
			SessionID: runtimeConfig.Assistant.SessionID,
			Error:     messageError,
		})
		_, _ = r.emitter.UpdateMessage(runtimeConfig.Assistant)
	}
}
