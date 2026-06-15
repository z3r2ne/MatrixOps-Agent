package services

import (
	"fmt"
	"log"
	"sync"
	"time"

	"matrixops-agent/util"
	"matrixops/services/task_runner"
	"pkgs/db/models"
	"pkgs/reminder"
	"pkgs/taskqueue"

	"gorm.io/gorm"
)

var (
	globalReminderWorker     *ReminderWorker
	globalReminderWorkerOnce sync.Once
)

func InitReminderWorker(db *gorm.DB) {
	globalReminderWorkerOnce.Do(func() {
		globalReminderWorker = &ReminderWorker{db: db}
		if err := reminder.Init(db, globalReminderWorker.handleFire); err != nil {
			log.Printf("[reminder] init failed: %v", err)
		}
	})
}

type ReminderWorker struct {
	db *gorm.DB
}

func (w *ReminderWorker) handleFire(job *models.ReminderJob) {
	if w == nil || w.db == nil || job == nil {
		return
	}
	content := fmt.Sprintf("⏰ 提醒：%s\n\n%s", job.Name, job.Content)
	item := models.TaskMessageQueueItem{
		ID:         util.Ascending("queue"),
		Type:       models.TaskMessageQueueTypeSystem,
		Content:    content,
		Source:     models.TaskMessageQueueSourceReminder,
		Supplement: true,
		CreatedAt:  time.Now().UnixMilli(),
	}
	queue := taskqueue.New(w.db, job.TaskID, GetGlobalWSHub(w.db).BroadcastTaskQueue)
	if err := queue.Prepend(item); err != nil {
		log.Printf("[reminder] enqueue failed job=%s task=%d: %v", job.ID, job.TaskID, err)
		return
	}
	hub := GetGlobalWSHub(w.db)
	_ = task_runner.TryAutoRunTaskQueue(job.TaskID,
		task_runner.WithDB(w.db),
		task_runner.WithWSHub(hub),
	)
	log.Printf("[reminder] fired job=%s task=%d name=%q", job.ID, job.TaskID, job.Name)
}
