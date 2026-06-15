package task_runner

import (
	"strings"

	"pkgs/db/models"
)

const (
	TaskInputSourceWeChat   = "wechat"
	TaskInputSourceFrontend = "frontend"
)

func inputSourceForwardsToWeChat(source string) bool {
	return strings.TrimSpace(source) == TaskInputSourceWeChat
}

func QueueItemInputSource(item *models.TaskMessageQueueItem) string {
	if item == nil {
		return TaskInputSourceFrontend
	}
	if strings.HasPrefix(strings.TrimSpace(item.ID), "wechat-") {
		return TaskInputSourceWeChat
	}
	source := strings.TrimSpace(item.ResolvedSource())
	if source == "" {
		return TaskInputSourceFrontend
	}
	return source
}

func queueSourceFromConfig(config *TaskRuntimeConfig) string {
	source := strings.TrimSpace(config.InputSource)
	if source == "" {
		return models.TaskMessageQueueSourceFrontend
	}
	return source
}
