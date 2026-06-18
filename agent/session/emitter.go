package session

import (
	"errors"
	"log"
	"maps"
	"time"

	"matrixops-agent/types"
	"pkgs/db/storage"
	"pkgs/emitter"

	"gorm.io/gorm"
)

// 事件常量
const (
	EventSessionCreated      = "session.created"
	EventSessionUpdated      = "session.updated"
	EventSessionDeleted      = "session.deleted"
	EventMessageUpdated      = "message.updated"
	EventMessageRemoved      = "message.removed"
	EventPartUpdated         = "message.part.updated"
	EventPartRemoved         = "message.part.removed"
	EventError               = "error"
	EventSessionError        = "session.error"
	EventSessionTitleUpdated = "session.title.updated"
	EventPluginVarSet        = "plugin.var.set"
	EventWaitUserInput       = "wait.user.input"
	EventPlanUpdated         = "plan.updated"
	// EventAssistantFooterStatus 助手消息底部状态行（不落库，经 task_runner 直推 WS）
	EventAssistantFooterStatus = "message.footer_status"
)

// AssistantFooterStatusEvent 与 core_agent.AssistantFooterStatusPayload 字段一致
type AssistantFooterStatusEvent struct {
	MessageID string `json:"messageID"`
	Text      string `json:"text"`
	Loading   bool   `json:"loading"`
}

type Emitter struct {
	SessionID string
	db        *gorm.DB
	// Session   *types.Info
	*emitter.EventEmitter
}

// NewEmitter 创建新的 Emitter 实例
func NewEmitter(db *gorm.DB, sessionID string) *Emitter {
	e := &Emitter{
		SessionID: sessionID,
		db:        db,
		// Session:      sessionInfo,
		EventEmitter: emitter.New(),
	}
	initEmitter(e)
	return e
}

func (e *Emitter) UpdateMessage(info *types.MessageInfo) (*types.MessageInfo, error) {
	if info.ID == "" {
		return nil, errors.New("message id required")
	}
	if info.SessionID == "" {
		return nil, errors.New("session id required")
	}
	if e.db != nil {
		storage.UpdateMessageInfo(e.db, info)
	}

	e.Emit(EventMessageUpdated, MessageEvent{Info: info})
	return info, nil
}

func (e *Emitter) WaitUserInput(questions map[string]interface{}) (map[string]any, error) {
	data := maps.Clone(questions)
	e.Emit(EventWaitUserInput, WaitUserInputEvent{Questions: data})
	if result, ok := data["result"]; ok {
		return result.(map[string]any), nil
	}
	return nil, errors.New("wait user input failed")
}

// UpdatePart 更新部件
func (e *Emitter) UpdatePart(part *types.Part) (*types.Part, error) {
	return e.UpdatePartDelta(part, "")
}

func (e *Emitter) SetPluginVar(key string, value any) error {
	e.Emit(EventPluginVarSet, PluginVarSetEvent{Key: key, Value: value})
	return nil
}

// UpdatePartDelta 更新部件（带增量）
func (e *Emitter) UpdatePartDelta(part *types.Part, delta string) (*types.Part, error) {
	if part.ID == "" || part.MessageID == "" {
		return nil, errors.New("part id and message id required")
	}
	if part.Time == nil {
		part.Time = &types.PartTime{}
	}
	if part.Time.Created == 0 {
		if part.Time.Start != 0 {
			part.Time.Created = part.Time.Start
		} else if part.Time.End != 0 {
			part.Time.Created = part.Time.End
		} else {
			part.Time.Created = time.Now().UnixMilli()
		}
	}
	if part.Time.Start == 0 {
		part.Time.Start = part.Time.Created
	}

	e.Emit(EventPartUpdated, PartEvent{Part: part, Delta: delta})
	return part, nil
}

// Diff 获取会话差异
// func (e *Emitter) Diff(sessionID string) ([]snapshot.FileDiff, error) {
// 	diffs, err := storage.Read[[]snapshot.FileDiff]([]string{"session_diff", sessionID})
// 	if err != nil {
// 		return []snapshot.FileDiff{}, nil
// 	}
// 	return diffs, nil
// }

// RemoveMessage 删除消息
func (e *Emitter) RemoveMessage(sessionID string, messageID string) error {
	if sessionID == "" || messageID == "" {
		return errors.New("message id required")
	}

	e.Emit(EventMessageRemoved, MessageRemovedEvent{
		SessionID: sessionID,
		MessageID: messageID,
	})

	return nil
}

func (e *Emitter) UpdateSessionTitle(sessionID string, title string) error {
	e.Emit(EventSessionTitleUpdated, title)
	return nil
}

// RemovePart 删除部件
func (e *Emitter) RemovePart(sessionID string, messageID string, partID string) error {
	if sessionID == "" || messageID == "" || partID == "" {
		return errors.New("session, message, and part id required")
	}

	e.Emit(EventPartRemoved, PartRemovedEvent{
		SessionID: sessionID,
		MessageID: messageID,
		PartID:    partID,
	})

	return nil
}

// initEmitter 初始化事件监听器（将事件持久化到存储）
func initEmitter(e *Emitter) {
	em := e.EventEmitter
	// 错误处理
	em.On(EventError, func(args ...interface{}) {
		err := args[0].(error)
		log.Printf("[Emitter] error: %v", err)
	})
	em.On(EventSessionError, func(args ...interface{}) {
		event := args[0].(SessionErrorEvent)
		log.Printf("[Emitter] session error: %v", event.Error)
	})

	if e.db == nil {
		return
	}

	// 会话创建
	em.On(EventSessionCreated, func(args ...interface{}) {
		event := args[0].(SessionEvent)
		if event.Info == nil {
			return
		}
		if err := storage.UpdateSession(e.db, event.Info); err != nil {
		}
	})

	// 会话更新（Update 方法中已经调用 storage，这里不需要再次调用）
	em.On(EventSessionUpdated, func(args ...interface{}) {
		if len(args) == 0 {
			return
		}
		event, ok := args[0].(SessionEvent)
		if !ok || event.Info == nil {
			return
		}
		if err := storage.UpdateSession(e.db, event.Info); err != nil {
			log.Printf("[Emitter] session.updated persist failed: %v", err)
		}
	})

	// 会话删除
	em.On(EventSessionDeleted, func(args ...interface{}) {
		event := args[0].(SessionEvent)
		if err := storage.DeleteSession(e.db, event.Info.ID); err != nil {
		}
		if err := storage.DeleteMessageBySession(e.db, event.Info.ID); err != nil {
		}
		if err := storage.DeletePromptSnapshotsBySessionID(e.db, event.Info.ID); err != nil {
		}
		if err := storage.DeleteMemoryEntriesBySession(e.db, event.Info.ID); err != nil {
		}
	})

	// 消息更新
	em.On(EventMessageUpdated, func(args ...interface{}) {
		event := args[0].(MessageEvent)
		if err := storage.UpdateMessageInfo(e.db, event.Info); err != nil {
		}
	})

	// 消息删除
	em.On(EventMessageRemoved, func(args ...interface{}) {
		event := args[0].(MessageRemovedEvent)
		if err := storage.DeleteMessage(e.db, &types.MessageInfo{
			ID:        event.MessageID,
			SessionID: event.SessionID,
		}); err != nil {
		}

		// 删除相关部件
		if err := storage.DeleteParts(e.db, event.MessageID); err != nil {
		}
		if err := storage.DeletePromptSnapshotByMessageID(e.db, event.MessageID); err != nil {
		}
		if err := storage.DeleteMemoryEntriesByMessage(e.db, event.SessionID, event.MessageID); err != nil {
		}
	})

	// 部件更新
	em.On(EventPartUpdated, func(args ...interface{}) {
		event := args[0].(PartEvent)
		var messageInfo *types.MessageInfo
		if err := e.db.Transaction(func(tx *gorm.DB) error {
			if _, err := storage.UpdatePart(tx, event.Part); err != nil {
				return err
			}
			info, err := storage.GetMessage(tx, event.Part.MessageID)
			if err != nil {
				return err
			}
			messageInfo = info
			return syncPartMemory(tx, info, event.Part)
		}); err != nil {
			return
		}
		if messageInfo != nil {
			em.Emit(EventMessageUpdated, MessageEvent{Info: messageInfo})
		}
	})

	// 部件删除
	em.On(EventPartRemoved, func(args ...interface{}) {
		event := args[0].(PartRemovedEvent)
		var messageInfo *types.MessageInfo
		if err := e.db.Transaction(func(tx *gorm.DB) error {
			if err := storage.DeletePart(tx, &types.Part{
				ID:        event.PartID,
				MessageID: event.MessageID,
				SessionID: event.SessionID,
			}); err != nil {
				return err
			}
			if err := storage.DeleteMemoryEntriesByPart(tx, event.SessionID, event.MessageID, event.PartID); err != nil {
				return err
			}
			info, err := storage.GetMessage(tx, event.MessageID)
			if err != nil {
				return err
			}
			messageInfo = info
			return nil
		}); err != nil {
			return
		}
		if messageInfo != nil {
			em.Emit(EventMessageUpdated, MessageEvent{Info: messageInfo})
		}
	})
}
