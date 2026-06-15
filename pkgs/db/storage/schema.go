package storage

import (
	"fmt"
	"strings"
	"sync"

	"pkgs/db/models"

	"gorm.io/gorm"
)

var (
	schemaEnsureMu sync.Mutex
	schemaCache    sync.Map
)

func ensureSessionDataSchema(db *gorm.DB) error {
	return ensureSchema(db, "session-data", func(tx *gorm.DB) error {
		if err := tx.AutoMigrate(&models.Session{}, &models.Message{}, &models.Part{}, &models.MessagePromptSnapshot{}, &models.MessagePromptSnapshotHistory{}, &models.MessageCodeSnapshot{}); err != nil {
			return err
		}
		// One-time backfill: older sessions may lack workspace_root / workspace_path.
		// Move the self-healing out of GetSession to keep reads side-effect free.
		return backfillMissingSessionWorkspaceInfo(tx)
	})
}

func backfillMissingSessionWorkspaceInfo(db *gorm.DB) error {
	if db == nil {
		return nil
	}

	var sessions []models.Session
	if err := db.
		Where("(workspace_root IS NULL OR workspace_root = '') OR (workspace_path IS NULL OR workspace_path = '')").
		Find(&sessions).Error; err != nil {
		return err
	}
	for i := range sessions {
		s := &sessions[i]
		if s == nil || strings.TrimSpace(s.ID) == "" {
			continue
		}
		info := models.SessionFromModel(s)
		if info == nil {
			continue
		}
		if err := ensureSessionWorkspaceInfo(db, info); err != nil {
			return err
		}
		if strings.TrimSpace(info.WorkspaceRoot) == "" || strings.TrimSpace(info.WorkspacePath) == "" {
			continue
		}
		if err := db.Model(&models.Session{}).Where("id = ?", s.ID).Updates(map[string]interface{}{
			"workspace_root": info.WorkspaceRoot,
			"workspace_path": info.WorkspacePath,
		}).Error; err != nil {
			return err
		}
	}
	return nil
}

func ensureMessageCodeSnapshotSchema(db *gorm.DB) error {
	return ensureSchema(db, "message-code-snapshot", func(tx *gorm.DB) error {
		return tx.AutoMigrate(&models.MessageCodeSnapshot{})
	})
}

func ensureSchema(db *gorm.DB, schemaName string, migrate func(*gorm.DB) error) error {
	if db == nil {
		return nil
	}

	key := schemaCacheKey(db, schemaName)
	if _, ok := schemaCache.Load(key); ok {
		return nil
	}

	schemaEnsureMu.Lock()
	defer schemaEnsureMu.Unlock()

	if _, ok := schemaCache.Load(key); ok {
		return nil
	}

	if err := migrate(db); err != nil {
		return err
	}

	schemaCache.Store(key, struct{}{})
	return nil
}

func schemaCacheKey(db *gorm.DB, schemaName string) string {
	if sqlDB, err := db.DB(); err == nil && sqlDB != nil {
		return fmt.Sprintf("%s:%p", schemaName, sqlDB)
	}
	return fmt.Sprintf("%s:%p", schemaName, db)
}
