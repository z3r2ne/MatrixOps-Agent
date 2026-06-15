package models

// MessageCodeSnapshot 会话内代码检查点（与消息关联，不将 hash存在 parts 表）
type MessageCodeSnapshot struct {
	ID          string `gorm:"primaryKey;size:255"`
	SessionID   string `gorm:"size:255;index;not null"`
	MessageID   string `gorm:"size:255;index;not null"`
	PartID      string `gorm:"size:255;index;not null"`
	StartHash   string `gorm:"size:255;not null"`
	EndHash     string `gorm:"size:255;not null"`
	Description string `gorm:"type:text"`
	Files       JSONField `gorm:"type:text"`
	Created     int64 `gorm:"index"`
}

func (MessageCodeSnapshot) TableName() string {
	return "message_code_snapshots"
}
