package session

import (
	"fmt"
	"strings"
	"time"

	coreagent "matrixops.local/core_agent"
	"pkgs/db/models"
)

const taskMessageQueueMetadataSourceStallWatchdog = models.TaskMessageQueueSourceStallWatchdog

func (r *AgentRunner) buildStallWatchdogToolCancelledHandler(runtimeConfig *RuntimeConfig) func(state *coreagent.RunState, toolName, callID, reason string, elapsed time.Duration) error {
	return func(_ *coreagent.RunState, toolName, callID, reason string, elapsed time.Duration) error {
		return r.handleStallWatchdogToolCancelled(runtimeConfig, toolName, callID, reason, elapsed)
	}
}

func (r *AgentRunner) handleStallWatchdogToolCancelled(runtimeConfig *RuntimeConfig, toolName, callID, reason string, elapsed time.Duration) error {
	if r == nil || r.db == nil || r.task == nil || r.task.ID == 0 {
		return nil
	}
	callID = strings.TrimSpace(callID)
	message := coreagent.FormatStallWatchdogToolCancelledWarning(toolName, reason, elapsed)

	if r.messageQueue == nil {
		return nil
	}
	queue, err := r.messageQueue.Load()
	if err != nil {
		return fmt.Errorf("load task queue: %w", err)
	}
	if queueHasStallWatchdogCancellation(queue, callID) {
		return nil
	}

	item := models.TaskMessageQueueItem{
		ID:         fmt.Sprintf("queue-stall-watchdog-%d", time.Now().UnixNano()),
		Type:       models.TaskMessageQueueTypeSystem,
		Content:    message,
		Source:     taskMessageQueueMetadataSourceStallWatchdog,
		Supplement: true,
		Metadata: map[string]interface{}{
			"toolName": strings.TrimSpace(toolName),
			"callID":   callID,
			"reason":   strings.TrimSpace(reason),
		},
		CreatedAt: time.Now().UnixMilli(),
	}
	if err := r.messageQueue.Prepend(item); err != nil {
		return fmt.Errorf("prepend stall watchdog queue item: %w", err)
	}
	return nil
}

func queueHasStallWatchdogCancellation(queue []models.TaskMessageQueueItem, callID string) bool {
	callID = strings.TrimSpace(callID)
	if len(queue) == 0 || callID == "" {
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
	if source != taskMessageQueueMetadataSourceStallWatchdog {
		return false
	}
	existing, _ := head.Metadata["callID"].(string)
	return strings.TrimSpace(existing) == callID
}
