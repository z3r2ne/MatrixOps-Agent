package models

import "time"

// ProviderSetting Provider 配置（仅保存开关）
type ProviderSetting struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	Name      string    `json:"name" gorm:"unique;not null"`
	Enabled   bool      `json:"enabled" gorm:"default:false"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// ProviderResponse Provider 展示信息
type ProviderResponse struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Type        string `json:"type"` // cli/app
	Enabled     bool   `json:"enabled"`
	Detected    bool   `json:"detected"`
	Path        string `json:"path,omitempty"`
	Status      string `json:"status"` // ready/need_install
	Message     string `json:"message,omitempty"`
}

// ProviderUpdate 更新 Provider
type ProviderUpdate struct {
	Enabled *bool `json:"enabled"`
}
