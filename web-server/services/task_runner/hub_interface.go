package task_runner

import (
	"matrixops/types"
	"pkgs/db/models"
)

// WSHub 定义 WebSocket Hub 需要的接口，避免循环导入
type WSHub interface {
	// BroadcastToTask 向订阅了某任务的客户端广播消息
	BroadcastToTask(taskID uint, msg types.WSOutgoingMessage)

	// BroadcastTaskMessage 广播任务消息
	BroadcastTaskMessage(taskID uint, message *models.TaskMessage)

	// BroadcastNormalizedEntry 广播规范化条目
	BroadcastNormalizedEntry(taskID uint, entry *models.NormalizedEntry)

	// BroadcastTaskStatus 广播任务状态变化
	BroadcastTaskStatus(taskID uint, status models.TaskStatus, sessionID string, msg string)

	// BroadcastIsWorking 广播任务开始工作
	BroadcastIsWorking(taskID uint)

	// BroadcastIsNotWorking 广播任务停止工作
	BroadcastIsNotWorking(taskID uint)

	// BroadcastError 广播错误
	BroadcastError(taskID uint, err string)

	// BroadcastSessionTitle 广播会话标题
	BroadcastSessionTitle(taskID uint, title string)

	// BroadcastRetry 广播重试
	BroadcastRetry(taskID uint)

	// BroadcastWaitUserInput 广播等待用户输入
	BroadcastWaitUserInput(taskID uint, id string, ack func(result map[string]interface{}), question map[string]interface{})

	// BroadcastTaskQueue 广播任务消息队列变更
	BroadcastTaskQueue(taskID uint, queue []models.TaskMessageQueueItem)

	// BroadcastTaskPlan 广播任务计划变更
	BroadcastTaskPlan(taskID uint, plan any)
}
