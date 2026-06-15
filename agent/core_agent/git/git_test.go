package git

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

func TestGetRepoStateIncludesModifiedAndUntrackedFiles(t *testing.T) {
	repoDir := initTestRepo(t)

	if err := os.WriteFile(filepath.Join(repoDir, "tracked.txt"), []byte("changed\n"), 0o644); err != nil {
		t.Fatalf("rewrite tracked file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "new.txt"), []byte("new\n"), 0o644); err != nil {
		t.Fatalf("write untracked file: %v", err)
	}

	state, err := GetRepoState(repoDir)
	if err != nil {
		t.Fatalf("GetRepoState: %v", err)
	}

	if !state.IsDirty {
		t.Fatal("expected repo to be dirty")
	}
	if state.ModifiedCount != 1 || len(state.ModifiedFiles) != 1 || state.ModifiedFiles[0] != "tracked.txt" {
		t.Fatalf("unexpected modified files: %+v", state.ModifiedFiles)
	}
	if state.UntrackedCount != 1 || len(state.UntrackedFiles) != 1 || state.UntrackedFiles[0] != "new.txt" {
		t.Fatalf("unexpected untracked files: %+v", state.UntrackedFiles)
	}
}

func TestListBranchesAndCurrentBranch(t *testing.T) {
	repoDir := initTestRepo(t)
	remoteDir := filepath.Join(t.TempDir(), "remote.git")
	runGitCommand(t, "", "init", "--bare", remoteDir)
	runGitCommand(t, repoDir, "remote", "add", "origin", remoteDir)
	runGitCommand(t, repoDir, "push", "-u", "origin", "main")

	runGitCommand(t, repoDir, "checkout", "-b", "feature/test")
	runGitCommand(t, repoDir, "push", "-u", "origin", "feature/test")

	branch, err := CurrentBranch(repoDir)
	if err != nil {
		t.Fatalf("CurrentBranch: %v", err)
	}
	if branch != "feature/test" {
		t.Fatalf("branch = %q, want %q", branch, "feature/test")
	}

	branches, err := ListBranches(repoDir)
	if err != nil {
		t.Fatalf("ListBranches: %v", err)
	}

	foundMain := false
	foundFeature := false
	foundRemoteFeature := false
	for _, item := range branches {
		if item.Name == "main" {
			foundMain = true
		}
		if item.Name == "feature/test" {
			foundFeature = true
			if !item.IsCurrent {
				t.Fatalf("expected feature/test to be current: %+v", item)
			}
			if item.IsRemote {
				t.Fatalf("expected local feature/test not remote: %+v", item)
			}
		}
		if item.Name == "origin/feature/test" {
			foundRemoteFeature = true
			if !item.IsRemote {
				t.Fatalf("expected origin/feature/test to be remote: %+v", item)
			}
		}
	}

	if !foundMain || !foundFeature || !foundRemoteFeature {
		t.Fatalf("expected main, feature/test and origin/feature/test branches, got %+v", branches)
	}
}

func TestDefaultBranch(t *testing.T) {
	repoDir := initTestRepo(t)
	remoteDir := filepath.Join(t.TempDir(), "remote.git")
	runGitCommand(t, "", "init", "--bare", remoteDir)
	runGitCommand(t, repoDir, "remote", "add", "origin", remoteDir)
	runGitCommand(t, repoDir, "push", "-u", "origin", "main")

	branch, err := DefaultBranch(repoDir)
	if err != nil {
		t.Fatalf("DefaultBranch: %v", err)
	}
	if branch != "main" {
		t.Fatalf("branch = %q, want %q", branch, "main")
	}
}

func TestListBranchesFiltersDetachedHEADPseudoBranch(t *testing.T) {
	repoDir := initTestRepo(t)
	remoteDir := filepath.Join(t.TempDir(), "remote.git")
	runGitCommand(t, "", "init", "--bare", remoteDir)
	runGitCommand(t, repoDir, "remote", "add", "origin", remoteDir)
	runGitCommand(t, repoDir, "push", "-u", "origin", "main")
	runGitCommand(t, repoDir, "checkout", "-b", "feature/test")
	runGitCommand(t, repoDir, "push", "-u", "origin", "feature/test")
	runGitCommand(t, repoDir, "checkout", "--detach", "origin/feature/test")

	branches, err := ListBranches(repoDir)
	if err != nil {
		t.Fatalf("ListBranches: %v", err)
	}
	for _, item := range branches {
		if strings.HasPrefix(item.Name, "(") {
			t.Fatalf("unexpected detached HEAD pseudo branch: %+v", item)
		}
	}
}

func initTestRepo(t *testing.T) string {
	t.Helper()

	repoDir := t.TempDir()
	runGitCommand(t, repoDir, "init", "-b", "main")
	runGitCommand(t, repoDir, "config", "user.email", "test@example.com")
	runGitCommand(t, repoDir, "config", "user.name", "test")

	if err := os.WriteFile(filepath.Join(repoDir, "tracked.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatalf("write tracked file: %v", err)
	}
	runGitCommand(t, repoDir, "add", ".")
	runGitCommand(t, repoDir, "commit", "-m", "init")
	return repoDir
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
