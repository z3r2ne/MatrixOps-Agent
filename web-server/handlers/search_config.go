package handlers

import (
	"net/http"
	"strconv"
	"strings"

	database "pkgs/db"
	"pkgs/db/models"
	"pkgs/search"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type SearchConfigHandler struct {
	db *gorm.DB
}

func NewSearchConfigHandler(db *gorm.DB) *SearchConfigHandler {
	return &SearchConfigHandler{db: db}
}

func (h *SearchConfigHandler) ListConfigs(c *gin.Context) {
	configs, err := database.ListSearchConfigs(h.db)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取搜索配置失败"})
		return
	}
	c.JSON(http.StatusOK, configs)
}

func (h *SearchConfigHandler) CreateConfig(c *gin.Context) {
	var req models.SearchConfigCreate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	config := models.SearchConfig{
		Name:    strings.TrimSpace(req.Name),
		Type:    models.NormalizeSearchConfigType(req.Type),
		APIKey:  strings.TrimSpace(req.APIKey),
		BaseURL: strings.TrimSpace(req.BaseURL),
		Enabled: req.Enabled != nil && *req.Enabled,
	}
	if config.BaseURL == "" {
		config.BaseURL = models.DefaultSearchConfigBaseURL
	}

	if err := database.CreateSearchConfig(h.db, &config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建搜索配置失败: " + err.Error()})
		return
	}
	if config.Enabled {
		if err := database.DisableOtherSearchConfigs(h.db, config.ID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "更新启用状态失败: " + err.Error()})
			return
		}
	}

	created, err := database.GetSearchConfigByID(h.db, config.ID)
	if err != nil {
		c.JSON(http.StatusCreated, config)
		return
	}
	c.JSON(http.StatusCreated, created)
}

func (h *SearchConfigHandler) UpdateConfig(c *gin.Context) {
	config, ok := h.lookupConfig(c)
	if !ok {
		return
	}

	var req models.SearchConfigUpdate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	if req.Name != nil {
		config.Name = strings.TrimSpace(*req.Name)
	}
	if req.Type != nil {
		config.Type = models.NormalizeSearchConfigType(*req.Type)
	}
	if req.APIKey != nil {
		config.APIKey = strings.TrimSpace(*req.APIKey)
	}
	if req.BaseURL != nil {
		config.BaseURL = strings.TrimSpace(*req.BaseURL)
		if config.BaseURL == "" {
			config.BaseURL = models.DefaultSearchConfigBaseURL
		}
	}
	if req.Enabled != nil {
		config.Enabled = *req.Enabled
	}

	if err := database.UpdateSearchConfig(h.db, config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新搜索配置失败: " + err.Error()})
		return
	}
	if config.Enabled {
		if err := database.DisableOtherSearchConfigs(h.db, config.ID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "更新启用状态失败: " + err.Error()})
			return
		}
	}

	updated, err := database.GetSearchConfigByID(h.db, config.ID)
	if err != nil {
		c.JSON(http.StatusOK, config)
		return
	}
	c.JSON(http.StatusOK, updated)
}

func (h *SearchConfigHandler) DeleteConfig(c *gin.Context) {
	config, ok := h.lookupConfig(c)
	if !ok {
		return
	}
	if err := database.DeleteSearchConfig(h.db, config.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除搜索配置失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "搜索配置已删除"})
}

type searchConfigTestRequest struct {
	Query              string `json:"query" binding:"required"`
	Limit              int    `json:"limit"`
	EnablePageCrawling bool   `json:"enablePageCrawling"`
}

func (h *SearchConfigHandler) TestConfig(c *gin.Context) {
	config, ok := h.lookupConfig(c)
	if !ok {
		return
	}

	var req searchConfigTestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	result, err := search.Search(*config, req.Query, req.Limit, req.EnablePageCrawling)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *SearchConfigHandler) lookupConfig(c *gin.Context) (*models.SearchConfig, bool) {
	idValue := c.Param("id")
	id, err := strconv.ParseUint(idValue, 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的配置 ID"})
		return nil, false
	}
	config, err := database.GetSearchConfigByID(h.db, uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "搜索配置不存在"})
		return nil, false
	}
	return config, true
}
