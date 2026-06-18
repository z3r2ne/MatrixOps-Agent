package handlers

import (
	"net/http"
	"strings"

	agentsession "matrixops-agent/session"

	"github.com/gin-gonic/gin"
)

func (h *SessionHandler) AnalyzeSessionMemory(c *gin.Context) {
	sessionID := strings.TrimSpace(c.Param("id"))
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的会话ID"})
		return
	}

	analysis, err := agentsession.RunSessionMemoryAnalysis(agentsession.SessionMemoryAnalysisOptions{
		DB:        h.db,
		SessionID: sessionID,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, analysis)
}
