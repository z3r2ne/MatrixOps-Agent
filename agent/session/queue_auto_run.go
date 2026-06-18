package session

import (
	database "pkgs/db"
	"pkgs/db/models"
)

// prependSupplementQueueItem 将补充消息插入队首，并在任务已结束时尝试自动启动以消费队列。
func (r *AgentRunner) prependSupplementQueueItem(item models.TaskMessageQueueItem) error {
	if r == nil || r.messageQueue == nil {
		return nil
	}
	if err := r.messageQueue.Prepend(item); err != nil {
		return err
	}
	if item.IsSupplement() && r.db != nil && r.task != nil && r.task.ID > 0 {
		_ = database.SetTaskMessageQueueAutoSend(r.db, r.task.ID, true)
	}
	if r.queueAutoRun != nil {
		r.queueAutoRun()
	}
	return nil
}
