package database

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewSessionWorkspaceCreatesExpectedLayout(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(envMatrixopsHome, tmp)

	root, workspace, err := NewSessionWorkspace("demo-project", "/tmp/fallback")
	if err != nil {
		t.Fatalf("NewSessionWorkspace: %v", err)
	}
	if !strings.HasPrefix(root, filepath.Join(tmp, "worktrees")+string(os.PathSeparator)) {
		t.Fatalf("workspace root = %q", root)
	}
	if filepath.Base(workspace) != "workspace" {
		t.Fatalf("workspace path = %q", workspace)
	}
	if filepath.Dir(workspace) != root {
		t.Fatalf("workspace parent = %q, want %q", filepath.Dir(workspace), root)
	}
	if _, err := os.Stat(workspace); err != nil {
		t.Fatalf("stat workspace: %v", err)
	}
}

func TestNewTaskWorkspaceCreatesExpectedLayout(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(envMatrixopsHome, tmp)

	root, workspace, err := NewTaskWorkspace(42, "demo-project", "/tmp/fallback")
	if err != nil {
		t.Fatalf("NewTaskWorkspace: %v", err)
	}
	if !strings.HasPrefix(root, filepath.Join(tmp, "tasks")+string(os.PathSeparator)) {
		t.Fatalf("workspace root = %q", root)
	}
	if !strings.Contains(filepath.Base(root), "task-42-") {
		t.Fatalf("workspace root should contain task-42-, got %q", root)
	}
	if filepath.Base(workspace) != "workspace" {
		t.Fatalf("workspace path = %q", workspace)
	}
	if filepath.Dir(workspace) != root {
		t.Fatalf("workspace parent = %q, want %q", filepath.Dir(workspace), root)
	}
	if _, err := os.Stat(workspace); err != nil {
		t.Fatalf("stat workspace: %v", err)
	}
}

func TestWorktreePathUsesGlobalDataDir(t *testing.T) {
	dataDir := t.TempDir()
	projectDir := t.TempDir()
	t.Setenv(envMatrixopsHome, dataDir)

	worktree := WorktreePath(projectDir, "feature/test")
	if !strings.HasPrefix(worktree, filepath.Join(dataDir, "worktrees")+string(os.PathSeparator)) {
		t.Fatalf("worktree path = %q, want under global data dir %q", worktree, filepath.Join(dataDir, "worktrees"))
	}
	if strings.HasPrefix(worktree, filepath.Join(projectDir, "worktrees")+string(os.PathSeparator)) {
		t.Fatalf("worktree path should not be under project dir: %q", worktree)
	}
	if !strings.Contains(filepath.Base(worktree), "feature_test") {
		t.Fatalf("worktree path should include sanitized branch name, got %q", worktree)
	}
}

func TestProjectDataDirUsesGlobalDataDir(t *testing.T) {
	dataDir := t.TempDir()
	projectDir := t.TempDir()
	t.Setenv(envMatrixopsHome, dataDir)

	path, err := ProjectDataDir("42", projectDir)
	if err != nil {
		t.Fatalf("ProjectDataDir: %v", err)
	}
	if !strings.HasPrefix(path, filepath.Join(dataDir, "projects")+string(os.PathSeparator)) {
		t.Fatalf("project data dir = %q, want under %q", path, filepath.Join(dataDir, "projects"))
	}
	if !strings.Contains(filepath.Base(path), "project-42-") {
		t.Fatalf("project data dir should include project id, got %q", path)
	}
}

func TestProjectCodeMapFilePathUsesProjectDataDir(t *testing.T) {
	dataDir := t.TempDir()
	projectDir := t.TempDir()
	t.Setenv(envMatrixopsHome, dataDir)

	path, err := ProjectCodeMapFilePath("7", projectDir)
	if err != nil {
		t.Fatalf("ProjectCodeMapFilePath: %v", err)
	}
	if filepath.Base(path) != "code-map.md" {
		t.Fatalf("unexpected code map file name: %q", path)
	}
	if !strings.HasPrefix(filepath.Dir(path), filepath.Join(dataDir, "projects")+string(os.PathSeparator)) {
		t.Fatalf("code map path = %q, want under project data dir", path)
	}
}
