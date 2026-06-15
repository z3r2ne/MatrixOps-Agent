package handlers

import (
	"net/http"
	"os/exec"
	"strings"

	database "pkgs/db"
	"pkgs/db/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type providerSpec struct {
	Name        string
	DisplayName string
	Type        string
	Command     string
	InstallHint string
}

var providerSpecs = []providerSpec{
	{
		Name:        "codex",
		DisplayName: "Codex",
		Type:        "cli",
		Command:     "codex",
		InstallHint: "请先安装 Codex CLI 并确保在 PATH 中可用",
	},
	{
		Name:        "cursor",
		DisplayName: "Cursor",
		Type:        "app",
		Command:     "cursor",
		InstallHint: "请先安装 Cursor 并确保命令行工具可用",
	},
}

// ProviderHandler Provider 处理器
type ProviderHandler struct {
	db *gorm.DB
}

// NewProviderHandler 创建 Provider 处理器
func NewProviderHandler(db *gorm.DB) *ProviderHandler {
	return &ProviderHandler{db: db}
}

func ensureProviderSetting(db *gorm.DB, name string) (models.ProviderSetting, error) {
	setting, err := database.GetProviderSettingByName(db, name)
	if err == nil {
		return *setting, nil
	}

	newSetting := models.ProviderSetting{
		Name:    name,
		Enabled: false,
	}
	if err := database.CreateProviderSetting(db, &newSetting); err != nil {
		return models.ProviderSetting{}, err
	}
	return newSetting, nil
}

func detectProvider(cmd string) (bool, string) {
	path, err := exec.LookPath(cmd)
	if err != nil {
		return false, ""
	}
	return true, path
}

// GetProviders 获取 Provider 列表
func (h *ProviderHandler) GetProviders(c *gin.Context) {
	results := make([]models.ProviderResponse, 0, len(providerSpecs))

	for _, spec := range providerSpecs {
		setting, err := ensureProviderSetting(h.db, spec.Name)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "读取 Provider 配置失败"})
			return
		}

		detected := true
		path := ""
		status := "ready"
		message := ""
		if spec.Type != "embedded" {
			detected, path = detectProvider(spec.Command)
			if !detected {
				status = "need_install"
				message = spec.InstallHint
			}
		} else {
			path = "embedded"
		}

		results = append(results, models.ProviderResponse{
			Name:        spec.Name,
			DisplayName: spec.DisplayName,
			Type:        spec.Type,
			Enabled:     setting.Enabled,
			Detected:    detected,
			Path:        path,
			Status:      status,
			Message:     message,
		})
	}

	c.JSON(http.StatusOK, results)
}

// UpdateProvider 更新 Provider
func (h *ProviderHandler) UpdateProvider(c *gin.Context) {
	name := strings.ToLower(c.Param("name"))

	var req models.ProviderUpdate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	setting, err := ensureProviderSetting(h.db, name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取 Provider 配置失败"})
		return
	}

	if req.Enabled != nil {
		setting.Enabled = *req.Enabled
	}

	if err := database.UpdateProviderSetting(h.db, &setting); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新 Provider 失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Provider 已更新"})
}
