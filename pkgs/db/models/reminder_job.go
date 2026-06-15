package models

import "time"

const (
	ReminderScheduleRelative = "relative"
	ReminderScheduleCron     = "cron"

	ReminderStatusPending   = "pending"
	ReminderStatusCancelled = "cancelled"
	ReminderStatusCompleted = "completed"
)

type ReminderJob struct {
	ID           string     `json:"id" gorm:"primaryKey;size:64"`
	TaskID       uint       `json:"taskId" gorm:"index;not null"`
	SessionID    string     `json:"sessionId" gorm:"index;size:255"`
	Name         string     `json:"name" gorm:"size:255;not null"`
	Content      string     `json:"content" gorm:"type:text;not null"`
	TimeSpec     string     `json:"timeSpec" gorm:"size:128;not null"`
	ScheduleKind string     `json:"scheduleKind" gorm:"size:32;not null"`
	RunAt        *time.Time `json:"runAt,omitempty"`
	CronExpr     string     `json:"cronExpr,omitempty" gorm:"size:128"`
	NextRunAt    *time.Time `json:"nextRunAt,omitempty" gorm:"index"`
	Status       string     `json:"status" gorm:"size:32;not null;default:pending;index"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
}
