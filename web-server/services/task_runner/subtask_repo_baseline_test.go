package task_runner

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	coregit "matrixops.local/core_agent/git"
)

func TestSubtaskFileChangesSinceBaseline(t *testing.T) {
	repoDir := t.TempDir()
	runGit(t, repoDir, "init")
	runGit(t, repoDir, "config", "user.email", "test@example.com")
	runGit(t, repoDir, "config", "user.name", "Test")
	tracked := filepath.Join(repoDir, "tracked.txt")
	if err := os.WriteFile(tracked, []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, repoDir, "add", "tracked.txt")
	runGit(t, repoDir, "commit", "-m", "init")

	beforeState, err := coregit.GetRepoState(repoDir)
	if err != nil {
		t.Fatal(err)
	}
	baseline := coregit.FileSnapshotFromRepoState(beforeState)

	if err := os.WriteFile(tracked, []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "new.txt"), []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	modified, created := subtaskFileChangesSinceBaseline(repoDir, baseline)
	if len(modified) != 1 || modified[0] != "tracked.txt" {
		t.Fatalf("modified = %#v, want [tracked.txt]", modified)
	}
	if len(created) != 1 || created[0] != "new.txt" {
		t.Fatalf("created = %#v, want [new.txt]", created)
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}
