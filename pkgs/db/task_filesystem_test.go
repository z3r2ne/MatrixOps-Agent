package database

import (
	"os"
	"path/filepath"
	"testing"

	"pkgs/db/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestResolveProjectAIWorkspacePathUsesTaskWorkDirForGitWorktree(t *testing.T) {
	worktreePath := filepath.Join(t.TempDir(), "worktree")
	projectPath := filepath.Join(t.TempDir(), "project")
	got := ResolveProjectAIWorkspacePath(worktreePath, projectPath, projectPath)
	want := filepath.Join(worktreePath, "ai_workspace")
	if got != want {
		t.Fatalf("ResolveProjectAIWorkspacePath() = %q, want %q", got, want)
	}
}

func TestResolveProjectAIWorkspacePathUsesProjectWorktreePathForMainCheckout(t *testing.T) {
	projectPath := filepath.Join(t.TempDir(), "project")
	auxPath := filepath.Join(t.TempDir(), "aux")
	got := ResolveProjectAIWorkspacePath(projectPath, projectPath, auxPath)
	want := filepath.Join(auxPath, "ai_workspace")
	if got != want {
		t.Fatalf("ResolveProjectAIWorkspacePath() = %q, want %q", got, want)
	}
}

func TestSafeJoinUnderRootRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	if _, err := SafeJoinUnderRoot(root, "../escape"); err == nil {
		t.Fatal("expected traversal to be rejected")
	}
}

func TestSafeJoinUnderRootAllowsNestedPath(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "nested")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := SafeJoinUnderRoot(root, "nested")
	if err != nil {
		t.Fatal(err)
	}
	if got != nested {
		t.Fatalf("got %q, want %q", got, nested)
	}
}

func TestResolveTaskFilesystemRootsWithoutProject(t *testing.T) {
	workDir := t.TempDir()
	task := &models.Task{
		ID:        1,
		ProjectID: 0,
		WorkDir:   workDir,
	}
	roots, err := ResolveTaskFilesystemRoots(nil, task)
	if err != nil {
		t.Fatal(err)
	}
	if len(roots) != 1 {
		t.Fatalf("len(roots) = %d, want 1", len(roots))
	}
	if roots[0].ID != TaskFilesystemRootWorkspace {
		t.Fatalf("root id = %q", roots[0].ID)
	}
	want := AIWorkspacePath(workDir, "")
	if roots[0].Path != want {
		t.Fatalf("root path = %q, want %q", roots[0].Path, want)
	}
}

func TestResolveTaskAIWorkspaceDirPrefersSessionWorkspacePath(t *testing.T) {
	sessionWorkspace := filepath.Join(t.TempDir(), "worktrees", "MatrixOps-abc", "workspace")
	projectPath := filepath.Join(t.TempDir(), "project")
	task := &models.Task{
		ID:        2,
		ProjectID: 1,
		WorkDir:   projectPath,
		SessionID: "sess-1",
	}

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.Session{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.Session{
		ID:            "sess-1",
		ProjectID:     "1",
		Directory:     projectPath,
		WorkspacePath: sessionWorkspace,
		Created:       1,
		Updated:       1,
	}).Error; err != nil {
		t.Fatal(err)
	}

	got := resolveTaskAIWorkspaceDir(db, task)
	if got != sessionWorkspace {
		t.Fatalf("resolveTaskAIWorkspaceDir() = %q, want %q", got, sessionWorkspace)
	}
}
