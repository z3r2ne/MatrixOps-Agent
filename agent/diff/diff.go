package diff

import (
	coregit "matrixops.local/core_agent/git"
)

type Result = coregit.Result

func SnapshotDiff(projectID string, directory string, hash string) (Result, error) {
	return coregit.SnapshotDiff(projectID, directory, hash)
}

func SnapshotDiffRange(projectID string, directory string, from string, to string) (Result, error) {
	return coregit.SnapshotDiffRange(projectID, directory, from, to)
}

func WorkingTreeDiff(workDir string) (Result, error) {
	return coregit.WorkingTreeDiff(workDir)
}

func WorktreeDiffFromBase(workDir string, baseRef string) (Result, error) {
	return coregit.WorktreeDiffFromBase(workDir, baseRef)
}

func BranchDiffFromMain(workDir string, branch string) (Result, error) {
	return coregit.BranchDiffFromMain(workDir, branch)
}

func BranchDiff(workDir string, from string, to string) (Result, error) {
	return coregit.BranchDiff(workDir, from, to)
}

func ParseDiffFiles(diffOutput string) []map[string]interface{} {
	return coregit.ParseDiffFiles(diffOutput)
}
