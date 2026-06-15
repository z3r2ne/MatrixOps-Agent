package handlers

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	database "pkgs/db"
	"pkgs/db/models"
	mcppkg "pkgs/mcp"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type McpHandler struct {
	db *gorm.DB
}

func NewMcpHandler(db *gorm.DB) *McpHandler {
	return &McpHandler{db: db}
}

func (h *McpHandler) ListServers(c *gin.Context) {
	servers, err := database.ListMcpServers(h.db)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取 MCP 服务器失败"})
		return
	}
	c.JSON(http.StatusOK, servers)
}

func (h *McpHandler) CreateServer(c *gin.Context) {
	var req models.McpServerCreate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}
	server := models.McpServer{
		Name:        strings.TrimSpace(req.Name),
		Transport:   strings.TrimSpace(req.Transport),
		Command:     strings.TrimSpace(req.Command),
		ArgsJSON:    strings.TrimSpace(req.ArgsJSON),
		EnvJSON:     strings.TrimSpace(req.EnvJSON),
		URL:         strings.TrimSpace(req.URL),
		HeadersJSON: strings.TrimSpace(req.HeadersJSON),
		Enabled:     req.Enabled == nil || *req.Enabled,
	}
	if server.Transport == "" {
		server.Transport = models.McpTransportStdio
	}
	if strings.TrimSpace(server.ArgsJSON) == "" {
		server.ArgsJSON = "[]"
	}
	if strings.TrimSpace(server.EnvJSON) == "" {
		server.EnvJSON = "{}"
	}
	if strings.TrimSpace(server.HeadersJSON) == "" {
		server.HeadersJSON = "{}"
	}
	if err := database.CreateMcpServer(h.db, &server); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建 MCP 服务器失败"})
		return
	}
	if server.Enabled {
		if err := h.connectServer(server.ID); err != nil {
			c.JSON(http.StatusCreated, gin.H{
				"error":  err.Error(),
				"server": server,
			})
			return
		}
		updated, _ := database.GetMcpServerByID(h.db, server.ID)
		if updated != nil {
			server = *updated
		}
	}
	c.JSON(http.StatusCreated, server)
}

func (h *McpHandler) UpdateServer(c *gin.Context) {
	server, ok := h.lookupServer(c)
	if !ok {
		return
	}
	var req models.McpServerUpdate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}
	if req.Name != nil {
		server.Name = strings.TrimSpace(*req.Name)
	}
	if req.Transport != nil {
		server.Transport = strings.TrimSpace(*req.Transport)
	}
	if req.Command != nil {
		server.Command = strings.TrimSpace(*req.Command)
	}
	if req.ArgsJSON != nil {
		server.ArgsJSON = strings.TrimSpace(*req.ArgsJSON)
	}
	if req.EnvJSON != nil {
		server.EnvJSON = strings.TrimSpace(*req.EnvJSON)
	}
	if req.URL != nil {
		server.URL = strings.TrimSpace(*req.URL)
	}
	if req.HeadersJSON != nil {
		server.HeadersJSON = strings.TrimSpace(*req.HeadersJSON)
	}
	if req.Enabled != nil {
		server.Enabled = *req.Enabled
	}
	if err := database.UpdateMcpServer(h.db, server); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新 MCP 服务器失败"})
		return
	}
	manager := mcppkg.GetManager()
	if manager != nil {
		if server.Enabled {
			if err := h.connectServer(server.ID); err != nil {
				c.JSON(http.StatusOK, gin.H{
					"error":  err.Error(),
					"server": server,
				})
				return
			}
		} else {
			manager.DisconnectServer(server.ID)
		}
	}
	updated, _ := database.GetMcpServerByID(h.db, server.ID)
	if updated != nil {
		server = updated
	}
	c.JSON(http.StatusOK, server)
}

func (h *McpHandler) DeleteServer(c *gin.Context) {
	server, ok := h.lookupServer(c)
	if !ok {
		return
	}
	if manager := mcppkg.GetManager(); manager != nil {
		manager.DisconnectServer(server.ID)
	}
	if err := database.DeleteMcpServer(h.db, server.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除 MCP 服务器失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "MCP 服务器已删除"})
}

func (h *McpHandler) ReconnectServer(c *gin.Context) {
	server, ok := h.lookupServer(c)
	if !ok {
		return
	}
	if !server.Enabled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "服务器未启用"})
		return
	}
	if err := h.connectServer(server.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	updated, _ := database.GetMcpServerByID(h.db, server.ID)
	c.JSON(http.StatusOK, updated)
}

func (h *McpHandler) ListServerTools(c *gin.Context) {
	server, ok := h.lookupServer(c)
	if !ok {
		return
	}
	manager := mcppkg.GetManager()
	if manager == nil {
		c.JSON(http.StatusOK, []models.McpToolInfo{})
		return
	}
	c.JSON(http.StatusOK, manager.ToolsForServer(server.ID))
}

func (h *McpHandler) lookupServer(c *gin.Context) (*models.McpServer, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的服务器 ID"})
		return nil, false
	}
	server, err := database.GetMcpServerByID(h.db, uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "MCP 服务器不存在"})
		return nil, false
	}
	return server, true
}

func (h *McpHandler) connectServer(id uint) error {
	manager := mcppkg.GetManager()
	if manager == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	return manager.ConnectServer(ctx, id)
}
