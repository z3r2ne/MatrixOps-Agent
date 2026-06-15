package models

import "time"

// WechatAccount 存储微信 iLink 机器人账号信息
type WechatAccount struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	BotID       string    `json:"botId" gorm:"not null;uniqueIndex"`
	BotToken    string    `json:"botToken" gorm:"not null"`
	BaseURL     string    `json:"baseURL"`
	ILinkUserID string    `json:"ilinkUserId"`
	Status      string    `json:"status" gorm:"default:'offline'"` // online | offline | error
	Enabled     bool      `json:"enabled" gorm:"default:true"`
	BoundTaskID *uint     `json:"boundTaskId" gorm:"index"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// WechatAccountUpdate 更新请求
type WechatAccountUpdate struct {
	Enabled     *bool   `json:"enabled"`
	BoundTaskID *uint   `json:"boundTaskId"`
}
