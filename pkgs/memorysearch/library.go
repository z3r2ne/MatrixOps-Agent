package memorysearch

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"pkgs/db/models"
	database "pkgs/db"
	"pkgs/db/storage"
	"pkgs/embedding"

	"gorm.io/gorm"
)

type SearchResult struct {
	MemoryEntryID   uint    `json:"memoryEntryId,omitempty"`
	MemoryLibraryID uint    `json:"memoryLibraryId,omitempty"`
	SessionID       string  `json:"sessionId,omitempty"`
	SourceKind      string  `json:"sourceKind"`
	Score           float64 `json:"score"`
	Content         string  `json:"content"`
}

func hashContent(content string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(content)))
	return hex.EncodeToString(sum[:])
}

func IndexMemoryLibrarySearchDocument(ctx context.Context, db *gorm.DB, library *models.MemoryLibrary) error {
	if library == nil {
		return nil
	}
	content := strings.TrimSpace(library.Content)
	if content == "" {
		store, err := ensureStore()
		if err != nil {
			return err
		}
		return store.DeleteMemoryLibraryDocument(ctx, db, library.ID)
	}
	client, _, err := embedding.GetActiveClient(db)
	if err != nil {
		return err
	}
	vectors, err := client.Embed(ctx, []string{content})
	if err != nil {
		return err
	}
	store, err := ensureStore()
	if err != nil {
		return err
	}
	return store.Upsert(ctx, db, indexedDoc{
		ID:              memoryLibraryDocID(library.ID),
		MemoryLibraryID: library.ID,
		SourceKind:      "memory_library",
		Content:         content,
		ContentHash:     hashContent(content),
		Embedding:       vectors[0],
	})
}

func DeleteMemoryLibrarySearchIndex(ctx context.Context, db *gorm.DB, libraryID uint) error {
	store, err := ensureStore()
	if err != nil {
		return err
	}
	return store.DeleteMemoryLibraryDocument(ctx, db, libraryID)
}

func GetMemoryLibrarySearchIndexStatus(db *gorm.DB, libraryID uint) (map[string]interface{}, error) {
	library, libraryErr := database.GetMemoryLibraryByID(db, libraryID)
	aggregate, err := storage.AggregateMemoryLibrarySearchDocuments(db, libraryID)
	if err != nil {
		return nil, err
	}
	documentCount := aggregate.DocumentCount
	job, jobErr := storage.GetLatestMemoryLibraryIndexJob(db, libraryID)
	status := models.MemoryLibraryIndexJobStatusCompleted
	progress := 100
	lastError := ""
	if documentCount == 0 {
		status = "idle"
		progress = 0
	}
	if jobErr == nil && job != nil {
		status = job.Status
		progress = job.Progress
		lastError = job.LastError
	}

	contentBytes := int64(0)
	if libraryErr == nil && library != nil {
		contentBytes = int64(len([]byte(library.Name)) + len([]byte(library.Content)))
	}
	vectorBytes := documentCount * int64(aggregate.MaxDimension) * 4
	totalBytes := contentBytes + aggregate.IndexContentBytes + vectorBytes

	return map[string]interface{}{
		"memoryLibraryId":   libraryID,
		"documentCount":     documentCount,
		"vectorCount":       documentCount,
		"vectorDimension":   aggregate.MaxDimension,
		"contentBytes":      contentBytes,
		"indexContentBytes": aggregate.IndexContentBytes,
		"vectorBytes":       vectorBytes,
		"totalBytes":        totalBytes,
		"status":            status,
		"progress":          progress,
		"lastError":         lastError,
		"hasIndex":          documentCount > 0,
	}, nil
}
