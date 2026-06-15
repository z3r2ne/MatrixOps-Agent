package models

// MessagePromptSnapshotHistory 记录 assistant message 每次实际发给模型的 prompt 与原始响应历史。
type MessagePromptSnapshotHistory struct {
	ID          uint   `gorm:"primaryKey"`
	MessageID   string `gorm:"size:255;index;not null"`
	SessionID   string `gorm:"size:255;index;not null"`
	Prompt      string `gorm:"type:text"`
	RawResponse string `gorm:"type:text"`
	Created     int64  `gorm:"index"`
}

func (MessagePromptSnapshotHistory) TableName() string {
	return "message_prompt_snapshot_histories"
}
