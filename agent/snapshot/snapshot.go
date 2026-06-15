package snapshot

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"matrixops-agent/global"
	"matrixops-agent/types"
)

type Patch struct {
	Hash  string   `json:"hash"`
	Files []string `json:"files"`
}

type FileDiff = types.FileDiff

func Track(projectID string, directory string) (string, error) {
	gitDir := gitdir(projectID, directory)
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		return "", err
	}
	if _, err := os.Stat(filepath.Join(gitDir, "HEAD")); err != nil {
		if _, err := runGit(projectID, directory, "init"); err != nil {
			return "", err
		}
		_, _ = runGit(projectID, directory, "config", "core.autocrlf", "false")
	}
	_, _ = runGit(projectID, directory, "add", ".")
	output, err := runGit(projectID, directory, "write-tree")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(output), nil
}

func PatchFiles(projectID string, directory string, hash string) (Patch, error) {
	if hash == "" {
		return Patch{}, errors.New("snapshot hash required")
	}
	_, _ = runGit(projectID, directory, "add", ".")
	output, err := runGit(projectID, directory, "-c", "core.autocrlf=false", "diff", "--no-ext-diff", "--name-only", hash, "--", ".")
	if err != nil {
		return Patch{Hash: hash, Files: []string{}}, nil
	}
	files := []string{}
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		files = append(files, filepath.Join(directory, trimmed))
	}
	return Patch{Hash: hash, Files: files}, nil
}

func Restore(projectID string, directory string, snapshot string, clean bool) error {
	if snapshot == "" {
		return errors.New("snapshot hash required")
	}
	_, err := runGit(projectID, directory, "read-tree", snapshot)
	if err != nil {
		return err
	}
	_, err = runGit(projectID, directory, "checkout-index", "-a", "-f")
	if err != nil {
		return err
	}

	if clean {
		// 清理掉快照中不存在的冗余文件和目录
		_, err = runGit(projectID, directory, "clean", "-fd")
		return err
	}
	return nil
}

func Revert(projectID string, directory string, patches []Patch) error {
	seen := map[string]struct{}{}
	for _, patch := range patches {
		for _, file := range patch.Files {
			if _, ok := seen[file]; ok {
				continue
			}
			seen[file] = struct{}{}
			_, err := runGit(projectID, directory, "checkout", patch.Hash, "--", file)
			if err != nil {
				rel, relErr := filepath.Rel(directory, file)
				if relErr == nil {
					if output, checkErr := runGit(projectID, directory, "ls-tree", patch.Hash, "--", rel); checkErr == nil && strings.TrimSpace(output) != "" {
						continue
					}
				}
				_ = os.Remove(file)
			}
		}
	}
	return nil
}

func Diff(projectID string, directory string, hash string) (string, error) {
	if hash == "" {
		return "", errors.New("snapshot hash required")
	}
	_, _ = runGit(projectID, directory, "add", ".")
	output, err := runGit(projectID, directory, "-c", "core.autocrlf=false", "diff", "--no-ext-diff", hash, "--", ".")
	if err != nil {
		return "", nil
	}
	return strings.TrimSpace(output), nil
}

func DiffRange(projectID string, directory string, from string, to string) (string, error) {
	if from == "" || to == "" {
		return "", errors.New("both from and to hashes are required")
	}
	output, err := runGit(projectID, directory, "-c", "core.autocrlf=false", "diff", "--no-ext-diff", from, to, "--", ".")
	if err != nil {
		return "", nil
	}
	return strings.TrimSpace(output), nil
}

func DiffFull(projectID string, directory string, from string, to string) ([]FileDiff, error) {
	if from == "" || to == "" {
		return []FileDiff{}, nil
	}
	output, err := runGit(projectID, directory, "-c", "core.autocrlf=false", "diff", "--no-ext-diff", "--no-renames", "--numstat", from, to, "--", ".")
	if err != nil {
		return []FileDiff{}, nil
	}
	results := []FileDiff{}
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 3 {
			continue
		}
		additions := parts[0]
		deletions := parts[1]
		file := parts[2]
		isBinary := additions == "-" && deletions == "-"
		before := ""
		after := ""
		if !isBinary {
			before, _ = runGit(projectID, directory, "-c", "core.autocrlf=false", "show", fmt.Sprintf("%s:%s", from, file))
			after, _ = runGit(projectID, directory, "-c", "core.autocrlf=false", "show", fmt.Sprintf("%s:%s", to, file))
		}
		added := 0
		deleted := 0
		if !isBinary {
			if parsed, err := strconv.Atoi(additions); err == nil {
				added = parsed
			}
			if parsed, err := strconv.Atoi(deletions); err == nil {
				deleted = parsed
			}
		}
		results = append(results, FileDiff{
			File:      file,
			Before:    before,
			After:     after,
			Additions: added,
			Deletions: deleted,
		})
	}
	return results, nil
}

// func enabled(projectID string, directory string) bool {
// 	cfg, err := config.Get(projectID, directory)
// 	if err != nil {
// 		return true
// 	}
// 	if cfg.Snapshot != nil && !*cfg.Snapshot {
// 		return false
// 	}
// 	return true
// }

func gitdir(projectID string, directory string) string {
	_ = global.Init()
	return filepath.Join(global.Path.Data, "snapshot", projectID)
}

func runGit(projectID string, directory string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = directory
	env := append(os.Environ(), "GIT_DIR="+gitdir(projectID, directory), "GIT_WORK_TREE="+directory)

	// 让影子仓库能够引用项目原生的对象库
	if alternateObjectsDir, err := resolveAlternateObjectDir(directory); err == nil && alternateObjectsDir != "" {
		env = append(env, "GIT_ALTERNATE_OBJECT_DIRECTORIES="+alternateObjectsDir)
	}

	cmd.Env = env
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		output := strings.TrimSpace(stdout.String())
		errText := strings.TrimSpace(stderr.String())
		if output == "" {
			output = errText
		} else if errText != "" {
			output = output + "\n" + errText
		}
		return output, err
	}
	return stdout.String(), nil
}

func resolveAlternateObjectDir(directory string) (string, error) {
	gitPath := filepath.Join(directory, ".git")
	info, err := os.Stat(gitPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}

	if info.IsDir() {
		objectsDir := filepath.Join(gitPath, "objects")
		if hasDir(objectsDir) {
			return objectsDir, nil
		}
		return "", nil
	}

	gitDir, err := resolveGitDirFromFile(directory, gitPath)
	if err != nil {
		return "", err
	}
	commonDir, err := resolveCommonGitDir(gitDir)
	if err != nil {
		return "", err
	}
	objectsDir := filepath.Join(commonDir, "objects")
	if hasDir(objectsDir) {
		return objectsDir, nil
	}
	return "", nil
}

func resolveGitDirFromFile(worktreeDir string, gitFile string) (string, error) {
	content, err := os.ReadFile(gitFile)
	if err != nil {
		return "", err
	}
	line := strings.TrimSpace(string(content))
	const prefix = "gitdir:"
	if !strings.HasPrefix(strings.ToLower(line), prefix) {
		return "", fmt.Errorf("invalid gitdir file: %s", gitFile)
	}
	gitDir := strings.TrimSpace(line[len(prefix):])
	if gitDir == "" {
		return "", fmt.Errorf("empty gitdir in %s", gitFile)
	}
	if filepath.IsAbs(gitDir) {
		return filepath.Clean(gitDir), nil
	}
	return filepath.Clean(filepath.Join(worktreeDir, gitDir)), nil
}

func resolveCommonGitDir(gitDir string) (string, error) {
	commondirFile := filepath.Join(gitDir, "commondir")
	content, err := os.ReadFile(commondirFile)
	if err != nil {
		if os.IsNotExist(err) {
			return gitDir, nil
		}
		return "", err
	}
	commonDir := strings.TrimSpace(string(content))
	if commonDir == "" {
		return gitDir, nil
	}
	if filepath.IsAbs(commonDir) {
		return filepath.Clean(commonDir), nil
	}
	return filepath.Clean(filepath.Join(gitDir, commonDir)), nil
}

func hasDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
