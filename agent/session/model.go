package session

import (
	"pkgs/db/models"
	"pkgs/db/storage"

	"gorm.io/gorm"
)

func LastModel(db *gorm.DB, sessionID string, modelSettings *models.ModelSettings) (string, error) {

	history, err := storage.GetSessionMessageParts(db, sessionID)
	if err != nil {
		return "", err
	}
	for i := len(history) - 1; i >= 0; i-- {
		msg := history[i].Info
		if msg.Role == RoleUser && msg.Model != "" {
			return msg.Provider + "/" + msg.Model, nil
		}
	}
	if modelSettings != nil {
		return modelSettings.Name, nil
	}
	return "", nil
}
