package storage

import (
	"pkgs/db/models"

	"gorm.io/gorm"
)

func CreateMessageCodeSnapshot(db *gorm.DB, row *models.MessageCodeSnapshot) error {
	if db == nil || row == nil {
		return nil
	}
	if err := ensureMessageCodeSnapshotSchema(db); err != nil {
		return err
	}
	return db.Create(row).Error
}

func ListMessageCodeSnapshotsBySessionID(db *gorm.DB, sessionID string) ([]models.MessageCodeSnapshot, error) {
	if db == nil {
		return nil, nil
	}
	if err := ensureMessageCodeSnapshotSchema(db); err != nil {
		return nil, err
	}
	var rows []models.MessageCodeSnapshot
	err := db.Where("session_id = ?", sessionID).
		Order("created ASC, message_id ASC").
		Find(&rows).Error
	return rows, err
}

func GetMessageCodeSnapshotByID(db *gorm.DB, id string) (*models.MessageCodeSnapshot, error) {
	if db == nil || id == "" {
		return nil, gorm.ErrRecordNotFound
	}
	if err := ensureMessageCodeSnapshotSchema(db); err != nil {
		return nil, err
	}
	var row models.MessageCodeSnapshot
	if err := db.Where("id = ?", id).First(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

func DeleteMessageCodeSnapshotByID(db *gorm.DB, id string) error {
	if db == nil || id == "" {
		return nil
	}
	return db.Delete(&models.MessageCodeSnapshot{}, "id = ?", id).Error
}

func MessageCodeSnapshotExistsForPartID(db *gorm.DB, partID string) (bool, error) {
	if db == nil || partID == "" {
		return false, nil
	}
	if err := ensureMessageCodeSnapshotSchema(db); err != nil {
		return false, err
	}
	var n int64
	if err := db.Model(&models.MessageCodeSnapshot{}).Where("part_id = ?", partID).Count(&n).Error; err != nil {
		return false, err
	}
	return n > 0, nil
}
