package models

import (
	"strings"
	"time"
)

const (
	WorkerAnalyst      = "analyst"
	WorkerCompaction   = "compaction"
	WorkerGeneral      = "general"
	WorkerSummary      = "summary"
	WorkerBuild        = "build"
	WorkerExplore      = "explore"
	WorkerPlan         = "plan"
	WorkerTitle        = "title"
	WorkerVerification = "verification"
)

// Worker Worker 配置
type Worker struct {
	ID                uint   `json:"id" gorm:"primaryKey"`
	Name              string `json:"name" gorm:"not null"`
	Provider          string `json:"provider" gorm:"not null"`
	Model             string `json:"model" gorm:"not null"`
	ModelSettingsName string `json:"modelSettingsName" gorm:"default:'default_model_config';index"`

	Description  string  `json:"description" gorm:"default:''"`
	Temperature  *float64 `json:"temperature"`
	TopP         float64 `json:"topP" gorm:"default:1.0"`
	SystemPrompt string  `json:"systemPrompt" gorm:"type:text"`
	Mode         string  `json:"mode" gorm:"default:''"`
	Native       bool    `json:"native" gorm:"default:false"`
	Hidden       bool    `json:"hidden" gorm:"default:false"`
	Color        string  `json:"color" gorm:"default:''"`
	Steps        int     `json:"steps" gorm:"default:0"`
	EnabledTools string  `json:"enabledTools" gorm:"type:text"`
	EnabledSkills string `json:"enabledSkills" gorm:"type:text"`
	Options      string  `json:"options" gorm:"type:text"`
	// ExtConfig JSON 扩展（如记忆策略、workersv2 额外开关）；由应用层解析。
	ExtConfig string `json:"extConfig" gorm:"type:text"`

	ContextLimit int       `json:"contextLimit" gorm:"default:0"`
	OutputLimit  int       `json:"outputLimit" gorm:"default:0"`
	Occupation   string    `json:"occupation" gorm:"default:''"`
	LLMConfigID  *uint     `json:"llmConfigId"`
	WorkingDir   string    `json:"workingDir" gorm:"default:''"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type ModelSettings struct {
	Name         string `json:"name" gorm:"primaryKey"`
	ContextLimit int    `json:"contextLimit" gorm:"default:0"`
	OutputLimit  int    `json:"outputLimit" gorm:"default:0"`
	// BudgetTokens 为 Claude/Anthropic 原生 thinking.budget_tokens；nil 表示不发送该字段。
	BudgetTokens     *int     `json:"budgetTokens"`
	TopP             *float64 `json:"topP"`
	TopK             *int     `json:"topK"`
	FrequencyPenalty *float64 `json:"frequencyPenalty"`
	// EnableThinking 非 nil 时，原生 OpenAI 请求 JSON 顶层增加 enable_thinking（与 thinking.type 独立）。
	EnableThinking        *bool   `json:"enableThinking"`
	ReasoningEffort       *string `json:"reasoningEffort"`
	TextVerbosity         *string `json:"textVerbosity"`
	EnableEncryptedReason *bool   `json:"enableEncryptedReasoning"`
	ParallelToolCalls     *bool   `json:"parallelToolCalls"`
	EnablePromptCacheKey  *bool   `json:"enablePromptCacheKey"`
	// EnableSilentToolWatchdog 为 true 时，连续无思考/文本输出的工具调用达到阈值后注入阶段性总结补充提示。
	EnableSilentToolWatchdog *bool `json:"enableSilentToolWatchdog"`
	Prompt                string  `json:"prompt" gorm:"type:text"`
	SystemPromptPlacement string  `json:"systemPromptPlacement" gorm:"default:'system'"`
	NativeOpenAIToolCalls bool    `json:"nativeOpenAIToolCalls" gorm:"default:false"`
	// ThinkingType 非空时原生 OpenAI 请求 body 增加 thinking.type（enabled/disabled）。
	ThinkingType string `json:"thinkingType"`
}

// EffectiveThinkingType 返回用于原生 OpenAI 请求的 thinking.type（仅来自 ModelSettings.ThinkingType）。
// EnableThinking 见请求体 enable_thinking 字段，二者语义独立。
func EffectiveThinkingType(ms *ModelSettings) string {
	if ms == nil {
		return ""
	}
	return NormalizeLLMThinkingType(ms.ThinkingType)
}

// OpenAIReasoningEffortValue 返回写入 OpenAI 兼容请求的 reasoning.effort（或 Responses reasoning）的字符串；无效或未设置返回 ""。
// 与 EnableThinking / ThinkingType 无关，单独由 ModelSettings.reasoningEffort 决定。
func OpenAIReasoningEffortValue(ms *ModelSettings) string {
	if ms == nil || ms.ReasoningEffort == nil {
		return ""
	}
	v := strings.TrimSpace(*ms.ReasoningEffort)
	switch v {
	case "low", "medium", "high", "xhigh", "none", "max":
		return v
	default:
		return ""
	}
}

// OpenAITextVerbosityValue 返回 Responses API text.verbosity 等使用的字符串；无效或未设置返回 ""。
func OpenAITextVerbosityValue(ms *ModelSettings) string {
	if ms == nil || ms.TextVerbosity == nil {
		return ""
	}
	v := strings.TrimSpace(*ms.TextVerbosity)
	switch v {
	case "low", "medium", "high", "xhigh":
		return v
	default:
		return ""
	}
}

// EncryptedReasoningEnabled 解析 ModelSettings.enableEncryptedReasoning。
func EncryptedReasoningEnabled(ms *ModelSettings) bool {
	return ms != nil && ms.EnableEncryptedReason != nil && *ms.EnableEncryptedReason
}

// ParallelToolCallsEnabled 解析 ModelSettings.parallelToolCalls（nil 视为 false）。
func ParallelToolCallsEnabled(ms *ModelSettings) bool {
	return ms != nil && ms.ParallelToolCalls != nil && *ms.ParallelToolCalls
}

// SilentToolWatchdogEnabled 解析 ModelSettings.enableSilentToolWatchdog（nil 视为 false）。
func SilentToolWatchdogEnabled(ms *ModelSettings) bool {
	return ms != nil && ms.EnableSilentToolWatchdog != nil && *ms.EnableSilentToolWatchdog
}

// WorkerCreate 创建 Worker
type WorkerCreate struct {
	Name              string  `json:"name" binding:"required"`
	Provider          string  `json:"provider" binding:"required"`
	Model             string  `json:"model" binding:"required"`
	ModelSettingsName string  `json:"modelSettingsName"`
	Description       string  `json:"description"`
	Temperature       *float64 `json:"temperature"`
	SystemPrompt      string  `json:"systemPrompt"`
	Occupation        string  `json:"occupation"`
	EnabledTools      string  `json:"enabledTools"`
	EnabledSkills     string  `json:"enabledSkills"`
	LLMConfigID       *uint   `json:"llmConfigId"`
	WorkingDir        string  `json:"workingDir"`
}

// WorkerUpdate 更新 Worker
type WorkerUpdate struct {
	Name              *string  `json:"name"`
	Provider          *string  `json:"provider"`
	Model             *string  `json:"model"`
	ModelSettingsName *string  `json:"modelSettingsName"`
	Description       *string  `json:"description"`
	Temperature       *float64 `json:"temperature"`
	SystemPrompt      *string  `json:"systemPrompt"`
	Occupation        *string  `json:"occupation"`
	EnabledTools      *string  `json:"enabledTools"`
	EnabledSkills     *string  `json:"enabledSkills"`
	LLMConfigID       *uint    `json:"llmConfigId"`
	WorkingDir        *string  `json:"workingDir"`
}

// WorkerResponse Worker 响应
type WorkerResponse struct {
	ID                uint       `json:"id"`
	Name              string     `json:"name"`
	Provider          string     `json:"provider"`
	Model             string     `json:"model"`
	ModelSettingsName string     `json:"modelSettingsName"`
	Description       string     `json:"description"`
	Temperature       *float64   `json:"temperature"`
	SystemPrompt      string     `json:"systemPrompt"`
	Occupation        string     `json:"occupation"`
	EnabledTools      string     `json:"enabledTools"`
	EnabledSkills     string     `json:"enabledSkills"`
	LLMConfigID       *uint      `json:"llmConfigId"`
	WorkingDir        string     `json:"workingDir"`
	CreatedAt         time.Time  `json:"createdAt"`
	UpdatedAt         time.Time  `json:"updatedAt"`
}
