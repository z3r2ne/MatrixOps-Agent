package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	database "pkgs/db"
	"pkgs/db/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// OpenUIApplicationHandler 桌面端「已打开」工作区/项目列表。
type OpenUIApplicationHandler struct {
	db *gorm.DB
}

// NewOpenUIApplicationHandler 创建处理器。
func NewOpenUIApplicationHandler(db *gorm.DB) *OpenUIApplicationHandler {
	return &OpenUIApplicationHandler{db: db}
}

type openUIApplicationListResponse struct {
	Items               []openUIApplicationItemJSON `json:"items"`
	LastClosedWorkspace *models.WorkspaceResponse   `json:"lastClosedWorkspace,omitempty"`
}

type openUIApplicationItemJSON struct {
	Kind        string                     `json:"kind"`
	WorkspaceID uint                       `json:"workspaceId"`
	Workspace   *models.WorkspaceResponse  `json:"workspace,omitempty"`
	Project     *models.ProjectResponse    `json:"project,omitempty"`
}

// GetOpen 返回当前已打开项（按打开顺序），并剔除已不存在的资源对应记录。
func (h *OpenUIApplicationHandler) GetOpen(c *gin.Context) {
	rows, err := database.ListOpenUIApplicationItems(h.db)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取已打开列表失败"})
		return
	}

	out := make([]openUIApplicationItemJSON, 0, len(rows))
	for _, row := range rows {
		switch row.Kind {
		case models.OpenUIKindWorkspace:
			ws, err := database.GetWorkspaceByIDWithProjects(h.db, row.ResourceID)
			if err != nil {
				_ = database.DeleteOpenUIApplicationByKindResource(h.db, row.Kind, row.ResourceID)
				continue
			}
			pathExists := checkPathExists(ws.Path)
			resp := models.WorkspaceResponse{
				Workspace:  *ws,
				PathExists: pathExists,
			}
			if !pathExists {
				resp.Error = "目录不存在"
			}
			out = append(out, openUIApplicationItemJSON{
				Kind:        models.OpenUIKindWorkspace,
				WorkspaceID: ws.ID,
				Workspace:   &resp,
			})
		case models.OpenUIKindProject:
			proj, err := database.GetProjectByID(h.db, row.ResourceID)
			if err != nil {
				_ = database.DeleteOpenUIApplicationByKindResource(h.db, row.Kind, row.ResourceID)
				continue
			}
			parentWID := uint(0)
			if wid, ok, err := database.GetFirstWorkspaceIDContainingProject(h.db, row.ResourceID); err == nil && ok {
				parentWID = wid
			}
			resp := models.ProjectResponse{
				Project:    *proj,
				PathExists: checkPathExists(proj.Path),
			}
			out = append(out, openUIApplicationItemJSON{
				Kind:        models.OpenUIKindProject,
				WorkspaceID: parentWID,
				Project:     &resp,
			})
		default:
			_ = database.DeleteOpenUIApplicationByKindResource(h.db, row.Kind, row.ResourceID)
		}
	}
	var lastClosedWorkspace *models.WorkspaceResponse
	if state, err := database.GetUIState(h.db, database.UIStateKeyLastClosedWorkspace); err == nil && state != nil {
		var payload struct {
			ID uint `json:"id"`
		}
		if json.Unmarshal([]byte(state.Value), &payload) == nil && payload.ID > 0 {
			if ws, wsErr := database.GetWorkspaceByIDWithProjects(h.db, payload.ID); wsErr == nil && ws != nil {
				resp := models.WorkspaceResponse{
					Workspace:  *ws,
					PathExists: checkPathExists(ws.Path),
				}
				if !resp.PathExists {
					resp.Error = "目录不存在"
				}
				lastClosedWorkspace = &resp
			}
		}
	}

	c.JSON(http.StatusOK, openUIApplicationListResponse{
		Items:               out,
		LastClosedWorkspace: lastClosedWorkspace,
	})
}

// PostOpenWorkspace 将工作区加入已打开列表。
func (h *OpenUIApplicationHandler) PostOpenWorkspace(c *gin.Context) {
	idStr := c.Param("id")
	wid64, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil || wid64 == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的工作区 ID"})
		return
	}
	wid := uint(wid64)
	if _, err := database.GetWorkspaceByID(h.db, wid); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "工作区不存在"})
		return
	}
	if err := database.AddOpenUIApplicationItem(h.db, models.OpenUIKindWorkspace, wid); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "登记已打开失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// DeleteOpenWorkspace 从已打开列表移除工作区（不删除工作区本身）。
func (h *OpenUIApplicationHandler) DeleteOpenWorkspace(c *gin.Context) {
	idStr := c.Param("id")
	wid64, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil || wid64 == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的工作区 ID"})
		return
	}
	wid := uint(wid64)
	if err := database.DeleteOpenUIApplicationByKindResource(h.db, models.OpenUIKindWorkspace, wid); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "移除已打开失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// PostOpenProject 将项目加入已打开列表。
func (h *OpenUIApplicationHandler) PostOpenProject(c *gin.Context) {
	idStr := c.Param("id")
	pid64, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil || pid64 == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的项目 ID"})
		return
	}
	pid := uint(pid64)
	if _, err := database.GetProjectByID(h.db, pid); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "项目不存在"})
		return
	}
	if err := database.AddOpenUIApplicationItem(h.db, models.OpenUIKindProject, pid); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "登记已打开失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// DeleteOpenProject 从已打开列表移除项目（不删除项目本身）。
func (h *OpenUIApplicationHandler) DeleteOpenProject(c *gin.Context) {
	idStr := c.Param("id")
	pid64, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil || pid64 == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的项目 ID"})
		return
	}
	pid := uint(pid64)
	if err := database.DeleteOpenUIApplicationByKindResource(h.db, models.OpenUIKindProject, pid); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "移除已打开失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// PostLastClosedWorkspace 记录最后关闭的工作区。
func (h *OpenUIApplicationHandler) PostLastClosedWorkspace(c *gin.Context) {
	var req struct {
		ID uint `json:"id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.ID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的工作区 ID"})
		return
	}
	if _, err := database.GetWorkspaceByID(h.db, req.ID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "工作区不存在"})
		return
	}
	payload := `{"id":` + strconv.FormatUint(uint64(req.ID), 10) + `}`
	if err := database.SetUIState(h.db, database.UIStateKeyLastClosedWorkspace, payload); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "记录最后关闭工作区失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
