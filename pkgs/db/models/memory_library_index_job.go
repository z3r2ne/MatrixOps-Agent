package models

import "time"

const (
	MemoryLibraryIndexJobStatusPending   = "pending"
	MemoryLibraryIndexJobStatusRunning   = "running"
	MemoryLibraryIndexJobStatusCompleted = "completed"
	MemoryLibraryIndexJobStatusFailed    = "failed"
)

// MemoryLibraryIndexJob 记忆库检索索引异步任务。
type MemoryLibraryIndexJob struct {
	ID              uint      `json:"id" gorm:"primaryKey"`
	MemoryLibraryID uint      `json:"memoryLibraryId" gorm:"index;not null"`
	Status         string    `json:"status" gorm:"index;not null;default:pending"`
	Progress       int       `json:"progress"`
	TotalItems     int       `json:"totalItems"`
	ProcessedItems int       `json:"processedItems"`
	LastError      string    `json:"lastError"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

func (MemoryLibraryIndexJob) TableName() string {
	return "memory_library_index_jobs"
}
