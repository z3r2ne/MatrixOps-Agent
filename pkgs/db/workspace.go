package database

import (
	"pkgs/db/models"

	"gorm.io/gorm"
)

// Workspace 相关数据库操作

// GetAllWorkspaces 获取所有工作区（带项目）
func GetAllWorkspaces(db *gorm.DB) ([]models.Workspace, error) {
	var workspaces []models.Workspace
	err := db.Order("created_at DESC").Find(&workspaces).Error
	if err != nil {
		return nil, err
	}

	// 收集所有需要查询的项目ID
	projectIDsMap := make(map[uint]bool)
	for _, ws := range workspaces {
		for _, pid := range ws.ProjectIDs {
			projectIDsMap[pid] = true
		}

	}

	// 批量查询所有项目
	if len(projectIDsMap) > 0 {
		projectIDs := make([]uint, 0, len(projectIDsMap))
		for pid := range projectIDsMap {
			projectIDs = append(projectIDs, pid)
		}

		var allProjects []models.Project
		err = db.Where("id IN ?", projectIDs).Find(&allProjects).Error
		if err != nil {
			return workspaces, nil // 即使项目查询失败，也返回工作区列表
		}

		// 创建项目ID到项目对象的映射
		projectsMap := make(map[uint]models.Project)
		for _, proj := range allProjects {
			projectsMap[proj.ID] = proj
		}

		// 为每个工作区填充项目列表
		for i := range workspaces {
			if len(workspaces[i].ProjectIDs) > 0 {
				projects := make([]models.Project, 0, len(workspaces[i].ProjectIDs))
				for _, pid := range workspaces[i].ProjectIDs {
					if proj, exists := projectsMap[pid]; exists {
						projects = append(projects, proj)
					}
				}
				workspaces[i].Projects = projects
			}
		}
	}

	return workspaces, nil
}

// GetWorkspaceByID 根据 ID 获取工作区
func GetWorkspaceByID(db *gorm.DB, id uint) (*models.Workspace, error) {
	var workspace models.Workspace
	err := db.First(&workspace, id).Error
	return &workspace, err
}

// GetWorkspaceByIDWithProjects 根据 ID 获取工作区（带项目）
func GetWorkspaceByIDWithProjects(db *gorm.DB, id uint) (*models.Workspace, error) {
	var workspace models.Workspace
	err := db.First(&workspace, id).Error
	if err != nil {
		return nil, err
	}

	// 手动填充项目列表
	if len(workspace.ProjectIDs) > 0 {
		var projects []models.Project
		err = db.Where("id IN ?", workspace.ProjectIDs).Find(&projects).Error
		if err != nil {
			return nil, err
		}
		workspace.Projects = projects
	}

	return &workspace, nil
}

// CreateWorkspace 创建工作区
func CreateWorkspace(db *gorm.DB, workspace *models.Workspace) error {
	return db.Create(workspace).Error
}

// UpdateWorkspace 更新工作区
func UpdateWorkspace(db *gorm.DB, workspace *models.Workspace) error {
	return db.Save(workspace).Error
}

// UpdateWorkspaceFields 更新工作区的指定字段
func UpdateWorkspaceFields(db *gorm.DB, workspaceID uint, updates map[string]interface{}) error {
	return db.Model(&models.Workspace{}).Where("id = ?", workspaceID).Updates(updates).Error
}

// DeleteWorkspace 删除工作区
func DeleteWorkspace(db *gorm.DB, workspaceID uint) error {
	return db.Delete(&models.Workspace{}, workspaceID).Error
}

// DeleteWorkspacesBatch 批量删除工作区
func DeleteWorkspacesBatch(db *gorm.DB, workspaceIDs []uint) error {
	return db.Delete(&models.Workspace{}, workspaceIDs).Error
}

// DeactivateAllWorkspaces 将所有工作区设为非活跃
func DeactivateAllWorkspaces(db *gorm.DB) error {
	return db.Model(&models.Workspace{}).Where("active = ?", true).Update("active", false).Error
}

// SetActiveWorkspace 设置活跃工作区
func SetActiveWorkspace(db *gorm.DB, workspaceID uint) error {
	// 先将所有工作区设为非活跃
	if err := DeactivateAllWorkspaces(db); err != nil {
		return err
	}
	// 设置指定工作区为活跃
	return UpdateWorkspaceFields(db, workspaceID, map[string]interface{}{
		"active": true,
	})
}

// GetFirstWorkspaceIDContainingProject 返回第一个在 project_ids 中包含该项目的 workspace ID。
func GetFirstWorkspaceIDContainingProject(db *gorm.DB, projectID uint) (uint, bool, error) {
	var workspaces []models.Workspace
	if err := db.Find(&workspaces).Error; err != nil {
		return 0, false, err
	}
	for _, w := range workspaces {
		for _, pid := range w.ProjectIDs {
			if pid == projectID {
				return w.ID, true, nil
			}
		}
	}
	return 0, false, nil
}

// GetWorkspaceByProjectID 根据项目 ID 查找所属工作区。
func GetWorkspaceByProjectID(db *gorm.DB, projectID uint) (*models.Workspace, error) {
	workspaces, err := GetAllWorkspaces(db)
	if err != nil {
		return nil, err
	}
	for _, workspace := range workspaces {
		for _, pid := range workspace.ProjectIDs {
			if pid == projectID {
				ws := workspace
				return &ws, nil
			}
		}
	}
	return nil, gorm.ErrRecordNotFound
}
