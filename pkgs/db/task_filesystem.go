package database

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"pkgs/db/models"

	"gorm.io/gorm"
)

const (
	TaskFilesystemRootProject   = "project"
	TaskFilesystemRootWorkspace = "workspace"
)

type TaskFilesystemRoot struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Path  string `json:"path"`
}

type TaskFilesystemEntry struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	IsDir bool   `json:"isDir"`
}

// ResolveProjectAIWorkspacePath 解析项目级 ai_workspace 路径。
func ResolveProjectAIWorkspacePath(workDir, projectPath, projectWorktreePath string) string {
	workDir = strings.TrimSpace(workDir)
	projectPath = strings.TrimSpace(projectPath)
	projectWorktreePath = strings.TrimSpace(projectWorktreePath)
	if workDir != "" && projectPath != "" && filepath.Clean(workDir) != filepath.Clean(projectPath) {
		return AIWorkspacePath(workDir, "")
	}
	return AIWorkspacePath(workDir, projectWorktreePath)
}

// resolveTaskAIWorkspaceDir 与 agent/session/prompt_history.go 中 ai_workspace 解析逻辑一致：
// 优先使用会话 WorkspacePath，其次项目 ai_workspace 目录，最后回退到 workDir/ai_workspace。
func resolveTaskAIWorkspaceDir(db *gorm.DB, task *models.Task) string {
	if path := resolveTaskSessionWorkspacePath(db, task); path != "" {
		return path
	}
	workDir := ""
	if task != nil {
		workDir = strings.TrimSpace(task.WorkDir)
	}
	if task != nil && task.ProjectID > 0 && db != nil {
		project, err := GetProjectByID(db, task.ProjectID)
		if err == nil {
			projectPath := strings.TrimSpace(project.Path)
			if workDir == "" {
				workDir = projectPath
			}
			return ResolveProjectAIWorkspacePath(workDir, projectPath, project.WorktreePath)
		}
	}
	if workDir != "" {
		return AIWorkspacePath(workDir, "")
	}
	return ""
}

func ResolveTaskFilesystemRoots(db *gorm.DB, task *models.Task) ([]TaskFilesystemRoot, error) {
	if task == nil {
		return nil, errors.New("task is required")
	}

	if task.ProjectID > 0 {
		project, err := GetProjectByID(db, task.ProjectID)
		if err != nil {
			return nil, err
		}
		projectPath := strings.TrimSpace(project.Path)
		workDir := strings.TrimSpace(task.WorkDir)
		if workDir == "" {
			workDir = projectPath
		}
		aiWorkspace := resolveTaskAIWorkspaceDir(db, task)
		return []TaskFilesystemRoot{
			{ID: TaskFilesystemRootProject, Label: "当前项目", Path: workDir},
			{ID: TaskFilesystemRootWorkspace, Label: "当前项目的工作空间", Path: aiWorkspace},
		}, nil
	}

	return []TaskFilesystemRoot{
		{ID: TaskFilesystemRootWorkspace, Label: "工作空间", Path: resolveTaskAIWorkspaceDir(db, task)},
	}, nil
}

func resolveTaskSessionWorkspacePath(db *gorm.DB, task *models.Task) string {
	if task == nil || db == nil {
		return ""
	}
	sessionID := strings.TrimSpace(task.SessionID)
	if sessionID == "" {
		executions, err := GetExecutionsByTaskID(db, task.ID, 1)
		if err == nil && len(executions) > 0 {
			sessionID = strings.TrimSpace(executions[0].AgentSessionID)
		}
	}
	if sessionID == "" {
		return ""
	}
	var sess models.Session
	if err := db.Select("workspace_path").First(&sess, "id = ?", sessionID).Error; err != nil {
		return ""
	}
	return strings.TrimSpace(sess.WorkspacePath)
}

func ResolveTaskFilesystemAbsolutePath(db *gorm.DB, task *models.Task, rootID, relativePath string) (string, error) {
	roots, err := ResolveTaskFilesystemRoots(db, task)
	if err != nil {
		return "", err
	}
	rootID = strings.TrimSpace(rootID)
	var rootPath string
	for _, root := range roots {
		if root.ID == rootID {
			rootPath = strings.TrimSpace(root.Path)
			break
		}
	}
	if rootPath == "" {
		return "", errors.New("无效的文件根目录")
	}
	return SafeJoinUnderRoot(rootPath, relativePath)
}

func SafeJoinUnderRoot(root, relative string) (string, error) {
	root = filepath.Clean(strings.TrimSpace(root))
	if root == "" {
		return "", errors.New("根目录为空")
	}
	rel := strings.TrimSpace(relative)
	if rel == "" || rel == "." {
		return root, nil
	}
	rel = filepath.Clean(rel)
	if filepath.IsAbs(rel) {
		return "", errors.New("路径必须是相对路径")
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("非法路径")
	}
	abs := filepath.Clean(filepath.Join(root, rel))
	if !pathWithinRoot(root, abs) {
		return "", errors.New("路径超出允许范围")
	}
	return abs, nil
}

func pathWithinRoot(root, target string) bool {
	root = filepath.Clean(root)
	target = filepath.Clean(target)
	if root == target {
		return true
	}
	return strings.HasPrefix(target, root+string(filepath.Separator))
}

func ListTaskFilesystemEntries(db *gorm.DB, task *models.Task, rootID, relativePath string) ([]TaskFilesystemEntry, error) {
	abs, err := ResolveTaskFilesystemAbsolutePath(db, task, rootID, relativePath)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, errors.New("路径不是目录")
	}

	rootAbs, err := ResolveTaskFilesystemAbsolutePath(db, task, rootID, "")
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(abs)
	if err != nil {
		return nil, err
	}

	result := make([]TaskFilesystemEntry, 0, len(entries))
	for _, entry := range entries {
		entryAbs := filepath.Join(abs, entry.Name())
		rel, relErr := filepath.Rel(rootAbs, entryAbs)
		if relErr != nil {
			continue
		}
		result = append(result, TaskFilesystemEntry{
			Name:  entry.Name(),
			Path:  filepath.ToSlash(rel),
			IsDir: entry.IsDir(),
		})
	}
	sortFilesystemEntries(result)
	return result, nil
}

func sortFilesystemEntries(entries []TaskFilesystemEntry) {
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})
}

const maxTaskFilesystemReadBytes = 2 * 1024 * 1024

func ReadTaskFilesystemFile(db *gorm.DB, task *models.Task, rootID, relativePath string) (content string, binary bool, err error) {
	abs, err := ResolveTaskFilesystemAbsolutePath(db, task, rootID, relativePath)
	if err != nil {
		return "", false, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", false, err
	}
	if info.IsDir() {
		return "", false, errors.New("路径不是文件")
	}
	if info.Size() > maxTaskFilesystemReadBytes {
		return "", false, errors.New("文件过大，无法在编辑器中打开")
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return "", false, err
	}
	if !isTextContent(data) {
		return "", true, nil
	}
	return string(data), false, nil
}

func WriteTaskFilesystemFile(db *gorm.DB, task *models.Task, rootID, relativePath, content string) error {
	abs, err := ResolveTaskFilesystemAbsolutePath(db, task, rootID, relativePath)
	if err != nil {
		return err
	}
	if info, statErr := os.Stat(abs); statErr == nil && info.IsDir() {
		return errors.New("路径不是文件")
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return err
	}
	return os.WriteFile(abs, []byte(content), 0o644)
}

func isTextContent(data []byte) bool {
	if len(data) == 0 {
		return true
	}
	if len(data) > 8192 {
		data = data[:8192]
	}
	for _, b := range data {
		if b == 0 {
			return false
		}
	}
	return true
}
