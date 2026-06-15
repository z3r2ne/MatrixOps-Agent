package repository

import (
	"pkgs/db/models"

	"gorm.io/gorm"
)

// WorkspaceRepository 工作空间数据访问接口
type WorkspaceRepository interface {
	Create(workspace *models.Workspace) error
	GetByID(id uint) (*models.Workspace, error)
	GetByIDWithProjects(id uint) (*models.Workspace, error)
	Update(workspace *models.Workspace) error
	Delete(id uint) error
	List() ([]*models.Workspace, error)
	GetActive() (*models.Workspace, error)
	SetActive(id uint) error
	DeactivateAll() error
}

type workspaceRepository struct {
	db *gorm.DB
}

// NewWorkspaceRepository 创建工作空间仓储实例
func NewWorkspaceRepository(db *gorm.DB) WorkspaceRepository {
	return &workspaceRepository{db: db}
}

func (r *workspaceRepository) Create(workspace *models.Workspace) error {
	return r.db.Create(workspace).Error
}

func (r *workspaceRepository) GetByID(id uint) (*models.Workspace, error) {
	var workspace models.Workspace
	err := r.db.First(&workspace, id).Error
	if err != nil {
		return nil, err
	}
	return &workspace, nil
}

func (r *workspaceRepository) GetByIDWithProjects(id uint) (*models.Workspace, error) {
	var workspace models.Workspace
	err := r.db.Preload("Projects").First(&workspace, id).Error
	if err != nil {
		return nil, err
	}
	return &workspace, nil
}

func (r *workspaceRepository) Update(workspace *models.Workspace) error {
	return r.db.Save(workspace).Error
}

func (r *workspaceRepository) Delete(id uint) error {
	return r.db.Delete(&models.Workspace{}, id).Error
}

func (r *workspaceRepository) List() ([]*models.Workspace, error) {
	var workspaces []*models.Workspace
	err := r.db.Order("created_at DESC").Find(&workspaces).Error
	return workspaces, err
}

func (r *workspaceRepository) GetActive() (*models.Workspace, error) {
	var workspace models.Workspace
	err := r.db.Where("active = ?", true).First(&workspace).Error
	if err != nil {
		return nil, err
	}
	return &workspace, nil
}

func (r *workspaceRepository) SetActive(id uint) error {
	return r.db.Model(&models.Workspace{}).Where("id = ?", id).Update("active", true).Error
}

func (r *workspaceRepository) DeactivateAll() error {
	return r.db.Model(&models.Workspace{}).Where("active = ?", true).Update("active", false).Error
}
