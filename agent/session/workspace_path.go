package session

import (
	"path/filepath"
	"strings"

	database "pkgs/db"
	"pkgs/db/storage"

	"gorm.io/gorm"
)

func (r *AgentRunner) resolveSessionWorkspacePath(db *gorm.DB, sessionID string) string {
	if r != nil && r.session != nil && strings.TrimSpace(r.session.ID) == strings.TrimSpace(sessionID) {
		if path := strings.TrimSpace(r.session.WorkspacePath); path != "" {
			return path
		}
	}
	if db == nil || strings.TrimSpace(sessionID) == "" {
		return ""
	}
	sess, err := storage.GetSession(db, sessionID)
	if err != nil || sess == nil {
		return ""
	}
	if r != nil && r.session != nil && r.session.ID == sess.ID {
		r.session.WorkspaceRoot = sess.WorkspaceRoot
		r.session.WorkspacePath = sess.WorkspacePath
	}
	return strings.TrimSpace(sess.WorkspacePath)
}

func replaceAIWorkspacePlaceholder(text, workspacePath string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	workspacePath = strings.TrimSpace(workspacePath)
	if workspacePath == "" {
		workspacePath = "ai_workspace"
	}
	return strings.ReplaceAll(text, "{{ai_workspace}}", workspacePath)
}

func fallbackAIWorkspacePath(workDir, worktreeRoot string) string {
	return database.AIWorkspacePath(workDir, worktreeRoot)
}

// resolveProjectAIWorkspacePath 解析项目级 ai_workspace 路径。
// 任务在独立 git worktree 检出目录执行时，应使用 task.WorkDir，而不是 Project.WorktreePath（原项目路径）。
func resolveProjectAIWorkspacePath(workDir, projectPath, projectWorktreePath string) string {
	workDir = strings.TrimSpace(workDir)
	projectPath = strings.TrimSpace(projectPath)
	projectWorktreePath = strings.TrimSpace(projectWorktreePath)
	if workDir != "" && projectPath != "" && filepath.Clean(workDir) != filepath.Clean(projectPath) {
		return fallbackAIWorkspacePath(workDir, "")
	}
	return fallbackAIWorkspacePath(workDir, projectWorktreePath)
}
