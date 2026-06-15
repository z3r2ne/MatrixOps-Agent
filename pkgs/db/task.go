package database

import (
	"encoding/json"
	"fmt"
	"pkgs/db/models"

	"gorm.io/gorm"
)

// Task 相关数据库操作

// GetTaskByID 根据 ID 获取任务
func GetTaskByID(db *gorm.DB, id uint) (*models.Task, error) {
	var task models.Task
	err := db.First(&task, id).Error
	return &task, err
}

// GetTasksByWorkspaceID 根据工作区 ID 获取任务列表
func GetTasksByWorkspaceID(db *gorm.DB, workspaceID uint) ([]models.Task, error) {
	var tasks []models.Task
	err := db.Where("workspace_id = ? AND parent_task_id IS NULL", workspaceID).
		Order("list_position ASC, created_at DESC, id ASC").
		Find(&tasks).Error
	return tasks, err
}

// ShiftTaskListPositionsDown 将工作区内所有任务的 list_position 整体 +1，便于在列表顶部插入 list_position=0 的新任务。
func ShiftTaskListPositionsDown(tx *gorm.DB, workspaceID uint) error {
	return tx.Model(&models.Task{}).
		Where("workspace_id = ?", workspaceID).
		UpdateColumn("list_position", gorm.Expr("list_position + ?", 1)).Error
}

// ReorderTasksInWorkspace 按 orderedIDs 顺序写入 list_position（0..n-1）。orderedIDs 须恰好包含该工作区下全部任务 id。
func ReorderTasksInWorkspace(db *gorm.DB, workspaceID uint, orderedIDs []uint) error {
	var existing []uint
	if err := db.Model(&models.Task{}).Where("workspace_id = ?", workspaceID).Pluck("id", &existing).Error; err != nil {
		return err
	}
	if len(orderedIDs) != len(existing) {
		return fmt.Errorf("任务数量与工作区不符")
	}
	want := make(map[uint]struct{}, len(existing))
	for _, id := range existing {
		want[id] = struct{}{}
	}
	for _, id := range orderedIDs {
		if _, ok := want[id]; !ok {
			return fmt.Errorf("无效的任务 id: %d", id)
		}
		delete(want, id)
	}
	if len(want) != 0 {
		return fmt.Errorf("须包含该项目全部任务 id")
	}

	return db.Transaction(func(tx *gorm.DB) error {
		for i, id := range orderedIDs {
			if err := tx.Model(&models.Task{}).
				Where("id = ? AND workspace_id = ?", id, workspaceID).
				Update("list_position", i).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// CreateTask 创建任务
func CreateTask(db *gorm.DB, task *models.Task) error {
	return db.Create(task).Error
}

// UpdateTask 更新任务（保存整个结构体）
func UpdateTask(db *gorm.DB, task *models.Task) error {
	return db.Save(task).Error
}

// UpdateTaskFields 更新任务的指定字段
func UpdateTaskFields(db *gorm.DB, taskID uint, updates map[string]interface{}) error {
	return db.Model(&models.Task{}).Where("id = ?", taskID).Updates(updates).Error
}

// UpdateTaskStatus 更新任务状态
func UpdateTaskStatus(db *gorm.DB, taskID uint, status string) error {
	return UpdateTaskFields(db, taskID, map[string]interface{}{
		"status": status,
	})
}

// UpdateTaskSessionID 更新任务的 session_id
func UpdateTaskSessionID(db *gorm.DB, taskID uint, sessionID string) error {
	return UpdateTaskFields(db, taskID, map[string]interface{}{
		"session_id": sessionID,
	})
}

// GetTaskSessionID 获取任务的 session_id
func GetTaskSessionID(db *gorm.DB, taskID uint) (string, error) {
	var task models.Task
	err := db.Select("session_id").First(&task, taskID).Error
	return task.SessionID, err
}

// DeleteTask 删除任务
func DeleteTask(db *gorm.DB, taskID uint) error {
	return db.Delete(&models.Task{}, taskID).Error
}

func DeleteTasksByWorkspaceID(db *gorm.DB, workspaceID uint) error {
	return db.Where("workspace_id = ?", workspaceID).Delete(&models.Task{}).Error
}

func GetTasksByWorkspaceIDAndProjectID(db *gorm.DB, workspaceID uint, projectID uint) ([]models.Task, error) {
	var tasks []models.Task
	err := db.Where("workspace_id = ? AND project_id = ? AND parent_task_id IS NULL", workspaceID, projectID).
		Order("list_position ASC, created_at DESC, id ASC").
		Find(&tasks).Error
	return tasks, err
}

func DeleteTasksByWorkspaceIDAndProjectID(db *gorm.DB, workspaceID uint, projectID uint) error {
	return db.Where("workspace_id = ? AND project_id = ?", workspaceID, projectID).Delete(&models.Task{}).Error
}

// GetTaskQueue 获取任务的消息队列
func GetTaskQueue(db *gorm.DB, taskID uint) ([]models.TaskMessageQueueItem, error) {
	queue, _, err := GetTaskQueueSettings(db, taskID)
	return queue, err
}

// GetTaskQueueSettings 获取任务消息队列及自动发送开关
func GetTaskQueueSettings(db *gorm.DB, taskID uint) ([]models.TaskMessageQueueItem, bool, error) {
	var task models.Task
	err := db.Select("message_queue", "message_queue_auto_send").First(&task, taskID).Error
	if err != nil {
		// 兼容尚未迁移 message_queue_auto_send 列的旧库
		var legacy models.Task
		if legacyErr := db.Select("message_queue").First(&legacy, taskID).Error; legacyErr != nil {
			return nil, true, err
		}
		return legacy.MessageQueue, true, nil
	}
	if task.MessageQueue == nil {
		task.MessageQueue = []models.TaskMessageQueueItem{}
	}
	return task.MessageQueue, task.MessageQueueAutoSend, nil
}

// SetTaskMessageQueueAutoSend 设置队列是否在对话结束后自动发送
func SetTaskMessageQueueAutoSend(db *gorm.DB, taskID uint, autoSend bool) error {
	return db.Model(&models.Task{}).Where("id = ?", taskID).Update("message_queue_auto_send", autoSend).Error
}

// UpdateTaskQueue 更新任务的消息队列（完整替换）
func UpdateTaskQueue(db *gorm.DB, taskID uint, queue []models.TaskMessageQueueItem) error {
	queueJSON, err := json.Marshal(queue)
	if err != nil {
		return fmt.Errorf("marshal task message queue: %w", err)
	}
	return db.Model(&models.Task{}).Where("id = ?", taskID).Update("message_queue", string(queueJSON)).Error
}

// AppendTaskQueueItem 向任务消息队列追加一条消息
func AppendTaskQueueItem(db *gorm.DB, taskID uint, item models.TaskMessageQueueItem) error {
	var task models.Task
	if err := db.First(&task, taskID).Error; err != nil {
		return err
	}
	queue := append(task.MessageQueue, item)
	return UpdateTaskQueue(db, taskID, queue)
}

// PrependTaskQueueItem 向任务消息队列头部插入一条消息
func PrependTaskQueueItem(db *gorm.DB, taskID uint, item models.TaskMessageQueueItem) error {
	var task models.Task
	if err := db.First(&task, taskID).Error; err != nil {
		return err
	}
	queue := append([]models.TaskMessageQueueItem{item}, task.MessageQueue...)
	return UpdateTaskQueue(db, taskID, queue)
}

// CountTasksByStatus 统计指定状态的任务数量
func CountTasksByStatus(db *gorm.DB, workspaceID uint, status string) (int64, error) {
	var count int64
	err := db.Model(&models.Task{}).
		Where("workspace_id = ? AND status = ?", workspaceID, status).
		Count(&count).Error
	return count, err
}

// GetTasksWithPagination 分页获取任务列表
func GetTasksWithPagination(db *gorm.DB, workspaceID uint, offset, limit int) ([]models.Task, error) {
	var tasks []models.Task
	err := db.Where("workspace_id = ?", workspaceID).
		Order("list_position ASC, created_at DESC, id ASC").
		Offset(offset).
		Limit(limit).
		Find(&tasks).Error
	return tasks, err
}

// TaskWithProject 任务与项目关联查询结果
type TaskWithProject struct {
	models.Task
	ProjectName  string `json:"projectName" gorm:"column:project_name"`
	SessionTitle string `json:"sessionTitle" gorm:"column:session_title"`
}

// GetTasksWithProjectByWorkspaceID 获取工作区任务（关联项目名称和会话标题）
func GetTasksWithProjectByWorkspaceID(db *gorm.DB, workspaceID uint) ([]TaskWithProject, error) {
	var tasks []TaskWithProject
	err := db.Table("tasks").
		Select("tasks.*, projects.name as project_name, sessions.title as session_title").
		Joins("left join projects on projects.id = tasks.project_id").
		Joins("left join sessions on sessions.id = tasks.session_id").
		Where("tasks.workspace_id = ? AND tasks.parent_task_id IS NULL", workspaceID).
		Order("tasks.list_position ASC, tasks.created_at DESC, tasks.id ASC").
		Find(&tasks).Error
	return tasks, err
}

// BackfillTaskWorkspaceIDs 将历史项目任务回填到所属工作区。
func BackfillTaskWorkspaceIDs(db *gorm.DB) error {
	if db == nil {
		return nil
	}

	workspaces, err := GetAllWorkspaces(db)
	if err != nil {
		return err
	}

	projectToWorkspace := make(map[uint]uint)
	for _, workspace := range workspaces {
		for _, projectID := range workspace.ProjectIDs {
			if projectID == 0 {
				continue
			}
			if _, exists := projectToWorkspace[projectID]; !exists {
				projectToWorkspace[projectID] = workspace.ID
			}
		}
	}

	return db.Transaction(func(tx *gorm.DB) error {
		for projectID, workspaceID := range projectToWorkspace {
			if err := tx.Model(&models.Task{}).
				Where("workspace_id = 0 AND project_id = ?", projectID).
				Update("workspace_id", workspaceID).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// BackfillSubtaskParentTaskIDs 修复历史上通过 run_worker_task 创建但 parent_task_id 未落库的子任务。
func BackfillSubtaskParentTaskIDs(db *gorm.DB) error {
	if db == nil || db.Dialector == nil || db.Dialector.Name() != "sqlite" {
		return nil
	}

	return db.Exec(`
WITH candidate_parts AS (
	SELECT session_id, time_created, tool
	FROM parts
	WHERE type = 'tool'
	  AND instr(tool, '"tool":"run_worker_task"') > 0
	  AND instr(tool, 'subtaskTaskId') > 0
),
candidate_matches AS (
	SELECT
		CAST(json_extract(candidate_parts.tool, '$.state.metadata.subtaskTaskId') AS INTEGER) AS child_id,
		parent.id AS parent_id,
		ROW_NUMBER() OVER (
			PARTITION BY CAST(json_extract(candidate_parts.tool, '$.state.metadata.subtaskTaskId') AS INTEGER)
			ORDER BY candidate_parts.time_created DESC, parent.id DESC
		) AS row_num
	FROM candidate_parts
	JOIN tasks AS parent ON parent.session_id = candidate_parts.session_id
	WHERE json_valid(candidate_parts.tool) = 1
	  AND json_extract(candidate_parts.tool, '$.tool') = 'run_worker_task'
	  AND json_extract(candidate_parts.tool, '$.state.metadata.subtaskTaskId') IS NOT NULL
),
matches AS (
	SELECT child_id, parent_id
	FROM candidate_matches
	WHERE row_num = 1
	  AND child_id <> parent_id
)
UPDATE tasks
SET parent_task_id = (
	SELECT parent_id
	FROM matches
	WHERE matches.child_id = tasks.id
)
WHERE parent_task_id IS NULL
  AND id IN (SELECT child_id FROM matches)`).Error
}

// CleanupStaleActiveTasks 清理所有异常状态的活跃任务
// 在系统启动时调用，将所有状态为 active 的任务标记为 failed
// 因为系统重启后这些任务实际上已经不在运行了
func CleanupStaleActiveTasks(db *gorm.DB) error {
	result := db.Model(&models.Task{}).
		Where("status = ?", "running").
		Updates(map[string]interface{}{
			"status": "failed",
			"error":  "被终止",
		})
	return result.Error
}
