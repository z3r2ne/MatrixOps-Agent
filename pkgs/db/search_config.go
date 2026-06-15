package database

import (
	"strings"

	"pkgs/db/models"

	"gorm.io/gorm"
)

func ListSearchConfigs(db *gorm.DB) ([]models.SearchConfig, error) {
	var configs []models.SearchConfig
	err := db.Order("created_at DESC").Find(&configs).Error
	return configs, err
}

func GetSearchConfigByID(db *gorm.DB, id uint) (*models.SearchConfig, error) {
	var config models.SearchConfig
	if err := db.First(&config, id).Error; err != nil {
		return nil, err
	}
	return &config, nil
}

func CreateSearchConfig(db *gorm.DB, config *models.SearchConfig) error {
	if config == nil {
		return nil
	}
	normalizeSearchConfigFields(config)
	return db.Create(config).Error
}

func UpdateSearchConfig(db *gorm.DB, config *models.SearchConfig) error {
	if config == nil {
		return nil
	}
	normalizeSearchConfigFields(config)
	return db.Save(config).Error
}

func DeleteSearchConfig(db *gorm.DB, id uint) error {
	return db.Delete(&models.SearchConfig{}, id).Error
}

func GetActiveSearchConfig(db *gorm.DB) (*models.SearchConfig, error) {
	if db == nil {
		return nil, gorm.ErrRecordNotFound
	}
	var config models.SearchConfig
	err := db.Where("enabled = ? AND TRIM(api_key) <> ''", true).
		Order("updated_at DESC").
		First(&config).Error
	if err != nil {
		return nil, err
	}
	normalizeSearchConfigFields(&config)
	return &config, nil
}

func HasEnabledSearchConfig(db *gorm.DB) bool {
	config, err := GetActiveSearchConfig(db)
	return err == nil && config != nil
}

func DisableOtherSearchConfigs(db *gorm.DB, exceptID uint) error {
	if db == nil {
		return nil
	}
	return db.Model(&models.SearchConfig{}).
		Where("id <> ? AND enabled = ?", exceptID, true).
		Update("enabled", false).Error
}

func normalizeSearchConfigFields(config *models.SearchConfig) {
	config.Name = strings.TrimSpace(config.Name)
	config.Type = models.NormalizeSearchConfigType(config.Type)
	config.APIKey = strings.TrimSpace(config.APIKey)
	config.BaseURL = strings.TrimSpace(config.BaseURL)
	if config.BaseURL == "" {
		config.BaseURL = models.DefaultSearchConfigBaseURL
	}
}
