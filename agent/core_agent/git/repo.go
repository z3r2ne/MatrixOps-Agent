package git

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	database "pkgs/db"
)

func IsGitRepo(path string) bool {
	gitDir := filepath.Join(path, ".git")
	_, err := os.Stat(gitDir)
	return err == nil
}

func CurrentBranch(workDir string) (string, error) {
	output, err := run(workDir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(output), nil
}

func DefaultBranch(workDir string) (string, error) {
	output, err := run(workDir, "branch", "-r")
	if err != nil {
		return "", err
	}

	defaultRemoteBranch := ""
	remoteBranchNames := make([]string, 0)
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !strings.Contains(line, "->") {
			remoteBranchNames = append(remoteBranchNames, line)
			continue
		}

		parts := strings.SplitN(line, "->", 2)
		if len(parts) != 2 {
			continue
		}

		left := strings.TrimSpace(parts[0])
		right := strings.TrimSpace(parts[1])
		if !strings.HasSuffix(left, "/HEAD") || right == "" {
			continue
		}
		defaultRemoteBranch = right
		break
	}

	branches, listErr := ListBranches(workDir)
	if defaultRemoteBranch != "" {
		if _, shortName, ok := strings.Cut(defaultRemoteBranch, "/"); ok && shortName != "" {
			if listErr == nil {
				for _, branch := range branches {
					if branch.Name == shortName && !branch.IsRemote {
						return shortName, nil
					}
				}
			}
			return defaultRemoteBranch, nil
		}
		return defaultRemoteBranch, nil
	}

	if listErr == nil {
		for _, preferred := range []string{"main", "master"} {
			for _, branch := range branches {
				if branch.Name == preferred && !branch.IsRemote {
					return branch.Name, nil
				}
			}
		}
	}

	for _, preferred := range []string{"origin/main", "origin/master"} {
		for _, branch := range remoteBranchNames {
			if branch == preferred {
				return branch, nil
			}
		}
	}

	return "", fmt.Errorf("default branch not found")
}

func ListBranches(workDir string) ([]BranchInfo, error) {
	remoteOutput, _ := run(workDir, "remote")
	remoteNames := map[string]struct{}{}
	for _, remote := range strings.Split(strings.TrimSpace(remoteOutput), "\n") {
		remote = strings.TrimSpace(remote)
		if remote != "" {
			remoteNames[remote] = struct{}{}
		}
	}

	output, err := run(workDir, "for-each-ref", "--format=%(refname:short)|%(HEAD)", "refs/heads", "refs/remotes")
	if err != nil {
		return nil, err
	}

	branches := make([]BranchInfo, 0)
	seen := map[string]struct{}{}
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "|")
		name := strings.TrimSpace(parts[0])
		if name == "" || strings.HasPrefix(name, "(") || strings.HasPrefix(name, "（") || strings.HasSuffix(name, "/HEAD") || strings.Contains(name, "HEAD ->") || strings.Contains(strings.ToLower(name), "detached") || strings.Contains(name, "头指针") || strings.Contains(name, "分离") {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		isCurrent := len(parts) > 1 && strings.TrimSpace(parts[1]) == "*"
		isRemote := false
		if head, _, ok := strings.Cut(name, "/"); ok {
			_, isRemote = remoteNames[head]
		}
		branches = append(branches, BranchInfo{
			Name:      name,
			IsCurrent: isCurrent,
			IsRemote:  isRemote,
		})
	}
	sort.Slice(branches, func(i, j int) bool {
		if branches[i].IsCurrent != branches[j].IsCurrent {
			return branches[i].IsCurrent
		}
		if branches[i].IsRemote != branches[j].IsRemote {
			return branches[i].IsRemote
		}
		return branches[i].Name < branches[j].Name
	})

	return branches, nil
}

func WorktreePath(projectWorktreePath, newBranch string) string {
	return database.WorktreePath(projectWorktreePath, newBranch)
}

func CreateWorktree(projectPath, projectWorktreePath, newBranch, baseBranch string) (string, error) {
	worktreePath := WorktreePath(projectWorktreePath, newBranch)
	if _, err := os.Stat(worktreePath); !os.IsNotExist(err) {
		return "", fmt.Errorf("worktree 路径已存在: %s", worktreePath)
	}
	if _, err := run(projectPath, "worktree", "add", "-b", newBranch, worktreePath, baseBranch); err != nil {
		return "", err
	}
	return worktreePath, nil
}

func ListWorktrees(projectPath string) ([]WorktreeInfo, error) {
	output, err := run(projectPath, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}

	worktrees := make([]WorktreeInfo, 0)
	var current WorktreeInfo
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			if current.Path != "" {
				worktrees = append(worktrees, current)
				current = WorktreeInfo{}
			}
			continue
		}
		switch {
		case strings.HasPrefix(line, "worktree "):
			current.Path = strings.TrimPrefix(line, "worktree ")
		case strings.HasPrefix(line, "HEAD "):
			current.Head = strings.TrimPrefix(line, "HEAD ")
		case strings.HasPrefix(line, "branch "):
			branch := strings.TrimPrefix(line, "branch ")
			current.Branch = strings.TrimPrefix(branch, "refs/heads/")
		}
	}
	if current.Path != "" {
		worktrees = append(worktrees, current)
	}

	return worktrees, nil
}

func RemoveWorktree(projectPath, worktreePath string) error {
	if _, err := run(projectPath, "worktree", "remove", worktreePath, "--force"); err != nil {
		return err
	}
	return nil
}
