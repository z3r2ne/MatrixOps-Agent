package database

import (
	"crypto/rand"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const envMatrixopsHome = "MATRIXOPS_HOME"

// DataDir 返回应用数据根目录（数据库文件、默认工作区等均在此之下）。
// 优先使用环境变量 MATRIXOPS_HOME；否则为当前用户主目录下的 .matrixops。
// 若目录不存在会尝试创建。
func DataDir() (string, error) {
	var dir string
	if v := strings.TrimSpace(os.Getenv(envMatrixopsHome)); v != "" {
		abs, err := filepath.Abs(v)
		if err != nil {
			return "", err
		}
		dir = abs
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, ".matrixops")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

// DefaultWorkspacePath 生成新的默认工作区磁盘路径（随机子目录），并创建该目录。
func DefaultWorkspacePath() (string, error) {
	base, err := DataDir()
	if err != nil {
		return "", err
	}
	var rnd [4]byte
	if _, err := rand.Read(rnd[:]); err != nil {
		return "", err
	}
	name := hex.EncodeToString(rnd[:])
	p := filepath.Join(base, "workspaces", name)
	if err := os.MkdirAll(p, 0755); err != nil {
		return "", err
	}
	return p, nil
}

// AIWorkspacePath 返回会话/记忆使用的 ai_workspace 目录（位于项目 worktree 根或当前工作目录下）。
// worktreeRoot 为数据库中的 Project.WorktreePath；为空时使用 workDir。
func AIWorkspacePath(workDir, worktreeRoot string) string {
	base := strings.TrimSpace(worktreeRoot)
	if base == "" {
		base = workDir
	}
	return filepath.Join(base, "ai_workspace")
}

// NewSessionWorkspace 在全局数据目录下创建与会话绑定的工作目录：
//
//	~/.matrixops/worktrees/<project-safe>-<random>/workspace
func NewSessionWorkspace(projectName, fallbackDir string) (workspaceRoot string, workspacePath string, err error) {
	base, err := DataDir()
	if err != nil {
		return "", "", err
	}
	label := sanitizeWorkspaceDirName(projectName)
	if label == "" {
		label = sanitizeWorkspaceDirName(filepath.Base(filepath.Clean(strings.TrimSpace(fallbackDir))))
	}
	if label == "" {
		label = "project"
	}
	var rnd [4]byte
	if _, err := rand.Read(rnd[:]); err != nil {
		return "", "", err
	}
	dirName := fmt.Sprintf("%s-%s", label, hex.EncodeToString(rnd[:]))
	workspaceRoot = filepath.Join(base, "worktrees", dirName)
	workspacePath = filepath.Join(workspaceRoot, "workspace")
	if err := os.MkdirAll(workspacePath, 0755); err != nil {
		return "", "", err
	}
	return workspaceRoot, workspacePath, nil
}

// NewTaskWorkspace 在全局数据目录下创建与任务绑定的工作目录：
//
//	~/.matrixops/tasks/task-<taskid>-<random>/workspace
func NewTaskWorkspace(taskID uint, projectName, fallbackDir string) (workspaceRoot string, workspacePath string, err error) {
	base, err := DataDir()
	if err != nil {
		return "", "", err
	}
	label := sanitizeWorkspaceDirName(projectName)
	if label == "" {
		label = sanitizeWorkspaceDirName(filepath.Base(filepath.Clean(strings.TrimSpace(fallbackDir))))
	}
	if label == "" {
		label = "project"
	}
	var rnd [4]byte
	if _, err := rand.Read(rnd[:]); err != nil {
		return "", "", err
	}
	dirName := fmt.Sprintf("task-%d-%s", taskID, hex.EncodeToString(rnd[:]))
	workspaceRoot = filepath.Join(base, "tasks", dirName)
	workspacePath = filepath.Join(workspaceRoot, "workspace")
	if err := os.MkdirAll(workspacePath, 0755); err != nil {
		return "", "", err
	}
	return workspaceRoot, workspacePath, nil
}

func SkillSourcesDir() (string, error) {
	base, err := DataDir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(base, "skill-sources")
	if err := os.MkdirAll(path, 0755); err != nil {
		return "", err
	}
	return path, nil
}

func InstalledSkillsDir() (string, error) {
	base, err := DataDir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(base, "skills")
	if err := os.MkdirAll(path, 0755); err != nil {
		return "", err
	}
	return path, nil
}

func PromptsDir() (string, error) {
	base, err := DataDir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(base, "prompts")
	if err := os.MkdirAll(path, 0o755); err != nil {
		return "", err
	}
	return path, nil
}

func GlobalPromptFilePath() (string, error) {
	base, err := PromptsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "global.md"), nil
}

func OccupationPromptsDir() (string, error) {
	base, err := PromptsDir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(base, "occupations")
	if err := os.MkdirAll(path, 0o755); err != nil {
		return "", err
	}
	return path, nil
}

func OccupationPromptFilePath(code string) (string, error) {
	base, err := OccupationPromptsDir()
	if err != nil {
		return "", err
	}
	name := sanitizeWorkspaceDirName(code)
	if name == "" {
		name = "occupation"
	}
	return filepath.Join(base, name+".md"), nil
}

func SkillSourceLocalPath(sourceID uint) (string, error) {
	base, err := SkillSourcesDir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(base, fmt.Sprintf("source-%d", sourceID))
	return path, nil
}

func InstalledSkillPath(sourceID uint, relativePath string) (string, error) {
	base, err := InstalledSkillsDir()
	if err != nil {
		return "", err
	}
	cleanRel := strings.TrimSpace(relativePath)
	cleanRel = strings.TrimPrefix(filepath.Clean(cleanRel), string(filepath.Separator))
	path := filepath.Join(base, fmt.Sprintf("source-%d", sourceID), cleanRel)
	return path, nil
}

// ProjectDataDir 返回当前项目在全局数据目录下的专属目录。
// 优先使用 projectID 作为稳定标识；若缺失则回退到 workDir 的目录名与哈希。
func ProjectDataDir(projectID, workDir string) (string, error) {
	base, err := DataDir()
	if err != nil {
		return "", err
	}

	labelSource := strings.TrimSpace(workDir)
	if labelSource == "" {
		labelSource = "project"
	}
	label := sanitizeWorkspaceDirName(filepath.Base(filepath.Clean(labelSource)))
	if label == "" {
		label = "project"
	}

	projectKey := sanitizeWorkspaceDirName(projectID)
	dirName := ""
	if projectKey != "" {
		dirName = fmt.Sprintf("project-%s-%s", projectKey, label)
	} else {
		dirName = fmt.Sprintf("%s-%s", label, shortPathHash(labelSource))
	}

	path := filepath.Join(base, "projects", dirName)
	if err := os.MkdirAll(path, 0o755); err != nil {
		return "", err
	}
	return path, nil
}

// ProjectCodeMapFilePath 返回建议的代码地图文档路径。
func ProjectCodeMapFilePath(projectID, workDir string) (string, error) {
	base, err := ProjectDataDir(projectID, workDir)
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "code-map.md"), nil
}

// WorktreePath 根据项目路径与新分支名，生成 git worktree 检出路径。
// 实际目录统一放在全局数据目录下，避免在项目目录内创建 worktrees/。
func WorktreePath(projectWorktreePath, newBranch string) string {
	base, err := DataDir()
	if err != nil {
		base = filepath.Join(os.TempDir(), ".matrixops")
	}
	projectName := sanitizeWorkspaceDirName(filepath.Base(filepath.Clean(strings.TrimSpace(projectWorktreePath))))
	if projectName == "" {
		projectName = "project"
	}
	projectHash := shortPathHash(projectWorktreePath)
	safe := sanitizeBranchDirName(newBranch)
	return filepath.Join(base, "worktrees", fmt.Sprintf("%s-%s-%s", projectName, projectHash, safe))
}

func shortPathHash(path string) string {
	sum := sha1.Sum([]byte(filepath.Clean(strings.TrimSpace(path))))
	return hex.EncodeToString(sum[:])[:8]
}

func sanitizeBranchDirName(branch string) string {
	s := strings.TrimSpace(branch)
	s = strings.ReplaceAll(s, string(os.PathSeparator), "_")
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, "..", "_")
	if s == "" {
		return "default"
	}
	return s
}

func sanitizeWorkspaceDirName(name string) string {
	s := strings.TrimSpace(name)
	s = strings.ReplaceAll(s, string(os.PathSeparator), "-")
	s = strings.ReplaceAll(s, "/", "-")
	s = strings.ReplaceAll(s, "..", "-")
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.Trim(s, "-_.")
	if s == "" {
		return ""
	}
	return s
}
