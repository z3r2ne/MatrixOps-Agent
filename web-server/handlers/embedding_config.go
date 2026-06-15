package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"pkgs/embedding"
	"pkgs/db/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type EmbeddingConfigHandler struct {
	db *gorm.DB
}

func NewEmbeddingConfigHandler(db *gorm.DB) *EmbeddingConfigHandler {
	return &EmbeddingConfigHandler{db: db}
}

func (h *EmbeddingConfigHandler) ListConfigs(c *gin.Context) {
	configs, err := embedding.ListEmbeddingConfigs(h.db)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取 embedding 配置失败"})
		return
	}
	c.JSON(http.StatusOK, configs)
}

func (h *EmbeddingConfigHandler) CreateConfig(c *gin.Context) {
	var req models.EmbeddingConfigCreate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	config := models.EmbeddingConfig{
		Name:           strings.TrimSpace(req.Name),
		Type:           models.NormalizeEmbeddingConfigType(req.Type),
		BaseURL:        strings.TrimSpace(req.BaseURL),
		BinaryPath:     strings.TrimSpace(req.BinaryPath),
		ModelPath:      strings.TrimSpace(req.ModelPath),
		Dimension:      req.Dimension,
		BatchSize:      req.BatchSize,
		MaxInputTokens: req.MaxInputTokens,
		Enabled:        req.Enabled != nil && *req.Enabled,
		AutoStart:      req.AutoStart != nil && *req.AutoStart,
		Status:         "idle",
	}
	if config.BaseURL == "" {
		config.BaseURL = models.DefaultEmbeddingConfigBaseURL
	}
	if config.BatchSize <= 0 {
		config.BatchSize = models.DefaultEmbeddingBatchSize
	}
	if config.MaxInputTokens <= 0 {
		config.MaxInputTokens = models.DefaultEmbeddingMaxInputTokens
	}

	if err := embedding.CreateEmbeddingConfig(h.db, &config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建 embedding 配置失败: " + err.Error()})
		return
	}
	if config.Enabled {
		if err := embedding.DisableOtherEmbeddingConfigs(h.db, config.ID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "更新启用状态失败: " + err.Error()})
			return
		}
	}

	created, err := embedding.GetEmbeddingConfigByID(h.db, config.ID)
	if err != nil {
		c.JSON(http.StatusCreated, config)
		return
	}
	c.JSON(http.StatusCreated, created)
}

func (h *EmbeddingConfigHandler) UpdateConfig(c *gin.Context) {
	config, ok := h.lookupConfig(c)
	if !ok {
		return
	}

	var req models.EmbeddingConfigUpdate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	if req.Name != nil {
		config.Name = strings.TrimSpace(*req.Name)
	}
	if req.Type != nil {
		config.Type = models.NormalizeEmbeddingConfigType(*req.Type)
	}
	if req.BaseURL != nil {
		config.BaseURL = strings.TrimSpace(*req.BaseURL)
		if config.BaseURL == "" {
			config.BaseURL = models.DefaultEmbeddingConfigBaseURL
		}
	}
	if req.BinaryPath != nil {
		config.BinaryPath = strings.TrimSpace(*req.BinaryPath)
	}
	if req.ModelPath != nil {
		config.ModelPath = strings.TrimSpace(*req.ModelPath)
	}
	if req.Dimension != nil {
		config.Dimension = *req.Dimension
	}
	if req.BatchSize != nil {
		config.BatchSize = *req.BatchSize
	}
	if req.MaxInputTokens != nil {
		config.MaxInputTokens = *req.MaxInputTokens
	}
	if req.Enabled != nil {
		config.Enabled = *req.Enabled
	}
	if req.AutoStart != nil {
		config.AutoStart = *req.AutoStart
	}
	if req.Status != nil {
		config.Status = strings.TrimSpace(*req.Status)
	}
	if req.LastError != nil {
		config.LastError = strings.TrimSpace(*req.LastError)
	}

	if err := embedding.UpdateEmbeddingConfig(h.db, config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新 embedding 配置失败: " + err.Error()})
		return
	}
	if config.Enabled {
		if err := embedding.DisableOtherEmbeddingConfigs(h.db, config.ID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "更新启用状态失败: " + err.Error()})
			return
		}
	}

	updated, err := embedding.GetEmbeddingConfigByID(h.db, config.ID)
	if err != nil {
		c.JSON(http.StatusOK, config)
		return
	}
	c.JSON(http.StatusOK, updated)
}

func (h *EmbeddingConfigHandler) DeleteConfig(c *gin.Context) {
	config, ok := h.lookupConfig(c)
	if !ok {
		return
	}
	if err := embedding.DeleteEmbeddingConfig(h.db, config.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除 embedding 配置失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "embedding 配置已删除"})
}

type embeddingConfigTestRequest struct {
	Sample string `json:"sample"`
}

func (h *EmbeddingConfigHandler) TestConfig(c *gin.Context) {
	config, ok := h.lookupConfig(c)
	if !ok {
		return
	}

	var req embeddingConfigTestRequest
	if err := c.ShouldBindJSON(&req); err != nil && err.Error() != "EOF" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	result, err := embedding.TestConfig(c.Request.Context(), *config, req.Sample)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *EmbeddingConfigHandler) HealthCheck(c *gin.Context) {
	config, ok := h.lookupConfig(c)
	if !ok {
		return
	}
	if _, err := embedding.TestConfig(c.Request.Context(), *config, "health check"); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *EmbeddingConfigHandler) lookupConfig(c *gin.Context) (*models.EmbeddingConfig, bool) {
	idValue := c.Param("id")
	id, err := strconv.ParseUint(idValue, 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的配置 ID"})
		return nil, false
	}
	config, err := embedding.GetEmbeddingConfigByID(h.db, uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "embedding 配置不存在"})
		return nil, false
	}
	return config, true
}
