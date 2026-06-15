package session

import (
	"fmt"
	"time"

	"matrixops.local/core_agent/streamtypes"
	"pkgs/db/models"
)

const (
	taskMessageQueueMetadataSourceEmptyStreamRetry = models.TaskMessageQueueSourceEmptyStreamRetry
	emptyStreamRetryContinueContent                  = "继续"
)

// handleEmptyStreamRetry enqueues a supplement system message so the next agent
// loop step consumes it via consumeSupplementsAfterStep (visible in the UI).
func (r *AgentRunner) handleEmptyStreamRetry(callErr error) error {
	if r == nil || callErr == nil {
		return nil
	}
	if !streamtypes.IsEmptyStreamOutputError(callErr) {
		return nil
	}
	if r.db == nil || r.task == nil || r.task.ID == 0 {
		return nil
	}
	if r.messageQueue == nil {
		return nil
	}

	queue, err := r.messageQueue.Load()
	if err != nil {
		return fmt.Errorf("load task queue: %w", err)
	}
	if queueHasEmptyStreamRetrySupplement(queue) {
		return nil
	}

	item := models.TaskMessageQueueItem{
		ID:         fmt.Sprintf("queue-empty-stream-retry-%d", time.Now().UnixNano()),
		Type:       models.TaskMessageQueueTypeSystem,
		Content:    emptyStreamRetryContinueContent,
		Source:     taskMessageQueueMetadataSourceEmptyStreamRetry,
		Supplement: true,
		CreatedAt:  time.Now().UnixMilli(),
	}
	if err := r.messageQueue.Prepend(item); err != nil {
		return fmt.Errorf("prepend empty stream retry queue item: %w", err)
	}
	return nil
}

func queueHasEmptyStreamRetrySupplement(queue []models.TaskMessageQueueItem) bool {
	if len(queue) == 0 {
		return false
	}
	head := queue[0]
	if !head.IsSystem() || !head.Supplement {
		return false
	}
	return head.ResolvedSource() == taskMessageQueueMetadataSourceEmptyStreamRetry
}
