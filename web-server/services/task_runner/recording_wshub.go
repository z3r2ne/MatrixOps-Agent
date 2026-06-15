package task_runner

import (
	"matrixops/types"
	"pkgs/db/models"
)

// RecordingWSHub 记录 WS 广播，供单测断言队列与状态变更。
type RecordingWSHub struct {
	Messages   []types.WSOutgoingMessage
	TaskQueues map[uint][]models.TaskMessageQueueItem
	Errors     []string
}

func NewRecordingWSHub() *RecordingWSHub {
	return &RecordingWSHub{
		TaskQueues: make(map[uint][]models.TaskMessageQueueItem),
	}
}

var _ WSHub = (*RecordingWSHub)(nil)

func (h *RecordingWSHub) BroadcastToTask(taskID uint, msg types.WSOutgoingMessage) {
	h.Messages = append(h.Messages, msg)
}

func (h *RecordingWSHub) BroadcastTaskMessage(taskID uint, message *models.TaskMessage) {}

func (h *RecordingWSHub) BroadcastNormalizedEntry(taskID uint, entry *models.NormalizedEntry) {}

func (h *RecordingWSHub) BroadcastTaskStatus(taskID uint, status models.TaskStatus, sessionID string, msg string) {}

func (h *RecordingWSHub) BroadcastIsWorking(taskID uint) {}

func (h *RecordingWSHub) BroadcastIsNotWorking(taskID uint) {}

func (h *RecordingWSHub) BroadcastError(taskID uint, err string) {
	h.Errors = append(h.Errors, err)
}

func (h *RecordingWSHub) BroadcastSessionTitle(taskID uint, title string) {}

func (h *RecordingWSHub) BroadcastRetry(taskID uint) {}

func (h *RecordingWSHub) BroadcastWaitUserInput(taskID uint, id string, ack func(result map[string]interface{}), question map[string]interface{}) {
}

func (h *RecordingWSHub) BroadcastTaskQueue(taskID uint, queue []models.TaskMessageQueueItem) {
	h.TaskQueues[taskID] = append([]models.TaskMessageQueueItem(nil), queue...)
}

func (h *RecordingWSHub) BroadcastTaskPlan(taskID uint, plan any) {}

func (h *RecordingWSHub) LastTaskQueue(taskID uint) []models.TaskMessageQueueItem {
	return append([]models.TaskMessageQueueItem(nil), h.TaskQueues[taskID]...)
}
