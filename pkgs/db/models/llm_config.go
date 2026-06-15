package models

import (
	"strings"
	"time"
)

const (
	LLMAPITypeResponse = "response"
	LLMAPITypeChat     = "chat"
)

func NormalizeLLMAPIType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case LLMAPITypeChat:
		return LLMAPITypeChat
	default:
		return LLMAPITypeResponse
	}
}

func NormalizeLLMSystemPromptPlacement(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "system":
		return "system"
	case "instruction":
		return "instruction"
	case "user_input":
		return "user_input"
	default:
		return "instruction"
	}
}

const (
	LLMThinkingTypeEnabled  = "enabled"
	LLMThinkingTypeDisabled = "disabled"
)

// NormalizeLLMThinkingType 返回 enabled / disabled 或空字符串表示未设置。
func NormalizeLLMThinkingType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case LLMThinkingTypeEnabled:
		return LLMThinkingTypeEnabled
	case LLMThinkingTypeDisabled:
		return LLMThinkingTypeDisabled
	default:
		return ""
	}
}

// LLMConfig 大模型配置
type LLMConfig struct {
	ID      uint   `json:"id" gorm:"primaryKey"`
	Name    string `json:"name" gorm:"not null"`   // 配置名称
	Type    string `json:"type" gorm:"not null"`   // 类型：openai, claude, custom
	APIKey  string `json:"apiKey" gorm:"not null"` // API Key
	Model   string `json:"model" gorm:"not null"`  // 模型名称
	BaseURL string `json:"baseUrl"`                // 自定义 BaseURL（可选）
	// APIType 决定 OpenAI 兼容接口使用 /chat/completions 还是 /responses。
	APIType string `json:"apiType" gorm:"default:response"`
	// SystemPromptPlacement 控制系统提示词如何进入模型请求。
	SystemPromptPlacement string `json:"systemPromptPlacement" gorm:"default:'instruction'"`
	// Proxy 可选 HTTP(S) 代理完整 URL（须含 scheme，如 http://127.0.0.1:7890），用于访问该 provider 的 API。
	Proxy      string `json:"proxy"`
	MaxRetries int    `json:"maxRetries" gorm:"default:5"`
	// NativeOpenAIToolCalls 为 true 时，Agent 使用 OpenAI 兼容 Chat Completions 的原生 tools/tool_calls（openai-go），
	// 而非提示词内嵌 JSON 动作流。
	NativeOpenAIToolCalls bool      `json:"nativeOpenAIToolCalls" gorm:"default:false"`
	CreatedAt             time.Time `json:"createdAt"`
	UpdatedAt             time.Time `json:"updatedAt"`
}

// LLMConfigCreate 创建大模型配置
type LLMConfigCreate struct {
	Name                  string `json:"name" binding:"required"`
	Type                  string `json:"type" binding:"required"` // openai, claude, custom
	APIKey                string `json:"apiKey" binding:"required"`
	Model                 string `json:"model" binding:"required"`
	BaseURL               string `json:"baseUrl"`
	APIType               string `json:"apiType"`
	SystemPromptPlacement string `json:"systemPromptPlacement"`
	Proxy                 string `json:"proxy"`
	MaxRetries            int    `json:"maxRetries"`
	// NativeOpenAIToolCalls 见 LLMConfig；省略则为 false。
	NativeOpenAIToolCalls bool `json:"nativeOpenAIToolCalls"`
}

// LLMConfigUpdate 更新大模型配置
type LLMConfigUpdate struct {
	Name                  *string `json:"name"`
	Type                  *string `json:"type"`
	APIKey                *string `json:"apiKey"`
	Model                 *string `json:"model"`
	BaseURL               *string `json:"baseUrl"`
	APIType               *string `json:"apiType"`
	SystemPromptPlacement *string `json:"systemPromptPlacement"`
	Proxy                 *string `json:"proxy"`
	MaxRetries            *int    `json:"maxRetries"`
	// NativeOpenAIToolCalls 为 nil 时不修改该字段。
	NativeOpenAIToolCalls *bool `json:"nativeOpenAIToolCalls"`
}

// GenerateCommitMessageRequest 生成提交消息请求
type GenerateCommitMessageRequest struct {
	Diff     string `json:"diff" binding:"required"` // diff 内容
	ConfigID *uint  `json:"configId"`                // 可选：指定使用的配置 ID
}

// GenerateCommitMessageResponse 生成提交消息响应
type GenerateCommitMessageResponse struct {
	Message string `json:"message"`
}

// DebugLLMRequest 调试调用请求
type DebugLLMRequest struct {
	Input       string   `json:"input" binding:"required"`
	ConfigID    *uint    `json:"configId"`
	Model       string   `json:"model"`
	Temperature *float64 `json:"temperature"`
	MaxTokens   *int     `json:"maxTokens"`
}

// DebugLLMResponse 调试调用响应
type DebugLLMResponse struct {
	Text string `json:"text"`
}
