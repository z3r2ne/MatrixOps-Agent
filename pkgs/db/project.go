package database

import (
	"pkgs/db/models"

	"gorm.io/gorm"
)

// Project 相关数据库操作

// GetProjectByID 根据 ID 获取项目
func GetProjectByID(db *gorm.DB, id uint) (*models.Project, error) {
	var project models.Project
	err := db.First(&project, id).Error
	return &project, err
}

func GetProjectByStringID(db *gorm.DB, id string) (*models.Project, error) {
	var project models.Project
	err := db.Where("id = ?", id).First(&project).Error
	return &project, err
}

// GetProjectsByWorkspaceID 根据工作区 ID 获取项目列表
func GetProjectsByWorkspaceID(db *gorm.DB, workspaceID uint) ([]models.Project, error) {
	workspace, err := GetWorkspaceByID(db, workspaceID)
	if err != nil {
		return nil, err
	}

	if len(workspace.ProjectIDs) == 0 {
		return []models.Project{}, nil
	}

	var projects []models.Project
	err = db.Where("id IN ?", workspace.ProjectIDs).Find(&projects).Error
	return projects, err
}

// CreateProject 创建项目
func CreateProject(db *gorm.DB, project *models.Project) error {
	return db.Create(project).Error
}

// UpdateProject 更新项目
func UpdateProject(db *gorm.DB, project *models.Project) error {
	return db.Save(project).Error
}

// UpdateProjectFields 更新项目的指定字段
func UpdateProjectFields(db *gorm.DB, projectID uint, updates map[string]interface{}) error {
	return db.Model(&models.Project{}).Where("id = ?", projectID).Updates(updates).Error
}

// UpdateProjectActiveTasks 更新项目的活跃任务数
func UpdateProjectActiveTasks(db *gorm.DB, projectID uint, count int) error {
	return UpdateProjectFields(db, projectID, map[string]interface{}{
		"active_tasks": count,
	})
}

// DeleteProject 删除项目
func DeleteProject(db *gorm.DB, projectID uint) error {
	return db.Delete(&models.Project{}, projectID).Error
}

// GetProjectIDsByWorkspaceID 根据工作区 ID 获取项目 ID 列表
func GetProjectIDsByWorkspaceID(db *gorm.DB, workspaceID uint) ([]uint, error) {
	workspace, err := GetWorkspaceByID(db, workspaceID)
	if err != nil {
		return nil, err
	}
	return workspace.ProjectIDs, nil
}

// DeleteProjectsBatch 批量删除项目
func DeleteProjectsBatch(db *gorm.DB, workspaceID uint, projectIDs []uint) error {
	return db.Delete(&models.Project{}, projectIDs).Error
}

func RemoveProjectFromAllWorkspaces(db *gorm.DB, projectID uint) error {
	workspaces, err := GetAllWorkspaces(db)
	if err != nil {
		return err
	}

	for _, workspace := range workspaces {
		nextProjectIDs := make([]uint, 0, len(workspace.ProjectIDs))
		changed := false
		for _, pid := range workspace.ProjectIDs {
			if pid == projectID {
				changed = true
				continue
			}
			nextProjectIDs = append(nextProjectIDs, pid)
		}
		if !changed {
			continue
		}
		if err := db.Model(&models.Workspace{}).
			Where("id = ?", workspace.ID).
			Update("project_ids", nextProjectIDs).Error; err != nil {
			return err
		}
	}

	return nil
}

func RemoveMemoryLibraryFromAllProjects(db *gorm.DB, memoryLibraryID uint) error {
	projects, err := GetAllProjects(db)
	if err != nil {
		return err
	}

	for _, project := range projects {
		if len(project.MemoryLibraryIDs) == 0 {
			continue
		}
		nextIDs := make([]uint, 0, len(project.MemoryLibraryIDs))
		changed := false
		for _, id := range project.MemoryLibraryIDs {
			if id == memoryLibraryID {
				changed = true
				continue
			}
			nextIDs = append(nextIDs, id)
		}
		if !changed {
			continue
		}
		if err := db.Model(&models.Project{}).Where("id = ?", project.ID).
			Select("MemoryLibraryIDs").
			Updates(models.Project{MemoryLibraryIDs: models.UintSlice(nextIDs)}).Error; err != nil {
			return err
		}
	}

	return nil
}

// GetAllProjects 获取所有项目
func GetAllProjects(db *gorm.DB) ([]models.Project, error) {
	var projects []models.Project
	err := db.Order("created_at DESC").Find(&projects).Error
	return projects, err
}

// GetProjectsWithoutWorkspace 获取未关联工作区的项目
func GetProjectsWithoutWorkspace(db *gorm.DB) ([]models.Project, error) {
	// 获取所有工作区
	workspaces, err := GetAllWorkspaces(db)
	if err != nil {
		return nil, err
	}

	// 收集所有已被关联的项目ID
	usedProjectIDs := make(map[uint]bool)
	for _, ws := range workspaces {
		for _, pid := range ws.ProjectIDs {
			usedProjectIDs[pid] = true
		}
	}

	// 获取所有项目
	allProjects, err := GetAllProjects(db)
	if err != nil {
		return nil, err
	}

	// 过滤出未被使用的项目
	var unusedProjects []models.Project
	for _, proj := range allProjects {
		if !usedProjectIDs[proj.ID] {
			unusedProjects = append(unusedProjects, proj)
		}
	}

	return unusedProjects, nil
}

// AddProjectToWorkspace 添加项目到工作区
func AddProjectToWorkspace(db *gorm.DB, workspaceID, projectID uint) error {
	workspace, err := GetWorkspaceByID(db, workspaceID)
	if err != nil {
		return err
	}

	// 检查项目是否已存在
	for _, pid := range workspace.ProjectIDs {
		if pid == projectID {
			return nil // 已存在，无需添加
		}
	}

	// 添加项目ID
	workspace.ProjectIDs = append(workspace.ProjectIDs, projectID)
	return db.Model(&models.Workspace{}).Where("id = ?", workspaceID).
		Save(&workspace).Error
}

// RemoveProjectFromWorkspace 从工作区移除项目
func RemoveProjectFromWorkspace(db *gorm.DB, workspaceID, projectID uint) error {
	workspace, err := GetWorkspaceByID(db, workspaceID)
	if err != nil {
		return err
	}

	// 移除项目ID
	newProjectIDs := make([]uint, 0)
	for _, pid := range workspace.ProjectIDs {
		if pid != projectID {
			newProjectIDs = append(newProjectIDs, pid)
		}
	}

	workspace.ProjectIDs = newProjectIDs
	return db.Model(&models.Workspace{}).
		Where("id = ?", workspaceID).
		Update("project_ids", workspace.ProjectIDs).Error
}

// IsProjectInWorkspace 检查项目是否在工作区中
func IsProjectInWorkspace(db *gorm.DB, workspaceID, projectID uint) (bool, error) {
	workspace, err := GetWorkspaceByID(db, workspaceID)
	if err != nil {
		return false, err
	}

	for _, pid := range workspace.ProjectIDs {
		if pid == projectID {
			return true, nil
		}
	}
	return false, nil
}
