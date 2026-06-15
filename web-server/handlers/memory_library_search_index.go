package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"pkgs/memorysearch"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type MemoryLibrarySearchIndexHandler struct {
	db *gorm.DB
}

func NewMemoryLibrarySearchIndexHandler(db *gorm.DB) *MemoryLibrarySearchIndexHandler {
	return &MemoryLibrarySearchIndexHandler{db: db}
}

func (h *MemoryLibrarySearchIndexHandler) GetStatus(c *gin.Context) {
	libraryID, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	status, err := memorysearch.GetMemoryLibrarySearchIndexStatus(h.db, libraryID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, status)
}

func (h *MemoryLibrarySearchIndexHandler) Rebuild(c *gin.Context) {
	libraryID, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	job, err := memorysearch.EnqueueMemoryLibrarySearchIndexJob(h.db, libraryID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, job)
}

func parseUintParam(c *gin.Context, name string) (uint, bool) {
	value := strings.TrimSpace(c.Param(name))
	id, err := strconv.ParseUint(value, 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return 0, false
	}
	return uint(id), true
}
