package database

import (
	"fmt"
	"strings"

	defaultconfig "matrixops-agent/default_config"
	"pkgs/db/models"

	"gorm.io/gorm"
)

const DefaultModelSettingsName = "default_model_config"

func normalizeLLMConfig(config *models.LLMConfig) {
	if config == nil {
		return
	}
	config.APIType = models.NormalizeLLMAPIType(config.APIType)
	config.SystemPromptPlacement = models.NormalizeLLMSystemPromptPlacement(config.SystemPromptPlacement)
}

// LLMConfig 相关数据库操作

// GetAllLLMConfigs 获取所有 LLM 配置
func GetAllLLMConfigs(db *gorm.DB) ([]models.LLMConfig, error) {
	var configs []models.LLMConfig
	err := db.Order("created_at DESC").Find(&configs).Error
	for index := range configs {
		normalizeLLMConfig(&configs[index])
	}
	return configs, err
}

// GetLLMConfigByID 根据 ID 获取 LLM 配置
func GetLLMConfigByID(db *gorm.DB, id uint) (*models.LLMConfig, error) {
	var config models.LLMConfig
	err := db.First(&config, id).Error
	normalizeLLMConfig(&config)
	return &config, err
}

// CountLLMConfigsByName 根据名称统计 LLM 配置数量
func CountLLMConfigsByName(db *gorm.DB, name string) (int64, error) {
	var count int64
	err := db.Model(&models.LLMConfig{}).Where("LOWER(name) = ?", name).Count(&count).Error
	return count, err
}

// CreateLLMConfig 创建 LLM 配置
func CreateLLMConfig(db *gorm.DB, config *models.LLMConfig) error {
	normalizeLLMConfig(config)
	return db.Create(config).Error
}

// UpdateLLMConfig 更新 LLM 配置
func UpdateLLMConfig(db *gorm.DB, config *models.LLMConfig) error {
	normalizeLLMConfig(config)
	return db.Save(config).Error
}

// UpdateLLMConfigFields 更新 LLM 配置的指定字段
func UpdateLLMConfigFields(db *gorm.DB, configID uint, updates map[string]interface{}) error {
	return db.Model(&models.LLMConfig{}).Where("id = ?", configID).Updates(updates).Error
}

// DeleteLLMConfig 删除 LLM 配置
func DeleteLLMConfig(db *gorm.DB, configID uint) error {
	return db.Delete(&models.LLMConfig{}, configID).Error
}

// GetDefaultLLMConfigID 获取默认 LLM 配置 ID
func GetDefaultLLMConfigID(db *gorm.DB) (uint, error) {
	config, err := GetGlobalConfigByKey(db, models.ConfigKeyDefaultLLMConfig)
	if err != nil {
		return 0, err
	}
	var id uint
	if _, err := fmt.Sscanf(config.Value, "%d", &id); err != nil {
		return 0, err
	}
	return id, nil
}

// SetDefaultLLMConfigID 设置默认 LLM 配置 ID
func SetDefaultLLMConfigID(db *gorm.DB, configID uint) error {
	// 先验证配置是否存在
	if _, err := GetLLMConfigByID(db, configID); err != nil {
		return fmt.Errorf("LLM 配置不存在: %w", err)
	}

	return UpsertGlobalConfig(db, models.ConfigKeyDefaultLLMConfig, fmt.Sprintf("%d", configID))
}

// GetDefaultLLMConfig 获取默认 LLM 配置
func GetDefaultLLMConfig(db *gorm.DB) (*models.LLMConfig, error) {
	id, err := GetDefaultLLMConfigID(db)
	if err != nil {
		return nil, err
	}
	return GetLLMConfigByID(db, id)
}

// GetModelSettingsByName 根据名称获取模型设置
func GetModelSettingsByName(db *gorm.DB, name string) (*models.ModelSettings, error) {
	var settings models.ModelSettings
	err := db.Where("name = ?", name).First(&settings).Error
	normalizeModelSettings(&settings)
	return &settings, err
}

func GetDefaultModelSettings(db *gorm.DB) (*models.ModelSettings, error) {
	return GetModelSettingsByName(db, DefaultModelSettingsName)
}

func BuildProviderModelSettingsKey(providerName, modelName string) string {
	providerName = strings.TrimSpace(providerName)
	modelName = strings.TrimSpace(modelName)
	if providerName == "" {
		return modelName
	}
	if modelName == "" {
		return providerName
	}
	return providerName + "/" + modelName
}

func GetModelSettingsByProviderAndModel(db *gorm.DB, providerName, modelName string) (*models.ModelSettings, error) {
	key := BuildProviderModelSettingsKey(providerName, modelName)
	if key != "" {
		if settings, err := GetModelSettingsByName(db, key); err == nil && settings != nil {
			return settings, nil
		}
	}
	return GetModelSettingsByName(db, modelName)
}

func GetModelSettingsForWorker(db *gorm.DB, worker *models.Worker) (*models.ModelSettings, error) {
	if worker == nil {
		return GetDefaultModelSettings(db)
	}
	name := strings.TrimSpace(worker.ModelSettingsName)
	if name == "" {
		name = DefaultModelSettingsName
	}
	return GetModelSettingsByName(db, name)
}

// GetAllModelSettings 获取所有模型设置
func GetAllModelSettings(db *gorm.DB) ([]models.ModelSettings, error) {
	var settings []models.ModelSettings
	err := db.Find(&settings).Error
	for index := range settings {
		normalizeModelSettings(&settings[index])
	}
	return settings, err
}

// CreateModelSettings 创建模型设置
func CreateModelSettings(db *gorm.DB, settings *models.ModelSettings) error {
	return db.Create(settings).Error
}

// UpdateModelSettings 更新模型设置
func UpdateModelSettings(db *gorm.DB, settings *models.ModelSettings) error {
	return db.Save(settings).Error
}

func UpdateModelSettingsWithRename(db *gorm.DB, currentName string, settings *models.ModelSettings) error {
	if db == nil || settings == nil {
		return nil
	}
	currentName = strings.TrimSpace(currentName)
	nextName := strings.TrimSpace(settings.Name)
	if currentName == "" || nextName == "" {
		return fmt.Errorf("model settings name is required")
	}
	if currentName == nextName {
		return db.Save(settings).Error
	}

	return db.Transaction(func(tx *gorm.DB) error {
		var existing models.ModelSettings
		if err := tx.Where("name = ?", nextName).First(&existing).Error; err == nil {
			return fmt.Errorf("该模型配置名称已存在")
		} else if err != gorm.ErrRecordNotFound {
			return err
		}
		if err := tx.Create(settings).Error; err != nil {
			return err
		}
		if err := tx.Where("name = ?", currentName).Delete(&models.ModelSettings{}).Error; err != nil {
			return err
		}
		return tx.Model(&models.Worker{}).
			Where("model_settings_name = ?", currentName).
			Update("model_settings_name", nextName).Error
	})
}

// DeleteModelSettings 删除模型设置
func DeleteModelSettings(db *gorm.DB, name string) error {
	return db.Where("name = ?", name).Delete(&models.ModelSettings{}).Error
}

func EnsureDefaultModelSettings(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	builtinSettings, err := defaultconfig.LoadBuiltinModelSettings()
	if err != nil {
		return err
	}
	for _, builtin := range builtinSettings {
		if err := ensureModelSettings(db, builtin); err != nil {
			return err
		}
	}
	return nil
}

func ensureModelSettings(db *gorm.DB, settings *defaultconfig.DefaultModelSettings) error {
	if settings == nil {
		return nil
	}
	settings.Name = strings.TrimSpace(settings.Name)
	if settings.Name == "" {
		settings.Name = DefaultModelSettingsName
	}

	existing, err := GetModelSettingsByName(db, settings.Name)
	if err == nil {
		if defaultconfig.GetDefaultModelConfigApplyMode() != defaultconfig.DefaultModelConfigApplyModeForceOverwrite {
			return nil
		}
		existing.ContextLimit = settings.ContextLimit
		existing.OutputLimit = settings.OutputLimit
		existing.Prompt = settings.Prompt
		existing.SystemPromptPlacement = settings.SystemPromptPlacement
		existing.NativeOpenAIToolCalls = settings.NativeOpenAIToolCalls
		existing.ReasoningEffort = normalizeStringPointer(settings.ReasoningEffort)
		existing.TextVerbosity = normalizeStringPointer(settings.TextVerbosity)
		existing.EnableEncryptedReason = settings.EnableEncryptedReason
		existing.ParallelToolCalls = settings.ParallelToolCalls
		existing.EnablePromptCacheKey = settings.EnablePromptCacheKey
		normalizeModelSettings(existing)
		return UpdateModelSettings(db, existing)
	}
	if err != gorm.ErrRecordNotFound {
		return err
	}
	modelSettings := &models.ModelSettings{
		Name:                  settings.Name,
		ContextLimit:          settings.ContextLimit,
		OutputLimit:           settings.OutputLimit,
		Prompt:                settings.Prompt,
		SystemPromptPlacement: settings.SystemPromptPlacement,
		NativeOpenAIToolCalls: settings.NativeOpenAIToolCalls,
		ReasoningEffort:       normalizeStringPointer(settings.ReasoningEffort),
		TextVerbosity:         normalizeStringPointer(settings.TextVerbosity),
		EnableEncryptedReason: settings.EnableEncryptedReason,
		ParallelToolCalls:     settings.ParallelToolCalls,
		EnablePromptCacheKey:  settings.EnablePromptCacheKey,
	}
	normalizeModelSettings(modelSettings)
	return CreateModelSettings(db, modelSettings)
}

func normalizeStringPointer(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func normalizeModelSettings(settings *models.ModelSettings) {
	if settings == nil {
		return
	}
	switch strings.TrimSpace(settings.SystemPromptPlacement) {
	case "instruction", "user_input", "system":
	default:
		settings.SystemPromptPlacement = "system"
	}
	if settings.ReasoningEffort != nil {
		switch strings.TrimSpace(*settings.ReasoningEffort) {
		case "low", "medium", "high", "xhigh", "none", "max":
		default:
			settings.ReasoningEffort = nil
		}
	}
	if settings.TextVerbosity != nil {
		switch strings.TrimSpace(*settings.TextVerbosity) {
		case "low", "medium", "high", "xhigh":
		default:
			settings.TextVerbosity = nil
		}
	}
	if settings.BudgetTokens != nil && *settings.BudgetTokens <= 0 {
		settings.BudgetTokens = nil
	}
	settings.ThinkingType = models.NormalizeLLMThinkingType(settings.ThinkingType)
}

func InitDefaultLLMConfigs(db *gorm.DB) error {
	config := &models.LLMConfig{
		Name:                  "openai",
		Type:                  "openai",
		APIKey:                "sk-proj-1234567890",
		Model:                 "gpt-4",
		BaseURL:               "https://api.openai.com/v1",
		APIType:               models.LLMAPITypeResponse,
		SystemPromptPlacement: "instruction",
		MaxRetries:            5,
	}
	if err := db.Create(config).Error; err != nil {
		return err
	}
	// 设置为默认配置
	return SetDefaultLLMConfigID(db, config.ID)
}
