package handlers

import (
	"net/http"
	"strings"

	database "pkgs/db"

	"github.com/gin-gonic/gin"
)

func (h *TaskHandler) GetTaskFilesystemRoots(c *gin.Context) {
	taskID := parseUint(c.Param("id"))
	if taskID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效任务ID"})
		return
	}

	task, err := database.GetTaskByID(h.db, taskID)
	if err != nil || task == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
		return
	}

	roots, err := database.ResolveTaskFilesystemRoots(h.db, task)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"roots": roots})
}

func (h *TaskHandler) ListTaskFilesystem(c *gin.Context) {
	taskID := parseUint(c.Param("id"))
	if taskID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效任务ID"})
		return
	}

	task, err := database.GetTaskByID(h.db, taskID)
	if err != nil || task == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
		return
	}

	rootID := strings.TrimSpace(c.Query("root"))
	path := strings.TrimSpace(c.Query("path"))
	entries, err := database.ListTaskFilesystemEntries(h.db, task, rootID, path)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"entries": entries})
}

func (h *TaskHandler) ReadTaskFilesystem(c *gin.Context) {
	taskID := parseUint(c.Param("id"))
	if taskID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效任务ID"})
		return
	}

	task, err := database.GetTaskByID(h.db, taskID)
	if err != nil || task == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
		return
	}

	rootID := strings.TrimSpace(c.Query("root"))
	path := strings.TrimSpace(c.Query("path"))
	if path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path 不能为空"})
		return
	}

	content, binary, err := database.ReadTaskFilesystemFile(h.db, task, rootID, path)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"content": content,
		"binary":  binary,
	})
}

func (h *TaskHandler) WriteTaskFilesystem(c *gin.Context) {
	taskID := parseUint(c.Param("id"))
	if taskID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效任务ID"})
		return
	}

	task, err := database.GetTaskByID(h.db, taskID)
	if err != nil || task == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
		return
	}

	var req struct {
		Root    string `json:"root"`
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}
	if strings.TrimSpace(req.Path) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path 不能为空"})
		return
	}

	if err := database.WriteTaskFilesystemFile(h.db, task, req.Root, req.Path, req.Content); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
