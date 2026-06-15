package handlers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"pkgs/db/storage"
	"sort"
	"strings"

	"matrixops-agent/types"

	coregit "matrixops.local/core_agent/git"
	"matrixops/services"
	"matrixops/services/task_runner"
	database "pkgs/db"
	"pkgs/db/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// GitHandler Git 相关操作处理器
type GitHandler struct {
	db *gorm.DB
}

// NewGitHandler 创建 Git 处理器
func NewGitHandler(db *gorm.DB) *GitHandler {
	return &GitHandler{db: db}
}

type BranchInfo = coregit.BranchInfo
type WorktreeInfo = coregit.WorktreeInfo

type taskDiffPatch struct {
	ID            string   `json:"id,omitempty"`
	PartID        string   `json:"partId"`
	Hash          string   `json:"hash"`
	Snapshot      string   `json:"snapshot"`
	MessageID     string   `json:"messageId"`
	Timestamp     int64    `json:"timestamp"`
	Files         []string `json:"files"`
	Description   string   `json:"description,omitempty"`
	SessionID     string   `json:"sessionId,omitempty"`
	StartSnapshot string   `json:"startSnapshot,omitempty"`
}

// GetBranches 获取项目的所有分支
func (h *GitHandler) GetBranches(c *gin.Context) {
	projectID := c.Param("id")

	var pid uint
	fmt.Sscanf(projectID, "%d", &pid)

	project, err := database.GetProjectByID(h.db, pid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "项目不存在"})
		return
	}

	// 检查项目路径
	if _, err := os.Stat(project.Path); os.IsNotExist(err) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "项目路径不存在"})
		return
	}

	if !coregit.IsGitRepo(project.Path) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "项目不是 Git 仓库"})
		return
	}

	branches, err := coregit.ListBranches(project.Path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取分支列表失败"})
		return
	}

	c.JSON(http.StatusOK, branches)
}

// GetCurrentBranch 获取当前分支
func (h *GitHandler) GetCurrentBranch(c *gin.Context) {
	projectID := c.Param("id")

	var pid uint
	fmt.Sscanf(projectID, "%d", &pid)

	project, err := database.GetProjectByID(h.db, pid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "项目不存在"})
		return
	}

	branch, err := coregit.CurrentBranch(project.Path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取当前分支失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"branch": branch})
}

// GetDefaultBranch 获取默认主分支
func (h *GitHandler) GetDefaultBranch(c *gin.Context) {
	projectID := c.Param("id")

	var pid uint
	fmt.Sscanf(projectID, "%d", &pid)

	project, err := database.GetProjectByID(h.db, pid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "项目不存在"})
		return
	}

	if _, err := os.Stat(project.Path); os.IsNotExist(err) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "项目路径不存在"})
		return
	}

	if !coregit.IsGitRepo(project.Path) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "项目不是 Git 仓库"})
		return
	}

	branch, err := coregit.DefaultBranch(project.Path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取默认分支失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"branch": branch})
}

// CreateWorktreeRequest 创建 Worktree 请求
type CreateWorktreeRequest struct {
	NewBranch  string `json:"newBranch" binding:"required"`  // 新分支名
	BaseBranch string `json:"baseBranch" binding:"required"` // 基础分支
}

// CreateWorktree 创建 Git Worktree
func (h *GitHandler) CreateWorktree(c *gin.Context) {
	projectID := c.Param("id")

	var pid uint
	fmt.Sscanf(projectID, "%d", &pid)

	project, err := database.GetProjectByID(h.db, pid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "项目不存在"})
		return
	}

	var req CreateWorktreeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	// 验证新分支名
	if req.NewBranch == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "新分支名不能为空"})
		return
	}

	worktreePath, err := coregit.CreateWorktree(project.Path, project.WorktreePath, req.NewBranch, req.BaseBranch)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "创建 Worktree 失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "Worktree 创建成功",
		"path":       worktreePath,
		"branch":     req.NewBranch,
		"baseBranch": req.BaseBranch,
	})
}

// GetWorktrees 获取项目的所有 Worktree
func (h *GitHandler) GetWorktrees(c *gin.Context) {
	projectID := c.Param("id")

	var pid uint
	fmt.Sscanf(projectID, "%d", &pid)

	project, err := database.GetProjectByID(h.db, pid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "项目不存在"})
		return
	}

	worktrees, err := coregit.ListWorktrees(project.Path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取 Worktree 列表失败"})
		return
	}

	c.JSON(http.StatusOK, worktrees)
}

// DeleteWorktree 删除 Worktree
func (h *GitHandler) DeleteWorktree(c *gin.Context) {
	projectID := c.Param("id")
	worktreePath := c.Query("path")

	if worktreePath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 worktree 路径"})
		return
	}

	var pid uint
	fmt.Sscanf(projectID, "%d", &pid)

	project, err := database.GetProjectByID(h.db, pid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "项目不存在"})
		return
	}

	// 不允许删除主 worktree
	if worktreePath == project.Path {
		c.JSON(http.StatusBadRequest, gin.H{"error": "不能删除主 worktree"})
		return
	}

	if err := coregit.RemoveWorktree(project.Path, worktreePath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除 Worktree 失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Worktree 删除成功"})
}

// CheckGitRepo 检查项目是否为 Git 仓库
func (h *GitHandler) CheckGitRepo(c *gin.Context) {
	projectID := c.Param("id")

	var pid uint
	fmt.Sscanf(projectID, "%d", &pid)

	project, err := database.GetProjectByID(h.db, pid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "项目不存在"})
		return
	}

	// 检查项目路径
	if _, err := os.Stat(project.Path); os.IsNotExist(err) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "项目路径不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"isGitRepo": coregit.IsGitRepo(project.Path)})
}

// InitGitRepoRequest 初始化 Git 仓库请求
type InitGitRepoRequest struct {
	CommitMessage string `json:"commitMessage"` // 可选的提交信息
}

// InitGitRepo 初始化 Git 仓库并进行首次提交
func (h *GitHandler) InitGitRepo(c *gin.Context) {
	projectID := c.Param("id")

	var pid uint
	fmt.Sscanf(projectID, "%d", &pid)

	project, err := database.GetProjectByID(h.db, pid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "项目不存在"})
		return
	}

	// 检查项目路径
	if _, err := os.Stat(project.Path); os.IsNotExist(err) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "项目路径不存在"})
		return
	}

	if coregit.IsGitRepo(project.Path) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "该项目已经是 Git 仓库"})
		return
	}

	var req InitGitRepoRequest
	c.ShouldBindJSON(&req)

	commitMessage := req.CommitMessage
	if commitMessage == "" {
		commitMessage = "Initial commit"
	}

	logger := task_runner.GetCommandLogger(h.db)
	sourceID := project.ID
	sourceName := "初始化 Git 仓库: " + project.Name

	// 初始化 git 仓库
	logID := logger.LogCommand(models.CommandLogCreate{
		Source:     "git_handler",
		SourceID:   &sourceID,
		SourceName: sourceName,
		Command:    "git",
		Args:       []string{"init"},
		WorkDir:    project.Path,
		Fields: models.BuildCommandLogFields(
			models.NewCommandLogField("operation", "操作", "初始化仓库", "default"),
		),
	})

	output, err := coregit.InitRepo(project.Path)
	exitCode := 0
	if err != nil {
		exitCode = -1
		logger.UpdateCommandResult(logID, output, "", &exitCode, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":  "Git 初始化失败: " + err.Error(),
			"output": output,
		})
		return
	}
	logger.UpdateCommandResult(logID, output, "", &exitCode, nil)

	// 检查是否已有 .gitignore
	gitignorePath := filepath.Join(project.Path, ".gitignore")
	if _, err := os.Stat(gitignorePath); os.IsNotExist(err) {
		// 创建基础 .gitignore 文件
		gitignoreContent := `# Dependencies
node_modules/
vendor/

# Build outputs
dist/
build/
*.exe
*.dll
*.so
*.dylib

# IDE
.vscode/
.idea/
*.swp
*.swo
*~

# OS
.DS_Store
Thumbs.db

# Env files
.env
.env.local
`
		if err := os.WriteFile(gitignorePath, []byte(gitignoreContent), 0644); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "创建 .gitignore 失败: " + err.Error()})
			return
		}
	}

	// 检查是否已有 README.md
	readmePath := filepath.Join(project.Path, "README.md")
	if _, err := os.Stat(readmePath); os.IsNotExist(err) {
		// 创建基础 README.md
		readmeContent := "# " + project.Name + "\n\nProject initialized by MatrixOps.\n"
		if err := os.WriteFile(readmePath, []byte(readmeContent), 0644); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "创建 README.md 失败: " + err.Error()})
			return
		}
	}

	// 添加所有文件
	logID = logger.LogCommand(models.CommandLogCreate{
		Source:     "git_handler",
		SourceID:   &sourceID,
		SourceName: sourceName,
		Command:    "git",
		Args:       []string{"add", "-A"},
		WorkDir:    project.Path,
		Fields: models.BuildCommandLogFields(
			models.NewCommandLogField("operation", "操作", "暂存全部变更", "default"),
		),
	})

	output, err = coregit.AddAll(project.Path)
	exitCode = 0
	if err != nil {
		exitCode = -1
		logger.UpdateCommandResult(logID, output, "", &exitCode, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":  "Git add 失败: " + err.Error(),
			"output": output,
		})
		return
	}
	logger.UpdateCommandResult(logID, output, "", &exitCode, nil)

	// 进行首次提交
	logID = logger.LogCommand(models.CommandLogCreate{
		Source:     "git_handler",
		SourceID:   &sourceID,
		SourceName: sourceName,
		Command:    "git",
		Args:       []string{"commit", "-m", commitMessage},
		WorkDir:    project.Path,
		Fields: models.BuildCommandLogFields(
			models.NewCommandLogField("operation", "操作", "创建初始提交", "default"),
			models.NewCommandLogField("commit_message", "提交信息", commitMessage, "default"),
		),
	})

	output, err = coregit.Commit(project.Path, commitMessage)
	exitCode = 0
	if err != nil {
		// 如果没有变更可提交，也视为成功
		if !coregit.NothingToCommit(output) {
			exitCode = -1
			logger.UpdateCommandResult(logID, output, "", &exitCode, err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":  "Git commit 失败: " + err.Error(),
				"output": output,
			})
			return
		}
	}
	logger.UpdateCommandResult(logID, output, "", &exitCode, nil)

	c.JSON(http.StatusOK, gin.H{
		"message": "Git 仓库初始化成功",
		"project": project.Name,
		"path":    project.Path,
	})
}

// GitCommit 提交任务的代码变更
func (h *GitHandler) GitCommit(c *gin.Context) {
	taskIDStr := c.Param("id")

	var req struct {
		Message string `json:"message" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	// 获取任务信息
	var taskID uint
	fmt.Sscanf(taskIDStr, "%d", &taskID)

	task, err := database.GetTaskByID(h.db, taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
		return
	}

	// 获取项目信息
	project, err := database.GetProjectByID(h.db, task.ProjectID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "项目不存在"})
		return
	}

	// 确定工作目录
	workDir := task.WorkDir
	if workDir == "" {
		workDir = project.Path
	}

	// 检查工作目录
	if _, err := os.Stat(workDir); os.IsNotExist(err) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "工作目录不存在: " + workDir})
		return
	}

	logger := task_runner.GetCommandLogger(h.db)

	// 1. git add -A
	addLogID := logger.LogCommand(models.CommandLogCreate{
		Source:     "git_commit",
		SourceID:   &task.ID,
		SourceName: "Git Commit: " + task.Content,
		Command:    "git",
		Args:       []string{"add", "-A"},
		WorkDir:    workDir,
		Fields: models.BuildCommandLogFields(
			models.NewCommandLogField("operation", "操作", "暂存全部变更", "default"),
		),
	})

	addOutput, addErr := coregit.AddAll(workDir)
	addExitCode := 0
	if addErr != nil {
		addExitCode = -1
		logger.UpdateCommandResult(addLogID, addOutput, "", &addExitCode, addErr)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "git add 失败: " + addErr.Error()})
		return
	}
	logger.UpdateCommandResult(addLogID, addOutput, "", &addExitCode, nil)

	// 2. git commit -m "message"
	commitLogID := logger.LogCommand(models.CommandLogCreate{
		Source:     "git_commit",
		SourceID:   &task.ID,
		SourceName: "Git Commit: " + task.Content,
		Command:    "git",
		Args:       []string{"commit", "-m", req.Message},
		WorkDir:    workDir,
		Fields: models.BuildCommandLogFields(
			models.NewCommandLogField("operation", "操作", "提交变更", "default"),
			models.NewCommandLogField("commit_message", "提交信息", req.Message, "default"),
		),
	})

	commitOutput, commitErr := coregit.Commit(workDir, req.Message)
	commitExitCode := 0
	if commitErr != nil {
		commitExitCode = -1
		logger.UpdateCommandResult(commitLogID, commitOutput, "", &commitExitCode, commitErr)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "git commit 失败: " + commitErr.Error()})
		return
	}
	logger.UpdateCommandResult(commitLogID, commitOutput, "", &commitExitCode, nil)

	// 3. 获取提交哈希
	commitHash, _ := coregit.HeadCommit(workDir)

	c.JSON(http.StatusOK, gin.H{
		"message": "提交成功",
		"commit":  commitHash[:7], // 短哈希
		"branch":  task.Branch,
	})
}

// GitMerge 合并分支到主分支
func (h *GitHandler) GitMerge(c *gin.Context) {
	taskIDStr := c.Param("id")

	var req struct {
		Message string `json:"message"` // 可选的提交消息
	}
	c.ShouldBindJSON(&req) // 不强制要求

	// 获取任务信息
	var taskID uint
	fmt.Sscanf(taskIDStr, "%d", &taskID)

	task, err := database.GetTaskByID(h.db, taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
		return
	}

	if task.Branch == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "任务没有关联分支"})
		return
	}

	// 获取项目信息
	project, err := database.GetProjectByID(h.db, task.ProjectID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "项目不存在"})
		return
	}

	// 使用项目主路径（不是 worktree）
	workDir := project.Path

	// 检查工作目录
	if _, err := os.Stat(workDir); os.IsNotExist(err) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "项目目录不存在: " + workDir})
		return
	}

	logger := task_runner.GetCommandLogger(h.db)

	// 确定提交消息
	commitMessage := req.Message
	if commitMessage == "" {
		commitMessage = task.Content
	}

	// 0. 先提交当前分支的变更（如果有的话）
	if task.WorkDir != "" && task.WorkDir != workDir {
		_, addErr := coregit.AddAll(task.WorkDir)
		if addErr == nil {
			commitOutput, commitErr := coregit.Commit(task.WorkDir, commitMessage)
			if commitErr != nil && !coregit.NothingToCommit(commitOutput) {
				_ = commitErr
			}
		}
	}

	// 1. 切换到 main 分支
	checkoutLogID := logger.LogCommand(models.CommandLogCreate{
		Source:     "git_merge",
		SourceID:   &task.ID,
		SourceName: "Git Merge: " + task.Content,
		Command:    "git",
		Args:       []string{"checkout", "main"},
		WorkDir:    workDir,
		Fields: models.BuildCommandLogFields(
			models.NewCommandLogField("operation", "操作", "切换分支", "default"),
			models.NewCommandLogField("target_branch", "目标分支", "main", "default"),
		),
	})

	checkoutOutput, checkoutErr := coregit.Checkout(workDir, "main")
	checkoutExitCode := 0
	if checkoutErr != nil {
		checkoutExitCode = -1
		logger.UpdateCommandResult(checkoutLogID, checkoutOutput, "", &checkoutExitCode, checkoutErr)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "切换到 main 分支失败: " + checkoutErr.Error()})
		return
	}
	logger.UpdateCommandResult(checkoutLogID, checkoutOutput, "", &checkoutExitCode, nil)

	// 2. 合并分支
	mergeLogID := logger.LogCommand(models.CommandLogCreate{
		Source:     "git_merge",
		SourceID:   &task.ID,
		SourceName: "Git Merge: " + task.Content,
		Command:    "git",
		Args:       []string{"merge", task.Branch, "-m", "Merge " + task.Branch + ": " + commitMessage},
		WorkDir:    workDir,
		Fields: models.BuildCommandLogFields(
			models.NewCommandLogField("operation", "操作", "合并分支", "default"),
			models.NewCommandLogField("source_branch", "来源分支", task.Branch, "default"),
			models.NewCommandLogField("target_branch", "目标分支", "main", "default"),
			models.NewCommandLogField("commit_message", "提交信息", "Merge "+task.Branch+": "+commitMessage, "default"),
		),
	})

	mergeOutput, mergeErr := coregit.Merge(workDir, task.Branch, "Merge "+task.Branch+": "+commitMessage)
	mergeExitCode := 0
	if mergeErr != nil {
		mergeExitCode = -1
		logger.UpdateCommandResult(mergeLogID, mergeOutput, "", &mergeExitCode, mergeErr)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "合并失败: " + mergeErr.Error() + "\n输出: " + mergeOutput})
		return
	}
	logger.UpdateCommandResult(mergeLogID, mergeOutput, "", &mergeExitCode, nil)

	c.JSON(http.StatusOK, gin.H{
		"message": "合并成功: " + task.Branch + " -> main",
		"branch":  task.Branch,
		"output":  mergeOutput,
	})
}

// RestoreSnapshot 恢复到指定的快照
func (h *GitHandler) RestoreSnapshot(c *gin.Context) {
	taskIDStr := c.Param("id")

	var req struct {
		Hash  string `json:"hash" binding:"required"`
		Clean bool   `json:"clean"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	// 获取任务信息
	var taskID uint
	fmt.Sscanf(taskIDStr, "%d", &taskID)

	task, err := database.GetTaskByID(h.db, taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
		return
	}

	// 获取项目信息
	dbProject, err := database.GetProjectByID(h.db, task.ProjectID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "项目不存在"})
		return
	}

	// 确定工作目录
	workDir := task.WorkDir
	if workDir == "" {
		workDir = dbProject.Path
	}

	// 执行恢复
	task.WorkDir = workDir
	err = coregit.RestoreSnapshot(dbProject.GetID(), workDir, req.Hash, req.Clean)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "恢复快照失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "快照恢复成功"})
}

// ApplyTaskSnapshot 按模式撤销单步变更或恢复到某检查点并修剪后续快照
func (h *GitHandler) ApplyTaskSnapshot(c *gin.Context) {
	taskIDStr := c.Param("id")
	var req struct {
		Mode           string `json:"mode" binding:"required"` // undo_step | restore_checkpoint
		CodeSnapshotID string `json:"codeSnapshotId"`
		PartID         string `json:"partId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}
	if req.Mode != "undo_step" && req.Mode != "restore_checkpoint" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 mode"})
		return
	}

	var taskID uint
	fmt.Sscanf(taskIDStr, "%d", &taskID)

	task, err := database.GetTaskByID(h.db, taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
		return
	}

	dbProject, err := database.GetProjectByID(h.db, task.ProjectID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "项目不存在"})
		return
	}

	workDir := task.WorkDir
	if workDir == "" {
		workDir = dbProject.Path
	}
	if _, err := os.Stat(workDir); os.IsNotExist(err) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "工作目录不存在: " + workDir})
		return
	}

	sessionID := resolveTaskSessionID(h.db, task)
	_, patches, err := h.collectTaskDiffPatches(task, sessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	idx := findTaskDiffPatchIndex(patches, req.CodeSnapshotID, req.PartID)
	if idx < 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "未找到对应快照"})
		return
	}

	p := patches[idx]
	if !taskDiffPatchBelongsToTask(h.db, task, p.SessionID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "快照不属于当前任务"})
		return
	}

	clean := true
	if req.Mode == "undo_step" {
		if strings.TrimSpace(p.Hash) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "无法撤销：缺少起始快照"})
			return
		}
		if err := coregit.RestoreSnapshot(dbProject.GetID(), workDir, p.Hash, clean); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "撤销变更失败: " + err.Error()})
			return
		}
		if p.ID != "" {
			_ = storage.DeleteMessageCodeSnapshotByID(h.db, p.ID)
		}
		if err := storage.DeletePart(h.db, &types.Part{
			ID:        p.PartID,
			MessageID: p.MessageID,
			SessionID: p.SessionID,
		}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "删除快照部件失败: " + err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "已撤销本次更改"})
		return
	}

	// restore_checkpoint
	if strings.TrimSpace(p.Snapshot) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无法恢复：缺少结束快照"})
		return
	}
	if err := coregit.RestoreSnapshot(dbProject.GetID(), workDir, p.Snapshot, clean); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "恢复快照失败: " + err.Error()})
		return
	}
	for j := idx + 1; j < len(patches); j++ {
		q := patches[j]
		if q.ID != "" {
			_ = storage.DeleteMessageCodeSnapshotByID(h.db, q.ID)
		}
		if err := storage.DeletePart(h.db, &types.Part{
			ID:        q.PartID,
			MessageID: q.MessageID,
			SessionID: q.SessionID,
		}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "删除后续快照部件失败: " + err.Error()})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"message": "已恢复到此快照并移除后续检查点"})
}

func findTaskDiffPatchIndex(patches []taskDiffPatch, codeSnapshotID, partID string) int {
	for i, p := range patches {
		if codeSnapshotID != "" && p.ID == codeSnapshotID {
			return i
		}
		if partID != "" && p.PartID == partID {
			return i
		}
	}
	return -1
}

func taskDiffPatchBelongsToTask(db *gorm.DB, task *models.Task, sessionID string) bool {
	if task == nil || db == nil || strings.TrimSpace(sessionID) == "" {
		return false
	}
	for _, sid := range collectTaskSessionIDs(db, task, "") {
		if sid == sessionID {
			return true
		}
	}
	return false
}

func stringSliceFromSnapshotFilesField(f models.JSONField) []string {
	if f.Data == nil {
		return nil
	}
	if s, ok := f.Data.([]string); ok {
		return append([]string(nil), s...)
	}
	arr, ok := f.Data.([]interface{})
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, v := range arr {
		if str, ok := v.(string); ok {
			out = append(out, str)
		}
	}
	return out
}

// GetExecutionGitState 获取指定执行的 Git 状态
func (h *GitHandler) GetExecutionGitState(c *gin.Context) {
	executionIDStr := c.Param("execId")
	executionID := parseUint(executionIDStr)
	if executionID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的执行 ID"})
		return
	}

	execution, err := database.GetTaskExecutionByID(h.db, executionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "执行不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"commitHash":     execution.GitCommitBefore,
		"branch":         execution.GitBranchBefore,
		"isDirty":        execution.GitDirtyBefore,
		"modifiedCount":  execution.GitModifiedCount,
		"untrackedCount": execution.GitUntrackedCount,
	})
}

// GetCurrentGitState 获取任务当前的 Git 状态
func (h *GitHandler) GetCurrentGitState(c *gin.Context) {
	taskIDStr := c.Param("id")
	taskID := parseUint(taskIDStr)
	if taskID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的任务 ID"})
		return
	}

	task, err := database.GetTaskByID(h.db, taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
		return
	}

	workDir := task.WorkDir
	if workDir == "" {
		project, err := database.GetProjectByID(h.db, task.ProjectID)
		if err == nil {
			workDir = project.Path
		}
	}

	if workDir == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无法确定工作目录"})
		return
	}

	state, err := services.GetGitRepoState(workDir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取 Git 状态失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, state)
}

// GetTaskDiff 获取任务的 Git diff
func (h *GitHandler) GetTaskDiff(c *gin.Context) {
	taskIDStr := c.Param("id")
	hash := strings.TrimSpace(c.Query("hash"))
	toHash := strings.TrimSpace(c.Query("toHash"))
	gitFrom := strings.TrimSpace(c.Query("gitFrom"))
	gitTo := strings.TrimSpace(c.Query("gitTo"))
	atCommit := strings.TrimSpace(c.Query("atCommit"))
	basis := strings.TrimSpace(c.Query("basis"))
	if basis == "" {
		basis = "default"
	}

	var taskID uint
	fmt.Sscanf(taskIDStr, "%d", &taskID)

	task, err := database.GetTaskByID(h.db, taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
		return
	}

	dbProject, err := database.GetProjectByID(h.db, task.ProjectID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "项目不存在"})
		return
	}

	workDir := task.WorkDir
	if workDir == "" {
		workDir = dbProject.Path
	}

	if _, err := os.Stat(workDir); os.IsNotExist(err) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "工作目录不存在: " + workDir})
		return
	}
	task.WorkDir = workDir

	baseRef := strings.TrimSpace(task.BaseBranch)
	if baseRef == "" {
		baseRef = "main"
	}

	sessionID := resolveTaskSessionID(h.db, task)

	includePatches := c.Query("patches") != "0"
	var firstStartSnapshot string
	var patches []taskDiffPatch
	if includePatches {
		firstStartSnapshot, patches, err = h.collectTaskDiffPatches(task, sessionID)
	} else {
		firstStartSnapshot, err = firstStartSnapshotOnly(h.db, task, sessionID)
		patches = []taskDiffPatch{}
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var diffResult coregit.Result

	if gitFrom != "" && gitTo != "" {
		diffResult, err = coregit.NativeGitDiffRange(workDir, gitFrom, gitTo)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "获取 diff 失败: " + err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"type":    diffResult.Type,
			"diff":    diffResult.Diff,
			"files":   diffResult.Files,
			"patches": patches,
		})
		return
	}

	if atCommit != "" {
		diffResult, err = coregit.DiffCommitRange(workDir, atCommit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "获取提交 diff 失败: " + err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"type":    diffResult.Type,
			"diff":    diffResult.Diff,
			"files":   diffResult.Files,
			"patches": patches,
		})
		return
	}

	if hash == "" {
		baseHash := firstStartSnapshot

		if toHash != "" {
			diffResult, err = coregit.SnapshotDiffRange(dbProject.GetID(), workDir, baseHash, toHash)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "获取 snapshot diff 失败: " + err.Error()})
				return
			}
		} else {
			switch basis {
			case "base":
				diffResult, err = coregit.WorktreeDiffFromBase(workDir, baseRef)
			case "parent":
				diffResult, err = coregit.WorkingTreeDiffFromParent(workDir)
			default:
				diffResult, err = coregit.WorkingTreeDiff(workDir)
			}
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "获取工作区 diff 失败: " + err.Error()})
				return
			}
		}
	} else {
		if toHash != "" {
			diffResult, err = coregit.SnapshotDiffRange(dbProject.GetID(), workDir, hash, toHash)
		} else {
			diffResult, err = coregit.SnapshotDiff(dbProject.GetID(), workDir, hash)
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "获取 snapshot diff 失败: " + err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"type":    diffResult.Type,
		"diff":    diffResult.Diff,
		"files":   diffResult.Files,
		"patches": patches,
	})
}

// firstStartSnapshotOnly 仅解析首个会话 StartSnapshot，不加载消息/部件（供 diff 快速路径）。
func firstStartSnapshotOnly(db *gorm.DB, task *models.Task, preferredSessionID string) (string, error) {
	if task == nil || db == nil {
		return "", nil
	}
	for _, sid := range collectTaskSessionIDs(db, task, preferredSessionID) {
		sess, err := storage.GetSession(db, sid)
		if err != nil {
			return "", fmt.Errorf("获取会话信息失败: %w", err)
		}
		if sess != nil && strings.TrimSpace(sess.StartSnapshot) != "" {
			return sess.StartSnapshot, nil
		}
	}
	return "", nil
}

// patchTimestampMs 将 collectTaskDiffPatches 中的 Timestamp 统一为毫秒。
func patchTimestampMs(ts int64) int64 {
	if ts <= 0 {
		return 0
	}
	if ts < 1_000_000_000_000 {
		return ts * 1000
	}
	return ts
}

// collectTaskDiffPatchesTimeline 仅基于 message_code_snapshots 表组装检查点，不扫描全会话消息（时间线用）。
func (h *GitHandler) collectTaskDiffPatchesTimeline(task *models.Task, preferredSessionID string) (anchorMs int64, patches []taskDiffPatch, err error) {
	if task == nil {
		return 0, nil, nil
	}
	sessionIDs := collectTaskSessionIDs(h.db, task, preferredSessionID)
	if len(sessionIDs) == 0 {
		if !task.CreatedAt.IsZero() {
			return task.CreatedAt.UnixMilli(), nil, nil
		}
		return 0, nil, nil
	}
	patches = make([]taskDiffPatch, 0, 16)
	anchorMs = 0
	for _, sessionID := range sessionIDs {
		sess, err := storage.GetSession(h.db, sessionID)
		if err != nil {
			return 0, nil, fmt.Errorf("获取会话信息失败: %w", err)
		}
		if sess == nil || strings.TrimSpace(sess.StartSnapshot) == "" {
			continue
		}
		if anchorMs == 0 {
			if sess.Time.Created > 0 {
				anchorMs = sess.Time.Created
			} else if !task.CreatedAt.IsZero() {
				anchorMs = task.CreatedAt.UnixMilli()
			}
		}
		tableRows, err := storage.ListMessageCodeSnapshotsBySessionID(h.db, sessionID)
		if err != nil {
			return 0, nil, fmt.Errorf("读取代码快照失败: %w", err)
		}
		for _, row := range tableRows {
			patches = append(patches, taskDiffPatch{
				ID:            row.ID,
				PartID:        row.PartID,
				Hash:          row.StartHash,
				Snapshot:      row.EndHash,
				MessageID:     row.MessageID,
				Timestamp:     row.Created,
				Files:         stringSliceFromSnapshotFilesField(row.Files),
				Description:   row.Description,
				SessionID:     sessionID,
				StartSnapshot: sess.StartSnapshot,
			})
		}
	}
	if anchorMs == 0 && !task.CreatedAt.IsZero() {
		anchorMs = task.CreatedAt.UnixMilli()
	}
	sort.SliceStable(patches, func(i, j int) bool {
		if patches[i].Timestamp == patches[j].Timestamp {
			if patches[i].MessageID == patches[j].MessageID {
				return patches[i].PartID < patches[j].PartID
			}
			return patches[i].MessageID < patches[j].MessageID
		}
		return patches[i].Timestamp < patches[j].Timestamp
	})
	return anchorMs, patches, nil
}

// GetTaskGitTimeline 当前任务分支相对 base 的提交时间线 + 会话快照（合并排序）。
// 仅包含「任务首次会话 StartSnapshot 落库之后」的提交与检查点，避免展示任务开始前分支上已有历史。
func (h *GitHandler) GetTaskGitTimeline(c *gin.Context) {
	taskIDStr := c.Param("id")
	var taskID uint
	fmt.Sscanf(taskIDStr, "%d", &taskID)
	if taskID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的任务 ID"})
		return
	}

	task, err := database.GetTaskByID(h.db, taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
		return
	}

	dbProject, err := database.GetProjectByID(h.db, task.ProjectID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "项目不存在"})
		return
	}

	workDir := task.WorkDir
	if workDir == "" {
		workDir = dbProject.Path
	}
	if _, err := os.Stat(workDir); os.IsNotExist(err) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "工作目录不存在: " + workDir})
		return
	}

	baseBranch := strings.TrimSpace(task.BaseBranch)
	if baseBranch == "" {
		baseBranch = "main"
	}
	baseCommitHash := strings.TrimSpace(task.BaseCommitHash)
	if baseCommitHash == "" {
		baseCommitHash, _ = coregit.RevParse(dbProject.Path, baseBranch)
	}

	commits, err := coregit.LogCommitsSince(workDir, baseCommitHash, 200)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取提交历史失败: " + err.Error()})
		return
	}

	sessionID := resolveTaskSessionID(h.db, task)
	anchorMs, patches, err := h.collectTaskDiffPatchesTimeline(task, sessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	type timelineItem struct {
		Kind      string             `json:"kind"`
		Timestamp int64              `json:"timestamp"`
		Commit    *coregit.LogCommit `json:"commit,omitempty"`
		Snapshot  *taskDiffPatch     `json:"snapshot,omitempty"`
	}

	items := make([]timelineItem, 0, len(commits)+len(patches))
	for i := range commits {
		cm := commits[i]
		if anchorMs > 0 {
			if cm.UnixSec*1000 <= anchorMs {
				continue
			}
		}
		copied := cm
		items = append(items, timelineItem{
			Kind:      "commit",
			Timestamp: cm.UnixSec,
			Commit:    &copied,
		})
	}
	for i := range patches {
		p := patches[i]
		pc := p
		pMs := patchTimestampMs(pc.Timestamp)
		if anchorMs > 0 && pMs > 0 && pMs <= anchorMs {
			continue
		}
		ts := pc.Timestamp
		if ts > 1_000_000_000_000 {
			ts /= 1000
		}
		items = append(items, timelineItem{
			Kind:      "snapshot",
			Timestamp: ts,
			Snapshot:  &pc,
		})
	}

	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Timestamp == items[j].Timestamp {
			return items[i].Kind < items[j].Kind
		}
		return items[i].Timestamp > items[j].Timestamp
	})

	c.JSON(http.StatusOK, gin.H{
		"baseBranch":     baseBranch,
		"baseCommitHash": baseCommitHash,
		"items":          items,
	})
}

// GitRestoreWorktreeRef 将工作区恢复到指定 Git tree-ish（提交哈希等）。
func (h *GitHandler) GitRestoreWorktreeRef(c *gin.Context) {
	taskIDStr := c.Param("id")
	var taskID uint
	fmt.Sscanf(taskIDStr, "%d", &taskID)
	if taskID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的任务 ID"})
		return
	}

	var req struct {
		Ref   string `json:"ref" binding:"required"`
		Clean bool   `json:"clean"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	task, err := database.GetTaskByID(h.db, taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
		return
	}

	dbProject, err := database.GetProjectByID(h.db, task.ProjectID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "项目不存在"})
		return
	}

	workDir := task.WorkDir
	if workDir == "" {
		workDir = dbProject.Path
	}
	if _, err := os.Stat(workDir); os.IsNotExist(err) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "工作目录不存在: " + workDir})
		return
	}

	if err := coregit.RestoreWorktreeTreeish(workDir, req.Ref, req.Clean); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "恢复失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "工作区已恢复到指定提交"})
}

func (h *GitHandler) collectTaskDiffPatches(task *models.Task, preferredSessionID string) (string, []taskDiffPatch, error) {
	sessionIDs := collectTaskSessionIDs(h.db, task, preferredSessionID)
	if len(sessionIDs) == 0 {
		return "", nil, nil
	}

	patches := make([]taskDiffPatch, 0, 8)
	firstStartSnapshot := ""
	for _, sessionID := range sessionIDs {
		sess, err := storage.GetSession(h.db, sessionID)
		if err != nil {
			return "", nil, fmt.Errorf("获取会话信息失败: %w", err)
		}
		if strings.TrimSpace(sess.StartSnapshot) == "" {
			continue
		}
		if firstStartSnapshot == "" {
			firstStartSnapshot = sess.StartSnapshot
		}

		tableRows, err := storage.ListMessageCodeSnapshotsBySessionID(h.db, sessionID)
		if err != nil {
			return "", nil, fmt.Errorf("读取代码快照失败: %w", err)
		}
		for _, row := range tableRows {
			patches = append(patches, taskDiffPatch{
				ID:            row.ID,
				PartID:        row.PartID,
				Hash:          row.StartHash,
				Snapshot:      row.EndHash,
				MessageID:     row.MessageID,
				Timestamp:     row.Created,
				Files:         stringSliceFromSnapshotFilesField(row.Files),
				Description:   row.Description,
				SessionID:     sessionID,
				StartSnapshot: sess.StartSnapshot,
			})
		}

		messagesWithParts, err := storage.GetMessageWithPartsBySessionID(h.db, sessionID)
		if err != nil {
			continue
		}
		for _, msg := range messagesWithParts {
			if msg == nil || msg.Info == nil {
				continue
			}
			for _, part := range msg.Parts {
				if part == nil || part.Type != "patch" || strings.TrimSpace(part.Hash) == "" {
					continue
				}
				hasRow, rowErr := storage.MessageCodeSnapshotExistsForPartID(h.db, part.ID)
				if rowErr != nil || hasRow {
					continue
				}
				patches = append(patches, taskDiffPatch{
					PartID:        part.ID,
					Hash:          part.Hash,
					Snapshot:      part.Snapshot,
					MessageID:     part.MessageID,
					Timestamp:     msg.Info.Time.Created,
					Files:         append([]string(nil), part.Files...),
					Description:   part.Description,
					SessionID:     sessionID,
					StartSnapshot: sess.StartSnapshot,
				})
			}
		}
	}

	sort.SliceStable(patches, func(i, j int) bool {
		if patches[i].Timestamp == patches[j].Timestamp {
			if patches[i].MessageID == patches[j].MessageID {
				return patches[i].PartID < patches[j].PartID
			}
			return patches[i].MessageID < patches[j].MessageID
		}
		return patches[i].Timestamp < patches[j].Timestamp
	})

	return firstStartSnapshot, patches, nil
}

func collectTaskSessionIDs(db *gorm.DB, task *models.Task, preferredSessionID string) []string {
	ordered := make([]string, 0, 4)
	seen := map[string]struct{}{}
	appendSession := func(sessionID string) {
		trimmed := strings.TrimSpace(sessionID)
		if trimmed == "" {
			return
		}
		if _, ok := seen[trimmed]; ok {
			return
		}
		seen[trimmed] = struct{}{}
		ordered = append(ordered, trimmed)
	}

	appendSession(preferredSessionID)
	if task != nil {
		appendSession(task.SessionID)
	}
	if task == nil || db == nil {
		return ordered
	}

	executions, err := database.GetExecutionsByTaskIDOrdered(db, task.ID, true)
	if err != nil {
		return ordered
	}
	for _, execution := range executions {
		appendSession(execution.AgentSessionID)
	}
	return ordered
}
