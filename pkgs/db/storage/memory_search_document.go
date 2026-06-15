package storage

import (
	"time"

	"pkgs/db/models"

	"gorm.io/gorm"
)

func UpsertMemorySearchDocument(db *gorm.DB, doc *models.MemorySearchDocument) error {
	if doc == nil {
		return nil
	}
	doc.UpdatedAt = time.Now()
	var existing models.MemorySearchDocument
	query := db.Where("content_hash = ?", doc.ContentHash)
	if doc.MemoryEntryID > 0 {
		query = query.Where("memory_entry_id = ?", doc.MemoryEntryID)
	} else if doc.MemoryLibraryID > 0 {
		query = query.Where("memory_library_id = ?", doc.MemoryLibraryID)
	}
	err := query.First(&existing).Error
	if err == nil {
		doc.ID = existing.ID
		return db.Save(doc).Error
	}
	if err != gorm.ErrRecordNotFound {
		return err
	}
	return db.Create(doc).Error
}

func DeleteMemorySearchDocumentsByLibrary(db *gorm.DB, libraryID uint) error {
	return db.Where("memory_library_id = ?", libraryID).Delete(&models.MemorySearchDocument{}).Error
}

func CountMemorySearchDocumentsByLibrary(db *gorm.DB, libraryID uint) (int64, error) {
	var count int64
	err := db.Model(&models.MemorySearchDocument{}).Where("memory_library_id = ?", libraryID).Count(&count).Error
	return count, err
}

type MemoryLibrarySearchDocumentAggregate struct {
	DocumentCount    int64
	IndexContentBytes int64
	MaxDimension     int
}

func AggregateMemoryLibrarySearchDocuments(db *gorm.DB, libraryID uint) (*MemoryLibrarySearchDocumentAggregate, error) {
	if libraryID == 0 {
		return &MemoryLibrarySearchDocumentAggregate{}, nil
	}
	var row struct {
		DocumentCount     int64
		IndexContentBytes int64
		MaxDimension      int
	}
	err := db.Model(&models.MemorySearchDocument{}).
		Where("memory_library_id = ?", libraryID).
		Select("COUNT(*) AS document_count, COALESCE(SUM(LENGTH(content)), 0) AS index_content_bytes, COALESCE(MAX(dimension), 0) AS max_dimension").
		Scan(&row).Error
	if err != nil {
		return nil, err
	}
	return &MemoryLibrarySearchDocumentAggregate{
		DocumentCount:     row.DocumentCount,
		IndexContentBytes: row.IndexContentBytes,
		MaxDimension:      row.MaxDimension,
	}, nil
}
