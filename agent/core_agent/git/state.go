package git

import (
	"fmt"
	"strings"
)

func GetRepoState(workDir string) (*RepoState, error) {
	state := &RepoState{}

	commitHash, err := HeadCommit(workDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get commit hash: %w", err)
	}
	state.CommitHash = commitHash

	branch, err := CurrentBranch(workDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get branch: %w", err)
	}
	state.Branch = branch

	modifiedOutput, _ := run(workDir, "diff", "--name-only", "HEAD")
	if trimmed := strings.TrimSpace(modifiedOutput); trimmed != "" {
		state.ModifiedFiles = strings.Split(trimmed, "\n")
		state.ModifiedCount = len(state.ModifiedFiles)
	}

	untrackedOutput, _ := run(workDir, "ls-files", "--others", "--exclude-standard")
	if trimmed := strings.TrimSpace(untrackedOutput); trimmed != "" {
		state.UntrackedFiles = strings.Split(trimmed, "\n")
		state.UntrackedCount = len(state.UntrackedFiles)
	}

	state.IsDirty = state.ModifiedCount > 0 || state.UntrackedCount > 0
	return state, nil
}

func RestoreRepoState(workDir string, commitHash string, forceWhenDirty bool) error {
	state, err := GetRepoState(workDir)
	if err != nil {
		return err
	}

	if state.IsDirty && !forceWhenDirty {
		return fmt.Errorf("工作目录有未提交的更改，无法恢复（已修改: %d, 未跟踪: %d）",
			state.ModifiedCount, state.UntrackedCount)
	}

	resetOutput, err := run(workDir, "reset", "--hard", commitHash)
	if err != nil {
		return fmt.Errorf("git reset 失败: %w, output: %s", err, resetOutput)
	}

	cleanOutput, err := run(workDir, "clean", "-fd")
	if err != nil {
		return fmt.Errorf("git clean 失败: %w, output: %s", err, cleanOutput)
	}

	return nil
}
