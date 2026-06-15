package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"matrixops/pkg/service/terminal"

	"github.com/gin-gonic/gin"
)

type TerminalHandler struct {
	manager *terminal.Manager
}

func NewTerminalHandler(manager *terminal.Manager) *TerminalHandler {
	return &TerminalHandler{manager: manager}
}

func (h *TerminalHandler) CreateSession(c *gin.Context) {
	var req struct {
		WorkDir string `json:"workDir"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}
	session, err := h.manager.Create(req.WorkDir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, session)
}

func (h *TerminalHandler) PollSession(c *gin.Context) {
	sessionID := strings.TrimSpace(c.Param("id"))
	cursor := int64(0)
	if raw := strings.TrimSpace(c.Query("cursor")); raw != "" {
		if _, err := fmt.Sscan(raw, &cursor); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "cursor 参数错误"})
			return
		}
	}
	result, err := h.manager.Poll(sessionID, cursor)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *TerminalHandler) WriteSession(c *gin.Context) {
	sessionID := strings.TrimSpace(c.Param("id"))
	var req struct {
		Input string `json:"input"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}
	if err := h.manager.Write(sessionID, req.Input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *TerminalHandler) ResizeSession(c *gin.Context) {
	sessionID := strings.TrimSpace(c.Param("id"))
	var req struct {
		Cols uint16 `json:"cols"`
		Rows uint16 `json:"rows"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}
	if err := h.manager.Resize(sessionID, req.Cols, req.Rows); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *TerminalHandler) CloseSession(c *gin.Context) {
	sessionID := strings.TrimSpace(c.Param("id"))
	if err := h.manager.Close(sessionID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
