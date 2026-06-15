package models

// MessagePromptSnapshot 记录某条 assistant message 最近一次实际发给模型的 prompt 与原始响应。
type MessagePromptSnapshot struct {
	MessageID   string `gorm:"primaryKey;size:255"`
	SessionID   string `gorm:"size:255;index;not null"`
	Prompt      string `gorm:"type:text"`
	RawResponse string `gorm:"type:text"`
	Created     int64  `gorm:"index"`
	Updated     int64  `gorm:"index"`
}

func (MessagePromptSnapshot) TableName() string {
	return "message_prompt_snapshots"
}
