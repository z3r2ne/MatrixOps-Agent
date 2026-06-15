package database

import (
	"errors"
	"pkgs/db/models"

	"gorm.io/gorm"
)

const UIStateKeyLastClosedWorkspace = "last_closed_workspace"

func GetUIState(db *gorm.DB, key string) (*models.UIState, error) {
	var state models.UIState
	if err := db.Where("key = ?", key).First(&state).Error; err != nil {
		return nil, err
	}
	return &state, nil
}

func SetUIState(db *gorm.DB, key string, value string) error {
	state, err := GetUIState(db, key)
	if err == nil && state != nil {
		return db.Model(&models.UIState{}).Where("key = ?", key).Update("value", value).Error
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return db.Create(&models.UIState{
		Key:   key,
		Value: value,
	}).Error
}

func DeleteUIState(db *gorm.DB, key string) error {
	return db.Where("key = ?", key).Delete(&models.UIState{}).Error
}
