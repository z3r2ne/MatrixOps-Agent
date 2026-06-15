package database

import (
	"pkgs/db/models"
	"time"

	"gorm.io/gorm"
)

// CommandLog 相关数据库操作

// GetCommandLogByID 根据 ID 获取命令日志
func GetCommandLogByID(db *gorm.DB, id uint) (*models.CommandLog, error) {
	var log models.CommandLog
	err := db.First(&log, id).Error
	return &log, err
}

// GetCommandLogsByTaskID 根据任务 ID 获取命令日志列表
func GetCommandLogsByTaskID(db *gorm.DB, taskID uint, status string, limit int) ([]models.CommandLog, error) {
	var logs []models.CommandLog
	query := db.Model(&models.CommandLog{}).
		Where("task_id = ?", taskID)

	if status != "" {
		query = query.Where("status = ?", status)
	}

	query = query.Order("created_at ASC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Find(&logs).Error
	return logs, err
}

// CreateCommandLog 创建命令日志
func CreateCommandLog(db *gorm.DB, log *models.CommandLog) error {
	return db.Create(log).Error
}

// UpdateCommandLog 更新命令日志
func UpdateCommandLog(db *gorm.DB, log *models.CommandLog) error {
	return db.Save(log).Error
}

// UpdateCommandLogFields 更新命令日志的指定字段
func UpdateCommandLogFields(db *gorm.DB, logID uint, updates map[string]interface{}) error {
	return db.Model(&models.CommandLog{}).Where("id = ?", logID).Updates(updates).Error
}

// UpdateCommandLogStatus 更新命令日志状态
func UpdateCommandLogStatus(db *gorm.DB, logID uint, status string) error {
	return UpdateCommandLogFields(db, logID, map[string]interface{}{
		"status": status,
	})
}

// AppendCommandLogOutput 追加命令日志输出
func AppendCommandLogOutput(db *gorm.DB, logID uint, output string) error {
	return db.Exec(
		"UPDATE command_logs SET output = output || ? WHERE id = ?",
		output, logID,
	).Error
}

// AppendCommandLogStderr 追加命令日志错误输出
func AppendCommandLogStderr(db *gorm.DB, logID uint, stderr string) error {
	return db.Exec(
		"UPDATE command_logs SET stderr = stderr || ? WHERE id = ?",
		stderr, logID,
	).Error
}

// DeleteCommandLog 删除命令日志
func DeleteCommandLog(db *gorm.DB, logID uint) error {
	return db.Delete(&models.CommandLog{}, logID).Error
}

// DeleteOldCommandLogs 删除指定时间之前的命令日志
func DeleteOldCommandLogs(db *gorm.DB, before time.Time) (int64, error) {
	result := db.Where("created_at < ?", before).Delete(&models.CommandLog{})
	return result.RowsAffected, result.Error
}

// QueryCommandLogs 根据查询条件获取命令日志列表
func QueryCommandLogs(originDb *gorm.DB, query models.CommandLogQuery) ([]models.CommandLog, int64, error) {
	db := originDb.Model(&models.CommandLog{})

	if query.Source != "" {
		db = db.Where("source = ?", query.Source)
	}
	if query.SourceID != nil {
		db = db.Where("source_id = ?", *query.SourceID)
	}
	if query.Status != "" {
		db = db.Where("status = ?", query.Status)
	}

	var total int64
	db.Count(&total)

	if query.Limit <= 0 {
		query.Limit = 50
	}
	if query.Limit > 200 {
		query.Limit = 200
	}

	var logs []models.CommandLog
	err := db.Order("created_at DESC").
		Offset(query.Offset).
		Limit(query.Limit).
		Find(&logs).Error

	return logs, total, err
}
