package snapshot

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveAlternateObjectDir_FromGitDirectory(t *testing.T) {
	root := t.TempDir()
	objectsDir := filepath.Join(root, ".git", "objects")
	if err := os.MkdirAll(objectsDir, 0o755); err != nil {
		t.Fatalf("mkdir objects dir: %v", err)
	}

	resolved, err := resolveAlternateObjectDir(root)
	if err != nil {
		t.Fatalf("resolveAlternateObjectDir returned error: %v", err)
	}
	if resolved != objectsDir {
		t.Fatalf("resolved = %q, want %q", resolved, objectsDir)
	}
}

func TestResolveAlternateObjectDir_FromWorktreeGitFile(t *testing.T) {
	root := t.TempDir()
	commonGitDir := filepath.Join(root, "repo", ".git")
	worktreeGitDir := filepath.Join(commonGitDir, "worktrees", "feature")
	objectsDir := filepath.Join(commonGitDir, "objects")
	if err := os.MkdirAll(objectsDir, 0o755); err != nil {
		t.Fatalf("mkdir objects dir: %v", err)
	}
	if err := os.MkdirAll(worktreeGitDir, 0o755); err != nil {
		t.Fatalf("mkdir worktree git dir: %v", err)
	}

	worktreeDir := filepath.Join(root, "worktree")
	if err := os.MkdirAll(worktreeDir, 0o755); err != nil {
		t.Fatalf("mkdir worktree dir: %v", err)
	}
	gitFile := filepath.Join(worktreeDir, ".git")
	if err := os.WriteFile(gitFile, []byte("gitdir: ../repo/.git/worktrees/feature\n"), 0o644); err != nil {
		t.Fatalf("write git file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(worktreeGitDir, "commondir"), []byte("../..\n"), 0o644); err != nil {
		t.Fatalf("write commondir: %v", err)
	}

	resolved, err := resolveAlternateObjectDir(worktreeDir)
	if err != nil {
		t.Fatalf("resolveAlternateObjectDir returned error: %v", err)
	}
	if resolved != objectsDir {
		t.Fatalf("resolved = %q, want %q", resolved, objectsDir)
	}
}
