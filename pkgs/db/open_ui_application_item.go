package database

import (
	"errors"

	"pkgs/db/models"

	"gorm.io/gorm"
)

// ListOpenUIApplicationItems 按打开顺序返回记录（先打开的在前）。
func ListOpenUIApplicationItems(db *gorm.DB) ([]models.OpenUIApplicationItem, error) {
	var rows []models.OpenUIApplicationItem
	err := db.Order("id ASC").Find(&rows).Error
	return rows, err
}

// AddOpenUIApplicationItem 将资源加入已打开列表；已存在则忽略。
func AddOpenUIApplicationItem(db *gorm.DB, kind string, resourceID uint) error {
	var existing models.OpenUIApplicationItem
	err := db.Where("kind = ? AND resource_id = ?", kind, resourceID).First(&existing).Error
	if err == nil {
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	row := models.OpenUIApplicationItem{Kind: kind, ResourceID: resourceID}
	return db.Create(&row).Error
}

// DeleteOpenUIApplicationByKindResource 从已打开列表移除。
func DeleteOpenUIApplicationByKindResource(db *gorm.DB, kind string, resourceID uint) error {
	return db.Where("kind = ? AND resource_id = ?", kind, resourceID).Delete(&models.OpenUIApplicationItem{}).Error
}
