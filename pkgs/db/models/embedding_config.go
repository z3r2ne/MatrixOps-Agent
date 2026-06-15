package models

import "time"

const (
	EmbeddingConfigTypeLlamaCpp = "llama_cpp"

	DefaultEmbeddingConfigBaseURL   = "http://127.0.0.1:8081"
	DefaultEmbeddingBatchSize       = 16
	DefaultEmbeddingMaxInputTokens  = 512
)

// EmbeddingConfig 本地 embedding 配置
type EmbeddingConfig struct {
	ID             uint      `json:"id" gorm:"primaryKey"`
	Name           string    `json:"name" gorm:"not null"`
	Type           string    `json:"type" gorm:"not null"`
	BaseURL        string    `json:"baseUrl"`
	BinaryPath     string    `json:"binaryPath"`
	ModelPath      string    `json:"modelPath"`
	Dimension      int       `json:"dimension"`
	BatchSize      int       `json:"batchSize" gorm:"not null;default:16"`
	MaxInputTokens int       `json:"maxInputTokens" gorm:"not null;default:512"`
	Enabled        bool      `json:"enabled" gorm:"not null;default:false"`
	AutoStart      bool      `json:"autoStart" gorm:"not null;default:false"`
	Status         string    `json:"status"`
	LastError      string    `json:"lastError"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

type EmbeddingConfigCreate struct {
	Name           string `json:"name" binding:"required"`
	Type           string `json:"type" binding:"required"`
	BaseURL        string `json:"baseUrl"`
	BinaryPath     string `json:"binaryPath"`
	ModelPath      string `json:"modelPath"`
	Dimension      int    `json:"dimension"`
	BatchSize      int    `json:"batchSize"`
	MaxInputTokens int    `json:"maxInputTokens"`
	Enabled        *bool  `json:"enabled"`
	AutoStart      *bool  `json:"autoStart"`
}

type EmbeddingConfigUpdate struct {
	Name           *string `json:"name"`
	Type           *string `json:"type"`
	BaseURL        *string `json:"baseUrl"`
	BinaryPath     *string `json:"binaryPath"`
	ModelPath      *string `json:"modelPath"`
	Dimension      *int    `json:"dimension"`
	BatchSize      *int    `json:"batchSize"`
	MaxInputTokens *int    `json:"maxInputTokens"`
	Enabled        *bool   `json:"enabled"`
	AutoStart      *bool   `json:"autoStart"`
	Status         *string `json:"status"`
	LastError      *string `json:"lastError"`
}

func NormalizeEmbeddingConfigType(value string) string {
	switch value {
	case EmbeddingConfigTypeLlamaCpp:
		return EmbeddingConfigTypeLlamaCpp
	default:
		return EmbeddingConfigTypeLlamaCpp
	}
}
