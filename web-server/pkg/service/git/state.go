package git

import coregit "matrixops.local/core_agent/git"

// RepoState Git 仓库状态
type RepoState = coregit.RepoState

// GetRepoState 获取 Git 仓库状态
func GetRepoState(workDir string) (*RepoState, error) {
	return coregit.GetRepoState(workDir)
}

// RestoreRepoState 恢复 Git 状态到指定 commit
func RestoreRepoState(workDir string, commitHash string, forceWhenDirty bool) error {
	return coregit.RestoreRepoState(workDir, commitHash, forceWhenDirty)
}
