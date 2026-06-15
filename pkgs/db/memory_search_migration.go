package database

import (
	"fmt"

	"gorm.io/gorm"
)

func migrateMemorySearchSchema(db *gorm.DB) error {
	if db.Migrator().HasTable("memory_embedding_chunks") && !db.Migrator().HasTable("memory_search_documents") {
		if err := db.Exec("ALTER TABLE memory_embedding_chunks RENAME TO memory_search_documents").Error; err != nil {
			return fmt.Errorf("rename memory_embedding_chunks: %w", err)
		}
	}
	if db.Migrator().HasTable("memory_search_documents") && db.Migrator().HasColumn("memory_search_documents", "embedding_blob") {
		if err := db.Migrator().DropColumn("memory_search_documents", "embedding_blob"); err != nil {
			return fmt.Errorf("drop memory_search_documents.embedding_blob: %w", err)
		}
	}
	return nil
}

func finishMemorySearchSchemaMigration(db *gorm.DB) error {
	if !db.Migrator().HasTable("embedding_index_jobs") {
		return nil
	}
	if db.Migrator().HasTable("memory_library_index_jobs") {
		var count int64
		if err := db.Table("memory_library_index_jobs").Count(&count).Error; err != nil {
			return fmt.Errorf("count memory_library_index_jobs: %w", err)
		}
		if count == 0 {
			if err := db.Exec(`
				INSERT INTO memory_library_index_jobs (
					memory_library_id, status, progress, total_items, processed_items, last_error, created_at, updated_at
				)
				SELECT
					CAST(target_id AS INTEGER),
					status,
					progress,
					total_items,
					processed_items,
					last_error,
					created_at,
					updated_at
				FROM embedding_index_jobs
				WHERE job_type = 'memory_library_index'
				  AND CAST(target_id AS INTEGER) > 0
			`).Error; err != nil {
				return fmt.Errorf("migrate embedding_index_jobs: %w", err)
			}
		}
	}
	if err := db.Migrator().DropTable("embedding_index_jobs"); err != nil {
		return fmt.Errorf("drop embedding_index_jobs: %w", err)
	}
	return nil
}
