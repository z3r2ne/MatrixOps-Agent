package database

import (
	"fmt"

	"pkgs/db/models"

	"gorm.io/gorm"
)

// Worker 相关数据库操作

// GetAllWorkers 获取所有 Worker
func GetAllWorkers(db *gorm.DB) ([]models.Worker, error) {
	var workers []models.Worker
	err := db.Order("created_at DESC").Find(&workers).Error
	return workers, err
}

// GetWorkerByID 根据 ID 获取 Worker
func GetWorkerByID(db *gorm.DB, id uint) (*models.Worker, error) {
	var worker models.Worker
	err := db.First(&worker, id).Error
	return &worker, err
}

// GetWorkerByName 根据 Name 获取 Worker
func GetWorkerByName(db *gorm.DB, name string) (*models.Worker, error) {
	var worker models.Worker
	err := db.First(&worker, "name = ?", name).Error
	return &worker, err
}

// WorkerModelContext 从数据库解析出的 Worker 及其关联模型配置（不含会话、工具注册表等运行时依赖）。
type WorkerModelContext struct {
	Worker        *models.Worker
	LLMConfig     *models.LLMConfig
	ModelSettings *models.ModelSettings
}

// LoadWorkerModelContext 按 worker名称加载 Worker、关联 LLMConfig 与 ModelSettings。
func LoadWorkerModelContext(db *gorm.DB, workerName string) (*WorkerModelContext, error) {
	if db == nil {
		return nil, fmt.Errorf("db is required")
	}
	worker, err := GetWorkerByName(db, workerName)
	if err != nil {
		return nil, fmt.Errorf("get worker failed: %w", err)
	}
	out := &WorkerModelContext{Worker: worker}
	if worker.LLMConfigID != nil {
		llmConfig, err := GetLLMConfigByID(db, *worker.LLMConfigID)
		if err != nil {
			return nil, fmt.Errorf("get llm config failed: %w", err)
		}
		out.LLMConfig = llmConfig
	}
	modelSettings, _ := GetModelSettingsForWorker(db, worker)
	out.ModelSettings = modelSettings
	return out, nil
}

// HasWorkerByName 判断 Worker 是否存在
func HasWorkerByName(db *gorm.DB, name string) (bool, error) {
	var count int64
	err := db.Model(&models.Worker{}).Where("name = ?", name).Count(&count).Error
	return count > 0, err
}

// CreateWorker 创建 Worker
func CreateWorker(db *gorm.DB, worker *models.Worker) error {
	return db.Create(worker).Error
}

// UpdateWorker 更新 Worker
func UpdateWorker(db *gorm.DB, worker *models.Worker) error {
	return db.Save(worker).Error
}

// UpdateWorkerByName 根据 Name 更新 Worker
func UpdateWorkerByName(db *gorm.DB, name string, worker *models.Worker) error {
	return db.Model(&models.Worker{}).Where("name = ?", name).Updates(worker).Error
}

// DeleteWorker 删除 Worker
func DeleteWorker(db *gorm.DB, workerID uint) error {
	return db.Delete(&models.Worker{}, workerID).Error
}
