package diff

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorktreeDiffFromBaseIncludesUntrackedFiles(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")
	worktreeDir := filepath.Join(root, "worktree")

	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatalf("mkdir repo dir: %v", err)
	}

	runGitCommand(t, repoDir, "init", "-b", "main")
	runGitCommand(t, repoDir, "config", "user.email", "test@example.com")
	runGitCommand(t, repoDir, "config", "user.name", "test")

	if err := os.WriteFile(filepath.Join(repoDir, "base.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatalf("write base file: %v", err)
	}
	runGitCommand(t, repoDir, "add", ".")
	runGitCommand(t, repoDir, "commit", "-m", "init")
	runGitCommand(t, repoDir, "worktree", "add", "-b", "test", worktreeDir)

	if err := os.WriteFile(filepath.Join(worktreeDir, "a.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("write worktree file: %v", err)
	}

	result, err := WorktreeDiffFromBase(worktreeDir, "main")
	if err != nil {
		t.Fatalf("WorktreeDiffFromBase: %v", err)
	}

	if !strings.Contains(result.Diff, "a.txt") {
		t.Fatalf("diff does not include new file: %s", result.Diff)
	}
	if len(result.Files) != 1 {
		t.Fatalf("files count = %d, want 1", len(result.Files))
	}
	if got := result.Files[0]["path"]; got != "a.txt" {
		t.Fatalf("path = %v, want a.txt", got)
	}
}

func runGitCommand(t *testing.T, dir string, args ...string) string {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, string(output))
	}

	return string(output)
}
