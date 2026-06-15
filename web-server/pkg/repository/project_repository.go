package repository

import (
	"pkgs/db/models"

	"gorm.io/gorm"
)

// ProjectRepository 项目数据访问接口
type ProjectRepository interface {
	Create(project *models.Project) error
	GetByID(id uint) (*models.Project, error)
	Update(project *models.Project) error
	Delete(id uint) error
	ListByWorkspaceID(workspaceID uint) ([]*models.Project, error)
	UpdateActiveTasks(id uint, count int) error
}

type projectRepository struct {
	db *gorm.DB
}

// NewProjectRepository 创建项目仓储实例
func NewProjectRepository(db *gorm.DB) ProjectRepository {
	return &projectRepository{db: db}
}

func (r *projectRepository) Create(project *models.Project) error {
	return r.db.Create(project).Error
}

func (r *projectRepository) GetByID(id uint) (*models.Project, error) {
	var project models.Project
	err := r.db.First(&project, id).Error
	if err != nil {
		return nil, err
	}
	return &project, nil
}

func (r *projectRepository) Update(project *models.Project) error {
	return r.db.Save(project).Error
}

func (r *projectRepository) Delete(id uint) error {
	return r.db.Delete(&models.Project{}, id).Error
}

func (r *projectRepository) ListByWorkspaceID(workspaceID uint) ([]*models.Project, error) {
	var projects []*models.Project
	err := r.db.Where("workspace_id = ?", workspaceID).Order("created_at DESC").Find(&projects).Error
	return projects, err
}

func (r *projectRepository) UpdateActiveTasks(id uint, count int) error {
	return r.db.Model(&models.Project{}).Where("id = ?", id).Update("active_tasks", count).Error
}
