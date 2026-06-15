package task_runner

import (
	"strings"
	"sync"

	database "pkgs/db"
	"pkgs/db/models"
	"pkgs/taskqueue"

	"gorm.io/gorm"
)

var taskLaunching sync.Map

func markTaskLaunching(taskID uint) bool {
	_, loaded := taskLaunching.LoadOrStore(taskID, true)
	return !loaded
}

func unmarkTaskLaunching(taskID uint) {
	taskLaunching.Delete(taskID)
}

func queueItemHasRunnablePayload(item models.TaskMessageQueueItem) bool {
	if strings.TrimSpace(item.Content) != "" {
		return true
	}
	for _, part := range item.Parts {
		if strings.TrimSpace(part.Type) == "file" && strings.TrimSpace(part.URL) != "" {
			return true
		}
	}
	return false
}

func dequeueRunnableHead(db *gorm.DB, taskID uint, broadcast taskqueue.BroadcastFunc) (*models.TaskMessageQueueItem, error) {
	q := taskqueue.New(db, taskID, broadcast)
	for {
		queue, err := q.Load()
		if err != nil {
			return nil, err
		}
		if len(queue) == 0 {
			return nil, nil
		}
		if queue[0].IsAppend() {
			return nil, nil
		}
		item, err := q.DequeueNext()
		if err != nil {
			return nil, err
		}
		if item == nil {
			return nil, nil
		}
		if queueItemHasRunnablePayload(*item) {
			return item, nil
		}
	}
}

func taskStatusAllowsQueueAutoRun(status string) bool {
	switch models.TaskStatus(status) {
	case models.TaskStatusDone, models.TaskStatusQueue:
		return true
	default:
		return false
	}
}

// TryAutoRunTaskQueue 在任务未运行、autoSend 开启且队列有可执行消息时，自动 dequeue 队首并启动任务。
func TryAutoRunTaskQueue(taskID uint, opts ...TaskRuntimeConfigOption) error {
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
	if !task.MessageQueueAutoSend || len(task.MessageQueue) == 0 {
		return nil
	}
	if !taskStatusAllowsQueueAutoRun(task.Status) {
		return nil
	}

	if err := TryConsumeAppendQueue(taskID, opts...); err != nil {
		return err
	}

	item, err := dequeueRunnableHead(db, taskID, wsHub.BroadcastTaskQueue)
	if err != nil {
		return err
	}
	if item == nil {
		return nil
	}

	runOpts := []TaskRuntimeConfigOption{
		WithTaskID(taskID),
		WithContent(item.Content),
		WithWSHub(wsHub),
		WithDB(db),
	}
	if parts := taskQueueItemToInputParts(item); len(parts) > 0 {
		runOpts = append(runOpts, WithInputParts(parts))
	}
	runOpts = append(runOpts, WithInputSource(QueueItemInputSource(item)))
	runOpts = append(runOpts, WithMessageKind(QueueItemMessageKind(item)))
	runOpts = append(runOpts, WithMessageOrigin(QueueItemMessageOrigin(item)))
	runOpts = append(runOpts, opts...)

	mergedOpts, config, err := prepareTaskRun(taskID, runOpts...)
	if err != nil {
		return err
	}
	return launchTaskAsync(taskID, mergedOpts, config)
}
