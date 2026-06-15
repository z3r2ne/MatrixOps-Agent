package handlers

import (
	"matrixops/services"
	"net/http"
	database "pkgs/db"
	"pkgs/db/models"
	"pkgs/llmheaders"
	"pkgs/shellutil"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type ConfigHandler struct {
	db *gorm.DB
}

func NewConfigHandler(db *gorm.DB) *ConfigHandler {
	return &ConfigHandler{db: db}
}

func (h *ConfigHandler) GetCurrentShell(c *gin.Context) {
	current := shellutil.Current()
	options := shellutil.Options()

	customShell := ""
	if config, err := database.GetGlobalConfigByKey(h.db, models.ConfigKeyCustomShellCommand); err == nil {
		customShell = config.Value
	}
	if config, err := database.GetGlobalConfigByKey(h.db, models.ConfigKeyDefaultShell); err == nil && config.Value != "" {
		if resolved, err := shellutil.Resolve(config.Value, customShell); err == nil {
			for index := range options {
				options[index].IsCurrent = options[index].ID == resolved.ID
			}
		}
	}

	c.JSON(http.StatusOK, shellutil.CurrentResponse{
		Current: current,
		Options: options,
	})
}

// GetConfig 获取配置
func (h *ConfigHandler) GetConfig(c *gin.Context) {
	key := c.Param("key")

	config, err := database.GetGlobalConfigByKey(h.db, key)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "配置不存在"})
		return
	}

	c.JSON(http.StatusOK, config)
}

// UpdateConfig 更新配置
func (h *ConfigHandler) UpdateConfig(c *gin.Context) {
	key := c.Param("key")

	var req struct {
		Value string `json:"value"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}
	if key == models.ConfigKeyDefaultProjectToolPermissions && strings.TrimSpace(req.Value) == "" {
		req.Value = "{}"
	}
	if key != models.ConfigKeyLLMCustomHeaders && strings.TrimSpace(req.Value) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "value 不能为空"})
		return
	}

	if key == models.ConfigKeyLLMCustomHeaders {
		if err := llmheaders.ValidateJSON(req.Value); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "llm_custom_headers 必须是合法 JSON 对象: " + err.Error()})
			return
		}
	}
	if key == models.ConfigKeyDefaultTaskListGroupMode {
		mode, ok := models.NormalizeTaskListGroupMode(req.Value)
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效的任务列表分组方式"})
			return
		}
		req.Value = string(mode)
	}
	if key == models.ConfigKeyDefaultProjectToolPermissions {
		values, err := models.ParseProjectToolPermissions(req.Value)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "默认项目权限必须是合法 JSON 对象"})
			return
		}
		req.Value = models.NormalizeProjectToolPermissionsJSON(values)
	}

	if err := database.UpsertGlobalConfig(h.db, key, req.Value); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存配置失败: " + err.Error()})
		return
	}

	if key == models.ConfigKeyLLMCustomHeaders {
		if err := llmheaders.SetFromJSON(req.Value); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "应用 llm_custom_headers 失败: " + err.Error()})
			return
		}
	}

	// 重新读取配置
	config, err := database.GetGlobalConfigByKey(h.db, key)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取配置失败"})
		return
	}

	c.JSON(http.StatusOK, config)
}

// GetKeepProcessAlive 获取常驻进程配置
func (h *ConfigHandler) GetKeepProcessAlive(c *gin.Context) {
	config, err := database.GetGlobalConfigByKey(h.db, models.ConfigKeyKeepProcessAlive)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"enabled": false})
		return
	}

	c.JSON(http.StatusOK, gin.H{"enabled": config.Value == "true"})
}

// UpdateKeepProcessAlive 更新常驻进程配置
func (h *ConfigHandler) UpdateKeepProcessAlive(c *gin.Context) {
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	value := "false"
	if req.Enabled {
		value = "true"
	}

	if err := database.UpsertGlobalConfig(h.db, models.ConfigKeyKeepProcessAlive, value); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存配置失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"enabled": req.Enabled})
}

// GetActiveProcesses 获取活跃的常驻进程列表
func (h *ConfigHandler) GetActiveProcesses(c *gin.Context) {
	pm := services.GetProcessManager()
	taskIDs := pm.List()

	c.JSON(http.StatusOK, gin.H{
		"taskIds": taskIDs,
		"count":   len(taskIDs),
	})
}

// KillProcess 关闭指定任务的常驻进程
func (h *ConfigHandler) KillProcess(c *gin.Context) {
	var req struct {
		TaskID uint `json:"taskId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	pm := services.GetProcessManager()
	pm.Remove(req.TaskID)

	c.JSON(http.StatusOK, gin.H{
		"message": "进程已关闭",
		"taskId":  req.TaskID,
	})
}
