package storage

import (
	"sort"
	"strings"
	"time"

	"matrixops-agent/types"
	"pkgs/db/models"

	"gorm.io/gorm"
)

func ensureMemoryEntriesSchema(db *gorm.DB) error {
	return ensureSchema(db, "memory-entries", migrateMemoryEntriesSchema)
}

func migrateMemoryEntriesSchema(db *gorm.DB) error {
	requiredColumns := []string{
		"session_id",
		"source_message_id",
		"source_part_id",
		"entry_kind",
		"role",
		"content",
		"raw_output",
		"phase",
		"responses_output_message_raw",
		"responses_reasoning_item_raws_json",
		"call_tool_info",
		"tool_call_id",
		"tool_name",
		"tool_status",
		"tool_reason",
		"tool_request_raw_json",
		"tool_input_json",
		"tool_output",
		"tool_error",
		"tool_title",
		"tool_metadata_json",
		"synthetic",
		"search_archived",
		"sequence",
		"token_count",
		"created",
		"updated",
	}

	if !db.Migrator().HasTable(&models.MemoryEntry{}) {
		return db.AutoMigrate(&models.MemoryEntry{})
	}

	for _, column := range requiredColumns {
		if db.Migrator().HasColumn(&models.MemoryEntry{}, column) {
			continue
		}

		// 记忆表是从消息表派生出来的，可安全重建。
		if err := db.Migrator().DropTable(&models.MemoryEntry{}); err != nil {
			return err
		}
		return db.AutoMigrate(&models.MemoryEntry{})
	}

	return db.AutoMigrate(&models.MemoryEntry{})
}

func ListMemoryEntriesBySession(db *gorm.DB, sessionID string) ([]*types.MemoryEntry, error) {
	if err := ensureMemoryEntriesSchema(db); err != nil {
		return nil, err
	}

	var entries []models.MemoryEntry
	if err := db.Where("session_id = ?", sessionID).Order("sequence ASC, id ASC").Find(&entries).Error; err != nil {
		return nil, err
	}

	result := make([]*types.MemoryEntry, 0, len(entries))
	for index := range entries {
		result = append(result, models.MemoryEntryFromModel(&entries[index]))
	}
	return result, nil
}

func CountMemoryEntriesBySession(db *gorm.DB, sessionID string) (int64, error) {
	if err := ensureMemoryEntriesSchema(db); err != nil {
		return 0, err
	}

	var count int64
	err := db.Model(&models.MemoryEntry{}).Where("session_id = ?", sessionID).Count(&count).Error
	return count, err
}

func ReplaceMemoryEntriesForMessage(db *gorm.DB, sessionID, sourceMessageID string, entries []*types.MemoryEntry) error {
	if err := ensureMemoryEntriesSchema(db); err != nil {
		return err
	}

	err := db.Transaction(func(tx *gorm.DB) error {
		if delErr := tx.Where("session_id = ? AND source_message_id = ?", sessionID, sourceMessageID).Delete(&models.MemoryEntry{}).Error; delErr != nil {
			return delErr
		}

		for _, entry := range entries {
			modelEntry := models.MemoryEntryToModel(entry)
			if modelEntry == nil {
				continue
			}
			if modelEntry.Created == 0 {
				modelEntry.Created = time.Now().UnixMilli()
			}
			if modelEntry.Updated == 0 {
				modelEntry.Updated = modelEntry.Created
			}
			if createErr := tx.Create(modelEntry).Error; createErr != nil {
				return createErr
			}
			entry.ID = modelEntry.ID
		}

		return nil
	})
	return err
}

func ReplaceMemoryEntriesForPart(db *gorm.DB, sessionID, sourceMessageID, sourcePartID string, entries []*types.MemoryEntry) error {
	if err := ensureMemoryEntriesSchema(db); err != nil {
		return err
	}

	err := db.Transaction(func(tx *gorm.DB) error {
		if delErr := tx.Where(
			"session_id = ? AND source_message_id = ? AND source_part_id = ?",
			sessionID,
			sourceMessageID,
			sourcePartID,
		).Delete(&models.MemoryEntry{}).Error; delErr != nil {
			return delErr
		}

		for _, entry := range entries {
			modelEntry := models.MemoryEntryToModel(entry)
			if modelEntry == nil {
				continue
			}
			if modelEntry.Created == 0 {
				modelEntry.Created = time.Now().UnixMilli()
			}
			if modelEntry.Updated == 0 {
				modelEntry.Updated = modelEntry.Created
			}
			if createErr := tx.Create(modelEntry).Error; createErr != nil {
				return createErr
			}
			entry.ID = modelEntry.ID
		}

		return nil
	})
	return err
}

// PatchThinkingSignatureForAssistantMessage 将 Anthropic thinking 签名写回该消息下所有 assistant 正文 memory 行。
func PatchThinkingSignatureForAssistantMessage(db *gorm.DB, sessionID, sourceMessageID, signature string) error {
	if err := ensureMemoryEntriesSchema(db); err != nil {
		return err
	}
	sig := strings.TrimSpace(signature)
	if sig == "" {
		return nil
	}
	now := time.Now().UnixMilli()
	return db.Model(&models.MemoryEntry{}).
		Where("session_id = ? AND source_message_id = ? AND entry_kind = ? AND role = ?", sessionID, sourceMessageID, "text", "assistant").
		Updates(map[string]interface{}{
			"thinking_signature": sig,
			"updated":              now,
		}).Error
}

func nextMemoryEntrySequence(tx *gorm.DB, sessionID string) (int64, error) {
	var maxSequence int64
	if err := tx.Model(&models.MemoryEntry{}).
		Where("session_id = ?", sessionID).
		Select("COALESCE(MAX(sequence), 0)").
		Scan(&maxSequence).Error; err != nil {
		return 0, err
	}
	return maxSequence + 1, nil
}

func CreateMemoryEntry(db *gorm.DB, entry *types.MemoryEntry) error {
	if entry == nil || entry.SessionID == "" {
		return nil
	}
	if err := ensureMemoryEntriesSchema(db); err != nil {
		return err
	}

	err := db.Transaction(func(tx *gorm.DB) error {
		if entry.Sequence == 0 {
			sequence, seqErr := nextMemoryEntrySequence(tx, entry.SessionID)
			if seqErr != nil {
				return seqErr
			}
			entry.Sequence = sequence
		}
		if entry.Created == 0 {
			entry.Created = time.Now().UnixMilli()
		}
		entry.Updated = time.Now().UnixMilli()

		modelEntry := models.MemoryEntryToModel(entry)
		if modelEntry == nil {
			return nil
		}
		if createErr := tx.Create(modelEntry).Error; createErr != nil {
			return createErr
		}
		entry.ID = modelEntry.ID
		return nil
	})
	return err
}

func UpdateMemoryEntry(db *gorm.DB, sessionID string, entry *types.MemoryEntry) error {
	if entry == nil || entry.ID == 0 || sessionID == "" {
		return nil
	}
	if err := ensureMemoryEntriesSchema(db); err != nil {
		return err
	}

	entry.Updated = time.Now().UnixMilli()
	updates := map[string]interface{}{
		"entry_kind":                         entry.EntryKind,
		"role":                               entry.Role,
		"content":                            entry.Content,
		"raw_output":                         entry.RawOutput,
		"phase":                              entry.Phase,
		"responses_output_message_raw":       entry.ResponsesOutputMessageRaw,
		"responses_reasoning_item_raws_json": entry.ResponsesReasoningItemRawsJSON,
		"reasoning_content":                  entry.ReasoningContent,
		"call_tool_info":                     entry.CallToolInfo,
		"tool_call_id":                       entry.ToolCallID,
		"tool_name":                          entry.ToolName,
		"tool_status":                        entry.ToolStatus,
		"tool_reason":                        entry.ToolReason,
		"tool_request_raw_json":              entry.ToolRequestRawJSON,
		"tool_input_json":                    entry.ToolInputJSON,
		"tool_output":                        entry.ToolOutput,
		"tool_error":                         entry.ToolError,
		"tool_title":                         entry.ToolTitle,
		"tool_metadata_json":                 entry.ToolMetadataJSON,
		"synthetic":                          entry.Synthetic,
		"search_archived":                    entry.SearchArchived,
		"sequence":                           entry.Sequence,
		"token_count":                        entry.TokenCount,
		"updated":                            entry.Updated,
	}

	return db.Model(&models.MemoryEntry{}).
		Where("session_id = ? AND id = ?", sessionID, entry.ID).
		Updates(updates).Error
}

func DeleteMemoryEntryByID(db *gorm.DB, sessionID string, entryID uint) error {
	if sessionID == "" || entryID == 0 {
		return nil
	}
	if err := ensureMemoryEntriesSchema(db); err != nil {
		return err
	}
	return db.Where("session_id = ? AND id = ?", sessionID, entryID).Delete(&models.MemoryEntry{}).Error
}

func DeleteMemoryEntriesByMessage(db *gorm.DB, sessionID, sourceMessageID string) error {
	if err := ensureMemoryEntriesSchema(db); err != nil {
		return err
	}
	return db.Where("session_id = ? AND source_message_id = ?", sessionID, sourceMessageID).Delete(&models.MemoryEntry{}).Error
}

func DeleteMemoryEntriesByPart(db *gorm.DB, sessionID, sourceMessageID, sourcePartID string) error {
	if err := ensureMemoryEntriesSchema(db); err != nil {
		return err
	}
	return db.Where(
		"session_id = ? AND source_message_id = ? AND source_part_id = ?",
		sessionID,
		sourceMessageID,
		sourcePartID,
	).Delete(&models.MemoryEntry{}).Error
}

func DeleteMemoryEntriesBySession(db *gorm.DB, sessionID string) error {
	if err := ensureMemoryEntriesSchema(db); err != nil {
		return err
	}
	return db.Where("session_id = ?", sessionID).Delete(&models.MemoryEntry{}).Error
}

func ReplaceSessionMemoryWithEntries(db *gorm.DB, sessionID string, entries []*types.MemoryEntry) error {
	if err := ensureMemoryEntriesSchema(db); err != nil {
		return err
	}

	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("session_id = ?", sessionID).Delete(&models.MemoryEntry{}).Error; err != nil {
			return err
		}

		sort.Slice(entries, func(i, j int) bool {
			if entries[i] == nil || entries[j] == nil {
				return i < j
			}
			if entries[i].Sequence == entries[j].Sequence {
				return entries[i].Created < entries[j].Created
			}
			return entries[i].Sequence < entries[j].Sequence
		})

		for _, entry := range entries {
			modelEntry := models.MemoryEntryToModel(entry)
			if modelEntry == nil {
				continue
			}
			if modelEntry.Created == 0 {
				modelEntry.Created = time.Now().UnixMilli()
			}
			if modelEntry.Updated == 0 {
				modelEntry.Updated = modelEntry.Created
			}
			if err := tx.Create(modelEntry).Error; err != nil {
				return err
			}
			entry.ID = modelEntry.ID
		}

		return nil
	})
}
