package database

import (
	"pkgs/db/models"

	"gorm.io/gorm"
)

// ProviderSetting 相关数据库操作

// GetProviderSettingByName 根据名称获取提供商设置
func GetProviderSettingByName(db *gorm.DB, name string) (*models.ProviderSetting, error) {
	var setting models.ProviderSetting
	err := db.Where("name = ?", name).First(&setting).Error
	return &setting, err
}

// GetAllProviderSettings 获取所有提供商设置
func GetAllProviderSettings(db *gorm.DB) ([]models.ProviderSetting, error) {
	var settings []models.ProviderSetting
	err := db.Find(&settings).Error
	return settings, err
}

// CreateProviderSetting 创建提供商设置
func CreateProviderSetting(db *gorm.DB, setting *models.ProviderSetting) error {
	return db.Create(setting).Error
}

// UpdateProviderSetting 更新提供商设置
func UpdateProviderSetting(db *gorm.DB, setting *models.ProviderSetting) error {
	return db.Save(setting).Error
}

// DeleteProviderSetting 删除提供商设置
func DeleteProviderSetting(db *gorm.DB, name string) error {
	return db.Where("name = ?", name).Delete(&models.ProviderSetting{}).Error
}
