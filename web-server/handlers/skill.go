package handlers

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"matrixops/pkg/service/skillmarket"
	database "pkgs/db"
	"pkgs/db/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type SkillHandler struct {
	db      *gorm.DB
	service *skillmarket.Service
}

func NewSkillHandler(db *gorm.DB) *SkillHandler {
	return &SkillHandler{
		db:      db,
		service: skillmarket.NewService(),
	}
}

func (h *SkillHandler) ListSources(c *gin.Context) {
	sources, err := database.ListSkillSources(h.db)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取技能源失败"})
		return
	}
	c.JSON(http.StatusOK, sources)
}

func (h *SkillHandler) CreateSource(c *gin.Context) {
	var req models.SkillSourceCreate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}
	source := models.SkillSource{
		Name:       strings.TrimSpace(req.Name),
		RepoURL:    strings.TrimSpace(req.RepoURL),
		SkillsPath: strings.TrimSpace(req.SkillsPath),
		Enabled:    req.Enabled == nil || *req.Enabled,
	}
	if err := database.CreateSkillSource(h.db, &source); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建技能源失败"})
		return
	}
	_, syncErr := h.service.SyncSource(&source)
	_ = database.UpdateSkillSource(h.db, &source)
	if syncErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":  syncErr.Error(),
			"source": source,
		})
		return
	}
	c.JSON(http.StatusCreated, source)
}

func (h *SkillHandler) UpdateSource(c *gin.Context) {
	source, ok := h.lookupSource(c)
	if !ok {
		return
	}
	var req models.SkillSourceUpdate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}
	if req.Name != nil {
		source.Name = strings.TrimSpace(*req.Name)
	}
	if req.RepoURL != nil {
		source.RepoURL = strings.TrimSpace(*req.RepoURL)
	}
	if req.SkillsPath != nil {
		source.SkillsPath = strings.TrimSpace(*req.SkillsPath)
	}
	if req.Enabled != nil {
		source.Enabled = *req.Enabled
	}
	_, syncErr := h.service.SyncSource(source)
	_ = database.UpdateSkillSource(h.db, source)
	if syncErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":  syncErr.Error(),
			"source": source,
		})
		return
	}
	c.JSON(http.StatusOK, source)
}

func (h *SkillHandler) DeleteSource(c *gin.Context) {
	source, ok := h.lookupSource(c)
	if !ok {
		return
	}
	if strings.TrimSpace(source.LocalPath) != "" {
		_ = os.RemoveAll(source.LocalPath)
	}
	if err := database.DeleteSkillSource(h.db, source.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除技能源失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "技能源已删除"})
}

func (h *SkillHandler) SyncSource(c *gin.Context) {
	source, ok := h.lookupSource(c)
	if !ok {
		return
	}
	_, err := h.service.SyncSource(source)
	_ = database.UpdateSkillSource(h.db, source)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":  err.Error(),
			"source": source,
		})
		return
	}
	c.JSON(http.StatusOK, source)
}

func (h *SkillHandler) ListSkills(c *gin.Context) {
	installedOnly := strings.EqualFold(strings.TrimSpace(c.Query("installedOnly")), "true") || c.Query("installedOnly") == "1"
	sources, err := database.ListSkillSources(h.db)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取技能源失败"})
		return
	}
	skills, err := h.service.ListSkills(sources, installedOnly)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取技能列表失败"})
		return
	}
	c.JSON(http.StatusOK, skills)
}

func (h *SkillHandler) InstallSkill(c *gin.Context) {
	var req models.SkillInstallRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}
	source, err := database.GetSkillSourceByID(h.db, req.SourceID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "技能源不存在"})
		return
	}
	path, err := h.service.InstallSkill(*source, req.RelativePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "技能已安装", "path": path})
}

func (h *SkillHandler) UninstallSkill(c *gin.Context) {
	var req models.SkillInstallRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}
	if err := h.service.UninstallSkill(req.SourceID, req.RelativePath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "技能已卸载"})
}

func (h *SkillHandler) lookupSource(c *gin.Context) (*models.SkillSource, bool) {
	var id uint
	if _, err := fmt.Sscanf(c.Param("id"), "%d", &id); err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的源 ID"})
		return nil, false
	}
	source, err := database.GetSkillSourceByID(h.db, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "技能源不存在"})
		return nil, false
	}
	return source, true
}
