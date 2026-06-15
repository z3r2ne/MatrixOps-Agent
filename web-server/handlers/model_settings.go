package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	database "pkgs/db"
	"pkgs/db/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type ModelSettingsHandler struct {
	db *gorm.DB
}

func NewModelSettingsHandler(db *gorm.DB) *ModelSettingsHandler {
	return &ModelSettingsHandler{db: db}
}

// ModelSettingsCreate 创建模型设置的请求结构
type ModelSettingsCreate struct {
	Name                  string   `json:"name" binding:"required"`
	ContextLimit          int      `json:"contextLimit"`
	OutputLimit           int      `json:"outputLimit"`
	BudgetTokens          *int     `json:"budgetTokens"`
	TopP                  *float64 `json:"topP"`
	TopK                  *int     `json:"topK"`
	FrequencyPenalty      *float64 `json:"frequencyPenalty"`
	EnableThinking        *bool    `json:"enableThinking"`
	ReasoningEffort       *string  `json:"reasoningEffort"`
	TextVerbosity         *string  `json:"textVerbosity"`
	EnableEncryptedReason *bool    `json:"enableEncryptedReasoning"`
	ParallelToolCalls          *bool `json:"parallelToolCalls"`
	EnablePromptCacheKey       *bool `json:"enablePromptCacheKey"`
	EnableSilentToolWatchdog   *bool `json:"enableSilentToolWatchdog"`
	Prompt                string   `json:"prompt"`
	SystemPromptPlacement string   `json:"systemPromptPlacement"`
	NativeOpenAIToolCalls bool     `json:"nativeOpenAIToolCalls"`
	ThinkingType          string   `json:"thinkingType"`
}

// ModelSettingsUpdate 更新模型设置的请求结构
type ModelSettingsUpdate struct {
	Name                  *string  `json:"name"`
	ContextLimit          *int     `json:"contextLimit"`
	OutputLimit           *int     `json:"outputLimit"`
	BudgetTokens          *int     `json:"budgetTokens"`
	TopP                  *float64 `json:"topP"`
	TopK                  *int     `json:"topK"`
	FrequencyPenalty      *float64 `json:"frequencyPenalty"`
	EnableThinking        *bool    `json:"enableThinking"`
	ReasoningEffort       *string  `json:"reasoningEffort"`
	TextVerbosity         *string  `json:"textVerbosity"`
	EnableEncryptedReason *bool    `json:"enableEncryptedReasoning"`
	ParallelToolCalls          *bool `json:"parallelToolCalls"`
	EnablePromptCacheKey       *bool `json:"enablePromptCacheKey"`
	EnableSilentToolWatchdog   *bool `json:"enableSilentToolWatchdog"`
	Prompt                *string  `json:"prompt"`
	SystemPromptPlacement *string  `json:"systemPromptPlacement"`
	NativeOpenAIToolCalls *bool    `json:"nativeOpenAIToolCalls"`
	ThinkingType          *string  `json:"thinkingType"`
}

func normalizeReasoningEffort(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	switch trimmed {
	case "low", "medium", "high", "xhigh", "none", "max":
		return &trimmed
	default:
		return nil
	}
}

func normalizeThinkingType(value string) string {
	return models.NormalizeLLMThinkingType(value)
}

func normalizeTextVerbosity(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	switch trimmed {
	case "low", "medium", "high", "xhigh":
		return &trimmed
	default:
		return nil
	}
}

func hasJSONKey(fields map[string]json.RawMessage, key string) bool {
	if len(fields) == 0 {
		return false
	}
	_, ok := fields[key]
	return ok
}

func normalizeSystemPromptPlacement(value string) string {
	switch value {
	case "instruction", "user_input":
		return value
	default:
		return "system"
	}
}

// GetModelSettings 获取所有模型设置
func (h *ModelSettingsHandler) GetModelSettings(c *gin.Context) {
	settings, err := database.GetAllModelSettings(h.db)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取模型设置失败", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, settings)
}

// GetModelSetting 根据名称获取单个模型设置
func (h *ModelSettingsHandler) GetModelSetting(c *gin.Context) {
	name := c.Param("name")

	setting, err := database.GetModelSettingsByName(h.db, name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "模型设置未找到"})
		return
	}

	c.JSON(http.StatusOK, setting)
}

// CreateModelSetting 创建新的模型设置
func (h *ModelSettingsHandler) CreateModelSetting(c *gin.Context) {
	var req ModelSettingsCreate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求数据", "details": err.Error()})
		return
	}

	// 检查是否已存在相同名称的配置
	if existing, err := database.GetModelSettingsByName(h.db, req.Name); err == nil && existing != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "该模型名称已存在"})
		return
	}

	setting := models.ModelSettings{
		Name:                  req.Name,
		ContextLimit:          req.ContextLimit,
		OutputLimit:           req.OutputLimit,
		BudgetTokens:          req.BudgetTokens,
		TopP:                  req.TopP,
		TopK:                  req.TopK,
		FrequencyPenalty:      req.FrequencyPenalty,
		EnableThinking:        req.EnableThinking,
		ReasoningEffort:       normalizeReasoningEffort(req.ReasoningEffort),
		TextVerbosity:         normalizeTextVerbosity(req.TextVerbosity),
		EnableEncryptedReason: req.EnableEncryptedReason,
		ParallelToolCalls:        req.ParallelToolCalls,
		EnablePromptCacheKey:     req.EnablePromptCacheKey,
		EnableSilentToolWatchdog: req.EnableSilentToolWatchdog,
		Prompt:                   req.Prompt,
		SystemPromptPlacement: normalizeSystemPromptPlacement(req.SystemPromptPlacement),
		NativeOpenAIToolCalls: req.NativeOpenAIToolCalls,
		ThinkingType:          normalizeThinkingType(req.ThinkingType),
	}

	if err := database.CreateModelSettings(h.db, &setting); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建模型设置失败", "details": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, setting)
}

// UpdateModelSetting 更新模型设置
func (h *ModelSettingsHandler) UpdateModelSetting(c *gin.Context) {
	name := c.Param("name")

	setting, err := database.GetModelSettingsByName(h.db, name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "模型设置未找到"})
		return
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "读取请求数据失败", "details": err.Error()})
		return
	}

	var req ModelSettingsUpdate
	if err := json.Unmarshal(body, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求数据", "details": err.Error()})
		return
	}
	var rawFields map[string]json.RawMessage
	if err := json.Unmarshal(body, &rawFields); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求数据", "details": err.Error()})
		return
	}

	nextName := setting.Name
	if req.Name != nil && strings.TrimSpace(*req.Name) != "" {
		nextName = strings.TrimSpace(*req.Name)
	}
	if name == database.DefaultModelSettingsName && nextName != name {
		c.JSON(http.StatusBadRequest, gin.H{"error": "默认模型配置不能改名"})
		return
	}

	if hasJSONKey(rawFields, "contextLimit") {
		if req.ContextLimit != nil {
			setting.ContextLimit = *req.ContextLimit
		} else {
			setting.ContextLimit = 0
		}
	}
	if hasJSONKey(rawFields, "outputLimit") {
		if req.OutputLimit != nil {
			setting.OutputLimit = *req.OutputLimit
		} else {
			setting.OutputLimit = 0
		}
	}
	if hasJSONKey(rawFields, "budgetTokens") {
		setting.BudgetTokens = req.BudgetTokens
	}
	if req.Prompt != nil {
		setting.Prompt = *req.Prompt
	}
	if hasJSONKey(rawFields, "topP") {
		setting.TopP = req.TopP
	}
	if hasJSONKey(rawFields, "topK") {
		setting.TopK = req.TopK
	}
	if hasJSONKey(rawFields, "frequencyPenalty") {
		setting.FrequencyPenalty = req.FrequencyPenalty
	}
	if hasJSONKey(rawFields, "enableThinking") {
		setting.EnableThinking = req.EnableThinking
	}
	if hasJSONKey(rawFields, "reasoningEffort") {
		setting.ReasoningEffort = normalizeReasoningEffort(req.ReasoningEffort)
	}
	if hasJSONKey(rawFields, "textVerbosity") {
		setting.TextVerbosity = normalizeTextVerbosity(req.TextVerbosity)
	}
	if hasJSONKey(rawFields, "enableEncryptedReasoning") {
		setting.EnableEncryptedReason = req.EnableEncryptedReason
	}
	if hasJSONKey(rawFields, "parallelToolCalls") {
		setting.ParallelToolCalls = req.ParallelToolCalls
	}
	if hasJSONKey(rawFields, "enablePromptCacheKey") {
		setting.EnablePromptCacheKey = req.EnablePromptCacheKey
	}
	if hasJSONKey(rawFields, "enableSilentToolWatchdog") {
		setting.EnableSilentToolWatchdog = req.EnableSilentToolWatchdog
	}
	if req.SystemPromptPlacement != nil {
		setting.SystemPromptPlacement = normalizeSystemPromptPlacement(*req.SystemPromptPlacement)
	}
	if req.NativeOpenAIToolCalls != nil {
		setting.NativeOpenAIToolCalls = *req.NativeOpenAIToolCalls
	}
	if hasJSONKey(rawFields, "thinkingType") {
		if req.ThinkingType != nil {
			setting.ThinkingType = normalizeThinkingType(*req.ThinkingType)
		} else {
			setting.ThinkingType = ""
		}
	}
	setting.Name = nextName

	if err := database.UpdateModelSettingsWithRename(h.db, name, setting); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新模型设置失败", "details": err.Error()})
		return
	}

	updated, getErr := database.GetModelSettingsByName(h.db, nextName)
	if getErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取更新后的模型设置失败", "details": getErr.Error()})
		return
	}
	c.JSON(http.StatusOK, updated)
}

// DeleteModelSetting 删除模型设置
func (h *ModelSettingsHandler) DeleteModelSetting(c *gin.Context) {
	name := c.Param("name")

	// 检查是否存在
	if _, err := database.GetModelSettingsByName(h.db, name); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "模型设置未找到"})
		return
	}
	if name == database.DefaultModelSettingsName {
		c.JSON(http.StatusBadRequest, gin.H{"error": "默认模型配置不能删除"})
		return
	}

	if err := h.db.Model(&models.Worker{}).
		Where("model_settings_name = ?", name).
		Update("model_settings_name", database.DefaultModelSettingsName).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "迁移 Worker 模型配置失败", "details": err.Error()})
		return
	}

	if err := database.DeleteModelSettings(h.db, name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除模型设置失败", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "模型设置已删除"})
}

// SetupModelSettingsRoutes 设置模型设置相关的路由
func SetupModelSettingsRoutes(router *gin.RouterGroup, db *gorm.DB) {
	modelSettingsHandler := NewModelSettingsHandler(db)
	router.GET("/model-settings", modelSettingsHandler.GetModelSettings)
	router.GET("/model-settings/:name", modelSettingsHandler.GetModelSetting)
	router.POST("/model-settings", modelSettingsHandler.CreateModelSetting)
	router.PUT("/model-settings/:name", modelSettingsHandler.UpdateModelSetting)
	router.DELETE("/model-settings/:name", modelSettingsHandler.DeleteModelSetting)
}
