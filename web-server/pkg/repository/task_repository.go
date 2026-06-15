package repository

import (
	"pkgs/db/models"

	"gorm.io/gorm"
)

// TaskRepository 任务数据访问接口
type TaskRepository interface {
	Create(task *models.Task) error
	GetByID(id uint) (*models.Task, error)
	Update(task *models.Task) error
	UpdateStatus(id uint, status string) error
	Delete(id uint) error
	ListByWorkspaceID(workspaceID uint) ([]*models.Task, error)
}

type taskRepository struct {
	db *gorm.DB
}

// NewTaskRepository 创建任务仓储实例
func NewTaskRepository(db *gorm.DB) TaskRepository {
	return &taskRepository{db: db}
}

func (r *taskRepository) Create(task *models.Task) error {
	return r.db.Create(task).Error
}

func (r *taskRepository) GetByID(id uint) (*models.Task, error) {
	var task models.Task
	err := r.db.First(&task, id).Error
	if err != nil {
		return nil, err
	}
	return &task, nil
}

func (r *taskRepository) Update(task *models.Task) error {
	return r.db.Save(task).Error
}

func (r *taskRepository) UpdateStatus(id uint, status string) error {
	return r.db.Model(&models.Task{}).Where("id = ?", id).Update("status", status).Error
}

func (r *taskRepository) Delete(id uint) error {
	return r.db.Delete(&models.Task{}, id).Error
}

func (r *taskRepository) ListByWorkspaceID(workspaceID uint) ([]*models.Task, error) {
	var tasks []*models.Task
	err := r.db.Where("workspace_id = ?", workspaceID).
		Order("list_position ASC, created_at DESC, id ASC").
		Find(&tasks).Error
	return tasks, err
}
