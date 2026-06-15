package storage

import (
	"matrixops-agent/types"
	"pkgs/db/models"

	"gorm.io/gorm"
)

var lightMessageSelectColumns = []string{
	"id",
	"session_id",
	"role",
	"message_kind",
	"message_origin",
	"parent_id",
	"name",
	"occupation",
	"provider_id",
	"model_id",
	"system",
	"variant",
	"finish",
	"cost",
	"state",
	"snapshot",
	"created",
	"completed",
	"tools",
	"summary",
	"tokens",
	"error",
	"path",
}

var lightPartSelectColumns = []string{
	"id",
	"message_id",
	"session_id",
	"type",
	"text",
	"reasoning",
	"synthetic",
	"ignored",
	"snapshot",
	"hash",
	"mime",
	"filename",
	"url",
	"agent_name",
	"auto",
	"description",
	"subagent",
	"command",
	"attempt",
	"reason",
	"cost",
	"time_start",
	"time_end",
	"time_created",
	"time_compacted",
	"tool",
	"metadata",
	"files",
	"source",
	"model",
	"error",
	"tokens",
}

func GetMessage(db *gorm.DB, messageID string) (*types.MessageInfo, error) {
	var message models.Message
	if err := db.Where("id = ?", messageID).First(&message).Error; err != nil {
		return nil, err
	}
	return models.MessageFromModel(&message), nil
}

func GetMessageWithParts(db *gorm.DB, messageID string) (*types.WithParts, error) {
	if err := ensureSessionDataSchema(db); err != nil {
		return nil, err
	}
	message, err := GetMessage(db, messageID)
	if err != nil {
		return nil, err
	}
	parts, err := GetPartsByMessageID(db, messageID)
	if err != nil {
		return nil, err
	}
	partsInfo := []*types.Part{}
	for _, part := range parts {
		partsInfo = append(partsInfo, part)
	}
	return &types.WithParts{
		Info:  message,
		Parts: partsInfo,
	}, nil
}

func GetMessageWithPartsLight(db *gorm.DB, messageID string) (*types.WithParts, error) {
	if err := ensureSessionDataSchema(db); err != nil {
		return nil, err
	}

	var messageModel models.Message
	if err := db.Select(lightMessageSelectColumns).
		Where("id = ?", messageID).
		First(&messageModel).Error; err != nil {
		return nil, err
	}

	var partModels []*models.Part
	if err := db.Select(lightPartSelectColumns).
		Where("message_id = ?", messageID).
		Order("time_created ASC, id ASC").
		Find(&partModels).Error; err != nil {
		return nil, err
	}

	parts := make([]*types.Part, 0, len(partModels))
	for _, partModel := range partModels {
		if partModel == nil {
			continue
		}
		parts = append(parts, models.PartFromModel(partModel))
	}

	return &types.WithParts{
		Info:  models.MessageFromModel(&messageModel),
		Parts: parts,
	}, nil
}

func GetMessageWithPartsBySessionID(db *gorm.DB, sessionID string) ([]*types.WithParts, error) {
	if err := ensureSessionDataSchema(db); err != nil {
		return nil, err
	}

	var messages []*models.Message
	if err := db.Where("session_id = ?", sessionID).Order("created ASC").Find(&messages).Error; err != nil {
		return nil, err
	}

	var messagesWithParts []*types.WithParts
	for _, message := range messages {
		messageWithParts, err := GetMessageWithParts(db, message.ID)
		if err != nil {
			return nil, err
		}
		messagesWithParts = append(messagesWithParts, messageWithParts)
	}
	return messagesWithParts, nil
}

func GetMessageWithPartsBySessionIDLight(db *gorm.DB, sessionID string) ([]*types.WithParts, error) {
	if err := ensureSessionDataSchema(db); err != nil {
		return nil, err
	}

	var messageModels []*models.Message
	if err := db.Select(lightMessageSelectColumns).
		Where("session_id = ?", sessionID).
		Order("created ASC, id ASC").
		Find(&messageModels).Error; err != nil {
		return nil, err
	}

	messageOrder := make([]string, 0, len(messageModels))
	messageMap := make(map[string]*types.WithParts, len(messageModels))
	for _, messageModel := range messageModels {
		if messageModel == nil {
			continue
		}
		message := models.MessageFromModel(messageModel)
		messageOrder = append(messageOrder, messageModel.ID)
		messageMap[messageModel.ID] = &types.WithParts{
			Info:  message,
			Parts: []*types.Part{},
		}
	}

	if len(messageOrder) == 0 {
		return []*types.WithParts{}, nil
	}

	var partModels []*models.Part
	if err := db.Select(lightPartSelectColumns).
		Where("session_id = ?", sessionID).
		Order("time_created ASC, id ASC").
		Find(&partModels).Error; err != nil {
		return nil, err
	}

	for _, partModel := range partModels {
		if partModel == nil {
			continue
		}
		message := messageMap[partModel.MessageID]
		if message == nil {
			continue
		}
		message.Parts = append(message.Parts, models.PartFromModel(partModel))
	}

	results := make([]*types.WithParts, 0, len(messageOrder))
	for _, messageID := range messageOrder {
		if message := messageMap[messageID]; message != nil {
			results = append(results, message)
		}
	}
	return results, nil
}

func GetMessageWithPartsBySessionIDPageLight(db *gorm.DB, sessionID string, limit int, beforeMessageID string) ([]*types.WithParts, bool, string, error) {
	if err := ensureSessionDataSchema(db); err != nil {
		return nil, false, "", err
	}

	if limit <= 0 {
		limit = 100
	}

	query := db.Select(lightMessageSelectColumns).
		Where("session_id = ?", sessionID)

	if beforeMessageID != "" {
		var cursor models.Message
		if err := db.Select("id", "created").
			Where("id = ? AND session_id = ?", beforeMessageID, sessionID).
			First(&cursor).Error; err != nil {
			return nil, false, "", err
		}
		query = query.Where("(created < ?) OR (created = ? AND id < ?)", cursor.Created, cursor.Created, cursor.ID)
	}

	var messageModels []*models.Message
	if err := query.Order("created DESC, id DESC").Limit(limit + 1).Find(&messageModels).Error; err != nil {
		return nil, false, "", err
	}

	hasMore := len(messageModels) > limit
	if hasMore {
		messageModels = messageModels[:limit]
	}

	if len(messageModels) == 0 {
		return []*types.WithParts{}, false, "", nil
	}

	// reverse to ascending for rendering
	for left, right := 0, len(messageModels)-1; left < right; left, right = left+1, right-1 {
		messageModels[left], messageModels[right] = messageModels[right], messageModels[left]
	}

	messageOrder := make([]string, 0, len(messageModels))
	messageMap := make(map[string]*types.WithParts, len(messageModels))
	messageIDs := make([]string, 0, len(messageModels))
	for _, messageModel := range messageModels {
		if messageModel == nil {
			continue
		}
		message := models.MessageFromModel(messageModel)
		messageOrder = append(messageOrder, messageModel.ID)
		messageIDs = append(messageIDs, messageModel.ID)
		messageMap[messageModel.ID] = &types.WithParts{
			Info:  message,
			Parts: []*types.Part{},
		}
	}

	var partModels []*models.Part
	if err := db.Select(lightPartSelectColumns).
		Where("session_id = ? AND message_id IN ?", sessionID, messageIDs).
		Order("time_created ASC, id ASC").
		Find(&partModels).Error; err != nil {
		return nil, false, "", err
	}

	for _, partModel := range partModels {
		if partModel == nil {
			continue
		}
		message := messageMap[partModel.MessageID]
		if message == nil {
			continue
		}
		message.Parts = append(message.Parts, models.PartFromModel(partModel))
	}

	results := make([]*types.WithParts, 0, len(messageOrder))
	for _, messageID := range messageOrder {
		if message := messageMap[messageID]; message != nil {
			results = append(results, message)
		}
	}

	nextBeforeMessageID := ""
	if hasMore && len(results) > 0 {
		nextBeforeMessageID = results[0].Info.ID
	}

	return results, hasMore, nextBeforeMessageID, nil
}

func GetLatestPromptBySessionID(db *gorm.DB, sessionID string) (messageID string, partID string, prompt string, rawResponse string, err error) {
	snapshot, err := GetLatestPromptSnapshotBySessionID(db, sessionID)
	if err != nil {
		return "", "", "", "", err
	}
	if snapshot == nil {
		return "", "", "", "", gorm.ErrRecordNotFound
	}
	return snapshot.MessageID, "", snapshot.Prompt, snapshot.RawResponse, nil
}

func GetPromptByMessageID(db *gorm.DB, messageID string) (sessionID string, prompt string, rawResponse string, err error) {
	snapshot, err := GetPromptSnapshotByMessageID(db, messageID)
	if err != nil {
		return "", "", "", err
	}
	if snapshot == nil {
		return "", "", "", gorm.ErrRecordNotFound
	}
	return snapshot.SessionID, snapshot.Prompt, snapshot.RawResponse, nil
}
