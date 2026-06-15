package models

import (
	"encoding/json"
	"time"
)

// ExecutionLog 执行日志记录 (对应 vibe-kanban 的 ExecutionProcessLogs)
// 每条日志独立存储，支持流式读取和持久化
type ExecutionLog struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	ExecutionID uint      `json:"executionId" gorm:"not null;index:idx_exec_seq"` // 关联 TaskExecution
	Sequence    uint      `json:"sequence" gorm:"not null;index:idx_exec_seq"`    // 序号，保证顺序
	MsgType     string    `json:"msgType" gorm:"not null"`                        // stdout/stderr/normalized_entry/session_id/finished
	Content     string    `json:"content" gorm:"type:text"`                       // 消息内容
	EntryJSON   string    `json:"entryJson,omitempty" gorm:"type:text"`           // NormalizedEntry JSON (仅 normalized_entry 类型)
	CreatedAt   time.Time `json:"createdAt"`
}

// LogMsgType 日志消息类型
type LogMsgType string

const (
	LogMsgTypeStdout          LogMsgType = "stdout"
	LogMsgTypeStderr          LogMsgType = "stderr"
	LogMsgTypeNormalizedEntry LogMsgType = "normalized_entry"
	LogMsgTypeSessionID       LogMsgType = "session_id"
	LogMsgTypeFinished        LogMsgType = "finished"
)

// LogMsg 日志消息 (用于内存广播和 WebSocket 传输)
type LogMsg struct {
	Type      LogMsgType       `json:"type"`
	Content   string           `json:"content,omitempty"`
	Entry     *NormalizedEntry `json:"entry,omitempty"`
	SessionID string           `json:"sessionId,omitempty"`
	Sequence  uint             `json:"sequence"`
}

// ToJSON 转换为 JSON 字节
func (m *LogMsg) ToJSON() ([]byte, error) {
	return json.Marshal(m)
}

// ExecutionLogFromLogMsg 从 LogMsg 创建 ExecutionLog
func ExecutionLogFromLogMsg(executionID uint, msg *LogMsg) *ExecutionLog {
	log := &ExecutionLog{
		ExecutionID: executionID,
		Sequence:    msg.Sequence,
		MsgType:     string(msg.Type),
		Content:     msg.Content,
	}

	if msg.Entry != nil {
		if entryJSON, err := json.Marshal(msg.Entry); err == nil {
			log.EntryJSON = string(entryJSON)
		}
	}

	if msg.SessionID != "" {
		log.Content = msg.SessionID
	}

	return log
}

// ToLogMsg 转换回 LogMsg
func (l *ExecutionLog) ToLogMsg() *LogMsg {
	msg := &LogMsg{
		Type:     LogMsgType(l.MsgType),
		Content:  l.Content,
		Sequence: l.Sequence,
	}

	if l.MsgType == string(LogMsgTypeNormalizedEntry) && l.EntryJSON != "" {
		var entry NormalizedEntry
		if err := json.Unmarshal([]byte(l.EntryJSON), &entry); err == nil {
			msg.Entry = &entry
		}
	}

	if l.MsgType == string(LogMsgTypeSessionID) {
		msg.SessionID = l.Content
	}

	return msg
}
