package git

import (
	"errors"
	"strings"

	"matrixops-agent/snapshot"
)

func RawDiff(workDir string, staged bool) (string, error) {
	if workDir == "" {
		return "", errors.New("workdir required")
	}
	args := []string{"diff"}
	if staged {
		args = append(args, "--staged")
	}
	return run(workDir, args...)
}

func SnapshotDiff(projectID, directory, hash string) (Result, error) {
	if hash == "" {
		return Result{}, errors.New("snapshot hash required")
	}
	diffOutput, err := snapshot.Diff(projectID, directory, hash)
	if err != nil {
		return Result{}, err
	}
	return newResult("snapshot", diffOutput), nil
}

func SnapshotDiffRange(projectID, directory, from, to string) (Result, error) {
	if from == "" || to == "" {
		return Result{}, errors.New("both snapshot hashes are required")
	}
	diffOutput, err := snapshot.DiffRange(projectID, directory, from, to)
	if err != nil {
		return Result{}, err
	}
	return newResult("snapshot", diffOutput), nil
}

func WorkingTreeDiff(workDir string) (Result, error) {
	if workDir == "" {
		return Result{}, errors.New("workdir required")
	}
	prepareUntrackedForDiff(workDir)

	output, err := run(workDir, "diff", "HEAD")
	if err != nil {
		output, _ = run(workDir, "diff")
	}
	return newResult("working", output), nil
}

// NativeGitDiffRange 使用工作区原生 Git 对比两个 tree-ish（提交、树等）。
func NativeGitDiffRange(workDir, from, to string) (Result, error) {
	if workDir == "" || from == "" || to == "" {
		return Result{}, errors.New("workdir, from and to required")
	}
	prepareUntrackedForDiff(workDir)
	output, err := run(workDir, "diff", "--no-ext-diff", from, to, "--", ".")
	if err != nil {
		return Result{}, err
	}
	return newResult("branch", output), nil
}

// WorkingTreeDiffFromParent 对比 HEAD~1 与当前工作区（含未暂存；含未跟踪需 add -N）。
func WorkingTreeDiffFromParent(workDir string) (Result, error) {
	if workDir == "" {
		return Result{}, errors.New("workdir required")
	}
	prepareUntrackedForDiff(workDir)
	if _, err := run(workDir, "rev-parse", "HEAD~1"); err != nil {
		return WorkingTreeDiff(workDir)
	}
	output, err := run(workDir, "diff", "--no-ext-diff", "HEAD~1")
	if err != nil {
		return Result{}, err
	}
	return newResult("working", output), nil
}

// DiffCommitRange 单条提交的变更（相对其父提交）；根提交则相对空树。
func DiffCommitRange(workDir, commit string) (Result, error) {
	if workDir == "" || strings.TrimSpace(commit) == "" {
		return Result{}, errors.New("workdir and commit required")
	}
	c := strings.TrimSpace(commit)
	parentOut, err := run(workDir, "rev-parse", c+"^")
	if err != nil {
		const emptyTree = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"
		return NativeGitDiffRange(workDir, emptyTree, c)
	}
	return NativeGitDiffRange(workDir, strings.TrimSpace(parentOut), c)
}

func WorktreeDiffFromBase(workDir, baseRef string) (Result, error) {
	if workDir == "" {
		return Result{}, errors.New("workdir required")
	}
	if baseRef == "" {
		return Result{}, errors.New("base ref required")
	}

	prepareUntrackedForDiff(workDir)

	mergeBaseOutput, err := run(workDir, "merge-base", baseRef, "HEAD")
	if err != nil {
		return Result{}, err
	}

	mergeBase := strings.TrimSpace(mergeBaseOutput)
	if mergeBase == "" {
		return Result{}, errors.New("merge base not found")
	}

	output, err := run(workDir, "diff", mergeBase, "--", ".")
	if err != nil {
		return Result{}, err
	}

	return newResult("branch", output), nil
}

func BranchDiffFromMain(workDir, branch string) (Result, error) {
	if branch != "" {
		return BranchDiff(workDir, "main", branch)
	}
	return BranchDiff(workDir, "main", "")
}

func BranchDiff(workDir, from, to string) (Result, error) {
	if workDir == "" {
		return Result{}, errors.New("workdir required")
	}
	if from == "" {
		return Result{}, errors.New("from branch required")
	}

	args := []string{"diff"}
	if to != "" {
		args = append(args, from+"..."+to)
	} else {
		args = append(args, from)
	}

	output, err := run(workDir, args...)
	if err != nil {
		output, _ = run(workDir, "diff")
	}
	return newResult("branch", output), nil
}

func prepareUntrackedForDiff(workDir string) {
	_, _ = run(workDir, "add", "-N", ".")
}

func ParseDiffFiles(diffOutput string) []map[string]interface{} {
	files := []map[string]interface{}{}
	lines := strings.Split(diffOutput, "\n")

	var currentFile map[string]interface{}
	var currentDiff strings.Builder

	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git") {
			if currentFile != nil {
				currentFile["diff"] = currentDiff.String()
				files = append(files, currentFile)
			}

			currentFile = make(map[string]interface{})
			currentDiff.Reset()
			currentDiff.WriteString(line + "\n")

			parts := strings.Fields(line)
			if len(parts) >= 4 {
				currentFile["path"] = strings.TrimPrefix(parts[3], "b/")
			}
			continue
		}

		if currentFile == nil {
			continue
		}

		currentDiff.WriteString(line + "\n")
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			if additions, ok := currentFile["additions"].(int); ok {
				currentFile["additions"] = additions + 1
			} else {
				currentFile["additions"] = 1
			}
		} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			if deletions, ok := currentFile["deletions"].(int); ok {
				currentFile["deletions"] = deletions + 1
			} else {
				currentFile["deletions"] = 1
			}
		}
	}

	if currentFile != nil {
		currentFile["diff"] = currentDiff.String()
		files = append(files, currentFile)
	}

	return files
}

func newResult(diffType, diffOutput string) Result {
	return Result{
		Type:  diffType,
		Diff:  diffOutput,
		Files: ParseDiffFiles(diffOutput),
	}
}
