package storage

import (
	"strings"
	"time"

	"pkgs/db/models"

	"gorm.io/gorm"
)

type PromptSnapshotResult struct {
	MessageID   string
	SessionID   string
	Prompt      string
	RawResponse string
}

type PromptSnapshotHistoryResult struct {
	ID          uint
	MessageID   string
	SessionID   string
	Prompt      string
	RawResponse string
	Created     int64
}

func UpsertMessagePromptSnapshot(
	db *gorm.DB,
	messageID string,
	sessionID string,
	prompt string,
	rawResponse string,
) error {
	if db == nil {
		return nil
	}
	messageID = strings.TrimSpace(messageID)
	sessionID = strings.TrimSpace(sessionID)
	prompt = strings.TrimSpace(prompt)
	rawResponse = strings.TrimSpace(rawResponse)

	if messageID == "" || sessionID == "" || (prompt == "" && rawResponse == "") {
		return nil
	}
	if err := ensureSessionDataSchema(db); err != nil {
		return err
	}

	now := time.Now().UnixMilli()
	snapshot := &models.MessagePromptSnapshot{
		MessageID:   messageID,
		SessionID:   sessionID,
		Prompt:      prompt,
		RawResponse: rawResponse,
		Created:     now,
		Updated:     now,
	}

	return db.Transaction(func(tx *gorm.DB) error {
		history := &models.MessagePromptSnapshotHistory{
			MessageID:   messageID,
			SessionID:   sessionID,
			Prompt:      prompt,
			RawResponse: rawResponse,
			Created:     now,
		}
		if err := tx.Create(history).Error; err != nil {
			return err
		}

		var existing models.MessagePromptSnapshot
		if err := tx.Where("message_id = ?", messageID).First(&existing).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return tx.Create(snapshot).Error
			}
			return err
		}

		updates := map[string]interface{}{
			"session_id":   sessionID,
			"updated":      now,
			"prompt":       prompt,
			"raw_response": rawResponse,
		}
		if existing.Created == 0 {
			updates["created"] = now
		}
		return tx.Model(&models.MessagePromptSnapshot{}).Where("message_id = ?", messageID).Updates(updates).Error
	})
}

func GetLatestPromptSnapshotBySessionID(db *gorm.DB, sessionID string) (*PromptSnapshotResult, error) {
	if err := ensureSessionDataSchema(db); err != nil {
		return nil, err
	}
	var row models.MessagePromptSnapshot
	if err := db.Where("session_id = ?", strings.TrimSpace(sessionID)).
		Order("updated DESC, created DESC, message_id DESC").
		Take(&row).Error; err != nil {
		return nil, err
	}
	return &PromptSnapshotResult{
		MessageID:   row.MessageID,
		SessionID:   row.SessionID,
		Prompt:      row.Prompt,
		RawResponse: row.RawResponse,
	}, nil
}

func ListPromptSnapshotHistoryByMessageID(db *gorm.DB, messageID string) ([]PromptSnapshotHistoryResult, error) {
	if err := ensureSessionDataSchema(db); err != nil {
		return nil, err
	}
	var rows []models.MessagePromptSnapshotHistory
	if err := db.Where("message_id = ?", strings.TrimSpace(messageID)).
		Order("created ASC, id ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]PromptSnapshotHistoryResult, 0, len(rows))
	for _, row := range rows {
		out = append(out, PromptSnapshotHistoryResult{
			ID:          row.ID,
			MessageID:   row.MessageID,
			SessionID:   row.SessionID,
			Prompt:      row.Prompt,
			RawResponse: row.RawResponse,
			Created:     row.Created,
		})
	}
	return out, nil
}

func GetPromptSnapshotByMessageID(db *gorm.DB, messageID string) (*PromptSnapshotResult, error) {
	if err := ensureSessionDataSchema(db); err != nil {
		return nil, err
	}
	var row models.MessagePromptSnapshot
	if err := db.Where("message_id = ?", strings.TrimSpace(messageID)).First(&row).Error; err != nil {
		return nil, err
	}
	return &PromptSnapshotResult{
		MessageID:   row.MessageID,
		SessionID:   row.SessionID,
		Prompt:      row.Prompt,
		RawResponse: row.RawResponse,
	}, nil
}

func DeletePromptSnapshotByMessageID(db *gorm.DB, messageID string) error {
	if db == nil || strings.TrimSpace(messageID) == "" {
		return nil
	}
	if err := ensureSessionDataSchema(db); err != nil {
		return err
	}
	return db.Delete(&models.MessagePromptSnapshot{}, "message_id = ?", strings.TrimSpace(messageID)).Error
}

func DeletePromptSnapshotsBySessionID(db *gorm.DB, sessionID string) error {
	if db == nil || strings.TrimSpace(sessionID) == "" {
		return nil
	}
	if err := ensureSessionDataSchema(db); err != nil {
		return err
	}
	return db.Delete(&models.MessagePromptSnapshot{}, "session_id = ?", strings.TrimSpace(sessionID)).Error
}
