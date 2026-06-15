package session

import (
	"path/filepath"
	"testing"
)

func TestResolveProjectAIWorkspacePathUsesTaskWorkDirForGitWorktree(t *testing.T) {
	projectPath := filepath.Join("/tmp", "demo-project")
	worktreePath := filepath.Join("/tmp", ".matrixops", "worktrees", "demo-project-abc-feature")

	got := resolveProjectAIWorkspacePath(worktreePath, projectPath, projectPath)
	want := filepath.Join(worktreePath, "ai_workspace")
	if got != want {
		t.Fatalf("resolveProjectAIWorkspacePath() = %q, want %q", got, want)
	}
}

func TestResolveProjectAIWorkspacePathUsesProjectWorktreePathForMainCheckout(t *testing.T) {
	projectPath := filepath.Join("/tmp", "demo-project")
	auxPath := filepath.Join("/tmp", "demo-project-aux")

	got := resolveProjectAIWorkspacePath(projectPath, projectPath, auxPath)
	want := filepath.Join(auxPath, "ai_workspace")
	if got != want {
		t.Fatalf("resolveProjectAIWorkspacePath() = %q, want %q", got, want)
	}
}
