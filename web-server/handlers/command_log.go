package handlers

import (
	"net/http"
	"strconv"

	"matrixops/services/task_runner"
	"pkgs/db/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// CommandLogHandler 命令日志处理器
type CommandLogHandler struct {
	db     *gorm.DB
	logger *task_runner.CommandLogger
}

// NewCommandLogHandler 创建命令日志处理器
func NewCommandLogHandler(db *gorm.DB) *CommandLogHandler {
	return &CommandLogHandler{
		db:     db,
		logger: task_runner.GetCommandLogger(db),
	}
}

// GetLogs 获取命令日志列表
func (h *CommandLogHandler) GetLogs(c *gin.Context) {
	var query models.CommandLogQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	logs, total, err := h.logger.GetLogs(query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取日志失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"logs":  logs,
		"total": total,
	})
}

// GetLog 获取单条命令日志
func (h *CommandLogHandler) GetLog(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的日志 ID"})
		return
	}

	log, err := h.logger.GetLog(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "日志不存在"})
		return
	}

	c.JSON(http.StatusOK, log)
}

// ClearOldLogs 清理旧日志
func (h *CommandLogHandler) ClearOldLogs(c *gin.Context) {
	daysStr := c.DefaultQuery("days", "7")
	days, err := strconv.Atoi(daysStr)
	if err != nil || days < 1 {
		days = 7
	}

	count, err := h.logger.ClearOldLogs(days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "清理日志失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "日志清理完成",
		"deleted": count,
	})
}

// GetStats 获取日志统计
func (h *CommandLogHandler) GetStats(c *gin.Context) {
	// 统计各来源和状态的日志数量
	type Stats struct {
		Total    int64            `json:"total"`
		BySource map[string]int64 `json:"bySource"`
		ByStatus map[string]int64 `json:"byStatus"`
	}

	stats := Stats{
		BySource: make(map[string]int64),
		ByStatus: make(map[string]int64),
	}

	// 获取总数
	logs, total, _ := h.logger.GetLogs(models.CommandLogQuery{Limit: 1})
	stats.Total = total
	_ = logs

	// 按来源统计
	for _, source := range []string{"task_runner", "task_runner_followup", "git_handler", "memory_compaction", "llm_action_error", "llm_api_call"} {
		_, count, _ := h.logger.GetLogs(models.CommandLogQuery{Source: source, Limit: 1})
		if count > 0 {
			stats.BySource[source] = count
		}
	}

	// 按状态统计
	for _, status := range []string{"running", "success", "failed"} {
		_, count, _ := h.logger.GetLogs(models.CommandLogQuery{Status: status, Limit: 1})
		if count > 0 {
			stats.ByStatus[status] = count
		}
	}

	c.JSON(http.StatusOK, stats)
}
