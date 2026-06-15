package database

import (
	"pkgs/db/models"

	"gorm.io/gorm"
)

// ExecutionLog 相关数据库操作

// GetExecutionLogByID 根据 ID 获取执行日志
func GetExecutionLogByID(db *gorm.DB, id uint) (*models.ExecutionLog, error) {
	var log models.ExecutionLog
	err := db.First(&log, id).Error
	return &log, err
}

// GetExecutionLogsByExecutionID 根据执行 ID 获取日志列表（按序号排序）
func GetExecutionLogsByExecutionID(db *gorm.DB, executionID uint) ([]models.ExecutionLog, error) {
	var logs []models.ExecutionLog
	err := db.Where("execution_id = ?", executionID).Order("sequence ASC").Find(&logs).Error
	return logs, err
}

// GetExecutionLogsByExecutionIDOrdered 根据执行 ID 获取日志列表（可指定排序）
func GetExecutionLogsByExecutionIDOrdered(db *gorm.DB, executionID uint, ascending bool) ([]models.ExecutionLog, error) {
	var logs []models.ExecutionLog
	query := db.Where("execution_id = ?", executionID)

	if ascending {
		query = query.Order("sequence ASC")
	} else {
		query = query.Order("sequence DESC")
	}

	err := query.Find(&logs).Error
	return logs, err
}

// GetExecutionLogsByTaskID 根据任务 ID 获取日志列表
func GetExecutionLogsByTaskID(db *gorm.DB, taskID uint) ([]models.ExecutionLog, error) {
	var logs []models.ExecutionLog
	err := db.Where("task_id = ?", taskID).Order("created_at ASC").Find(&logs).Error
	return logs, err
}

// CreateExecutionLog 创建执行日志
func CreateExecutionLog(db *gorm.DB, log *models.ExecutionLog) error {
	return db.Create(log).Error
}

// UpdateExecutionLog 更新执行日志
func UpdateExecutionLog(db *gorm.DB, log *models.ExecutionLog) error {
	return db.Save(log).Error
}

// DeleteExecutionLog 删除执行日志
func DeleteExecutionLog(db *gorm.DB, logID uint) error {
	return db.Delete(&models.ExecutionLog{}, logID).Error
}

func DeleteExecutionLogsByTaskID(db *gorm.DB, taskID uint) error {
	return db.Where("task_id = ?", taskID).Delete(&models.ExecutionLog{}).Error
}

// DeleteExecutionLogsByTaskIDAfter 删除指定任务在某个执行 ID 之后的所有日志
func DeleteExecutionLogsByTaskIDAfter(db *gorm.DB, taskID uint, afterExecutionID uint) error {
	return db.Where("task_id = ? AND execution_id >= ?", taskID, afterExecutionID).Delete(&models.ExecutionLog{}).Error
}
