package storage

import (
	"time"

	"gorm.io/gorm"

	"pkgs/db/models"
)

func GetPlan(db *gorm.DB, sessionID string) (*models.Plan, error) {
	var plan models.Plan
	if err := db.Where("session_id = ?", sessionID).First(&plan).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, NotFoundError{Path: "plan/" + sessionID}
		}
		return nil, err
	}
	return &plan, nil
}

func UpsertPlan(db *gorm.DB, sessionID string, content interface{}) (*models.Plan, error) {
	now := time.Now().UnixMilli()
	var plan models.Plan
	if err := db.Where("session_id = ?", sessionID).First(&plan).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			plan = models.Plan{
				SessionID: sessionID,
				Content:   models.JSONField{Data: content},
				Created:   now,
				Updated:   now,
			}
			if err := db.Create(&plan).Error; err != nil {
				return nil, err
			}
			return &plan, nil
		}
		return nil, err
	}
	plan.Content = models.JSONField{Data: content}
	plan.Updated = now
	if err := db.Save(&plan).Error; err != nil {
		return nil, err
	}
	return &plan, nil
}
