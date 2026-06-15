package handlers

import (
	"net/http"

	"matrixops-agent/tool"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type ToolHandler struct {
	db *gorm.DB
}

func NewToolHandler(db *gorm.DB) *ToolHandler {
	return &ToolHandler{db: db}
}

func (h *ToolHandler) GetTools(c *gin.Context) {
	c.JSON(http.StatusOK, tool.CatalogForWorkerUI(h.db))
}
