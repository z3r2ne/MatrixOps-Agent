package git

import (
	coregit "matrixops.local/core_agent/git"

	"matrixops/pkg/repository"
)

// Service Git 服务
type Service struct {
	projectRepo   repository.ProjectRepository
	executionRepo repository.ExecutionRepository
	taskRepo      repository.TaskRepository
}

// NewService 创建 Git 服务
func NewService(
	projectRepo repository.ProjectRepository,
	executionRepo repository.ExecutionRepository,
	taskRepo repository.TaskRepository,
) *Service {
	return &Service{
		projectRepo:   projectRepo,
		executionRepo: executionRepo,
		taskRepo:      taskRepo,
	}
}

// IsGitRepo 检查路径是否是 Git 仓库
func (s *Service) IsGitRepo(path string) bool {
	return coregit.IsGitRepo(path)
}

// InitRepo 初始化 Git 仓库
func (s *Service) InitRepo(path string) error {
	_, err := coregit.InitRepo(path)
	return err
}

// GetCurrentBranch 获取当前分支
func (s *Service) GetCurrentBranch(path string) (string, error) {
	return coregit.CurrentBranch(path)
}

// CommitChanges 提交更改
func (s *Service) CommitChanges(workDir, message string) (string, error) {
	if _, err := coregit.AddAll(workDir); err != nil {
		return "", err
	}
	if _, err := coregit.Commit(workDir, message); err != nil {
		return "", err
	}
	return coregit.HeadCommit(workDir)
}

// GetDiff 获取 diff
func (s *Service) GetDiff(workDir string, staged bool) (string, error) {
	if staged {
		output, err := coregit.RawDiff(workDir, true)
		if err != nil {
			return "", err
		}
		return output, nil
	}
	output, err := coregit.RawDiff(workDir, false)
	if err != nil {
		return "", err
	}
	return output, nil
}
