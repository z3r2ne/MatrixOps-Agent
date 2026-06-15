package taskqueue

import (
	"strings"

	database "pkgs/db"
	"pkgs/db/models"

	"gorm.io/gorm"
)

// BroadcastFunc 在队列变更后通知订阅方（例如 WebSocket）。
type BroadcastFunc func(taskID uint, queue []models.TaskMessageQueueItem)

// SupplementHandler 消费一条 supplement 队列项（例如创建用户消息并同步会话 memory）。
type SupplementHandler func(item models.TaskMessageQueueItem) error

// Queue 封装任务消息队列的持久化与广播。
type Queue struct {
	db        *gorm.DB
	taskID    uint
	broadcast BroadcastFunc
}

// New 创建绑定到指定任务的消息队列；broadcast 可为 nil（例如 CLI）。
func New(db *gorm.DB, taskID uint, broadcast BroadcastFunc) *Queue {
	return &Queue{db: db, taskID: taskID, broadcast: broadcast}
}

func (q *Queue) valid() bool {
	return q != nil && q.db != nil && q.taskID > 0
}

func (q *Queue) persistAndBroadcast(queue []models.TaskMessageQueueItem) error {
	if !q.valid() {
		return nil
	}
	if err := database.UpdateTaskQueue(q.db, q.taskID, queue); err != nil {
		return err
	}
	if q.broadcast != nil {
		q.broadcast(q.taskID, queue)
	}
	return nil
}

// Load 返回当前队列快照。
func (q *Queue) Load() ([]models.TaskMessageQueueItem, error) {
	if !q.valid() {
		return nil, nil
	}
	return database.GetTaskQueue(q.db, q.taskID)
}

// Replace 完整替换队列。
func (q *Queue) Replace(queue []models.TaskMessageQueueItem) error {
	if !q.valid() {
		return nil
	}
	return q.persistAndBroadcast(queue)
}

// Append 在队尾追加一条消息。
func (q *Queue) Append(item models.TaskMessageQueueItem) error {
	if !q.valid() {
		return nil
	}
	queue, err := q.Load()
	if err != nil {
		return err
	}
	queue = append(queue, item)
	return q.persistAndBroadcast(queue)
}

// Prepend 在队首插入一条消息。
func (q *Queue) Prepend(item models.TaskMessageQueueItem) error {
	if !q.valid() {
		return nil
	}
	queue, err := q.Load()
	if err != nil {
		return err
	}
	queue = append([]models.TaskMessageQueueItem{item}, queue...)
	return q.persistAndBroadcast(queue)
}

// MoveToFront 将指定 ID 的项移到队首。
func (q *Queue) MoveToFront(itemID string) (*models.TaskMessageQueueItem, error) {
	if !q.valid() {
		return nil, nil
	}
	queue, err := q.Load()
	if err != nil {
		return nil, err
	}
	target, next := moveItemToFront(queue, itemID)
	if target == nil {
		return nil, nil
	}
	if err := q.persistAndBroadcast(next); err != nil {
		return nil, err
	}
	return target, nil
}

// SetSupplement 设置队列项的 supplement 标记。
func (q *Queue) SetSupplement(itemID string, supplement bool) error {
	if !q.valid() {
		return nil
	}
	queue, err := q.Load()
	if err != nil {
		return err
	}
	changed := false
	for i := range queue {
		if queue[i].ID == itemID {
			queue[i].Supplement = supplement
			changed = true
			break
		}
	}
	if !changed {
		return nil
	}
	return q.persistAndBroadcast(queue)
}

// ConsumeSupplements 从队首连续消费所有 supplement 消息，并交给 handler 处理（如创建用户消息）。
func (q *Queue) ConsumeSupplements(handler SupplementHandler) (bool, error) {
	if !q.valid() || handler == nil {
		return false, nil
	}

	consumedAny := false
	for {
		queue, err := q.Load()
		if err != nil {
			return consumedAny, err
		}
		if len(queue) == 0 || !queue[0].IsLoopInject() {
			return consumedAny, nil
		}

		item := queue[0]
		if !supplementItemHasPayload(item) {
			remaining := queue[1:]
			if err := q.persistAndBroadcast(remaining); err != nil {
				return consumedAny, err
			}
			consumedAny = true
			continue
		}
		remaining := queue[1:]
		if err := q.persistAndBroadcast(remaining); err != nil {
			return consumedAny, err
		}
		if err := handler(item); err != nil {
			return consumedAny, err
		}
		consumedAny = true
	}
}

func supplementItemHasPayload(item models.TaskMessageQueueItem) bool {
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

// DequeueNext 移除并返回队首消息（不区分 supplement）。
func (q *Queue) DequeueNext() (*models.TaskMessageQueueItem, error) {
	if !q.valid() {
		return nil, nil
	}
	queue, err := q.Load()
	if err != nil {
		return nil, err
	}
	if len(queue) == 0 {
		return nil, nil
	}
	item := queue[0]
	remaining := queue[1:]
	if err := q.persistAndBroadcast(remaining); err != nil {
		return nil, err
	}
	cp := item
	return &cp, nil
}

func moveItemToFront(queue []models.TaskMessageQueueItem, itemID string) (*models.TaskMessageQueueItem, []models.TaskMessageQueueItem) {
	var target *models.TaskMessageQueueItem
	for _, item := range queue {
		if item.ID == itemID {
			cp := item
			target = &cp
			break
		}
	}
	if target == nil {
		return nil, queue
	}

	next := make([]models.TaskMessageQueueItem, 0, len(queue))
	next = append(next, *target)
	for _, item := range queue {
		if item.ID == itemID {
			continue
		}
		next = append(next, item)
	}
	return target, next
}
