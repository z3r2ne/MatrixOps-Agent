package memorysearch

import (
	"pkgs/db/models"
	"pkgs/db/storage"

	"gorm.io/gorm"
)

func EnqueueMemoryLibrarySearchIndexJob(db *gorm.DB, libraryID uint) (*models.MemoryLibraryIndexJob, error) {
	return storage.EnqueueMemoryLibraryIndexJob(db, libraryID)
}
