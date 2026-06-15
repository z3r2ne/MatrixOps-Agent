package handlers

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	agentsession "matrixops-agent/session"
	database "pkgs/db"

	"github.com/gin-gonic/gin"
)

type uploadedUserInputFile struct {
	Path        string `json:"path"`
	Mime        string `json:"mime"`
	Filename    string `json:"filename"`
	InputSource string `json:"inputSource,omitempty"`
	URL         string `json:"url"`
}

// UploadTaskUserInputFiles POST /api/tasks/:id/user-input-files
func (h *TaskHandler) UploadTaskUserInputFiles(c *gin.Context) {
	taskID, err := parseTaskIDParam(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if _, err := database.GetTaskByID(h.db, taskID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
		return
	}
	if err := c.Request.ParseMultipartForm(32 << 20); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid multipart form"})
		return
	}
	source := strings.TrimSpace(c.PostForm("inputSource"))
	if source == "" {
		source = strings.TrimSpace(c.PostForm("source"))
	}
	form := c.Request.MultipartForm
	if form == nil || len(form.File) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no files uploaded"})
		return
	}
	uploaded := make([]uploadedUserInputFile, 0)
	for _, files := range form.File {
		for _, header := range files {
			file, err := header.Open()
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			data, err := io.ReadAll(file)
			_ = file.Close()
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			filename := strings.TrimSpace(header.Filename)
			if filename == "" {
				filename = "attachment.bin"
			}
			absPath, err := agentsession.SaveTempUserInputFile(filename, data)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			mimeType := mime.TypeByExtension(filepath.Ext(filename))
			if mimeType == "" {
				mimeType = strings.TrimSpace(header.Header.Get("Content-Type"))
			}
			if mimeType == "" {
				mimeType = "application/octet-stream"
			}
			uploaded = append(uploaded, uploadedUserInputFile{
				Path:        absPath,
				Mime:        mimeType,
				Filename:    filepath.Base(filename),
				InputSource: source,
				URL:         agentsession.TempUserInputFileAPIURL(absPath),
			})
		}
	}
	c.JSON(http.StatusOK, gin.H{"files": uploaded})
}

// GetTaskUserInputFile GET /api/tasks/:id/user-input-files?path=...
func (h *TaskHandler) GetTaskUserInputFile(c *gin.Context) {
	taskID, err := parseTaskIDParam(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	relPath := strings.TrimSpace(c.Query("path"))
	if relPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path is required"})
		return
	}
	task, err := database.GetTaskByID(h.db, taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
		return
	}
	workDir := strings.TrimSpace(task.WorkDir)
	abs, err := agentsession.ResolveStoredUserInputFilePath(relPath, workDir)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	mimeType := mime.TypeByExtension(filepath.Ext(abs))
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	c.Header("Content-Type", mimeType)
	c.File(abs)
}

func parseTaskIDParam(c *gin.Context) (uint, error) {
	raw := strings.TrimSpace(c.Param("id"))
	if raw == "" {
		return 0, fmt.Errorf("invalid task id")
	}
	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || id == 0 {
		return 0, fmt.Errorf("invalid task id")
	}
	return uint(id), nil
}

// resolveTaskWorkDir helper for tests/other handlers
func resolveTaskWorkDir(taskID uint) (string, error) {
	db := database.DB
	if db == nil {
		return "", fmt.Errorf("database unavailable")
	}
	task, err := database.GetTaskByID(db, taskID)
	if err != nil {
		return "", err
	}
	workDir := strings.TrimSpace(task.WorkDir)
	if workDir == "" {
		return "", fmt.Errorf("task workDir is empty")
	}
	return workDir, nil
}
