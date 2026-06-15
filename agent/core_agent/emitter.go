package coreagent

// EventAssistantFooterStatus 与 session.EventAssistantFooterStatus 字符串值须一致
const EventAssistantFooterStatus = "message.footer_status"

// AssistantFooterStatusPayload 助手消息底部一行状态（实时展示）
type AssistantFooterStatusPayload struct {
	MessageID string `json:"messageID"`
	Text      string `json:"text"`
	Loading   bool   `json:"loading"`
}

// Emitter receives streaming execution updates.
type Emitter interface {
	UpdateMessage(info *Message) (*Message, error)
	UpdatePart(part *Part) (*Part, error)
	Emit(name string, payload interface{})
}
