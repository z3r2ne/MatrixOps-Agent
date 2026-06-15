package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"pkgs/testrunner"
	taskrunner "web-server/services/task_runner"
)

// TestHandler 测试相关 handler
type TestHandler struct {
	db    *gorm.DB
	wsHub taskrunner.WSHub
}

// NewTestHandler 创建 TestHandler
func NewTestHandler(db *gorm.DB, wsHub taskrunner.WSHub) *TestHandler {
	return &TestHandler{db: db, wsHub: wsHub}
}

// ListScenarios 获取所有测试场景
func (h *TestHandler) ListScenarios(c *gin.Context) {
	var list []gin.H
	for _, s := range testrunner.Scenarios {
		list = append(list, gin.H{
			"id":          s.ID,
			"name":        s.Name,
			"description": s.Description,
		})
	}
	c.JSON(http.StatusOK, list)
}

// RunScenarioRequest 执行测试场景请求
type RunScenarioRequest struct {
	ScenarioID string `json:"scenarioId" binding:"required"`
}

// RunScenario 执行测试场景
func (h *TestHandler) RunScenario(c *gin.Context) {
	id := c.Param("id")
	var workspaceID uint
	if _, err := fmt.Sscanf(id, "%d", &workspaceID); err != nil || workspaceID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的工作区 ID"})
		return
	}

	var req RunScenarioRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	scenario, ok := testrunner.Scenarios[req.ScenarioID]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的测试场景"})
		return
	}

	// TODO: 获取默认 LLM client
	// 暂时传 nil，runner 会处理
	result, err := testrunner.ExecuteScenario(h.db, h.wsHub, nil, workspaceID, scenario)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "执行测试失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}
