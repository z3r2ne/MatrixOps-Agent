package models

import "time"

// UIState 保存桌面端 UI 的轻量状态。
type UIState struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	Key       string    `json:"key" gorm:"size:64;uniqueIndex;not null"`
	Value     string    `json:"value" gorm:"type:text"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
