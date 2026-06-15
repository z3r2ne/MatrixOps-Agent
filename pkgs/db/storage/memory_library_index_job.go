package storage

import (
	"fmt"
	"strings"
	"time"

	"pkgs/db/models"

	"gorm.io/gorm"
)

func EnqueueMemoryLibraryIndexJob(db *gorm.DB, libraryID uint) (*models.MemoryLibraryIndexJob, error) {
	if libraryID == 0 {
		return nil, fmt.Errorf("memoryLibraryID is required")
	}
	var existing models.MemoryLibraryIndexJob
	err := db.Where("memory_library_id = ? AND status IN ?", libraryID, []string{
		models.MemoryLibraryIndexJobStatusPending,
		models.MemoryLibraryIndexJobStatusRunning,
	}).First(&existing).Error
	if err == nil {
		return &existing, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, err
	}
	job := &models.MemoryLibraryIndexJob{
		MemoryLibraryID: libraryID,
		Status:          models.MemoryLibraryIndexJobStatusPending,
	}
	if err := db.Create(job).Error; err != nil {
		return nil, err
	}
	return job, nil
}

func GetLatestMemoryLibraryIndexJob(db *gorm.DB, libraryID uint) (*models.MemoryLibraryIndexJob, error) {
	var job models.MemoryLibraryIndexJob
	err := db.Where("memory_library_id = ?", libraryID).
		Order("updated_at DESC, id DESC").
		First(&job).Error
	if err != nil {
		return nil, err
	}
	return &job, nil
}

func UpdateMemoryLibraryIndexJobProgress(db *gorm.DB, jobID uint, processed, total int, status, lastError string) error {
	progress := 0
	if total > 0 {
		progress = processed * 100 / total
		if progress > 100 {
			progress = 100
		}
	}
	return db.Model(&models.MemoryLibraryIndexJob{}).Where("id = ?", jobID).Updates(map[string]interface{}{
		"processed_items": processed,
		"total_items":     total,
		"progress":        progress,
		"status":          status,
		"last_error":      strings.TrimSpace(lastError),
		"updated_at":      time.Now(),
	}).Error
}

func ClaimNextPendingMemoryLibraryIndexJob(db *gorm.DB) (*models.MemoryLibraryIndexJob, error) {
	var job models.MemoryLibraryIndexJob
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("status = ?", models.MemoryLibraryIndexJobStatusPending).
			Order("updated_at ASC, id ASC").
			First(&job).Error; err != nil {
			return err
		}
		return tx.Model(&job).Updates(map[string]interface{}{
			"status":     models.MemoryLibraryIndexJobStatusRunning,
			"updated_at": time.Now(),
		}).Error
	})
	if err != nil {
		return nil, err
	}
	return &job, nil
}
