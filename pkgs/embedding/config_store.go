package embedding

import (
	"strings"

	"pkgs/db/models"

	"gorm.io/gorm"
)

func ListEmbeddingConfigs(db *gorm.DB) ([]models.EmbeddingConfig, error) {
	var configs []models.EmbeddingConfig
	err := db.Order("created_at DESC").Find(&configs).Error
	return configs, err
}

func GetEmbeddingConfigByID(db *gorm.DB, id uint) (*models.EmbeddingConfig, error) {
	var config models.EmbeddingConfig
	if err := db.First(&config, id).Error; err != nil {
		return nil, err
	}
	normalizeEmbeddingConfigFields(&config)
	return &config, nil
}

func CreateEmbeddingConfig(db *gorm.DB, config *models.EmbeddingConfig) error {
	if config == nil {
		return nil
	}
	normalizeEmbeddingConfigFields(config)
	return db.Create(config).Error
}

func UpdateEmbeddingConfig(db *gorm.DB, config *models.EmbeddingConfig) error {
	if config == nil {
		return nil
	}
	normalizeEmbeddingConfigFields(config)
	return db.Save(config).Error
}

func DeleteEmbeddingConfig(db *gorm.DB, id uint) error {
	return db.Delete(&models.EmbeddingConfig{}, id).Error
}

func GetActiveEmbeddingConfig(db *gorm.DB) (*models.EmbeddingConfig, error) {
	if db == nil {
		return nil, gorm.ErrRecordNotFound
	}
	var config models.EmbeddingConfig
	err := db.Where("enabled = ?", true).
		Order("updated_at DESC").
		First(&config).Error
	if err != nil {
		return nil, err
	}
	normalizeEmbeddingConfigFields(&config)
	return &config, nil
}

func HasEnabledEmbeddingConfig(db *gorm.DB) bool {
	config, err := GetActiveEmbeddingConfig(db)
	return err == nil && config != nil
}

func DisableOtherEmbeddingConfigs(db *gorm.DB, exceptID uint) error {
	if db == nil {
		return nil
	}
	return db.Model(&models.EmbeddingConfig{}).
		Where("id <> ? AND enabled = ?", exceptID, true).
		Update("enabled", false).Error
}

func normalizeEmbeddingConfigFields(config *models.EmbeddingConfig) {
	config.Name = strings.TrimSpace(config.Name)
	config.Type = models.NormalizeEmbeddingConfigType(config.Type)
	config.BaseURL = strings.TrimSpace(config.BaseURL)
	if config.BaseURL == "" {
		config.BaseURL = models.DefaultEmbeddingConfigBaseURL
	}
	config.BinaryPath = strings.TrimSpace(config.BinaryPath)
	config.ModelPath = strings.TrimSpace(config.ModelPath)
	config.Status = strings.TrimSpace(config.Status)
	config.LastError = strings.TrimSpace(config.LastError)
	if config.BatchSize <= 0 {
		config.BatchSize = models.DefaultEmbeddingBatchSize
	}
	if config.MaxInputTokens <= 0 {
		config.MaxInputTokens = models.DefaultEmbeddingMaxInputTokens
	}
}
