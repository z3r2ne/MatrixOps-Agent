package handlers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	database "pkgs/db"
	"pkgs/db/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// WorkspaceHandler 工作区处理器
type WorkspaceHandler struct {
	db *gorm.DB
}

// NewWorkspaceHandler 创建工作区处理器
func NewWorkspaceHandler(db *gorm.DB) *WorkspaceHandler {
	return &WorkspaceHandler{db: db}
}

// checkPathExists 检查路径是否存在
func checkPathExists(path string) bool {
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

// generateDefaultWorkspacePath 生成默认工作区路径
func generateDefaultWorkspacePath() (string, error) {
	return database.DefaultWorkspacePath()
}

func resolveWorkspaceGroupMode(raw *string, fallback models.TaskListGroupMode) (models.TaskListGroupMode, error) {
	if raw == nil || *raw == "" {
		return fallback, nil
	}
	mode, ok := models.NormalizeTaskListGroupMode(*raw)
	if !ok {
		return "", fmt.Errorf("无效的任务列表分组方式")
	}
	return mode, nil
}

func resolveWorkspaceType(raw *string, fallback models.WorkspaceType) (models.WorkspaceType, error) {
	if raw == nil || *raw == "" {
		return fallback, nil
	}
	t, ok := models.NormalizeWorkspaceType(*raw)
	if !ok {
		return "", fmt.Errorf("无效的工作区类型")
	}
	return t, nil
}

// GetWorkspaces 获取所有工作区
func (h *WorkspaceHandler) GetWorkspaces(c *gin.Context) {
	workspaces, err := database.GetAllWorkspaces(h.db)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取工作区列表失败"})
		return
	}

	// 检查每个工作区的路径是否存在
	responses := make([]models.WorkspaceResponse, 0, len(workspaces))
	workspacesToDelete := make([]uint, 0)

	for _, ws := range workspaces {
		pathExists := checkPathExists(ws.Path)
		response := models.WorkspaceResponse{
			Workspace:  ws,
			PathExists: pathExists,
		}

		if !pathExists {
			response.Error = "目录不存在"
			workspacesToDelete = append(workspacesToDelete, ws.ID)
		}

		responses = append(responses, response)
	}

	// 自动删除路径不存在的工作区
	if len(workspacesToDelete) > 0 {
		err := database.DeleteWorkspacesBatch(h.db, workspacesToDelete)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "删除工作区失败: " + err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, responses)
}

// GetWorkspace 获取单个工作区
func (h *WorkspaceHandler) GetWorkspace(c *gin.Context) {
	id := c.Param("id")

	var wid uint
	fmt.Sscanf(id, "%d", &wid)

	// 使用 Preload 加载关联的项目
	workspace, err := database.GetWorkspaceByIDWithProjects(h.db, wid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "工作区不存在"})
		return
	}

	pathExists := checkPathExists(workspace.Path)
	response := models.WorkspaceResponse{
		Workspace:  *workspace,
		PathExists: pathExists,
	}

	if !pathExists {
		response.Error = "目录不存在"
		// 自动删除
		database.DeleteWorkspace(h.db, wid)
		c.JSON(http.StatusGone, response)
		return
	}

	c.JSON(http.StatusOK, response)
}

// CreateWorkspace 创建工作区
func (h *WorkspaceHandler) CreateWorkspace(c *gin.Context) {
	var req models.WorkspaceCreate

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	// 如果没有提供路径，生成默认路径
	if req.Path == "" {
		defaultPath, err := generateDefaultWorkspacePath()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "生成默认路径失败: " + err.Error()})
			return
		}
		req.Path = defaultPath
	} else {
		// 如果提供了路径，验证路径是否存在
		if !checkPathExists(req.Path) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "指定的路径不存在"})
			return
		}
	}

	// 设置默认值
	if req.Icon == "" {
		req.Icon = "folder"
	}
	if req.Color == "" {
		req.Color = "blue"
	}
	workspaceType, err := resolveWorkspaceType(&req.Type, models.DefaultWorkspaceType)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	groupMode, err := resolveWorkspaceGroupMode(req.GroupMode, database.GetDefaultTaskListGroupMode(h.db))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	workspace := models.Workspace{
		Name:      req.Name,
		Type:      workspaceType,
		Path:      req.Path,
		Icon:      req.Icon,
		Color:     req.Color,
		GroupMode: groupMode,
	}

	if err := database.CreateWorkspace(h.db, &workspace); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建工作区失败: " + err.Error()})
		return
	}

	// 创建关联的项目
	projectHandler := NewProjectHandler(h.db)
	createdProjects := make([]models.Project, 0)

	for _, projReq := range req.Projects {
		var projectPath string
		var err error

		if projReq.CreateNew {
			// 创建新项目
			parentPath := projReq.NewPath
			if parentPath == "" {
				parentPath = workspace.Path // 默认在工作区路径下创建
			}

			safeName := filepath.Base(projReq.Name)
			projectPath, err = projectHandler.createProjectDirectory(parentPath, safeName)
			if err != nil {
				// 记录错误但继续
				fmt.Printf("创建项目失败: %v\n", err)
				continue
			}
		} else {
			if projReq.Path != "" && checkPathExists(projReq.Path) {
				projectPath = projReq.Path
			} else {
				continue
			}
		}

		// 设置项目默认值
		if projReq.Icon == "" {
			projReq.Icon = "code"
		}
		if projReq.Color == "" {
			projReq.Color = "blue"
		}
		toolPermissions, err := resolveProjectToolPermissions(projReq.ToolPermissions, h.db)
		if err != nil {
			fmt.Printf("创建项目失败: toolPermissions 格式错误: %v\n", err)
			continue
		}

		project := models.Project{
			Name:            projReq.Name,
			Path:            projectPath,
			WorktreePath:    projectPath,
			Icon:            projReq.Icon,
			Color:           projReq.Color,
			Status:          "Idle",
			ToolPermissions: toolPermissions,
			YoloMode:        projReq.YoloMode,
		}

		if err := database.CreateProject(h.db, &project); err == nil {
			// 添加项目到工作区
			if err := database.AddProjectToWorkspace(h.db, workspace.ID, project.ID); err == nil {
				createdProjects = append(createdProjects, project)
			}
		}
	}

	// 重新加载工作区以包含项目
	workspace2, _ := database.GetWorkspaceByIDWithProjects(h.db, workspace.ID)
	if workspace2 != nil {
		workspace = *workspace2
	}

	response := models.WorkspaceResponse{
		Workspace:  workspace,
		PathExists: true,
	}

	c.JSON(http.StatusCreated, response)
}

// UpdateWorkspace 更新工作区
func (h *WorkspaceHandler) UpdateWorkspace(c *gin.Context) {
	id := c.Param("id")

	var wid uint
	fmt.Sscanf(id, "%d", &wid)

	workspace, err := database.GetWorkspaceByID(h.db, wid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "工作区不存在"})
		return
	}

	var req models.WorkspaceUpdate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	// 更新字段
	if req.Type != nil {
		workspaceType, err := resolveWorkspaceType(req.Type, workspace.Type)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		workspace.Type = workspaceType
	}
	if req.Name != nil {
		workspace.Name = *req.Name
	}
	if req.Path != nil {
		// 验证新路径是否存在
		if !checkPathExists(*req.Path) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "指定的路径不存在"})
			return
		}
		workspace.Path = *req.Path
	}
	if req.Icon != nil {
		workspace.Icon = *req.Icon
	}
	if req.Color != nil {
		workspace.Color = *req.Color
	}
	if req.GroupMode != nil {
		groupMode, err := resolveWorkspaceGroupMode(req.GroupMode, workspace.GroupMode)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		workspace.GroupMode = groupMode
	}
	if req.Active != nil {
		workspace.Active = *req.Active
	}

	if err := database.UpdateWorkspace(h.db, workspace); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新工作区失败"})
		return
	}

	response := models.WorkspaceResponse{
		Workspace:  *workspace,
		PathExists: checkPathExists(workspace.Path),
	}

	c.JSON(http.StatusOK, response)
}

// DeleteWorkspace 删除工作区
func (h *WorkspaceHandler) DeleteWorkspace(c *gin.Context) {
	id := c.Param("id")

	var wid uint
	fmt.Sscanf(id, "%d", &wid)

	workspace, err := database.GetWorkspaceByID(h.db, wid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "工作区不存在"})
		return
	}

	// 删除工作区目录
	if workspace.Path != "" {
		if err := os.RemoveAll(workspace.Path); err != nil && !os.IsNotExist(err) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "删除工作区目录失败: " + err.Error()})
			return
		}
	}

	_ = database.DeleteOpenUIApplicationByKindResource(h.db, models.OpenUIKindWorkspace, wid)

	if err := database.DeleteWorkspace(h.db, wid); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除工作区失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "工作区删除成功"})
}

// SetActiveWorkspace 设置活跃工作区
func (h *WorkspaceHandler) SetActiveWorkspace(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的工作区ID"})
		return
	}

	// 设置活跃工作区（会自动取消其他工作区的活跃状态）
	wid := uint(id)
	workspace, err := database.GetWorkspaceByID(h.db, wid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "工作区不存在"})
		return
	}

	// 验证路径是否存在
	if !checkPathExists(workspace.Path) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "工作区目录不存在"})
		// 自动删除
		database.DeleteWorkspace(h.db, wid)
		return
	}

	if err := database.SetActiveWorkspace(h.db, wid); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "设置活跃工作区失败"})
		return
	}

	workspace.Active = true

	response := models.WorkspaceResponse{
		Workspace:  *workspace,
		PathExists: true,
	}

	c.JSON(http.StatusOK, response)
}
