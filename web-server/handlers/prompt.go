package handlers

import (
	"net/http"
	"strconv"

	database "pkgs/db"
	"pkgs/db/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type PromptHandler struct {
	db *gorm.DB
}

func NewPromptHandler(db *gorm.DB) *PromptHandler {
	return &PromptHandler{db: db}
}

// GetGlobalPrompt 获取全局提示词
func (h *PromptHandler) GetGlobalPrompt(c *gin.Context) {
	prompt, err := database.GetGlobalPrompt(h.db)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取全局提示词失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"prompt": prompt})
}

// UpdateGlobalPrompt 更新全局提示词
func (h *PromptHandler) UpdateGlobalPrompt(c *gin.Context) {
	var req struct {
		Prompt string `json:"prompt"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	if err := database.SetGlobalPrompt(h.db, req.Prompt); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新全局提示词失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "全局提示词已更新"})
}

// GetAllOccupations 获取所有职业
func (h *PromptHandler) GetAllOccupations(c *gin.Context) {
	occupations, err := database.GetAllOccupations(h.db)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取职业列表失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, occupations)
}

// GetOccupation 获取单个职业
func (h *PromptHandler) GetOccupation(c *gin.Context) {
	id := c.Param("id")
	var occupationID uint
	if _, err := strconv.ParseUint(id, 10, 32); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的职业ID"})
		return
	}
	occupationID = uint(parseUint(id))

	occupation, err := database.GetOccupationByID(h.db, occupationID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "职业不存在"})
		return
	}
	c.JSON(http.StatusOK, occupation)
}

// GetOccupationByCode 根据代码获取职业
func (h *PromptHandler) GetOccupationByCode(c *gin.Context) {
	code := c.Param("code")
	occupation, err := database.GetOccupationByCode(h.db, code)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "职业不存在"})
		return
	}
	c.JSON(http.StatusOK, occupation)
}

// UpdateOccupation 更新职业（主要是更新提示词）
func (h *PromptHandler) UpdateOccupation(c *gin.Context) {
	id := c.Param("id")
	var occupationID uint
	if _, err := strconv.ParseUint(id, 10, 32); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的职业ID"})
		return
	}
	occupationID = uint(parseUint(id))

	var req models.OccupationUpdate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	// 构建更新字段
	updates := make(map[string]interface{})
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.Prompt != nil {
		updates["prompt"] = *req.Prompt
	}
	if req.Color != nil {
		updates["color"] = *req.Color
	}

	if err := database.UpdateOccupation(h.db, occupationID, updates); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新职业失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "职业已更新"})
}

// GetProjectPrompt 获取项目提示词
func (h *PromptHandler) GetProjectPrompt(c *gin.Context) {
	id := c.Param("id")
	var projectID uint
	if _, err := strconv.ParseUint(id, 10, 32); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的项目ID"})
		return
	}
	projectID = uint(parseUint(id))

	project, err := database.GetProjectByID(h.db, projectID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "项目不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"prompt": project.Prompt})
}

// UpdateProjectPrompt 更新项目提示词
func (h *PromptHandler) UpdateProjectPrompt(c *gin.Context) {
	id := c.Param("id")
	var projectID uint
	if _, err := strconv.ParseUint(id, 10, 32); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的项目ID"})
		return
	}
	projectID = uint(parseUint(id))

	var req struct {
		Prompt string `json:"prompt"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	updates := map[string]interface{}{
		"prompt": req.Prompt,
	}

	if err := database.UpdateProjectFields(h.db, projectID, updates); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新项目提示词失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "项目提示词已更新"})
}
