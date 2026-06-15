package session

import (
	"fmt"
	"strings"
	"time"

	coreagent "matrixops.local/core_agent"
	"pkgs/db/models"
)

const taskMessageQueueMetadataSourceToolRepeatWatchdog = models.TaskMessageQueueSourceToolRepeatWatchdog

func (r *AgentRunner) buildRepeatedToolCallHandler(runtimeConfig *RuntimeConfig) func(state *coreagent.RunState, toolName string, args map[string]interface{}, count int) error {
	return func(_ *coreagent.RunState, toolName string, args map[string]interface{}, count int) error {
		return r.handleRepeatedToolCall(runtimeConfig, toolName, args, count)
	}
}

func (r *AgentRunner) handleRepeatedToolCall(runtimeConfig *RuntimeConfig, toolName string, args map[string]interface{}, count int) error {
	if r == nil || r.db == nil || r.task == nil || r.task.ID == 0 {
		return nil
	}
	message := coreagent.FormatRepeatedToolCallWarning(toolName, count)
	fingerprint := coreagent.ToolCallFingerprint(toolName, args)

	if r.messageQueue == nil {
		return nil
	}
	queue, err := r.messageQueue.Load()
	if err != nil {
		return fmt.Errorf("load task queue: %w", err)
	}
	if queueHasToolRepeatWatchdogWarning(queue, fingerprint, count) {
		return nil
	}

	item := models.TaskMessageQueueItem{
		ID:         fmt.Sprintf("queue-watchdog-%d", time.Now().UnixNano()),
		Type:       models.TaskMessageQueueTypeSystem,
		Content:    message,
		Source:     taskMessageQueueMetadataSourceToolRepeatWatchdog,
		Supplement: true,
		Metadata: map[string]interface{}{
			"toolName":    strings.TrimSpace(toolName),
			"repeatCount": count,
			"fingerprint": fingerprint,
		},
		CreatedAt: time.Now().UnixMilli(),
	}
	if err := r.messageQueue.Prepend(item); err != nil {
		return fmt.Errorf("prepend watchdog queue item: %w", err)
	}
	return nil
}

func queueHasToolRepeatWatchdogWarning(queue []models.TaskMessageQueueItem, fingerprint string, repeatCount int) bool {
	if len(queue) == 0 || strings.TrimSpace(fingerprint) == "" || repeatCount <= 0 {
		return false
	}
	head := queue[0]
	if !head.IsSystem() {
		return false
	}
	if head.Metadata == nil {
		return false
	}
	source := head.ResolvedSource()
	if source != taskMessageQueueMetadataSourceToolRepeatWatchdog {
		return false
	}
	existing, _ := head.Metadata["fingerprint"].(string)
	if strings.TrimSpace(existing) != strings.TrimSpace(fingerprint) {
		return false
	}
	existingCount := metadataInt(head.Metadata["repeatCount"])
	return existingCount == repeatCount
}

func metadataInt(value interface{}) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case float32:
		return int(typed)
	case float64:
		return int(typed)
	default:
		return 0
	}
}
