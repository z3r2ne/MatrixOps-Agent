package database

import (
	"strconv"
	"strings"
	"time"

	"pkgs/db/models"

	"gorm.io/gorm"
)

// GlobalConfig 相关数据库操作

// GetGlobalConfigByKey 根据 key 获取全局配置
func GetGlobalConfigByKey(db *gorm.DB, key string) (*models.GlobalConfig, error) {
	var config models.GlobalConfig
	err := db.Where("key = ?", key).First(&config).Error
	return &config, err
}

// GetAllGlobalConfigs 获取所有全局配置
func GetAllGlobalConfigs(db *gorm.DB) ([]models.GlobalConfig, error) {
	var configs []models.GlobalConfig
	err := db.Find(&configs).Error
	return configs, err
}

// CreateGlobalConfig 创建全局配置
func CreateGlobalConfig(db *gorm.DB, config *models.GlobalConfig) error {
	return db.Create(config).Error
}

// UpdateGlobalConfig 更新全局配置
func UpdateGlobalConfig(db *gorm.DB, config *models.GlobalConfig) error {
	return db.Save(config).Error
}

// UpsertGlobalConfig 创建或更新全局配置
func UpsertGlobalConfig(db *gorm.DB, key, value string) error {
	var config models.GlobalConfig
	if err := db.Where("key = ?", key).First(&config).Error; err == nil {
		// 已存在，更新
		config.Value = value
		return UpdateGlobalConfig(db, &config)
	}
	// 不存在，创建
	config = models.GlobalConfig{Key: key, Value: value}
	return CreateGlobalConfig(db, &config)
}

// DeleteGlobalConfig 删除全局配置
func DeleteGlobalConfig(db *gorm.DB, key string) error {
	return db.Where("key = ?", key).Delete(&models.GlobalConfig{}).Error
}

// GetLLMHTTPClientTimeout 读取全局配置 llm_http_timeout_seconds（秒），用于 http.Client.Timeout。
// 未配置、非正整数或 ≤0 时返回 0，表示不设置 Timeout（流式长连接可一直读至结束）。
func GetLLMHTTPClientTimeout(db *gorm.DB) time.Duration {
	if db == nil {
		return 0
	}
	cfg, err := GetGlobalConfigByKey(db, models.ConfigKeyLLMHTTPTimeoutSeconds)
	if err != nil || cfg == nil {
		return 0
	}
	s := strings.TrimSpace(cfg.Value)
	if s == "" {
		return 0
	}
	sec, err := strconv.Atoi(s)
	if err != nil || sec <= 0 {
		return 0
	}
	return time.Duration(sec) * time.Second
}

// GetLLMHTTPConnectTimeout 读取全局配置 llm_http_connect_timeout_seconds（秒），用于 http.Transport.ResponseHeaderTimeout。
// 未配置、非正整数或 ≤0 时返回 0，表示不设置 ResponseHeaderTimeout。
func GetLLMHTTPConnectTimeout(db *gorm.DB) time.Duration {
	if db == nil {
		return 0
	}
	cfg, err := GetGlobalConfigByKey(db, models.ConfigKeyLLMHTTPConnectTimeoutSeconds)
	if err != nil || cfg == nil {
		return 0
	}
	s := strings.TrimSpace(cfg.Value)
	if s == "" {
		return 0
	}
	sec, err := strconv.Atoi(s)
	if err != nil || sec <= 0 {
		return 0
	}
	return time.Duration(sec) * time.Second
}

// GetLLMMaxOutputTokens 读取全局配置 llm_max_output_tokens。未配置、非正整数或 ≤0 时返回 models.DefaultLLMMaxOutputTokens。
func GetLLMMaxOutputTokens(db *gorm.DB) int {
	if db == nil {
		return models.DefaultLLMMaxOutputTokens
	}
	cfg, err := GetGlobalConfigByKey(db, models.ConfigKeyLLMMaxOutputTokens)
	if err != nil || cfg == nil {
		return models.DefaultLLMMaxOutputTokens
	}
	s := strings.TrimSpace(cfg.Value)
	if s == "" {
		return models.DefaultLLMMaxOutputTokens
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return models.DefaultLLMMaxOutputTokens
	}
	return n
}

// EffectiveLLMMaxOutputTokens 若 modelOutputLimit > 0（如 ModelSettings.OutputLimit）则使用该值，否则使用 GetLLMMaxOutputTokens(db)。
func EffectiveLLMMaxOutputTokens(db *gorm.DB, modelOutputLimit int) int {
	if modelOutputLimit > 0 {
		return modelOutputLimit
	}
	return GetLLMMaxOutputTokens(db)
}

func GetDefaultTaskListGroupMode(db *gorm.DB) models.TaskListGroupMode {
	if db == nil {
		return models.DefaultTaskListGroupMode
	}
	cfg, err := GetGlobalConfigByKey(db, models.ConfigKeyDefaultTaskListGroupMode)
	if err != nil || cfg == nil {
		return models.DefaultTaskListGroupMode
	}
	return models.TaskListGroupModeOrDefault(cfg.Value)
}

func GetDefaultProjectToolPermissions(db *gorm.DB) map[string]string {
	if db == nil {
		return map[string]string{}
	}
	cfg, err := GetGlobalConfigByKey(db, models.ConfigKeyDefaultProjectToolPermissions)
	if err != nil || cfg == nil {
		return map[string]string{}
	}
	values, err := models.ParseProjectToolPermissions(cfg.Value)
	if err != nil {
		return map[string]string{}
	}
	return values
}

func GetDefaultProjectToolPermissionsJSON(db *gorm.DB) string {
	return models.NormalizeProjectToolPermissionsJSON(GetDefaultProjectToolPermissions(db))
}

func parseOptionalPercent(raw string) (int, bool) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return 0, false
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 0, false
	}
	if n > 100 {
		return 100, true
	}
	return n, true
}

func parseGlobalConfigPercent(db *gorm.DB, key string, fallback int) int {
	if db == nil {
		return fallback
	}
	cfg, err := GetGlobalConfigByKey(db, key)
	if err != nil || cfg == nil {
		return fallback
	}
	s := strings.TrimSpace(cfg.Value)
	if s == "" {
		return fallback
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return fallback
	}
	if n > 100 {
		return 100
	}
	return n
}

// GetMemoryCompactionTriggerThresholdPercent 读取自动记忆压缩触发阈值（%）。
func GetMemoryCompactionTriggerThresholdPercent(db *gorm.DB) int {
	return parseGlobalConfigPercent(db, models.ConfigKeyMemoryCompactionTriggerThresholdPercent, models.DefaultMemoryCompactionTriggerThresholdPercent)
}

// GetMemoryCompactionTargetPercent 读取记忆压缩目标占用率（%）。
func GetMemoryCompactionTargetPercent(db *gorm.DB) int {
	return parseGlobalConfigPercent(db, models.ConfigKeyMemoryCompactionTargetPercent, models.DefaultMemoryCompactionTargetPercent)
}

// GetMemoryCompactionL2ScopePercent 读取 L2 摘要 batch 范围（%）。
func GetMemoryCompactionL2ScopePercent(db *gorm.DB) int {
	if db != nil {
		if cfg, err := GetGlobalConfigByKey(db, models.ConfigKeyMemoryCompactionL2ScopePercent); err == nil && cfg != nil {
			if n, ok := parseOptionalPercent(cfg.Value); ok {
				return n
			}
		}
	}
	return parseGlobalConfigPercent(db, models.ConfigKeyMemoryCompactionScopePercent, models.DefaultMemoryCompactionL2ScopePercent)
}

// GetMemoryCompactionScopePercent 已废弃，等价于 GetMemoryCompactionL2ScopePercent。
func GetMemoryCompactionScopePercent(db *gorm.DB) int {
	return GetMemoryCompactionL2ScopePercent(db)
}

// GetAgentMaxSteps 读取全局配置 agent_max_steps。
// 未配置、非正整数或 ≤0 时返回 models.DefaultAgentMaxSteps。
func GetAgentMaxSteps(db *gorm.DB) int {
	if db == nil {
		return models.DefaultAgentMaxSteps
	}
	cfg, err := GetGlobalConfigByKey(db, models.ConfigKeyAgentMaxSteps)
	if err != nil || cfg == nil {
		return models.DefaultAgentMaxSteps
	}
	s := strings.TrimSpace(cfg.Value)
	if s == "" {
		return models.DefaultAgentMaxSteps
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return models.DefaultAgentMaxSteps
	}
	return n
}

// DefaultStallWatchdogTimeout 为 stall_watchdog_timeout_seconds 未配置或无效时的默认值。
const DefaultStallWatchdogTimeout = 10 * time.Second

// GetStallWatchdogTimeout 读取全局配置 stall_watchdog_timeout_seconds（秒）。
// 未配置、非正整数或 ≤0 时返回 DefaultStallWatchdogTimeout。
func GetStallWatchdogTimeout(db *gorm.DB) time.Duration {
	if db == nil {
		return DefaultStallWatchdogTimeout
	}
	cfg, err := GetGlobalConfigByKey(db, models.ConfigKeyStallWatchdogTimeoutSeconds)
	if err != nil || cfg == nil {
		return DefaultStallWatchdogTimeout
	}
	s := strings.TrimSpace(cfg.Value)
	if s == "" {
		return DefaultStallWatchdogTimeout
	}
	sec, err := strconv.Atoi(s)
	if err != nil || sec <= 0 {
		return DefaultStallWatchdogTimeout
	}
	return time.Duration(sec) * time.Second
}
