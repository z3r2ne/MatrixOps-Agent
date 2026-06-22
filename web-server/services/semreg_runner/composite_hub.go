package semreg_runner

import (
	"matrixops/types"
	"pkgs/db/models"

	taskrunner "web-server/services/task_runner"
)

type compositeWSHub struct {
	collector *TraceCollector
	inner     taskrunner.WSHub
}

func newCompositeWSHub(collector *TraceCollector, inner taskrunner.WSHub) taskrunner.WSHub {
	if collector == nil {
		return inner
	}
	if inner == nil {
		return collector
	}
	return &compositeWSHub{collector: collector, inner: inner}
}

func (h *compositeWSHub) BroadcastToTask(taskID uint, msg types.WSOutgoingMessage) {
	h.collector.BroadcastToTask(taskID, msg)
	h.inner.BroadcastToTask(taskID, msg)
}

func (h *compositeWSHub) BroadcastTaskMessage(taskID uint, message *models.TaskMessage) {
	h.collector.BroadcastTaskMessage(taskID, message)
	h.inner.BroadcastTaskMessage(taskID, message)
}

func (h *compositeWSHub) BroadcastNormalizedEntry(taskID uint, entry *models.NormalizedEntry) {
	h.collector.BroadcastNormalizedEntry(taskID, entry)
	h.inner.BroadcastNormalizedEntry(taskID, entry)
}

func (h *compositeWSHub) BroadcastTaskStatus(taskID uint, status models.TaskStatus, sessionID string, msg string) {
	h.collector.BroadcastTaskStatus(taskID, status, sessionID, msg)
	h.inner.BroadcastTaskStatus(taskID, status, sessionID, msg)
}

func (h *compositeWSHub) BroadcastIsWorking(taskID uint) {
	h.collector.BroadcastIsWorking(taskID)
	h.inner.BroadcastIsWorking(taskID)
}

func (h *compositeWSHub) BroadcastIsNotWorking(taskID uint) {
	h.collector.BroadcastIsNotWorking(taskID)
	h.inner.BroadcastIsNotWorking(taskID)
}

func (h *compositeWSHub) BroadcastError(taskID uint, err string) {
	h.collector.BroadcastError(taskID, err)
	h.inner.BroadcastError(taskID, err)
}

func (h *compositeWSHub) BroadcastSessionTitle(taskID uint, title string) {
	h.collector.BroadcastSessionTitle(taskID, title)
	h.inner.BroadcastSessionTitle(taskID, title)
}

func (h *compositeWSHub) BroadcastRetry(taskID uint) {
	h.collector.BroadcastRetry(taskID)
	h.inner.BroadcastRetry(taskID)
}

func (h *compositeWSHub) BroadcastWaitUserInput(taskID uint, id string, ack func(map[string]interface{}), question map[string]interface{}) {
	h.collector.BroadcastWaitUserInput(taskID, id, ack, question)
	h.inner.BroadcastWaitUserInput(taskID, id, ack, question)
}

func (h *compositeWSHub) BroadcastTaskQueue(taskID uint, queue []models.TaskMessageQueueItem) {
	h.collector.BroadcastTaskQueue(taskID, queue)
	h.inner.BroadcastTaskQueue(taskID, queue)
}

func (h *compositeWSHub) BroadcastTaskPlan(taskID uint, plan any) {
	h.collector.BroadcastTaskPlan(taskID, plan)
	h.inner.BroadcastTaskPlan(taskID, plan)
}
