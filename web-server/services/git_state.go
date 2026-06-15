package services

import coregit "matrixops.local/core_agent/git"

// GitRepoState Git 仓库状态
type GitRepoState = coregit.RepoState

// GetGitRepoState 获取 Git 仓库状态
func GetGitRepoState(workDir string) (*GitRepoState, error) {
	return coregit.GetRepoState(workDir)
}
