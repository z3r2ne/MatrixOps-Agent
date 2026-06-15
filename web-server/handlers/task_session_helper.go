package handlers

import (
	"strings"

	database "pkgs/db"
	"pkgs/db/models"

	"gorm.io/gorm"
)

// resolveTaskSessionID finds the best available session ID for a task and
// persists it back to the task record when it can be recovered from executions.
func resolveTaskSessionID(db *gorm.DB, task *models.Task) string {
	if task == nil {
		return ""
	}

	sessionID := strings.TrimSpace(task.SessionID)
	if sessionID != "" {
		return sessionID
	}

	executions, err := database.GetExecutionsByTaskID(db, task.ID, 1)
	if err != nil || len(executions) == 0 {
		return ""
	}

	sessionID = strings.TrimSpace(executions[0].AgentSessionID)
	if sessionID == "" {
		return ""
	}

	task.SessionID = sessionID
	_ = database.UpdateTaskSessionID(db, task.ID, sessionID)
	return sessionID
}
