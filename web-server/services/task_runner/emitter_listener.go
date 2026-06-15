package task_runner

import (
	agentsession "matrixops-agent/session"
	"matrixops-agent/types"
	"matrixops-agent/util"
	"errors"
	"fmt"
	"log"
	database "pkgs/db"
	"pkgs/db/models"
	"pkgs/db/storage"
	"pkgs/waiter"
	"strings"

	servicestypes "matrixops/types"
)

func (r *TaskRuntime) onEmitterCreated(emitter *agentsession.Emitter) error {
	r.sessionEmitter = emitter
	emitter.On(agentsession.EventMessageUpdated, func(args ...interface{}) {
		event := args[0].(agentsession.MessageEvent)
		messageInfoWithParts, err := storage.GetMessageWithPartsLight(r.db, event.Info.ID)
		if err != nil {
			log.Printf("[onEmitterCreated] on message event error: %v", err)
			return
		}
		r.applyAssistantFooterOverlay(messageInfoWithParts)
		err = r.onMessagePartsUpdated(messageInfoWithParts)
		if err != nil {
			log.Printf("[onEmitterCreated] on message parts updated error: %v", err)
		}
	})
	emitter.On(agentsession.EventSessionCreated, func(args ...interface{}) {
		event := args[0].(agentsession.SessionEvent)
		if event.Info == nil || event.Info.ID == "" {
			return
		}
		r.sessionID = event.Info.ID
		if r.config != nil {
			r.config.SessionID = event.Info.ID
		}
		if r.execution.ID != 0 {
			r.execution.AgentSessionID = event.Info.ID
			if err := database.UpdateExecutionSessionID(r.db, r.execution.ID, event.Info.ID); err != nil {
				log.Printf("[onEmitterCreated] update execution session id error: %v", err)
			}
		}
		r.emitter.EmitSessionID(event.Info.ID)
	})
	emitter.On(agentsession.EventSessionError, func(args ...interface{}) {
		event := args[0].(agentsession.SessionErrorEvent)
		r.emitter.EmitError(errors.New(formatSessionErrorMessage(event.Error)))
	})
	emitter.On(agentsession.EventError, func(args ...interface{}) {
		err := args[0].(error)
		r.emitter.EmitError(err)
	})
	emitter.On(agentsession.EventSessionTitleUpdated, func(args ...interface{}) {
		title := args[0].(string)
		r.emitter.EmitSessionTitle(title)
	})
	emitter.On(agentsession.EventWaitUserInput, func(args ...interface{}) {
		event := args[0].(agentsession.WaitUserInputEvent)
		question := event.Questions

		waiterID := util.Ascending("waiter")
		waiterIns := waiter.NewWaiter()
		waitFunc := waiterIns.Create(waiterID)
		r.emitter.hub.BroadcastWaitUserInput(r.taskID, waiterID, func(result map[string]interface{}) {
			question["result"] = result
			waiterIns.Ack(waiterID)
		}, question)
		waitFunc()
	})
	emitter.On(agentsession.EventAssistantFooterStatus, func(args ...interface{}) {
		event := args[0].(agentsession.AssistantFooterStatusEvent)
		if strings.TrimSpace(event.MessageID) == "" {
			return
		}
		if strings.TrimSpace(event.Text) == "" && !event.Loading {
			r.footerStatusMu.Lock()
			delete(r.assistantFooterByMsg, event.MessageID)
			r.footerStatusMu.Unlock()
		} else {
			r.footerStatusMu.Lock()
			r.assistantFooterByMsg[event.MessageID] = &types.MessageFooterStatus{
				Text:    event.Text,
				Loading: event.Loading,
			}
			r.footerStatusMu.Unlock()
		}
		wp, err := storage.GetMessageWithPartsLight(r.db, event.MessageID)
		if err != nil {
			log.Printf("[onEmitterCreated] footer status get message: %v", err)
			return
		}
		r.applyAssistantFooterOverlay(wp)
		if err := r.onMessagePartsUpdated(wp); err != nil {
			log.Printf("[onEmitterCreated] footer status emit: %v", err)
		}
	})
	emitter.On(agentsession.EventPlanUpdated, func(args ...interface{}) {
		if len(args) == 0 {
			return
		}
		r.wsHub.BroadcastTaskPlan(r.taskID, args[0])
	})
	emitter.On("task.workdir.changed", func(args ...interface{}) {
		worktreePath := strings.TrimSpace(args[0].(string))
		if worktreePath == "" {
			return
		}
		r.workDir = worktreePath
		status := models.TaskStatusRunning
		if task, err := database.GetTaskByID(r.db, r.taskID); err == nil && task != nil {
			if trimmed := strings.TrimSpace(task.Status); trimmed != "" {
				status = models.TaskStatus(trimmed)
			}
		}
		_ = database.UpdateTaskFields(r.db, r.taskID, map[string]interface{}{
			"work_dir": worktreePath,
		})
		r.wsHub.BroadcastToTask(r.taskID, servicestypes.WSOutgoingMessage{
			Type:   servicestypes.WSTypeTaskStatus,
			TaskID: r.taskID,
			Status: string(status),
			Data: map[string]interface{}{
				"workDir": worktreePath,
			},
		})
	})

	return nil
}

func formatSessionErrorMessage(err *types.MessageError) string {
	if err == nil {
		return "未知错误"
	}

	lines := make([]string, 0, 6)
	if msg := strings.TrimSpace(err.Message); msg != "" {
		lines = append(lines, msg)
	}

	reason := ""
	if err.Metadata != nil {
		reason = strings.TrimSpace(err.Metadata["reason"])
	}
	if reason == "" {
		reason = strings.TrimSpace(err.ResponseBody)
	}
	if reason != "" && reason != strings.TrimSpace(err.Message) {
		lines = append(lines, "原因: "+reason)
	}

	if err.StatusCode > 0 {
		lines = append(lines, fmt.Sprintf("状态码: %d", err.StatusCode))
	}
	if err.ProviderID != "" {
		lines = append(lines, "Provider: "+err.ProviderID)
	}

	if len(lines) == 0 {
		return "未知错误"
	}
	return strings.Join(lines, "\n")
}

func (r *TaskRuntime) applyAssistantFooterOverlay(wp *types.WithParts) {
	if wp == nil || wp.Info == nil {
		return
	}
	if wp.Info.Role != types.RoleAssistant {
		return
	}
	r.footerStatusMu.Lock()
	defer r.footerStatusMu.Unlock()
	fs, ok := r.assistantFooterByMsg[wp.Info.ID]
	if !ok {
		return
	}
	wp.Info.FooterStatus = &types.MessageFooterStatus{Text: fs.Text, Loading: fs.Loading}
}

func (r *TaskRuntime) onMessagePartsUpdated(messageInfoWithParts *types.WithParts) error {

	if messageInfoWithParts.Info.SessionID != "" {
		// if r.msgStore != nil && r.msgStore.GetSessionID() == "" {
		// 	r.msgStore.PushSessionID(messageInfoWithParts.Info.SessionID)
		// 	r.emitter.EmitTaskMessage(&models.TaskMessage{
		// 		Type:      "session_id",
		// 		Role:      "system",
		// 		Content:   messageInfoWithParts.Info.SessionID,
		// 		Timestamp: time.Now().UnixMilli(),
		// 	})
		// }
	}

	r.emitter.EmitMessageUpdated(messageInfoWithParts)
	r.maybeForwardAssistantMessage(messageInfoWithParts)
	return nil
}

// func (r *TaskRuntime) onMessageEvent(event *agentsession.MessageEvent) error {
// 	r.streamMu.Lock()
// 	defer r.streamMu.Unlock()

// 	if event.Info == nil {
// 		return errors.New("message info is nil")
// 	}

// 	if event.Info.SessionID != "" {
// 		if r.msgStore != nil && r.msgStore.GetSessionID() == "" {
// 			r.msgStore.PushSessionID(event.Info.SessionID)
// 			r.emitter.EmitTaskMessage(&models.TaskMessage{
// 				Type:      "session_id",
// 				Role:      "system",
// 				Content:   event.Info.SessionID,
// 				Timestamp: time.Now().UnixMilli(),
// 			})
// 		}
// 	}

// 	// 消息开始：Finish 字段为空字符串
// 	if event.Info.Finish == "" {
// 		log.Printf("[onMessageEvent] Message started: %s", event.Info.ID)
// 		r.currentMessageID = event.Info.ID
// 		r.currentMessageStarted = true
// 		return nil
// 	}

// 	// 消息结束：Finish 字段为 "stop"
// 	if event.Info.Finish == "stop" {
// 		log.Printf("[onMessageEvent] Message finished: %s", event.Info.ID)
// 		r.currentMessageStarted = false
// 		r.currentMessageID = ""
// 		r.handleFinish()
// 		return nil
// 	}

// 	// 其他 Finish 状态（如 error、length 等）
// 	log.Printf("[onMessageEvent] Message finished with reason: %s, messageID: %s", event.Info.Finish, event.Info.ID)
// 	r.currentMessageStarted = false
// 	r.currentMessageID = ""

// 	return nil
// }
