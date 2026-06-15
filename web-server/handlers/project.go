package handlers

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	database "pkgs/db"
	"pkgs/db/models"
	"pkgs/db/storage"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ProjectHandler 项目处理器
type ProjectHandler struct {
	db *gorm.DB
}

// NewProjectHandler 创建项目处理器
func NewProjectHandler(db *gorm.DB) *ProjectHandler {
	return &ProjectHandler{db: db}
}

func (h *ProjectHandler) normalizeMemoryLibraryIDs(ids []uint) ([]uint, error) {
	if len(ids) == 0 {
		return []uint{}, nil
	}
	seen := make(map[uint]struct{}, len(ids))
	normalized := make([]uint, 0, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		normalized = append(normalized, id)
	}
	if len(normalized) == 0 {
		return []uint{}, nil
	}
	libraries, err := database.GetMemoryLibrariesByIDs(h.db, normalized)
	if err != nil {
		return nil, err
	}
	if len(libraries) != len(normalized) {
		return nil, fmt.Errorf("部分记忆库不存在")
	}
	for _, library := range libraries {
		if library.IsTemporary {
			return nil, fmt.Errorf("不能关联临时记忆库")
		}
		if library.IsRag {
			return nil, fmt.Errorf("不能关联 RAG 知识库")
		}
	}
	return normalized, nil
}

// createProjectDirectory 创建项目目录并初始化 git (导出方法)
func (h *ProjectHandler) createProjectDirectory(path, projectName string) (string, error) {
	// 构建完整路径
	projectPath := filepath.Join(path, projectName)

	// 创建目录
	if err := os.MkdirAll(projectPath, 0755); err != nil {
		return "", fmt.Errorf("创建项目目录失败: %v", err)
	}

	// 初始化 git 仓库
	cmd := exec.Command("git", "init")
	cmd.Dir = projectPath
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git 初始化失败: %v, output: %s", err, string(output))
	}

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
	gitignorePath := filepath.Join(projectPath, ".gitignore")
	if err := os.WriteFile(gitignorePath, []byte(gitignoreContent), 0644); err != nil {
		return "", fmt.Errorf("创建 .gitignore 失败: %v", err)
	}

	// 创建基础 README.md
	readmeContent := fmt.Sprintf("# %s\n\nProject initialized by MatrixOps.\n", projectName)
	readmePath := filepath.Join(projectPath, "README.md")
	if err := os.WriteFile(readmePath, []byte(readmeContent), 0644); err != nil {
		return "", fmt.Errorf("创建 README.md 失败: %v", err)
	}

	// 初始提交
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = projectPath
	if _, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git add 失败: %v", err)
	}

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = projectPath
	if _, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git commit 失败: %v", err)
	}

	return projectPath, nil
}

// GetAllProjects 获取所有项目
func (h *ProjectHandler) GetAllProjects(c *gin.Context) {
	projects, err := database.GetAllProjects(h.db)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取项目列表失败: " + err.Error()})
		return
	}

	// 检查每个项目的路径是否存在
	responses := make([]models.ProjectResponse, 0, len(projects))
	for _, proj := range projects {
		pathExists := checkPathExists(proj.Path)
		response := models.ProjectResponse{
			Project:    proj,
			PathExists: pathExists,
		}
		if !pathExists {
			response.Error = "目录不存在"
		}
		responses = append(responses, response)
	}

	c.JSON(http.StatusOK, responses)
}

// GetProjects 获取工作区的所有项目
func (h *ProjectHandler) GetProjects(c *gin.Context) {
	workspaceID := c.Param("id")

	// 解析 workspaceID
	var wid uint
	fmt.Sscanf(workspaceID, "%d", &wid)

	projects, err := database.GetProjectsByWorkspaceID(h.db, wid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取项目列表失败"})
		return
	}

	// 检查每个项目的路径是否存在
	responses := make([]models.ProjectResponse, 0, len(projects))
	projectsToDelete := make([]uint, 0)

	for _, proj := range projects {
		pathExists := checkPathExists(proj.Path)
		response := models.ProjectResponse{
			Project:    proj,
			PathExists: pathExists,
		}

		if !pathExists {
			response.Error = "目录不存在"
			projectsToDelete = append(projectsToDelete, proj.ID)
		}

		responses = append(responses, response)
	}

	// 自动删除路径不存在的项目
	if len(projectsToDelete) > 0 {
		database.DeleteProjectsBatch(h.db, wid, projectsToDelete)
	}

	c.JSON(http.StatusOK, responses)
}

// GetProject 获取单个项目
func (h *ProjectHandler) GetProject(c *gin.Context) {
	id := c.Param("id")

	var pid uint
	fmt.Sscanf(id, "%d", &pid)

	project, err := database.GetProjectByID(h.db, pid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "项目不存在"})
		return
	}

	pathExists := checkPathExists(project.Path)
	response := models.ProjectResponse{
		Project:    *project,
		PathExists: pathExists,
	}

	if !pathExists {
		response.Error = "目录不存在"
		database.DeleteProject(h.db, pid)
		c.JSON(http.StatusGone, response)
		return
	}

	c.JSON(http.StatusOK, response)
}

// CreateStandaloneProject 创建独立项目（不关联工作区）
func (h *ProjectHandler) CreateStandaloneProject(c *gin.Context) {
	var req models.ProjectCreate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	toolPermissions, err := resolveProjectToolPermissions(req.ToolPermissions, h.db)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "toolPermissions 格式错误"})
		return
	}
	memoryLibraryIDs, err := h.normalizeMemoryLibraryIDs(req.MemoryLibraryIDs)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "memoryLibraryIds 无效", "details": err.Error()})
		return
	}

	var projectPath string

	// 使用项目路径（独立项目必须提供路径）
	if req.Path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "必须提供项目路径"})
		return
	}

	if !checkPathExists(req.Path) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "项目路径不存在"})
		return
	}

	projectPath = req.Path

	// 设置默认值
	if req.Icon == "" {
		req.Icon = "code"
	}
	if req.Color == "" {
		req.Color = "blue"
	}

	project := models.Project{
		Name:             req.Name,
		Path:             projectPath,
		WorktreePath:     projectPath,
		Icon:             req.Icon,
		Color:            req.Color,
		Status:           "Idle",
		ToolPermissions:  toolPermissions,
		MemoryLibraryIDs: models.UintSlice(memoryLibraryIDs),
		YoloMode:         req.YoloMode,
	}

	if err := database.CreateProject(h.db, &project); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建项目失败: " + err.Error()})
		return
	}

	response := models.ProjectResponse{
		Project:    project,
		PathExists: true,
	}

	c.JSON(http.StatusCreated, response)
}

// CreateProject 创建项目并关联到工作区
func (h *ProjectHandler) CreateProject(c *gin.Context) {
	workspaceID := c.Param("id")

	var req models.ProjectCreate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	toolPermissions, err := resolveProjectToolPermissions(req.ToolPermissions, h.db)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "toolPermissions 格式错误"})
		return
	}
	memoryLibraryIDs, err := h.normalizeMemoryLibraryIDs(req.MemoryLibraryIDs)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "memoryLibraryIds 无效", "details": err.Error()})
		return
	}

	// 验证工作区是否存在
	var wid uint
	fmt.Sscanf(workspaceID, "%d", &wid)

	workspace, err := database.GetWorkspaceByID(h.db, wid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "工作区不存在"})
		return
	}

	var projectPath string

	if req.CreateNew {
		// 创建新项目
		parentPath := req.NewPath
		if parentPath == "" {
			parentPath = workspace.Path // 默认在工作区路径下创建
		}

		// 验证父目录是否存在
		if !checkPathExists(parentPath) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "父目录不存在"})
			return
		}

		// 清理项目名称，移除特殊字符
		safeName := strings.ReplaceAll(req.Name, " ", "-")
		safeName = strings.ToLower(safeName)

		projectPath, err = h.createProjectDirectory(parentPath, safeName)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	} else {
		// 使用现有项目路径
		if req.Path == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "必须提供项目路径"})
			return
		}

		if !checkPathExists(req.Path) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "项目路径不存在"})
			return
		}

		projectPath = req.Path
	}

	// 设置默认值
	if req.Icon == "" {
		req.Icon = "code"
	}
	if req.Color == "" {
		req.Color = "blue"
	}

	project := models.Project{
		Name:             req.Name,
		Path:             projectPath,
		WorktreePath:     projectPath,
		Icon:             req.Icon,
		Color:            req.Color,
		Status:           "Idle",
		ToolPermissions:  toolPermissions,
		MemoryLibraryIDs: models.UintSlice(memoryLibraryIDs),
		YoloMode:         req.YoloMode,
	}

	if err := database.CreateProject(h.db, &project); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建项目失败: " + err.Error()})
		return
	}

	// 添加项目到工作区
	if err := database.AddProjectToWorkspace(h.db, workspace.ID, project.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "添加项目到工作区失败: " + err.Error()})
		return
	}

	response := models.ProjectResponse{
		Project:    project,
		PathExists: true,
	}

	c.JSON(http.StatusCreated, response)
}

// UpdateProject 更新项目
func (h *ProjectHandler) UpdateProject(c *gin.Context) {
	id := c.Param("id")

	var pid uint
	fmt.Sscanf(id, "%d", &pid)

	project, err := database.GetProjectByID(h.db, pid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "项目不存在"})
		return
	}

	var req models.ProjectUpdate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	// 更新字段
	if req.Name != nil {
		project.Name = *req.Name
	}
	if req.Path != nil {
		if !checkPathExists(*req.Path) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "指定的路径不存在"})
			return
		}
		project.Path = *req.Path
		project.WorktreePath = *req.Path
	}
	if req.Icon != nil {
		project.Icon = *req.Icon
	}
	if req.Color != nil {
		project.Color = *req.Color
	}
	if req.Status != nil {
		project.Status = *req.Status
	}
	if req.ActiveTasks != nil {
		project.ActiveTasks = *req.ActiveTasks
	}
	if req.ToolPermissions != nil {
		toolPermissions, err := normalizeProjectToolPermissions(*req.ToolPermissions)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "toolPermissions 格式错误"})
			return
		}
		project.ToolPermissions = toolPermissions
	}
	if req.MemoryLibraryIDs != nil {
		memoryLibraryIDs, err := h.normalizeMemoryLibraryIDs(*req.MemoryLibraryIDs)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "memoryLibraryIds 无效", "details": err.Error()})
			return
		}
		project.MemoryLibraryIDs = models.UintSlice(memoryLibraryIDs)
	}
	if req.YoloMode != nil {
		project.YoloMode = *req.YoloMode
	}

	if err := database.UpdateProject(h.db, project); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新项目失败"})
		return
	}

	response := models.ProjectResponse{
		Project:    *project,
		PathExists: checkPathExists(project.Path),
	}

	c.JSON(http.StatusOK, response)
}

// DeleteProject 删除项目
func (h *ProjectHandler) DeleteProject(c *gin.Context) {
	id := c.Param("id")

	var pid uint
	fmt.Sscanf(id, "%d", &pid)

	if _, err := database.GetProjectByID(h.db, pid); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "项目不存在"})
		return
	}

	workspace, err := database.GetWorkspaceByProjectID(h.db, pid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询项目所属工作区失败"})
		return
	}

	tasks, err := database.GetTasksByWorkspaceIDAndProjectID(h.db, workspace.ID, pid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询项目任务失败"})
		return
	}

	for _, task := range tasks {
		if task.SessionID != "" {
			_ = storage.DeleteMemoryEntriesBySession(h.db, task.SessionID)
			_ = storage.DeleteMessageBySession(h.db, task.SessionID)
			_ = storage.DeleteSession(h.db, task.SessionID)
		}
		_ = database.DeleteExecutionLogsByTaskID(h.db, task.ID)
		_ = database.DeleteExecutionsByTaskID(h.db, task.ID)
	}

	if err := database.DeleteTasksByWorkspaceIDAndProjectID(h.db, workspace.ID, pid); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除项目任务失败"})
		return
	}

	if err := database.RemoveProjectFromAllWorkspaces(h.db, pid); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新工作区项目列表失败"})
		return
	}

	_ = database.DeleteOpenUIApplicationByKindResource(h.db, models.OpenUIKindProject, pid)

	if err := database.DeleteProject(h.db, pid); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除项目失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "项目删除成功"})
}

func normalizeProjectToolPermissions(raw string) (string, error) {
	values, err := models.ParseProjectToolPermissions(raw)
	if err != nil {
		return "", err
	}
	return models.NormalizeProjectToolPermissionsJSON(values), nil
}

func resolveProjectToolPermissions(raw string, db *gorm.DB) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return database.GetDefaultProjectToolPermissionsJSON(db), nil
	}
	return normalizeProjectToolPermissions(raw)
}

// AddProjectToWorkspace 添加已存在的项目到工作区
func (h *ProjectHandler) AddProjectToWorkspace(c *gin.Context) {
	workspaceID := c.Param("id")
	projectID := c.Param("projectId")

	var wid, pid uint
	fmt.Sscanf(workspaceID, "%d", &wid)
	fmt.Sscanf(projectID, "%d", &pid)

	// 验证工作区是否存在
	if _, err := database.GetWorkspaceByID(h.db, wid); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "工作区不存在"})
		return
	}

	// 验证项目是否存在
	if _, err := database.GetProjectByID(h.db, pid); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "项目不存在"})
		return
	}

	// 检查项目是否已在工作区中
	exists, err := database.IsProjectInWorkspace(h.db, wid, pid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "检查项目状态失败"})
		return
	}
	if exists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "项目已在该工作区中"})
		return
	}

	// 添加项目到工作区
	if err := database.AddProjectToWorkspace(h.db, wid, pid); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "添加项目失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "项目添加成功"})
}

// RemoveProjectFromWorkspace 从工作区移除项目（不删除项目本身）
func (h *ProjectHandler) RemoveProjectFromWorkspace(c *gin.Context) {
	workspaceID := c.Param("id")
	projectID := c.Param("projectId")

	var wid, pid uint
	fmt.Sscanf(workspaceID, "%d", &wid)
	fmt.Sscanf(projectID, "%d", &pid)

	// 从工作区移除项目
	if err := database.RemoveProjectFromWorkspace(h.db, wid, pid); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "移除项目失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "项目移除成功"})
}
