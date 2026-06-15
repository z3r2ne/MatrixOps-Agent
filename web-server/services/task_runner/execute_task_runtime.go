package task_runner

import (
	"context"
	"errors"
	"fmt"
	database "pkgs/db"
	"pkgs/db/models"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	agentsession "matrixops-agent/session"
	agenttool "matrixops-agent/tool"
	"matrixops-agent/types"
	"pkgs/taskqueue"
	"gorm.io/gorm"
)

type TaskRuntime struct {
	db    *gorm.DB
	wsHub WSHub

	ctx       context.Context
	sessionID string

	config     *TaskRuntimeConfig
	originOpts []TaskRuntimeConfigOption

	taskID  uint
	emitter *Emitter
	cfg     map[string]interface{}

	// 执行相关
	workDir         string
	execution       models.TaskExecution
	cancel          context.CancelCauseFunc
	agentResultChan chan matrixopsAgentResult
	// MatrixOps Agent 流式处理
	ctxMu           sync.Mutex
	activeToolCalls map[string]activeToolCall
	streamMu        sync.Mutex
	// 助手消息底部状态仅经 WS 推送、不落库；与 DB 合并后再广播
	footerStatusMu       sync.Mutex
	assistantFooterByMsg map[string]*types.MessageFooterStatus
	// 已转发到外部通道（如微信）的 assistant 文本，避免重复推送
	forwardMu                        sync.Mutex
	forwardedAssistantKeys           map[string]struct{}
	forwardedAssistantAttachmentKeys map[string]struct{}
	turnForwardToWeChat              atomic.Bool
	sessionEmitter                   *agentsession.Emitter

	OnEnd func()

	fromWorker string
	toWorker   string
	content    string

	messageQueue *taskqueue.Queue
}

type activeToolCall struct {
	toolName string
	cancel   context.CancelCauseFunc
}

func NewTaskRuntime(opts ...TaskRuntimeConfigOption) (*TaskRuntime, error) {
	config := NewTaskRuntimeConfig(opts...)
	db := config.db
	wsHub := config.wsHub
	if db == nil {
		return nil, fmt.Errorf("new task runtime db is nil")
	}
	if wsHub == nil {
		return nil, fmt.Errorf("wsHub is nil")
	}
	if err := assertTaskRuntimeConfig(config); err != nil {
		return nil, err
	}
	ctx := config.Ctx
	if ctx == nil {
		ctx = context.Background()
	}
	cancelCtx, cancelFunc := context.WithCancelCause(ctx)
	runtime := &TaskRuntime{
		ctx:                  cancelCtx,
		cancel:               cancelFunc,
		sessionID:            config.SessionID,
		config:               config,
		taskID:               config.TaskID,
		emitter:              config.Emitter,
		OnEnd:                config.OnEnd,
		workDir:              config.WorkDir,
		cfg:                  config.WorkerConfig,
		db:                   db,
		wsHub:                wsHub,
		activeToolCalls:      map[string]activeToolCall{},
		assistantFooterByMsg:             make(map[string]*types.MessageFooterStatus),
		forwardedAssistantKeys:           make(map[string]struct{}),
		forwardedAssistantAttachmentKeys: make(map[string]struct{}),
	}
	if config.TaskID > 0 {
		runtime.messageQueue = taskqueue.New(db, config.TaskID, wsHub.BroadcastTaskQueue)
	}
	runtime.ctx = agenttool.WithExecutionController(cancelCtx, runtime)
	if runtime.emitter == nil {
		runtime.emitter = NewEmitter(wsHub, db, runtime.taskID)
	}
	return runtime, nil
}

func generateUUID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func (r *TaskRuntime) Prepare(content string) error {
	// 获取 Git 状态
	// gitState, err := services.GetGitRepoState(r.workDir)
	// var gitCommit, gitBranch string
	// var gitDirty bool
	// var gitModifiedCount, gitUntrackedCount int
	// if err == nil && gitState != nil {
	// 	gitCommit = gitState.CommitHash
	// 	gitBranch = gitState.Branch
	// 	gitDirty = gitState.IsDirty
	// 	gitModifiedCount = gitState.ModifiedCount
	// 	gitUntrackedCount = gitState.UntrackedCount
	// }

	// cmdName, args, err := buildCommand(r.worker, r.cfg)
	// if err != nil {
	// 	return fmt.Errorf("failed to build command: %w", err)
	// }

	// 创建执行记录
	r.execution = models.TaskExecution{
		TaskID:    r.taskID,
		StartedAt: time.Now(),
	}
	if err := database.CreateExecution(r.db, &r.execution); err != nil {
		return fmt.Errorf("failed to create execution record: %w", err)
	}

	return nil
}

func (r *TaskRuntime) emitUserEntry(content string) {
}

func (r *TaskRuntime) Start(opts ...TaskRuntimeConfigOption) error {
	runConfig := r.config.applyOptions(opts...)
	if err := assertTaskRuntimeRunConfig(runConfig); err != nil {
		return err
	}
	runCtx := r.ensureRunContext(runConfig.Ctx)

	r.content = runConfig.Content
	r.fromWorker = runConfig.FromWorker
	r.toWorker = runConfig.ToWorker
	r.turnForwardToWeChat.Store(inputSourceForwardsToWeChat(runConfig.InputSource))

	opts = append(opts, WithMergeMessage(true))
	return r.runMatrixopsAgentPrompt(runCtx, runConfig.SessionID, runConfig.TaskID, runConfig.Content, runConfig.ToWorker, opts...)
}

func (r *TaskRuntime) Wait() (int, error) {
	return 0, nil
}

func (r *TaskRuntime) Finish(exitCode int, runErr error) {
	now := time.Now()
	r.execution.FinishedAt = &now
	r.execution.Duration = now.Sub(r.execution.StartedAt).Milliseconds()

	status := models.TaskStatusDone
	msg := "任务执行完成"
	execStatus := models.TaskStatusSuccess
	var errorMsg string
	cancelledByUser := r.runCancelledByUser(runErr)

	if cancelledByUser {
		status = models.TaskStatusCancelled
		msg = "任务已被用户取消"
		execStatus = models.TaskStatusCancelled
		errorMsg = models.TaskCancelledByUserMessage
	} else if runErr != nil || exitCode != 0 {
		status = models.TaskStatusFailed
		msg = "执行失败"
		execStatus = "failed"
		if runErr != nil {
			errorMsg = runErr.Error()
			msg += ": " + errorMsg
		} else {
			errorMsg = fmt.Sprintf("Exit code: %d", exitCode)
		}

	}

	if errorMsg != "" {
		r.emitter.EmitError(errors.New(errorMsg))
	}

	// 更新任务状态和错误信息
	fields := map[string]interface{}{
		"status": string(status),
		"error":  "",
	}
	if errorMsg != "" && (status == models.TaskStatusFailed || status == models.TaskStatusCancelled) {
		fields["error"] = errorMsg
	}
	database.UpdateTaskFields(r.db, r.taskID, fields)

	r.emitter.EmitTaskMessage(&models.TaskMessage{Type: "status", Role: "system", Content: msg})
	r.emitter.EmitIsNotWorking()
	r.emitter.EmitEndLoading()

	r.wsHub.BroadcastTaskStatus(r.taskID, status, r.config.SessionID, msg)

	// 更新执行记录
	database.UpdateExecutionFields(r.db, r.execution.ID, map[string]interface{}{
		"status":      string(execStatus),
		"finished_at": r.execution.FinishedAt,
		"duration":    r.execution.Duration,
	})

	if r.OnEnd != nil {
		r.OnEnd()
	}

}

func (r *TaskRuntime) dequeueNextMessageQueueItem() (*models.TaskMessageQueueItem, error) {
	// 给数据库一点时间完成事务
	time.Sleep(300 * time.Millisecond)

	if r.messageQueue == nil {
		return nil, nil
	}
	return r.messageQueue.DequeueNext()
}

func (r *TaskRuntime) updateTaskStatus(status models.TaskStatus, msg string) {
	// 更新状态字段和错误信息
	fields := map[string]interface{}{
		"status": string(status),
	}
	if msg != "" && (status == models.TaskStatusFailed || status == models.TaskStatusCancelled) {
		fields["error"] = msg
	}
	database.UpdateTaskFields(r.db, r.taskID, fields)

	if status == models.TaskStatusFailed || status == models.TaskStatusCancelled {
		_ = database.SetTaskMessageQueueAutoSend(r.db, r.taskID, false)
		if queue, queueErr := database.GetTaskQueue(r.db, r.taskID); queueErr == nil {
			r.wsHub.BroadcastTaskQueue(r.taskID, queue)
		}
	}

	r.emitter.EmitTaskMessage(&models.TaskMessage{Type: "status", Role: "system", Content: msg})
	if status == models.TaskStatusFailed {
		r.emitter.EmitError(errors.New(msg))
	}
	if r.isEndStatus(string(status)) {
		r.emitter.EmitEndLoading()
	}
	r.emitter.EmitTaskStatus(status, msg)
}

func (r *TaskRuntime) isEndStatus(status string) bool {
	return status == string(models.TaskStatusFailed) ||
		status == string(models.TaskStatusDone) ||
		status == string(models.TaskStatusCancelled)
}

func (r *TaskRuntime) ensureRunContext(parent context.Context) context.Context {
	r.ctxMu.Lock()
	defer r.ctxMu.Unlock()

	if parent == nil {
		parent = context.Background()
	}
	if r.ctx == nil || r.ctx.Err() != nil {
		cancelCtx, cancelFunc := context.WithCancelCause(parent)
		r.ctx = agenttool.WithExecutionController(cancelCtx, r)
		r.cancel = cancelFunc
	}
	return r.ctx
}

func (r *TaskRuntime) cancelCurrentRun() {
	r.ctxMu.Lock()
	cancel := r.cancel
	toolCancels := make([]context.CancelCauseFunc, 0, len(r.activeToolCalls))
	for _, active := range r.activeToolCalls {
		if active.cancel != nil {
			toolCancels = append(toolCancels, active.cancel)
		}
	}
	r.ctxMu.Unlock()

	for _, toolCancel := range toolCancels {
		toolCancel(agenttool.ErrTaskExecutionCancelledByUser)
	}
	if cancel != nil {
		cancel(agenttool.ErrTaskExecutionCancelledByUser)
	}
}

func (r *TaskRuntime) runCancelledByUser(runErr error) bool {
	if r == nil || runErr == nil || !errors.Is(runErr, context.Canceled) {
		return false
	}
	r.ctxMu.Lock()
	defer r.ctxMu.Unlock()
	if r.ctx == nil {
		return false
	}
	return errors.Is(context.Cause(r.ctx), agenttool.ErrTaskExecutionCancelledByUser)
}

func (r *TaskRuntime) RegisterCancelableToolCall(callID string, toolName string, cancel context.CancelCauseFunc) {
	if strings.TrimSpace(callID) == "" || cancel == nil {
		return
	}
	r.ctxMu.Lock()
	defer r.ctxMu.Unlock()
	if r.activeToolCalls == nil {
		r.activeToolCalls = map[string]activeToolCall{}
	}
	r.activeToolCalls[callID] = activeToolCall{
		toolName: strings.TrimSpace(toolName),
		cancel:   cancel,
	}
}

func (r *TaskRuntime) FinishCancelableToolCall(callID string) {
	if strings.TrimSpace(callID) == "" {
		return
	}
	r.ctxMu.Lock()
	defer r.ctxMu.Unlock()
	delete(r.activeToolCalls, callID)
}

func (r *TaskRuntime) cancelToolCall(callID string) error {
	trimmed := strings.TrimSpace(callID)
	if trimmed == "" {
		return fmt.Errorf("tool call id is empty")
	}
	r.ctxMu.Lock()
	entry, ok := r.activeToolCalls[trimmed]
	r.ctxMu.Unlock()
	if !ok || entry.cancel == nil {
		return fmt.Errorf("tool call %s not running", trimmed)
	}
	entry.cancel(agenttool.ErrToolExecutionCancelledByUser)
	return nil
}
