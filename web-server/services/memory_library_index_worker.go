package services

import (
	"context"
	"log"
	"sync"
	"time"

	database "pkgs/db"
	"pkgs/db/models"
	"pkgs/db/storage"
	"pkgs/memorysearch"

	"gorm.io/gorm"
)

type MemoryLibraryIndexWorker struct {
	db       *gorm.DB
	stopCh   chan struct{}
	stopOnce sync.Once
}

var (
	globalMemoryLibraryIndexWorker     *MemoryLibraryIndexWorker
	globalMemoryLibraryIndexWorkerOnce sync.Once
)

func InitMemoryLibraryIndexWorker(db *gorm.DB) *MemoryLibraryIndexWorker {
	globalMemoryLibraryIndexWorkerOnce.Do(func() {
		globalMemoryLibraryIndexWorker = &MemoryLibraryIndexWorker{
			db:     db,
			stopCh: make(chan struct{}),
		}
		go globalMemoryLibraryIndexWorker.loop()
	})
	return globalMemoryLibraryIndexWorker
}

func (w *MemoryLibraryIndexWorker) loop() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-w.stopCh:
			return
		case <-ticker.C:
			w.processOne(context.Background())
		}
	}
}

func (w *MemoryLibraryIndexWorker) processOne(ctx context.Context) {
	if w == nil || w.db == nil {
		return
	}
	job, err := storage.ClaimNextPendingMemoryLibraryIndexJob(w.db)
	if err != nil {
		return
	}
	runErr := w.runJob(ctx, job)
	if runErr != nil {
		log.Printf("memory library search index job %d failed: %v", job.ID, runErr)
		_ = storage.UpdateMemoryLibraryIndexJobProgress(
			w.db,
			job.ID,
			job.ProcessedItems,
			job.TotalItems,
			models.MemoryLibraryIndexJobStatusFailed,
			runErr.Error(),
		)
		return
	}
	_ = storage.UpdateMemoryLibraryIndexJobProgress(
		w.db,
		job.ID,
		job.TotalItems,
		job.TotalItems,
		models.MemoryLibraryIndexJobStatusCompleted,
		"",
	)
}

func (w *MemoryLibraryIndexWorker) runJob(ctx context.Context, job *models.MemoryLibraryIndexJob) error {
	library, err := database.GetMemoryLibraryByID(w.db, job.MemoryLibraryID)
	if err != nil {
		return err
	}
	return memorysearch.IndexMemoryLibrarySearchDocument(ctx, w.db, library)
}
