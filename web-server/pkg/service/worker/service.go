package worker

import (
	"errors"
	"os"

	"matrixops/pkg/repository"
	"pkgs/db/models"
)

// Service Worker 服务
type Service struct {
	projectRepo repository.ProjectRepository
}

// NewService 创建 Worker 服务
func NewService(projectRepo repository.ProjectRepository) *Service {
	return &Service{
		projectRepo: projectRepo,
	}
}

// ResolveWorkingDir 解析工作目录（从项目或配置）
func (s *Service) ResolveWorkingDir(projectID uint, cfg map[string]interface{}) (string, error) {
	if override := ToString(cfg["working_dir"]); override != "" {
		if stat, err := os.Stat(override); err == nil && stat.IsDir() {
			return override, nil
		}
		return "", errors.New("Worker 工作目录不存在或不可访问")
	}

	project, err := s.projectRepo.GetByID(projectID)
	if err != nil {
		return "", errors.New("项目不存在，无法执行任务")
	}
	if stat, err := os.Stat(project.Path); err != nil || !stat.IsDir() {
		return "", errors.New("项目路径不存在或不可访问")
	}
	return project.Path, nil
}

// ResolveTaskWorkingDir 解析任务的工作目录，优先使用任务的 WorkDir
func (s *Service) ResolveTaskWorkingDir(task models.Task, cfg map[string]interface{}) (string, error) {
	// 1. 优先使用任务指定的工作目录（worktree 路径）
	if task.WorkDir != "" {
		if stat, err := os.Stat(task.WorkDir); err == nil && stat.IsDir() {
			return task.WorkDir, nil
		}
		return "", errors.New("任务工作目录不存在或不可访问: " + task.WorkDir)
	}

	// 2. 使用 Worker 配置的工作目录
	if override := ToString(cfg["working_dir"]); override != "" {
		if stat, err := os.Stat(override); err == nil && stat.IsDir() {
			return override, nil
		}
		return "", errors.New("Worker 工作目录不存在或不可访问")
	}

	// 3. 使用项目路径
	project, err := s.projectRepo.GetByID(task.ProjectID)
	if err != nil {
		return "", errors.New("项目不存在，无法执行任务")
	}
	if stat, err := os.Stat(project.Path); err != nil || !stat.IsDir() {
		return "", errors.New("项目路径不存在或不可访问")
	}
	return project.Path, nil
}
