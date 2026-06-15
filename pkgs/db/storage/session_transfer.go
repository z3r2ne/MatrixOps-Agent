package storage

import (
	"fmt"
	"strings"
	"time"

	"matrixops-agent/types"
	"pkgs/db/models"

	"gorm.io/gorm"
)

// ReplaceSessionMessagesWithParts 用导入的消息完整覆盖指定 session 的消息与部件。
func ReplaceSessionMessagesWithParts(db *gorm.DB, sessionID string, messages []*types.WithParts) error {
	if err := ensureSessionDataSchema(db); err != nil {
		return err
	}

	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("session_id = ?", sessionID).Delete(&models.Part{}).Error; err != nil {
			return err
		}
		if err := tx.Where("session_id = ?", sessionID).Delete(&models.Message{}).Error; err != nil {
			return err
		}

		now := time.Now().UnixMilli()
		for index, message := range messages {
			if message == nil || message.Info == nil {
				continue
			}
			if strings.TrimSpace(message.Info.ID) == "" {
				return fmt.Errorf("message #%d id is empty", index+1)
			}

			info := *message.Info
			info.SessionID = sessionID
			if info.Time.Created == 0 {
				info.Time.Created = now + int64(index)
			}

			modelMessage := models.MessageToModel(&info)
			if modelMessage == nil {
				continue
			}
			modelMessage.SessionID = sessionID
			modelMessage.Name = info.Worker

			if err := tx.Create(modelMessage).Error; err != nil {
				return err
			}

			for partIndex, part := range message.Parts {
				if part == nil {
					continue
				}
				if strings.TrimSpace(part.ID) == "" {
					return fmt.Errorf("message #%d part #%d id is empty", index+1, partIndex+1)
				}

				partCopy := *part
				partCopy.SessionID = sessionID
				partCopy.MessageID = info.ID
				if partCopy.Time == nil {
					partCopy.Time = &types.PartTime{Created: info.Time.Created}
				} else if partCopy.Time.Created == 0 {
					timeCopy := *partCopy.Time
					timeCopy.Created = info.Time.Created
					partCopy.Time = &timeCopy
				}

				modelPart := models.PartToModel(&partCopy)
				if modelPart == nil {
					continue
				}
				modelPart.SessionID = sessionID
				modelPart.MessageID = info.ID

				if err := tx.Create(modelPart).Error; err != nil {
					return err
				}
			}
		}

		return nil
	})
}
