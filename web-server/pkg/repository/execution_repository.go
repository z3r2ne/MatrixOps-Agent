package repository

import (
	"pkgs/db/models"

	"gorm.io/gorm"
)

// ExecutionRepository 执行记录数据访问接口
type ExecutionRepository interface {
	Create(execution *models.TaskExecution) error
	GetByID(id uint) (*models.TaskExecution, error)
	Update(execution *models.TaskExecution) error
	UpdateStatus(id uint, status string) error
	ListByTaskID(taskID uint) ([]*models.TaskExecution, error)
	GetLatestByTaskID(taskID uint) (*models.TaskExecution, error)
	FindBySessionID(sessionID string) (*models.TaskExecution, error)
	UpdateAllRunningToFailed() error
}

type executionRepository struct {
	db *gorm.DB
}

// NewExecutionRepository 创建执行记录仓储实例
func NewExecutionRepository(db *gorm.DB) ExecutionRepository {
	return &executionRepository{db: db}
}

func (r *executionRepository) Create(execution *models.TaskExecution) error {
	return r.db.Create(execution).Error
}

func (r *executionRepository) GetByID(id uint) (*models.TaskExecution, error) {
	var execution models.TaskExecution
	err := r.db.First(&execution, id).Error
	if err != nil {
		return nil, err
	}
	return &execution, nil
}

func (r *executionRepository) Update(execution *models.TaskExecution) error {
	return r.db.Save(execution).Error
}

func (r *executionRepository) UpdateStatus(id uint, status string) error {
	return r.db.Model(&models.TaskExecution{}).Where("id = ?", id).Update("status", status).Error
}

func (r *executionRepository) ListByTaskID(taskID uint) ([]*models.TaskExecution, error) {
	var executions []*models.TaskExecution
	err := r.db.Where("task_id = ?", taskID).Order("created_at DESC").Find(&executions).Error
	return executions, err
}

func (r *executionRepository) GetLatestByTaskID(taskID uint) (*models.TaskExecution, error) {
	var execution models.TaskExecution
	err := r.db.Where("task_id = ?", taskID).Order("created_at DESC").First(&execution).Error
	if err != nil {
		return nil, err
	}
	return &execution, nil
}

func (r *executionRepository) FindBySessionID(sessionID string) (*models.TaskExecution, error) {
	var execution models.TaskExecution
	err := r.db.Where("agent_session_id = ?", sessionID).First(&execution).Error
	if err != nil {
		return nil, err
	}
	return &execution, nil
}

func (r *executionRepository) UpdateAllRunningToFailed() error {
	return r.db.Model(&models.TaskExecution{}).
		Where("status = ?", "running").
		Updates(map[string]interface{}{
			"status": "failed",
		}).Error
}
