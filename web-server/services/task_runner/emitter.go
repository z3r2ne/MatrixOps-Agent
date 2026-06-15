package task_runner

import (
	"matrixops-agent/types"
	servicesTypes "matrixops/types"
	"log"
	database "pkgs/db"
	"pkgs/db/models"
	"pkgs/emitter"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
)

const (
	EmitterTypeNormalizedEntry = "normalized_entry"
	EmitterTypeTaskMessage     = "task_message"
	EmitterTypeTaskStatus      = "task_status"
	EmitterTypeError           = "error"
	EmitterTypeSubscribed      = "subscribed"
	EmitterTypeHistory         = "history"
	EmitterTypeRetry           = "retry"
	EmitterTypeIsWorking    = "is_working"
	EmitterTypeIsNotWorking = "is_not_working"
	EmitterTypeMessageUpdated  = "message_updated"
	EmitterTypeSessionID       = "session_id"
	EmitterTypeSessionTitle    = "session_title"
)

type Emitter struct {
	hub       WSHub
	taskID    uint
	db        *gorm.DB
	sessionID string
	errMu     sync.Mutex
	lastError struct {
		message string
		at      time.Time
	}
	*emitter.EventEmitter
}

func NewEmitter(hub WSHub, db *gorm.DB, taskID uint) *Emitter {
	emitter := &Emitter{
		hub:          hub,
		db:           db,
		taskID:       taskID,
		sessionID:    "",
		EventEmitter: emitter.New(),
	}
	initEmitter(emitter)
	return emitter
}

func (e *Emitter) EmitTaskMessage(message *models.TaskMessage) {
	e.Emit(EmitterTypeTaskMessage, message)
}

func (e *Emitter) EmitTaskStatus(status models.TaskStatus, msg string) {
	e.Emit(EmitterTypeTaskStatus, status, msg)
}

func (e *Emitter) EmitMessageUpdated(messageInfoWithParts *types.WithParts) {
	e.Emit(EmitterTypeMessageUpdated, messageInfoWithParts)
}

func (e *Emitter) EmitEndLoading() {
	e.Emit(EmitterTypeIsNotWorking, nil)
}

func (e *Emitter) EmitStartLoading() {
	e.Emit(EmitterTypeIsWorking, nil)
}

func (e *Emitter) EmitIsWorking() {
	e.Emit(EmitterTypeIsWorking, nil)
}

func (e *Emitter) EmitIsNotWorking() {
	e.Emit(EmitterTypeIsNotWorking, nil)
}

func (e *Emitter) EmitSessionID(sessionID string) {
	e.Emit(EmitterTypeSessionID, sessionID)
}

func (e *Emitter) EmitSessionTitle(title string) {
	e.Emit(EmitterTypeSessionTitle, title)
}

func (e *Emitter) EmitError(err error) {
	e.Emit(EmitterTypeError, err)
}

func (e *Emitter) shouldSuppressDuplicateError(message string) bool {
	message = strings.TrimSpace(message)
	if message == "" {
		return true
	}
	e.errMu.Lock()
	defer e.errMu.Unlock()
	now := time.Now()
	if e.lastError.message == message && now.Sub(e.lastError.at) <= 2*time.Second {
		return true
	}
	e.lastError.message = message
	e.lastError.at = now
	return false
}

func (e *Emitter) getCurrentSessionID() string {
	sessionID := strings.TrimSpace(e.sessionID)
	if sessionID != "" {
		return sessionID
	}

	sessionID, err := database.GetTaskSessionID(e.db, e.taskID)
	if err != nil {
		return ""
	}

	sessionID = strings.TrimSpace(sessionID)
	if sessionID != "" {
		e.sessionID = sessionID
	}
	return sessionID
}

func (e *Emitter) getCurrentTaskStatus() models.TaskStatus {
	task, err := database.GetTaskByID(e.db, e.taskID)
	if err != nil || task == nil {
		return models.TaskStatusRunning
	}

	status := models.TaskStatus(strings.TrimSpace(task.Status))
	if status == "" {
		return models.TaskStatusRunning
	}
	return status
}

func initEmitter(e *Emitter) {
	e.On(EmitterTypeNormalizedEntry, func(args ...interface{}) {
		entry := args[0].(*models.NormalizedEntry)
		e.hub.BroadcastNormalizedEntry(e.taskID, entry)
	})
	e.On(EmitterTypeTaskMessage, func(args ...interface{}) {
		message := args[0].(*models.TaskMessage)
		e.hub.BroadcastTaskMessage(e.taskID, message)
	})
	e.On(EmitterTypeTaskStatus, func(args ...interface{}) {
		status := args[0].(models.TaskStatus)
		msg := ""
		if len(args) > 1 {
			msg = args[1].(string)
		}
		sessionID := e.getCurrentSessionID()
		// 更新任务状态和错误信息
		fields := map[string]interface{}{
			"status": string(status),
		}
		if msg != "" && status == models.TaskStatusFailed {
			fields["error"] = msg
		}
		database.UpdateTaskFields(e.db, e.taskID, fields)
		e.hub.BroadcastTaskStatus(e.taskID, status, sessionID, msg)
	})

	e.On(EmitterTypeMessageUpdated, func(args ...interface{}) {
		messageInfoWithParts := args[0].(*types.WithParts)
		e.hub.BroadcastToTask(e.taskID, servicesTypes.WSOutgoingMessage{
			TaskID: e.taskID,
			Type:   servicesTypes.WSTypeMessageV2,
			Data:   messageInfoWithParts,
		})
	})
	e.On(EmitterTypeIsWorking, func(args ...interface{}) {
		e.hub.BroadcastIsWorking(e.taskID)
	})
	e.On(EmitterTypeIsNotWorking, func(args ...interface{}) {
		e.hub.BroadcastIsNotWorking(e.taskID)
	})

	e.On(EmitterTypeSessionID, func(args ...interface{}) {
		sessionID := strings.TrimSpace(args[0].(string))
		if sessionID == "" {
			return
		}

		e.sessionID = sessionID

		// 使用数据访问层更新 session_id
		err := database.UpdateTaskSessionID(e.db, e.taskID, sessionID)
		if err != nil {
			log.Printf("[Emitter] 更新 session_id 失败: %v", err)
			return
		}

		e.hub.BroadcastTaskStatus(e.taskID, e.getCurrentTaskStatus(), sessionID, "")
	})
	e.On(EmitterTypeError, func(args ...interface{}) {
		err := args[0].(error)
		message := strings.TrimSpace(err.Error())
		if e.shouldSuppressDuplicateError(message) {
			return
		}
		e.hub.BroadcastError(e.taskID, message)
	})
	e.On(EmitterTypeSessionTitle, func(args ...interface{}) {
		title := args[0].(string)
		e.hub.BroadcastSessionTitle(e.taskID, title)
	})
}
