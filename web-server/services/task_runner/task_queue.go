package task_runner

import (
	"fmt"
	"strings"
	"time"

	"matrixops-agent/types"
	"pkgs/db/models"
	"pkgs/taskqueue"
)

func hasEnqueueableUserInput(config *TaskRuntimeConfig) bool {
	if config == nil {
		return false
	}
	if strings.TrimSpace(config.Content) != "" {
		return true
	}
	for _, part := range config.InputParts {
		if part == nil {
			continue
		}
		if strings.TrimSpace(part.Type) == "file" && strings.TrimSpace(part.URL) != "" {
			return true
		}
	}
	return false
}

func enqueueTaskUserMessage(config *TaskRuntimeConfig) error {
	if config == nil {
		return fmt.Errorf("task runtime config is nil")
	}
	if config.db == nil {
		return fmt.Errorf("run task db is nil")
	}
	if config.wsHub == nil {
		return fmt.Errorf("wsHub is nil")
	}
	if config.TaskID == 0 {
		return fmt.Errorf("task id is required")
	}
	if !hasEnqueueableUserInput(config) {
		return fmt.Errorf("消息内容为空")
	}

	item := buildTaskUserQueueItem(config)
	queue := taskqueue.New(config.db, config.TaskID, config.wsHub.BroadcastTaskQueue)
	if err := queue.Append(item); err != nil {
		return err
	}
	return TryAutoRunTaskQueue(config.TaskID, WithDB(config.db), WithWSHub(config.wsHub))
}

func buildTaskUserQueueItem(config *TaskRuntimeConfig) models.TaskMessageQueueItem {
	itemID := strings.TrimSpace(config.QueueItemID)
	if itemID == "" {
		itemID = fmt.Sprintf("queue-%d", time.Now().UnixNano())
	}
	return models.TaskMessageQueueItem{
		ID:        itemID,
		Type:      models.TaskMessageQueueTypeUser,
		Content:   strings.TrimSpace(config.Content),
		Source:    queueSourceFromConfig(config),
		Parts:     inputPartsToTaskInputParts(config.InputParts),
		CreatedAt: time.Now().UnixMilli(),
	}
}

func inputPartsToTaskInputParts(parts []*types.Part) []models.TaskInputPart {
	if len(parts) == 0 {
		return nil
	}
	out := make([]models.TaskInputPart, 0, len(parts))
	for _, part := range parts {
		if part == nil {
			continue
		}
		switch strings.TrimSpace(part.Type) {
		case types.PartTypeText:
			text := strings.TrimSpace(part.Text)
			if text == "" {
				continue
			}
			out = append(out, models.TaskInputPart{Type: types.PartTypeText, Text: text})
		case "file":
			if strings.TrimSpace(part.Path) == "" && strings.TrimSpace(part.URL) == "" {
				continue
			}
			out = append(out, models.TaskInputPart{
				Type:     part.Type,
				Path:     strings.TrimSpace(part.Path),
				URL:      strings.TrimSpace(part.URL),
				Mime:     part.Mime,
				Filename: part.Filename,
				InputSource: strings.TrimSpace(part.InputSource),
			})
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
