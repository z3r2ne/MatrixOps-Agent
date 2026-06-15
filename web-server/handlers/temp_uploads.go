package handlers

import (
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	agentsession "matrixops-agent/session"

	"github.com/gin-gonic/gin"
)

type uploadedTempFile struct {
	Path        string `json:"path"`
	Mime        string `json:"mime"`
	Filename    string `json:"filename"`
	InputSource string `json:"inputSource,omitempty"`
	URL         string `json:"url"`
}

// UploadTempFiles POST /api/temp-uploads
func (h *TaskHandler) UploadTempFiles(c *gin.Context) {
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
	uploaded := make([]uploadedTempFile, 0)
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
			uploaded = append(uploaded, uploadedTempFile{
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

// GetTempFile GET /api/temp-uploads?path=...
func (h *TaskHandler) GetTempFile(c *gin.Context) {
	relPath := strings.TrimSpace(c.Query("path"))
	if relPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path is required"})
		return
	}
	absPath, err := agentsession.ResolveStoredUserInputFilePath(relPath, "")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	mimeType := mime.TypeByExtension(filepath.Ext(absPath))
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	c.Header("Content-Type", mimeType)
	c.File(absPath)
}
