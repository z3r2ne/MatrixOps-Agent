package git

import (
	"fmt"
	"os/exec"
	"strings"
)

func run(workDir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = workDir
	output, err := cmd.CombinedOutput()
	text := string(output)
	if err != nil {
		trimmed := strings.TrimSpace(text)
		if trimmed == "" {
			return text, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
		}
		return text, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, trimmed)
	}
	return text, nil
}

func AddAll(workDir string) (string, error) {
	return run(workDir, "add", "-A")
}

func InitRepo(workDir string) (string, error) {
	return run(workDir, "init")
}

func HeadCommit(workDir string) (string, error) {
	output, err := run(workDir, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(output), nil
}

func Commit(workDir, message string) (string, error) {
	return run(workDir, "commit", "-m", message)
}

func Checkout(workDir, branch string) (string, error) {
	return run(workDir, "checkout", branch)
}

func Merge(workDir, branch, message string) (string, error) {
	return run(workDir, "merge", branch, "-m", message)
}

func NothingToCommit(output string) bool {
	return strings.Contains(strings.ToLower(output), "nothing to commit")
}
