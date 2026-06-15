package models

import "time"

const (
	SearchConfigTypeKimiSearchAPI = "kimi_search_api"

	DefaultSearchConfigBaseURL = "https://agent-gw.kimi.com/coding"
)

// SearchConfig 搜索插件配置
type SearchConfig struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	Name      string    `json:"name" gorm:"not null"`
	Type      string    `json:"type" gorm:"not null"`
	APIKey    string    `json:"apiKey" gorm:"not null"`
	BaseURL   string    `json:"baseUrl"`
	Enabled   bool      `json:"enabled" gorm:"not null;default:false"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type SearchConfigCreate struct {
	Name    string `json:"name" binding:"required"`
	Type    string `json:"type" binding:"required"`
	APIKey  string `json:"apiKey" binding:"required"`
	BaseURL string `json:"baseUrl"`
	Enabled *bool  `json:"enabled"`
}

type SearchConfigUpdate struct {
	Name    *string `json:"name"`
	Type    *string `json:"type"`
	APIKey  *string `json:"apiKey"`
	BaseURL *string `json:"baseUrl"`
	Enabled *bool   `json:"enabled"`
}

func NormalizeSearchConfigType(value string) string {
	switch value {
	case SearchConfigTypeKimiSearchAPI:
		return SearchConfigTypeKimiSearchAPI
	default:
		return SearchConfigTypeKimiSearchAPI
	}
}
