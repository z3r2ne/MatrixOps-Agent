package task_runner

import (
	"matrixops-agent/types"
	"pkgs/db/models"
)

func QueueItemMessageKind(item *models.TaskMessageQueueItem) string {
	if item != nil && (item.IsSystem() || item.IsAppend()) {
		return types.MessageKindSystem
	}
	return types.MessageKindUser
}

func QueueItemMessageOrigin(item *models.TaskMessageQueueItem) string {
	if item == nil {
		return ""
	}
	return item.ResolvedSource()
}
