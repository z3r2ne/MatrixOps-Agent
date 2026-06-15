package handlers

import (
	"net/http"
	"strings"

	database "pkgs/db"
	"pkgs/db/models"
	"pkgs/memorysearch"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type MemoryLibraryHandler struct {
	db *gorm.DB
}

func NewMemoryLibraryHandler(db *gorm.DB) *MemoryLibraryHandler {
	return &MemoryLibraryHandler{db: db}
}

func (h *MemoryLibraryHandler) GetMemoryLibraries(c *gin.Context) {
	includeTemporary := strings.EqualFold(strings.TrimSpace(c.Query("includeTemporary")), "true")
	isRag := strings.EqualFold(strings.TrimSpace(c.Query("isRag")), "true")
	libraries, err := database.ListMemoryLibraries(h.db, includeTemporary, isRag)
	if err != nil {
		resourceLabel := "记忆库"
		if isRag {
			resourceLabel = "RAG 知识库"
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取" + resourceLabel + "失败", "details": err.Error()})
		return
	}
	c.JSON(http.StatusOK, libraries)
}

func (h *MemoryLibraryHandler) GetMemoryLibrary(c *gin.Context) {
	id := parseUint(c.Param("id"))
	if id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的记忆库 ID"})
		return
	}
	library, err := database.GetMemoryLibraryByID(h.db, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "记忆库不存在"})
		return
	}
	c.JSON(http.StatusOK, library)
}

func (h *MemoryLibraryHandler) CreateMemoryLibrary(c *gin.Context) {
	var req models.MemoryLibraryCreate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误", "details": err.Error()})
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "记忆库名称不能为空"})
		return
	}
	library := &models.MemoryLibrary{
		Name:        name,
		Content:     req.Content,
		IsRag:       req.IsRag,
		IsTemporary: req.IsTemporary,
		TaskID:      req.TaskID,
	}
	if err := database.CreateMemoryLibrary(h.db, library); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建记忆库失败", "details": err.Error()})
		return
	}
	if library.IsRag {
		_, _ = memorysearch.EnqueueMemoryLibrarySearchIndexJob(h.db, library.ID)
	}
	c.JSON(http.StatusCreated, library)
}

func (h *MemoryLibraryHandler) UpdateMemoryLibrary(c *gin.Context) {
	id := parseUint(c.Param("id"))
	if id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的记忆库 ID"})
		return
	}
	library, err := database.GetMemoryLibraryByID(h.db, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "记忆库不存在"})
		return
	}

	var req models.MemoryLibraryUpdate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误", "details": err.Error()})
		return
	}
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "记忆库名称不能为空"})
			return
		}
		library.Name = name
	}
	if req.Content != nil {
		library.Content = *req.Content
	}
	if err := database.UpdateMemoryLibrary(h.db, library); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新记忆库失败", "details": err.Error()})
		return
	}
	if req.Content != nil && library.IsRag {
		_, _ = memorysearch.EnqueueMemoryLibrarySearchIndexJob(h.db, library.ID)
	}
	c.JSON(http.StatusOK, library)
}

func (h *MemoryLibraryHandler) PromoteMemoryLibrary(c *gin.Context) {
	id := parseUint(c.Param("id"))
	if id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的记忆库 ID"})
		return
	}
	var req models.MemoryLibraryPromoteRequest
	if err := c.ShouldBindJSON(&req); err != nil && err.Error() != "EOF" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误", "details": err.Error()})
		return
	}
	name := ""
	if req.Name != nil {
		name = strings.TrimSpace(*req.Name)
	}
	library, err := database.PromoteMemoryLibrary(h.db, id, name)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	_, _ = memorysearch.EnqueueMemoryLibrarySearchIndexJob(h.db, library.ID)
	c.JSON(http.StatusOK, library)
}

func (h *MemoryLibraryHandler) DeleteMemoryLibrary(c *gin.Context) {
	id := parseUint(c.Param("id"))
	if id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的记忆库 ID"})
		return
	}
	if _, err := database.GetMemoryLibraryByID(h.db, id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "记忆库不存在"})
		return
	}
	if err := database.RemoveMemoryLibraryFromAllProjects(h.db, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "移除项目关联失败", "details": err.Error()})
		return
	}
	if err := database.DeleteMemoryLibrary(h.db, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除记忆库失败", "details": err.Error()})
		return
	}
	_ = memorysearch.DeleteMemoryLibrarySearchIndex(c.Request.Context(), h.db, id)
	c.JSON(http.StatusOK, gin.H{"message": "记忆库已删除"})
}
