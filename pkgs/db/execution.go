package database

import (
	"pkgs/db/models"
	"time"

	"gorm.io/gorm"
)

// TaskExecution 相关数据库操作

// GetExecutionByID 根据 ID 获取任务执行记录
func GetExecutionByID(db *gorm.DB, id uint) (*models.TaskExecution, error) {
	var execution models.TaskExecution
	err := db.First(&execution, id).Error
	return &execution, err
}

// GetTaskExecutionByID 别名：根据 ID 获取任务执行记录
func GetTaskExecutionByID(db *gorm.DB, id uint) (*models.TaskExecution, error) {
	return GetExecutionByID(db, id)
}

// GetExecutionsByTaskIDOrdered 根据任务 ID 获取执行记录（按时间排序）
func GetExecutionsByTaskIDOrdered(db *gorm.DB, taskID uint, ascending bool) ([]models.TaskExecution, error) {
	var executions []models.TaskExecution
	query := db.Where("task_id = ?", taskID)

	if ascending {
		query = query.Order("created_at ASC")
	} else {
		query = query.Order("created_at DESC")
	}

	err := query.Find(&executions).Error
	return executions, err
}

// GetExecutionsBySessionID 根据 session_id 获取执行记录列表
func GetExecutionsBySessionID(db *gorm.DB, sessionID string) ([]models.TaskExecution, error) {
	var executions []models.TaskExecution
	err := db.
		Where("agent_session_id = ?", sessionID).
		Order("created_at ASC").
		Find(&executions).Error
	return executions, err
}

// GetExecutionsByTaskID 根据任务 ID 获取执行记录列表
func GetExecutionsByTaskID(db *gorm.DB, taskID uint, limit int) ([]models.TaskExecution, error) {
	var executions []models.TaskExecution
	query := db.Where("task_id = ?", taskID).Order("created_at DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Find(&executions).Error
	return executions, err
}

// GetExecutionsByTaskIDWithRange 根据任务 ID 和时间范围获取执行记录
func GetExecutionsByTaskIDWithRange(db *gorm.DB, taskID uint, startTime, endTime time.Time) ([]models.TaskExecution, error) {
	var executions []models.TaskExecution
	err := db.
		Where("task_id = ? AND created_at >= ? AND created_at <= ?", taskID, startTime, endTime).
		Order("created_at DESC").
		Find(&executions).Error
	return executions, err
}

// CreateExecution 创建任务执行记录
func CreateExecution(db *gorm.DB, execution *models.TaskExecution) error {
	return db.Create(execution).Error
}

// UpdateExecution 更新任务执行记录
func UpdateExecution(db *gorm.DB, execution *models.TaskExecution) error {
	return db.Save(execution).Error
}

// UpdateExecutionFields 更新任务执行记录的指定字段
func UpdateExecutionFields(db *gorm.DB, executionID uint, updates map[string]interface{}) error {
	return db.Model(&models.TaskExecution{}).Where("id = ?", executionID).Updates(updates).Error
}

// UpdateExecutionSessionID 更新任务执行记录的 session_id
func UpdateExecutionSessionID(db *gorm.DB, executionID uint, sessionID string) error {
	return db.Exec(
		"UPDATE task_executions SET agent_session_id = ? WHERE id = ?",
		sessionID, executionID,
	).Error
}

// DeleteExecution 删除任务执行记录
func DeleteExecution(db *gorm.DB, executionID uint) error {
	return db.Delete(&models.TaskExecution{}, executionID).Error
}

func DeleteExecutionsByTaskID(db *gorm.DB, taskID uint) error {
	return db.Where("task_id = ?", taskID).Delete(&models.TaskExecution{}).Error
}

// DeleteExecutionsByTaskIDAfter 删除指定任务 ID 在某个执行 ID 之后的所有执行记录
func DeleteExecutionsByTaskIDAfter(db *gorm.DB, taskID uint, afterExecutionID uint) error {
	return db.Where("task_id = ? AND id >= ?", taskID, afterExecutionID).Delete(&models.TaskExecution{}).Error
}
