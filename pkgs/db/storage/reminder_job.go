package storage

import (
	"strings"
	"time"

	"pkgs/db/models"

	"gorm.io/gorm"
)

func CreateReminderJob(db *gorm.DB, job *models.ReminderJob) error {
	if job == nil {
		return nil
	}
	now := time.Now()
	job.CreatedAt = now
	job.UpdatedAt = now
	return db.Create(job).Error
}

func GetReminderJobByID(db *gorm.DB, id string) (*models.ReminderJob, error) {
	var job models.ReminderJob
	err := db.First(&job, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &job, nil
}

func ListReminderJobs(db *gorm.DB, taskID uint, sessionID string) ([]models.ReminderJob, error) {
	query := db.Model(&models.ReminderJob{}).Where("status = ?", models.ReminderStatusPending)
	if taskID > 0 {
		query = query.Where("task_id = ?", taskID)
	}
	if sessionID = strings.TrimSpace(sessionID); sessionID != "" {
		query = query.Where("session_id = ?", sessionID)
	}
	var jobs []models.ReminderJob
	err := query.Order("created_at ASC, id ASC").Find(&jobs).Error
	return jobs, err
}

func ListPendingReminderJobs(db *gorm.DB) ([]models.ReminderJob, error) {
	var jobs []models.ReminderJob
	err := db.Where("status = ?", models.ReminderStatusPending).
		Order("COALESCE(next_run_at, run_at) ASC, id ASC").
		Find(&jobs).Error
	return jobs, err
}

func UpdateReminderJobFields(db *gorm.DB, id string, updates map[string]interface{}) error {
	if len(updates) == 0 {
		return nil
	}
	updates["updated_at"] = time.Now()
	return db.Model(&models.ReminderJob{}).Where("id = ?", id).Updates(updates).Error
}

func CancelReminderJob(db *gorm.DB, id string) error {
	return UpdateReminderJobFields(db, id, map[string]interface{}{
		"status": models.ReminderStatusCancelled,
	})
}
