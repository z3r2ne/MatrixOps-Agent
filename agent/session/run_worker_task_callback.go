package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"matrixops-agent/tool"
	"matrixops-agent/types"
	database "pkgs/db"
	"pkgs/db/storage"

	"gorm.io/gorm"
)

func buildRunWorkerTaskFunc(
	db *gorm.DB,
	parentTaskID uint,
	newTaskHandler func(args map[string]interface{}) (map[string]interface{}, error),
	parentMemorySnapshot func() *types.Memory,
) tool.RunWorkerTaskFunc {
	if newTaskHandler == nil {
		return nil
	}

	return func(ctx tool.Context, req tool.RunWorkerTaskRequest, onProgress func(tool.RunWorkerTaskProgress)) (tool.RunWorkerTaskResult, error) {
		if err := tool.CheckContext(ctx); err != nil {
			return tool.RunWorkerTaskResult{}, err
		}
		if err := validateRunWorkerTaskContinuation(db, parentTaskID, req); err != nil {
			return tool.RunWorkerTaskResult{}, err
		}

		previewBridge := newSubtaskPreviewBridge(db, ctx)
		args := map[string]interface{}{
			"workerName":              req.WorkerName,
			"inputText":               req.Content,
			"taskName":                req.TaskName,
			"parentTaskId":            parentTaskID,
			"mergeMessage":            true,
			"skipCreateUserMessage":   req.SkipCreateUserMessage,
			"sessionWindow":           req.SessionWindow,
			"onSubtaskEmitterCreated": previewBridge.Attach,
		}
		if ctx.Context != nil {
			args["parentContext"] = ctx.Context
		}
		if req.TaskID > 0 {
			args["taskId"] = req.TaskID
		}

		payload, err := newTaskHandler(args)
		if err != nil {
			return tool.RunWorkerTaskResult{}, err
		}

		taskID, ok := payload["taskId"].(uint)
		if !ok || taskID == 0 {
			return tool.RunWorkerTaskResult{}, errors.New("run_worker_task: invalid task id")
		}
		taskModel, err := database.GetTaskByID(db, taskID)
		if err != nil {
			return tool.RunWorkerTaskResult{}, err
		}

		taskName := strings.TrimSpace(taskModel.Name)
		if taskName == "" {
			taskName = strings.TrimSpace(taskModel.Content)
		}
		progress := tool.RunWorkerTaskProgress{
			TaskID:       taskModel.ID,
			SessionID:    taskModel.SessionID,
			ParentTaskID: taskModel.ParentTaskID,
			WorkerName:   taskModel.WorkerName,
			TaskName:     taskName,
			Content:      taskModel.Content,
			Status:       taskModel.Status,
		}
		previewBridge.SetSubtask(progress)
		if onProgress != nil {
			onProgress(progress)
		}

		waitResult, ok := payload["waitResult"].(func() (map[string]interface{}, error))
		if !ok {
			return tool.RunWorkerTaskResult{}, errors.New("run_worker_task: missing waitResult")
		}
		if err := tool.CheckContext(ctx); err != nil {
			return tool.RunWorkerTaskResult{
				TaskID:       taskModel.ID,
				SessionID:    taskModel.SessionID,
				ParentTaskID: taskModel.ParentTaskID,
				WorkerName:   taskModel.WorkerName,
				TaskName:     taskName,
				Content:      taskModel.Content,
				Status:       "cancelled",
			}, err
		}

		finalPayload, err := waitResult()
		if err != nil {
			status := "failed"
			if errors.Is(err, context.Canceled) {
				status = "cancelled"
			}
			return tool.RunWorkerTaskResult{
				TaskID:       taskModel.ID,
				SessionID:    taskModel.SessionID,
				ParentTaskID: taskModel.ParentTaskID,
				WorkerName:   taskModel.WorkerName,
				TaskName:     taskName,
				Content:      taskModel.Content,
				Status:       status,
			}, err
		}

		final := tool.RunWorkerTaskResult{
			TaskID:        taskModel.ID,
			SessionID:     toStringValue(finalPayload["sessionId"]),
			ParentTaskID:  taskModel.ParentTaskID,
			WorkerName:    taskModel.WorkerName,
			TaskName:      taskName,
			Content:       taskModel.Content,
			Status:        toStringValue(finalPayload["status"]),
			Answer:        toStringValue(finalPayload["answer"]),
			Summary:       toStringValue(finalPayload["summary"]),
			Error:         toStringValue(finalPayload["error"]),
			DurationMs:    toInt64Value(finalPayload["durationMs"]),
			WorkDir:       toStringValue(finalPayload["workDir"]),
			Branch:        toStringValue(finalPayload["branch"]),
			BaseBranch:    toStringValue(finalPayload["baseBranch"]),
			ModifiedFiles: toStringSliceValue(finalPayload["modifiedFiles"]),
			CreatedFiles:  toStringSliceValue(finalPayload["createdFiles"]),
		}
		finalProgress := tool.RunWorkerTaskProgress{
			TaskID:       final.TaskID,
			SessionID:    final.SessionID,
			ParentTaskID: final.ParentTaskID,
			WorkerName:   final.WorkerName,
			TaskName:     final.TaskName,
			Content:      final.Content,
			Status:       final.Status,
			Answer:       final.Answer,
		}
		previewBridge.SetSubtask(finalProgress)
		if onProgress != nil {
			onProgress(finalProgress)
		}
		return final, nil
	}
}

func validateRunWorkerTaskContinuation(db *gorm.DB, parentTaskID uint, req tool.RunWorkerTaskRequest) error {
	if req.TaskID == 0 {
		return nil
	}
	if db == nil {
		return errors.New("run_worker_task: database unavailable for task_id continuation")
	}
	task, err := database.GetTaskByID(db, req.TaskID)
	if err != nil {
		return fmt.Errorf("run_worker_task: load task %d: %w", req.TaskID, err)
	}
	if task == nil {
		return fmt.Errorf("run_worker_task: task %d not found", req.TaskID)
	}
	if parentTaskID == 0 {
		return fmt.Errorf("run_worker_task: task %d cannot be continued without parent task context", req.TaskID)
	}
	if task.ParentTaskID == nil || *task.ParentTaskID != parentTaskID {
		return fmt.Errorf("run_worker_task: task %d is not a subtask of current task %d", req.TaskID, parentTaskID)
	}
	workerName := strings.TrimSpace(req.WorkerName)
	taskWorker := strings.TrimSpace(task.WorkerName)
	if workerName != "" && taskWorker != "" && workerName != taskWorker {
		return fmt.Errorf("run_worker_task: task %d belongs to worker %q, not %q", req.TaskID, taskWorker, workerName)
	}
	return nil
}

const maxSubtaskPreviewMessages = 8
const maxSubtaskPreviewPartsPerMessage = 12
const maxSubtaskPreviewTextLen = 1200

type subtaskPreviewBridge struct {
	db          *gorm.DB
	ctx         tool.Context
	mu          sync.Mutex
	taskID      uint
	sessionID   string
	workerName  string
	taskName    string
	content     string
	status      string
	answer      string
	messageByID map[string]map[string]interface{}
	order       []string
}

func newSubtaskPreviewBridge(db *gorm.DB, ctx tool.Context) *subtaskPreviewBridge {
	return &subtaskPreviewBridge{
		db:          db,
		ctx:         ctx,
		messageByID: map[string]map[string]interface{}{},
		order:       make([]string, 0, maxSubtaskPreviewMessages),
	}
}

func (b *subtaskPreviewBridge) SetSubtask(progress tool.RunWorkerTaskProgress) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.taskID = progress.TaskID
	b.sessionID = strings.TrimSpace(progress.SessionID)
	b.workerName = strings.TrimSpace(progress.WorkerName)
	b.taskName = strings.TrimSpace(progress.TaskName)
	b.content = strings.TrimSpace(progress.Content)
	b.status = strings.TrimSpace(progress.Status)
	b.answer = strings.TrimSpace(progress.Answer)
}

func (b *subtaskPreviewBridge) Attach(emitter *Emitter) error {
	if b == nil || emitter == nil {
		return nil
	}

	emitter.On(EventSessionCreated, func(args ...interface{}) {
		event := args[0].(SessionEvent)
		if event.Info == nil {
			return
		}
		b.mu.Lock()
		b.sessionID = strings.TrimSpace(event.Info.ID)
		b.mu.Unlock()
		b.emitCurrent()
	})

	emitter.On(EventMessageUpdated, func(args ...interface{}) {
		event := args[0].(MessageEvent)
		if event.Info == nil || strings.TrimSpace(event.Info.ID) == "" || b.db == nil {
			return
		}
		messageInfoWithParts, err := storage.GetMessageWithPartsLight(b.db, event.Info.ID)
		if err != nil || messageInfoWithParts == nil || messageInfoWithParts.Info == nil {
			return
		}
		simplified := buildSubtaskPreviewMessage(messageInfoWithParts)
		if simplified == nil {
			return
		}

		b.mu.Lock()
		defer b.mu.Unlock()
		messageID := messageInfoWithParts.Info.ID
		if _, exists := b.messageByID[messageID]; !exists {
			b.order = append(b.order, messageID)
		}
		b.messageByID[messageID] = simplified
		if len(b.order) > maxSubtaskPreviewMessages {
			toDrop := b.order[0]
			delete(b.messageByID, toDrop)
			b.order = b.order[1:]
		}
		b.emitLocked()
	})

	return nil
}

func (b *subtaskPreviewBridge) emitCurrent() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.emitLocked()
}

func (b *subtaskPreviewBridge) emitLocked() {
	if b == nil {
		return
	}
	metadata := map[string]interface{}{
		"subtaskTaskId":          b.taskID,
		"subtaskSessionId":       b.sessionID,
		"subtaskWorkerName":      b.workerName,
		"subtaskTaskName":        b.taskName,
		"subtaskContent":         b.content,
		"subtaskStatus":          b.status,
		"subtaskAnswer":          b.answer,
		"subtaskPreviewMessages": b.snapshotMessagesLocked(),
	}
	b.ctx.EmitEvent(tool.StreamEvent{
		Status:   "running",
		Title:    "子任务输出同步中",
		Metadata: metadata,
	})
}

func (b *subtaskPreviewBridge) snapshotMessagesLocked() []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(b.order))
	for _, messageID := range b.order {
		msg := b.messageByID[messageID]
		if msg == nil {
			continue
		}
		out = append(out, msg)
	}
	return out
}

func buildSubtaskPreviewMessage(message *types.WithParts) map[string]interface{} {
	if message == nil || message.Info == nil {
		return nil
	}
	role := strings.TrimSpace(string(message.Info.Role))
	if role == "" {
		return nil
	}
	if role != string(types.RoleAssistant) && role != "system" {
		return nil
	}

	parts := make([]map[string]interface{}, 0, len(message.Parts))
	for _, part := range message.Parts {
		simplified := buildSubtaskPreviewPart(part)
		if simplified == nil {
			continue
		}
		parts = append(parts, simplified)
	}
	if len(parts) == 0 {
		return nil
	}
	if len(parts) > maxSubtaskPreviewPartsPerMessage {
		parts = parts[len(parts)-maxSubtaskPreviewPartsPerMessage:]
	}

	return map[string]interface{}{
		"id":         message.Info.ID,
		"role":       role,
		"workerName": strings.TrimSpace(message.Info.Worker),
		"parts":      parts,
	}
}

func buildSubtaskPreviewPart(part *types.Part) map[string]interface{} {
	if part == nil {
		return nil
	}

	switch part.Type {
	case types.PartTypeText, types.PartTypeTextDelta:
		text := truncateSubtaskPreviewText(strings.TrimSpace(part.Text))
		if text == "" {
			return nil
		}
		return map[string]interface{}{
			"id":   part.ID,
			"type": "text",
			"text": text,
		}
	case types.PartTypeTool, types.PartTypeToolDelta:
		if part.Tool == nil {
			return nil
		}
		inputPreview := truncateSubtaskPreviewText(strings.TrimSpace(renderSubtaskPreviewToolInput(part.Tool.State.Input)))
		output := ""
		if trimmed := strings.TrimSpace(part.Tool.State.Output); trimmed != "" {
			output = truncateSubtaskPreviewText(trimmed)
		} else if trimmed := strings.TrimSpace(part.Tool.State.Error); trimmed != "" {
			output = truncateSubtaskPreviewText(trimmed)
		}
		return map[string]interface{}{
			"id":       part.ID,
			"type":     "tool",
			"toolName": strings.TrimSpace(part.Tool.Name),
			"status":   strings.TrimSpace(part.Tool.State.Status),
			"title":    truncateSubtaskPreviewText(strings.TrimSpace(part.Tool.State.Title)),
			"input":    inputPreview,
			"output":   output,
		}
	case types.PartTypeError:
		message := ""
		if part.Error != nil {
			message = strings.TrimSpace(part.Error.Message)
		}
		if message == "" {
			message = strings.TrimSpace(part.Text)
		}
		message = truncateSubtaskPreviewText(message)
		if message == "" {
			return nil
		}
		return map[string]interface{}{
			"id":      part.ID,
			"type":    "error",
			"message": message,
		}
	default:
		return nil
	}
}

func truncateSubtaskPreviewText(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if len(value) <= maxSubtaskPreviewTextLen {
		return value
	}
	return strings.TrimSpace(value[:maxSubtaskPreviewTextLen]) + "…"
}

func toInt64Value(value interface{}) int64 {
	switch typed := value.(type) {
	case int64:
		return typed
	case int:
		return int64(typed)
	case uint:
		return int64(typed)
	case uint64:
		return int64(typed)
	case float64:
		return int64(typed)
	case float32:
		return int64(typed)
	default:
		return 0
	}
}

func toStringSliceValue(value interface{}) []string {
	switch typed := value.(type) {
	case []string:
		return typed
	case []interface{}:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if text := strings.TrimSpace(toStringValue(item)); text != "" {
				out = append(out, text)
			}
		}
		return out
	default:
		return nil
	}
}

func renderSubtaskPreviewToolInput(input interface{}) string {
	if input == nil {
		return ""
	}
	switch typed := input.(type) {
	case string:
		return typed
	case []byte:
		return string(typed)
	}
	payload, err := json.Marshal(input)
	if err != nil {
		return ""
	}
	return string(payload)
}
