package session

import (
	"fmt"
	"time"

	coreagent "matrixops.local/core_agent"
	"pkgs/db/models"
)

const taskMessageQueueMetadataSourceSilentToolWatchdog = models.TaskMessageQueueSourceSilentToolWatchdog

func (r *AgentRunner) buildSilentToolStreakHandler(runtimeConfig *RuntimeConfig) func(state *coreagent.RunState, count int) error {
	return func(_ *coreagent.RunState, count int) error {
		return r.handleSilentToolStreak(runtimeConfig, count)
	}
}

func (r *AgentRunner) handleSilentToolStreak(runtimeConfig *RuntimeConfig, count int) error {
	if r == nil || r.db == nil || r.task == nil || r.task.ID == 0 {
		return nil
	}
	message := coreagent.FormatSilentToolWatchdogPrompt(count)

	if r.messageQueue == nil {
		return nil
	}
	queue, err := r.messageQueue.Load()
	if err != nil {
		return fmt.Errorf("load task queue: %w", err)
	}
	if queueHasSilentToolWatchdogSupplement(queue) {
		return nil
	}

	item := models.TaskMessageQueueItem{
		ID:         fmt.Sprintf("queue-silent-tool-watchdog-%d", time.Now().UnixNano()),
		Type:       models.TaskMessageQueueTypeSystem,
		Content:    message,
		Source:     taskMessageQueueMetadataSourceSilentToolWatchdog,
		Supplement: true,
		Metadata: map[string]interface{}{
			"silentToolCount": count,
		},
		CreatedAt: time.Now().UnixMilli(),
	}
	if err := r.messageQueue.Prepend(item); err != nil {
		return fmt.Errorf("prepend silent tool watchdog queue item: %w", err)
	}
	return nil
}

func queueHasSilentToolWatchdogSupplement(queue []models.TaskMessageQueueItem) bool {
	if len(queue) == 0 {
		return false
	}
	head := queue[0]
	if !head.IsSystem() || !head.Supplement {
		return false
	}
	return head.ResolvedSource() == taskMessageQueueMetadataSourceSilentToolWatchdog
}
