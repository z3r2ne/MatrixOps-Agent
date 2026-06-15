package task_runner

import (
	agentsession "matrixops-agent/session"
	database "pkgs/db"
	"pkgs/db/models"
	"pkgs/taskqueue"
)

// TryConsumeAppendQueue 在任务未运行时，将队首连续的 append 项写入会话记忆并移出队列，不启动 Agent。
func TryConsumeAppendQueue(taskID uint, opts ...TaskRuntimeConfigOption) error {
	if taskID == 0 || IsRunning(taskID) {
		return nil
	}

	peek := NewTaskRuntimeConfig(opts...)
	db := peek.db
	if db == nil {
		db = database.DB
	}
	wsHub := peek.wsHub
	if wsHub == nil {
		return nil
	}

	task, err := database.GetTaskByID(db, taskID)
	if err != nil || task == nil {
		return err
	}

	q := taskqueue.New(db, taskID, wsHub.BroadcastTaskQueue)
	for {
		queue, err := q.Load()
		if err != nil {
			return err
		}
		if len(queue) == 0 || !queue[0].IsAppend() {
			return nil
		}
		if !queueItemHasRunnablePayload(queue[0]) {
			if err := q.Replace(queue[1:]); err != nil {
				return err
			}
			continue
		}

		item := queue[0]
		if err := q.Replace(queue[1:]); err != nil {
			return err
		}
		if err := agentsession.DeliverTaskQueueAppendItem(db, task, item); err != nil {
			return err
		}
	}
}

func (r *TaskRuntime) consumeAppendQueueHead(task *models.Task) error {
	if r == nil || r.messageQueue == nil || task == nil {
		return nil
	}
	for {
		queue, err := r.messageQueue.Load()
		if err != nil {
			return err
		}
		if len(queue) == 0 || !queue[0].IsAppend() {
			return nil
		}
		if !queueItemHasRunnablePayload(queue[0]) {
			if err := r.messageQueue.Replace(queue[1:]); err != nil {
				return err
			}
			continue
		}
		item := queue[0]
		if err := r.messageQueue.Replace(queue[1:]); err != nil {
			return err
		}
		if err := agentsession.DeliverTaskQueueAppendItem(r.db, task, item); err != nil {
			return err
		}
	}
}
