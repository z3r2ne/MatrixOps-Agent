package storage

import (
	"fmt"
	"strings"

	"matrixops-agent/types"
	"pkgs/db/models"

	"gorm.io/gorm"
)

type RetryUserMessageResult struct {
	Message *types.WithParts
	Text    string
	Parts   []*types.Part
}

func RetryFromUserMessage(db *gorm.DB, sessionID, messageID string) (*RetryUserMessageResult, error) {
	if db == nil {
		return nil, fmt.Errorf("db is nil")
	}
	sessionID = strings.TrimSpace(sessionID)
	messageID = strings.TrimSpace(messageID)
	if sessionID == "" || messageID == "" {
		return nil, fmt.Errorf("sessionID and messageID are required")
	}

	wp, err := GetMessageWithParts(db, messageID)
	if err != nil {
		return nil, err
	}
	if wp == nil || wp.Info == nil {
		return nil, fmt.Errorf("message not found")
	}
	if wp.Info.SessionID != sessionID {
		return nil, fmt.Errorf("message does not belong to session")
	}
	if wp.Info.Role != types.RoleUser {
		return nil, fmt.Errorf("message is not a user message")
	}

	text := extractUserRetryText(wp.Parts)
	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("user message text is empty")
	}

	err = db.Transaction(func(tx *gorm.DB) error {
		var target models.Message
		if err := tx.Where("id = ? AND session_id = ?", messageID, sessionID).First(&target).Error; err != nil {
			return err
		}

		var messageIDs []string
		if err := tx.Model(&models.Message{}).
			Where("session_id = ? AND created >= ?", sessionID, target.Created).
			Order("created ASC, id ASC").
			Pluck("id", &messageIDs).Error; err != nil {
			return err
		}

		var laterUserCount int64
		if err := tx.Model(&models.Message{}).
			Where("session_id = ? AND role = ? AND created > ?", sessionID, string(types.RoleUser), target.Created).
			Count(&laterUserCount).Error; err != nil {
			return err
		}
		if laterUserCount > 0 {
			return fmt.Errorf("only the last user message can be retried")
		}

		if len(messageIDs) == 0 {
			return nil
		}

		if err := tx.Where("session_id = ? AND message_id IN ?", sessionID, messageIDs).Delete(&models.Part{}).Error; err != nil {
			return err
		}
		if err := tx.Where("session_id = ? AND id IN ?", sessionID, messageIDs).Delete(&models.Message{}).Error; err != nil {
			return err
		}
		if err := tx.Where("session_id = ? AND message_id IN ?", sessionID, messageIDs).Delete(&models.MessagePromptSnapshot{}).Error; err != nil {
			return err
		}
		if err := tx.Where("session_id = ? AND message_id IN ?", sessionID, messageIDs).Delete(&models.MessageCodeSnapshot{}).Error; err != nil {
			return err
		}
		if err := tx.Where("session_id = ? AND created >= ?", sessionID, target.Created).Delete(&models.MemoryEntry{}).Error; err != nil {
			return err
		}
		return UpdateSessionTokens(tx, sessionID, nil)
	})
	if err != nil {
		return nil, err
	}

	return &RetryUserMessageResult{
		Message: wp,
		Text:    text,
		Parts:   cloneRetryableUserParts(wp.Parts),
	}, nil
}

func extractUserRetryText(parts []*types.Part) string {
	if len(parts) == 0 {
		return ""
	}
	lines := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == nil || part.Type != types.PartTypeText || part.Ignored {
			continue
		}
		if strings.TrimSpace(part.Text) != "" {
			lines = append(lines, part.Text)
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func cloneRetryableUserParts(parts []*types.Part) []*types.Part {
	if len(parts) == 0 {
		return nil
	}
	out := make([]*types.Part, 0, len(parts))
	for _, part := range parts {
		if part == nil || part.Type == types.PartTypeText {
			continue
		}
		copied := *part
		copied.ID = ""
		copied.MessageID = ""
		copied.SessionID = ""
		copied.Time = nil
		out = append(out, &copied)
	}
	return out
}
