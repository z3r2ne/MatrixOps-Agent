package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	agentsession "matrixops-agent/session"
	"matrixops-agent/token"
	agenttool "matrixops-agent/tool"
	"matrixops-agent/types"
	database "pkgs/db"
	"pkgs/db/models"
	"pkgs/db/storage"
	"pkgs/skillfs"

	"matrixops/services/task_runner"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type SessionHandler struct {
	db *gorm.DB
}

type sessionMemoryCompactionPreview struct {
	Message          string  `json:"message"`
	Count            int     `json:"count"`
	ScopePercent     int     `json:"scopePercent"`
	TargetPercent    int     `json:"targetPercent"`
	L2ScopePercent   int     `json:"l2ScopePercent"`
	LevelsExecuted   []int   `json:"levelsExecuted,omitempty"`
	BeforeCount      int     `json:"beforeCount"`
	AfterCount       int     `json:"afterCount"`
	CompressionRate  float64 `json:"compressionRate"`
	BeforePreview    string  `json:"beforePreview"`
	AfterPreview     string  `json:"afterPreview"`
	Summary          string  `json:"summary"`
}

func NewSessionHandler(db *gorm.DB) *SessionHandler {
	return &SessionHandler{db: db}
}

func (h *SessionHandler) GetSession(c *gin.Context) {
	sessionID := c.Param("id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的会话ID"})
		return
	}

	session, err := storage.GetSession(h.db, sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "会话不存在"})
		return
	}

	c.JSON(http.StatusOK, session)
}

func (h *SessionHandler) GetSessionLogsV2(c *gin.Context) {
	sessionID := strings.TrimSpace(c.Param("id"))
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的会话ID"})
		return
	}

	if _, err := storage.GetSession(h.db, sessionID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "会话不存在"})
		return
	}

	limit := 100
	if rawLimit := strings.TrimSpace(c.Query("limit")); rawLimit != "" {
		if parsed, parseErr := strconv.Atoi(rawLimit); parseErr == nil && parsed > 0 {
			if parsed > 200 {
				parsed = 200
			}
			limit = parsed
		}
	}
	beforeMessageID := strings.TrimSpace(c.Query("beforeMessageId"))

	messagesWithParts, hasMore, nextBeforeMessageID, err := storage.GetMessageWithPartsBySessionIDPageLight(h.db, sessionID, limit, beforeMessageID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取日志失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"items":               messagesWithParts,
		"hasMore":             hasMore,
		"nextBeforeMessageId": nextBeforeMessageID,
	})
}

func (h *SessionHandler) GetSessionPrompt(c *gin.Context) {
	sessionID := c.Param("id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的会话ID"})
		return
	}

	if _, err := storage.GetSession(h.db, sessionID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "会话不存在"})
		return
	}

	requestedMessageID := strings.TrimSpace(c.Query("messageId"))
	messageID := requestedMessageID
	partID := ""
	prompt := ""
	rawResponse := ""
	if requestedMessageID != "" {
		snapshotSessionID, resolvedPrompt, resolvedRawResponse, snapshotErr := storage.GetPromptByMessageID(h.db, requestedMessageID)
		if snapshotErr != nil || snapshotSessionID != sessionID {
			c.JSON(http.StatusNotFound, gin.H{"error": "当前会话还没有生成过可展示的 Prompt"})
			return
		}
		prompt = resolvedPrompt
		rawResponse = resolvedRawResponse
	} else {
		var promptErr error
		messageID, partID, prompt, rawResponse, promptErr = storage.GetLatestPromptBySessionID(h.db, sessionID)
		if promptErr != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "当前会话还没有生成过可展示的 Prompt"})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"sessionId":   sessionID,
		"messageId":   messageID,
		"partId":      partID,
		"prompt":      prompt,
		"rawResponse": rawResponse,
	})
}

func (h *SessionHandler) GetSessionMemory(c *gin.Context) {
	sessionID := c.Param("id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的会话ID"})
		return
	}

	session, err := storage.GetSession(h.db, sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "会话不存在"})
		return
	}

	memoryEntries, err := storage.ListMemoryEntriesBySession(h.db, sessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取历史记忆失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"session":       session,
		"memoryEntries": memoryEntries,
	})
}

func (h *SessionHandler) RemoveSessionSkill(c *gin.Context) {
	sessionID := c.Param("id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的会话ID"})
		return
	}
	if _, err := storage.GetSession(h.db, sessionID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "会话不存在"})
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "技能名不能为空"})
		return
	}
	session, err := storage.UpdateSessionByCallback(h.db, sessionID, func(info *types.Info) error {
		if info == nil {
			return nil
		}
		next := make([]string, 0, len(info.EnabledSkills))
		for _, enabled := range info.EnabledSkills {
			if strings.EqualFold(strings.TrimSpace(enabled), name) {
				continue
			}
			next = append(next, enabled)
		}
		info.EnabledSkills = next
		return nil
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "移除会话技能失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "已从当前会话移除技能",
		"session": session,
	})
}

func (h *SessionHandler) GetSessionContext(c *gin.Context) {
	sessionID := c.Param("id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的会话ID"})
		return
	}

	session, err := storage.GetSession(h.db, sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "会话不存在"})
		return
	}

	messagesWithParts, err := storage.GetMessageWithPartsBySessionID(h.db, sessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取会话消息失败"})
		return
	}
	memoryEntries, err := storage.ListMemoryEntriesBySession(h.db, sessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取历史记忆失败"})
		return
	}

	workerName := strings.TrimSpace(c.Query("workerName"))
	if workerName == "" {
		workerName = latestSessionWorkerName(messagesWithParts)
	}

	globalPrompt, _ := database.GetGlobalPrompt(h.db)
	workerPrompt := ""
	modelPrompt := ""
	occupationPrompt := ""
	projectPrompt := ""
	envPrompt := buildSessionEnvPrompt(session)
	toolPrompt := ""
	skillsPrompt := buildInstalledSkillsSummary()
	tools := make([]gin.H, 0)
	contextLimit := 0
	outputLimit := 0
	effectiveContextLimit := 0

	if workerName != "" {
		if worker, err := database.GetWorkerByName(h.db, workerName); err == nil && worker != nil {
			workerPrompt = strings.TrimSpace(worker.SystemPrompt)
			if modelSettings, err := database.GetModelSettingsForWorker(h.db, worker); err == nil && modelSettings != nil {
				modelPrompt = strings.TrimSpace(modelSettings.Prompt)
				contextLimit = modelSettings.ContextLimit
				outputLimit = modelSettings.OutputLimit
				if contextLimit > 0 {
					effectiveContextLimit = contextLimit - max(outputLimit, 0)
					if effectiveContextLimit <= 0 {
						effectiveContextLimit = contextLimit
					}
				}
			}
			occupationPrompt = loadWorkerOccupationPrompt(h.db, worker)
			var project *models.Project
			if projectID := strings.TrimSpace(session.ProjectID); projectID != "" {
				if parsed, parseErr := parseProjectID(projectID); parseErr == nil && parsed > 0 {
					project, _ = database.GetProjectByID(h.db, parsed)
				}
			}
			toolPrompt, tools = buildWorkerToolPrompt(worker, project)
		}
	}

	if projectID := strings.TrimSpace(session.ProjectID); projectID != "" {
		if parsed, err := parseProjectID(projectID); err == nil && parsed > 0 {
			if project, err := database.GetProjectByID(h.db, parsed); err == nil && project != nil {
				projectPrompt = strings.TrimSpace(project.Prompt)
			}
		}
	}

	loadedSkills, loadedSkillsBytes := summarizeLoadedSkills(session)
	regularMemoryBytes := summarizeRegularMemoryBytes(memoryEntries)
	projectFilePrompt := buildSessionProjectFilePrompts(h.db, sessionID)
	prompts := gin.H{
		"globalPrompt":      globalPrompt,
		"workerPrompt":      workerPrompt,
		"modelPrompt":       modelPrompt,
		"occupationPrompt":  occupationPrompt,
		"projectPrompt":     projectPrompt,
		"envPrompt":         envPrompt,
		"projectFilePrompt": projectFilePrompt,
	}

	components := []gin.H{
		buildContextComponent("global", "系统提示词", globalPrompt),
		buildContextComponent("worker", "Worker 提示词", workerPrompt),
		buildContextComponent("model", "Model 提示词", modelPrompt),
		buildContextComponent("occupation", "职业提示词", occupationPrompt),
		buildContextComponent("project", "项目提示词", projectPrompt),
		buildContextComponent("environment", "环境提示词", envPrompt),
		buildContextComponent("tools", "工具提示词", toolPrompt),
		buildContextComponent("skills_summary", "技能摘要", skillsPrompt),
		buildContextComponentWithBytes("loaded_skills", "已加载技能", loadedSkillsBytes),
		buildContextComponentWithBytes("memory", "记忆", regularMemoryBytes),
	}
	totalBytes := 0
	for _, item := range components {
		if value, ok := item["bytes"].(int); ok && value > 0 {
			totalBytes += value
		}
	}
	for _, item := range components {
		bytes, _ := item["bytes"].(int)
		percent := 0.0
		if totalBytes > 0 && bytes > 0 {
			percent = float64(bytes) * 100 / float64(totalBytes)
		}
		item["percent"] = percent
	}

	c.JSON(http.StatusOK, gin.H{
		"sessionId":             sessionID,
		"workerName":            workerName,
		"tools":                 tools,
		"loadedSkills":          loadedSkills,
		"prompts":               prompts,
		"projectFilePrompts":    projectFilePrompt,
		"components":            components,
		"totalBytes":            totalBytes,
		"contextLimit":          contextLimit,
		"outputLimit":           outputLimit,
		"effectiveContextLimit": effectiveContextLimit,
	})
}

func buildSessionProjectFilePrompts(db *gorm.DB, sessionID string) []gin.H {
	if db == nil || strings.TrimSpace(sessionID) == "" {
		return nil
	}

	var task models.Task
	if err := db.Where("session_id = ?", strings.TrimSpace(sessionID)).Order("created_at DESC").First(&task).Error; err != nil {
		return nil
	}

	customPrompts, err := agentsession.CustomWithSource(&task)
	if err != nil || len(customPrompts) == 0 {
		return nil
	}

	items := make([]gin.H, 0, len(customPrompts))
	for _, prompt := range customPrompts {
		if len(prompt) < 2 {
			continue
		}
		items = append(items, gin.H{
			"path":   prompt[0],
			"prompt": prompt[1],
		})
	}
	return items
}

func (h *SessionHandler) CreateSessionMemoryEntry(c *gin.Context) {
	sessionID := c.Param("id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的会话ID"})
		return
	}

	if _, err := storage.GetSession(h.db, sessionID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "会话不存在"})
		return
	}

	var req struct {
		EntryKind          string `json:"entryKind"`
		Role               string `json:"role"`
		Content            string `json:"content"`
		RawOutput          string `json:"rawOutput"`
		CallToolInfo       string `json:"callToolInfo"`
		ToolCallID         string `json:"toolCallID"`
		ToolName           string `json:"toolName"`
		ToolStatus         string `json:"toolStatus"`
		ToolReason         string `json:"toolReason"`
		ToolRequestRawJSON string `json:"toolRequestRawJSON"`
		ToolInputJSON      string `json:"toolInputJSON"`
		ToolOutput         string `json:"toolOutput"`
		ToolError          string `json:"toolError"`
		ToolTitle          string `json:"toolTitle"`
		ToolMetadataJSON   string `json:"toolMetadataJSON"`
		TokenCount         int    `json:"tokenCount"`
		Synthetic          bool   `json:"synthetic"`
		Sequence           int64  `json:"sequence"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	entry := &types.MemoryEntry{
		SessionID:          sessionID,
		EntryKind:          req.EntryKind,
		Role:               req.Role,
		Content:            req.Content,
		RawOutput:          req.RawOutput,
		CallToolInfo:       req.CallToolInfo,
		ToolCallID:         req.ToolCallID,
		ToolName:           req.ToolName,
		ToolStatus:         req.ToolStatus,
		ToolReason:         req.ToolReason,
		ToolRequestRawJSON: req.ToolRequestRawJSON,
		ToolInputJSON:      req.ToolInputJSON,
		ToolOutput:         req.ToolOutput,
		ToolError:          req.ToolError,
		ToolTitle:          req.ToolTitle,
		ToolMetadataJSON:   req.ToolMetadataJSON,
		TokenCount:         req.TokenCount,
		Synthetic:          req.Synthetic,
		Sequence:           req.Sequence,
	}
	if entry.EntryKind == "" {
		entry.EntryKind = "manual"
	}
	if entry.Role == "" {
		entry.Role = "system"
	}
	if entry.TokenCount == 0 {
		entry.TokenCount = token.Estimate(entry.SerializeText())
	}

	if err := storage.CreateMemoryEntry(h.db, entry); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建历史记忆失败"})
		return
	}

	c.JSON(http.StatusOK, entry)
}

func (h *SessionHandler) UpdateSessionMemoryEntry(c *gin.Context) {
	sessionID := c.Param("id")
	entryID := parseUint(c.Param("entryId"))
	if sessionID == "" || entryID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效参数"})
		return
	}

	var req struct {
		EntryKind          string `json:"entryKind"`
		Role               string `json:"role"`
		Content            string `json:"content"`
		RawOutput          string `json:"rawOutput"`
		CallToolInfo       string `json:"callToolInfo"`
		ToolCallID         string `json:"toolCallID"`
		ToolName           string `json:"toolName"`
		ToolStatus         string `json:"toolStatus"`
		ToolReason         string `json:"toolReason"`
		ToolRequestRawJSON string `json:"toolRequestRawJSON"`
		ToolInputJSON      string `json:"toolInputJSON"`
		ToolOutput         string `json:"toolOutput"`
		ToolError          string `json:"toolError"`
		ToolTitle          string `json:"toolTitle"`
		ToolMetadataJSON   string `json:"toolMetadataJSON"`
		TokenCount         int    `json:"tokenCount"`
		Synthetic          bool   `json:"synthetic"`
		Sequence           int64  `json:"sequence"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	entry := &types.MemoryEntry{
		ID:                 entryID,
		SessionID:          sessionID,
		EntryKind:          req.EntryKind,
		Role:               req.Role,
		Content:            req.Content,
		RawOutput:          req.RawOutput,
		CallToolInfo:       req.CallToolInfo,
		ToolCallID:         req.ToolCallID,
		ToolName:           req.ToolName,
		ToolStatus:         req.ToolStatus,
		ToolReason:         req.ToolReason,
		ToolRequestRawJSON: req.ToolRequestRawJSON,
		ToolInputJSON:      req.ToolInputJSON,
		ToolOutput:         req.ToolOutput,
		ToolError:          req.ToolError,
		ToolTitle:          req.ToolTitle,
		ToolMetadataJSON:   req.ToolMetadataJSON,
		TokenCount:         req.TokenCount,
		Synthetic:          req.Synthetic,
		Sequence:           req.Sequence,
	}
	if entry.TokenCount == 0 {
		entry.TokenCount = token.Estimate(entry.SerializeText())
	}

	if err := storage.UpdateMemoryEntry(h.db, sessionID, entry); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新历史记忆失败"})
		return
	}

	c.JSON(http.StatusOK, entry)
}

func (h *SessionHandler) DeleteSessionMemoryEntry(c *gin.Context) {
	sessionID := c.Param("id")
	entryID := parseUint(c.Param("entryId"))
	if sessionID == "" || entryID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效参数"})
		return
	}

	if err := storage.DeleteMemoryEntryByID(h.db, sessionID, entryID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除历史记忆失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

func (h *SessionHandler) CompressSessionMemoryEntries(c *gin.Context) {
	sessionID := c.Param("id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的会话ID"})
		return
	}

	if _, err := storage.GetSession(h.db, sessionID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "会话不存在"})
		return
	}

	var req struct {
		IDs []uint `json:"ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}
	if len(req.IDs) < 3 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "至少选择 3 条历史记忆才能压缩"})
		return
	}

	allEntries, err := storage.ListMemoryEntriesBySession(h.db, sessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "加载历史记忆失败"})
		return
	}

	selectedSet := make(map[uint]struct{}, len(req.IDs))
	for _, id := range req.IDs {
		selectedSet[id] = struct{}{}
	}

	selectedEntries := make([]*types.MemoryEntry, 0, len(req.IDs))
	remainingEntries := make([]*types.MemoryEntry, 0, len(allEntries))
	for _, entry := range allEntries {
		if _, ok := selectedSet[entry.ID]; ok {
			selectedEntries = append(selectedEntries, entry)
			continue
		}
		remainingEntries = append(remainingEntries, entry)
	}

	if len(selectedEntries) < 3 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "所选历史记忆不足 3 条，无法压缩"})
		return
	}

	logger := task_runner.GetCommandLogger(h.db)
	logID := logger.LogCommand(models.CommandLogCreate{
		Source:     "memory_compaction",
		SourceName: fmt.Sprintf("Session %s", sessionID),
		Command:    "compress_selected_memory",
		Args:       []string{fmt.Sprintf("selected=%d", len(selectedEntries))},
		StdinData:  renderMemoryEntriesTranscript(selectedEntries),
		Fields: models.MergeCommandLogFields(
			models.BuildCommandLogFields(
				models.NewCommandLogField("selected_count", "选中条数", fmt.Sprintf("%d", len(selectedEntries)), "default"),
				models.NewCommandLogField("input", "输入", renderMemoryEntriesTranscript(selectedEntries), "default"),
			),
			agentsession.MemoryCompactionSerializationFields(allEntries, nil)...,
		),
	})

	summary, err := h.summarizeSelectedMemoryEntries(c.Request.Context(), selectedEntries)
	if err != nil {
		exitCode := 1
		logger.UpdateCommandResultWithFields(logID, task_runner.CommandResultUpdate{
			Fields: []models.CommandLogField{
				{Key: "error", Label: "错误信息", Value: err.Error(), Tone: "error"},
			},
			ExitCode: &exitCode,
			Error:    err,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "压缩历史记忆失败: " + err.Error()})
		return
	}
	if strings.TrimSpace(summary) == "" {
		exitCode := 1
		emptyErr := fmt.Errorf("压缩结果为空")
		logger.UpdateCommandResultWithFields(logID, task_runner.CommandResultUpdate{
			Fields: []models.CommandLogField{
				{Key: "error", Label: "错误信息", Value: emptyErr.Error(), Tone: "error"},
			},
			ExitCode: &exitCode,
			Error:    emptyErr,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "压缩结果为空"})
		return
	}

	firstSelected := selectedEntries[0]
	rebuilt := make([]*types.MemoryEntry, 0, len(remainingEntries)+2)
	inserted := false
	for _, entry := range allEntries {
		if _, ok := selectedSet[entry.ID]; ok {
			if !inserted {
				rebuilt = append(rebuilt,
					&types.MemoryEntry{
						SessionID:  sessionID,
						EntryKind:  "summary_user",
						Role:       "user",
						Content:    "总结一下之前都做了什么。",
						Synthetic:  true,
						TokenCount: 0,
						Created:    firstSelected.Created,
					},
					&types.MemoryEntry{
						SessionID:  sessionID,
						EntryKind:  "summary_assistant",
						Role:       "assistant",
						Content:          strings.TrimSpace(summary),
						Synthetic:          true,
						CompressionLevel:   2,
						Created:    firstSelected.Created + 1,
					},
				)
				inserted = true
			}
			continue
		}
		rebuilt = append(rebuilt, entry)
	}

	now := time.Now().UnixMilli()
	for index, entry := range rebuilt {
		entry.ID = 0
		entry.SessionID = sessionID
		entry.Sequence = int64(index + 1)
		if entry.Created == 0 {
			entry.Created = now + int64(index)
		}
		entry.Updated = now
	}

	if err := storage.ReplaceSessionMemoryWithEntries(h.db, sessionID, rebuilt); err != nil {
		exitCode := 1
		logger.UpdateCommandResultWithFields(logID, task_runner.CommandResultUpdate{
			Fields: []models.CommandLogField{
				{Key: "error", Label: "错误信息", Value: err.Error(), Tone: "error"},
			},
			ExitCode: &exitCode,
			Error:    err,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "写回压缩记忆失败"})
		return
	}

	currentRate := 0.0
	if len(allEntries) > 0 {
		currentRate = float64(len(allEntries)-len(rebuilt)) * 100 / float64(len(allEntries))
	}
	stdout := fmt.Sprintf(
		"压缩了几个: %d\n当前会话的对话数量: %d -> %d\n当前压缩率: %.2f%%\n\n输入是什么:\n%s\n\n输出是什么:\n%s",
		len(selectedEntries),
		len(allEntries),
		len(rebuilt),
		currentRate,
		renderMemoryEntriesTranscript(selectedEntries),
		summary,
	)
	exitCode := 0
	logger.UpdateCommandResultWithFields(logID, task_runner.CommandResultUpdate{
		Fields: append([]models.CommandLogField{
			{Key: "summary", Label: "输出摘要", Value: summary, Tone: "default"},
			{Key: "stats", Label: "压缩统计", Value: fmt.Sprintf("%d -> %d (%.2f%%)", len(allEntries), len(rebuilt), currentRate), Tone: "default"},
			{Key: "stdout", Label: "执行摘要", Value: stdout, Tone: "default"},
		}, agentsession.MemoryCompactionSerializationFields(allEntries, rebuilt)...),
		ExitCode: &exitCode,
	})

	c.JSON(http.StatusOK, gin.H{
		"message": "压缩完成",
		"count":   len(selectedEntries),
	})
}

func (h *SessionHandler) CompactSessionMemory(c *gin.Context) {
	sessionID := c.Param("id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的会话ID"})
		return
	}
	if _, err := storage.GetSession(h.db, sessionID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "会话不存在"})
		return
	}
	allEntries, err := storage.ListMemoryEntriesBySession(h.db, sessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "加载历史记忆失败"})
		return
	}
	if len(allEntries) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "当前会话没有可压缩的历史记忆"})
		return
	}
	result, err := h.runGraduatedSessionCompaction(sessionID, allEntries, true, agentsession.MemoryCompactionUserInstruction, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "压缩历史记忆失败: " + err.Error()})
		return
	}
	if err := h.persistGraduatedCompaction(sessionID, allEntries, result, "compact_graduated_memory"); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	preview := h.buildSessionCompactionPreview(allEntries, result)
	c.JSON(http.StatusOK, preview)
}

func (h *SessionHandler) PreviewSessionMemoryCompaction(c *gin.Context) {
	sessionID := c.Param("id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的会话ID"})
		return
	}
	taskID := parseUint(c.Query("taskId"))
	_ = taskID
	if _, err := storage.GetSession(h.db, sessionID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "会话不存在"})
		return
	}

	allEntries, err := storage.ListMemoryEntriesBySession(h.db, sessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "加载历史记忆失败"})
		return
	}
	if len(allEntries) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "当前会话没有可压缩的历史记忆"})
		return
	}

	result, err := h.runGraduatedSessionCompaction(sessionID, allEntries, true, agentsession.MemoryCompactionUserInstruction, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "压缩历史记忆失败: " + err.Error()})
		return
	}
	if result.Skipped {
		c.JSON(http.StatusBadRequest, gin.H{"error": "当前上下文无需压缩"})
		return
	}
	preview := h.buildSessionCompactionPreview(allEntries, result)
	preview.Message = "已生成记忆压缩预览"
	c.JSON(http.StatusOK, preview)
}

func (h *SessionHandler) PreviewSessionMemoryCompactionStream(c *gin.Context) {
	sessionID := c.Param("id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的会话ID"})
		return
	}
	taskID := parseUint(c.Query("taskId"))
	_ = taskID
	if _, err := storage.GetSession(h.db, sessionID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "会话不存在"})
		return
	}

	allEntries, err := storage.ListMemoryEntriesBySession(h.db, sessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "加载历史记忆失败"})
		return
	}
	if len(allEntries) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "当前会话没有可压缩的历史记忆"})
		return
	}

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "当前连接不支持流式输出"})
		return
	}

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")

	writeSSE := func(event string, payload interface{}) {
		data, err := json.Marshal(payload)
		if err != nil {
			return
		}
		fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", event, data)
		flusher.Flush()
	}

	writeSSE("meta", gin.H{
		"count":         len(allEntries),
		"scopePercent":  h.memoryCompactionL2ScopePercent(),
		"targetPercent": h.memoryCompactionTargetPercent(),
		"beforeCount":   len(allEntries),
		"beforePreview": previewTranscriptText(renderMemoryEntriesTranscript(allEntries[:minInt(len(allEntries), 3)])),
	})

	result, err := h.runGraduatedSessionCompaction(sessionID, allEntries, true, agentsession.MemoryCompactionUserInstruction, func(partial string) {
		writeSSE("delta", gin.H{"summary": partial})
	})
	if err != nil {
		writeSSE("error", gin.H{"error": err.Error()})
		return
	}
	if result.Skipped {
		writeSSE("error", gin.H{"error": "当前上下文无需压缩"})
		return
	}
	preview := h.buildSessionCompactionPreview(allEntries, result)
	preview.Message = "已生成记忆压缩摘要"
	writeSSE("done", preview)
}

func (h *SessionHandler) summarizeSelectedMemoryEntriesForTask(ctx context.Context, taskID uint, entries []*types.MemoryEntry) (string, error) {
	return h.summarizeSelectedMemoryEntriesForTaskWithDelta(ctx, taskID, entries, nil)
}

func (h *SessionHandler) summarizeSelectedMemoryEntriesForTaskWithDelta(ctx context.Context, taskID uint, entries []*types.MemoryEntry, onSummaryDelta func(string)) (string, error) {
	_ = taskID
	return h.summarizeMemoryEntriesWithCompactionWorker(ctx, entries, onSummaryDelta)
}

func (h *SessionHandler) summarizeMemoryEntriesWithCompactionWorker(ctx context.Context, entries []*types.MemoryEntry, onSummaryDelta func(string)) (string, error) {
	summary, _, _, err := agentsession.RunMemoryCompactionWorkerChat(agentsession.MemoryCompactionWorkerChatOptions{
		Context: ctx,
		DB:      h.db,
		Memory: &types.Memory{
			Entries: entries,
		},
		UserInput:      agentsession.MemoryCompactionUserInstruction,
		OnSummaryDelta: onSummaryDelta,
	})
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(summary) == "" {
		return "", fmt.Errorf("memory compaction summary is empty")
	}
	return strings.TrimSpace(summary), nil
}

func (h *SessionHandler) ApplySessionMemoryCompaction(c *gin.Context) {
	sessionID := c.Param("id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的会话ID"})
		return
	}
	if _, err := storage.GetSession(h.db, sessionID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "会话不存在"})
		return
	}

	_ = c.ShouldBindJSON(&struct {
		Summary string `json:"summary"`
	}{})

	allEntries, err := storage.ListMemoryEntriesBySession(h.db, sessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "加载历史记忆失败"})
		return
	}
	if len(allEntries) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "当前会话没有可压缩的历史记忆"})
		return
	}

	result, err := h.runGraduatedSessionCompaction(sessionID, allEntries, true, agentsession.MemoryCompactionUserInstruction, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "压缩历史记忆失败: " + err.Error()})
		return
	}
	if err := h.persistGraduatedCompaction(sessionID, allEntries, result, "apply_graduated_memory_compaction"); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	preview := h.buildSessionCompactionPreview(allEntries, result)
	preview.Message = "记忆压缩已应用"
	c.JSON(http.StatusOK, preview)
}

func previewTranscriptText(text string) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return ""
	}
	const maxPreviewBytes = 4000
	bytes := []byte(trimmed)
	if len(bytes) <= maxPreviewBytes {
		return trimmed
	}
	return string(bytes[:maxPreviewBytes]) + "\n..."
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func selectOldestEntriesForCompaction(entries []*types.MemoryEntry, scopePercent int) []*types.MemoryEntry {
	if len(entries) == 0 {
		return nil
	}
	if scopePercent <= 0 {
		scopePercent = models.DefaultMemoryCompactionScopePercent
	}

	totalTokens := 0
	for _, entry := range entries {
		totalTokens += estimateMemoryEntryTokens(entry)
	}
	if totalTokens <= 0 {
		return append([]*types.MemoryEntry(nil), entries[:1]...)
	}

	target := totalTokens * scopePercent / 100
	if target <= 0 {
		target = 1
	}

	selected := make([]*types.MemoryEntry, 0, len(entries))
	accumulated := 0
	for _, entry := range entries {
		if entry == nil {
			continue
		}
		selected = append(selected, entry)
		accumulated += estimateMemoryEntryTokens(entry)
		if accumulated >= target {
			break
		}
	}
	return selected
}

func estimateMemoryEntryTokens(entry *types.MemoryEntry) int {
	if entry == nil {
		return 0
	}
	if entry.TokenCount > 0 {
		return entry.TokenCount
	}
	content := strings.TrimSpace(entry.SerializeText())
	if content == "" {
		content = strings.TrimSpace(entry.RenderContent())
	}
	if content == "" {
		return 0
	}
	return token.Estimate(content)
}

func buildCompactedSessionEntries(sessionID string, selectedEntries []*types.MemoryEntry, remainingEntries []*types.MemoryEntry, summary string) []*types.MemoryEntry {
	if len(selectedEntries) == 0 {
		return append([]*types.MemoryEntry(nil), remainingEntries...)
	}

	firstSelected := selectedEntries[0]
	now := time.Now().UnixMilli()
	rebuilt := []*types.MemoryEntry{
		{
			SessionID:  sessionID,
			EntryKind:  "summary_user",
			Role:       "user",
			Content:    "总结一下之前都做了什么。",
			Synthetic:  true,
			TokenCount: token.Estimate("总结一下之前都做了什么。"),
			Sequence:   1,
			Created:    firstSelected.Created,
			Updated:    now,
		},
		{
			SessionID:  sessionID,
			EntryKind:  "summary_assistant",
			Role:       "assistant",
			Content:          strings.TrimSpace(summary),
			Synthetic:          true,
			CompressionLevel: 2,
			TokenCount:         token.Estimate(summary),
			Sequence:   2,
			Created:    firstSelected.Created + 1,
			Updated:    now,
		},
	}

	rebuilt = append(rebuilt, remainingEntries...)
	for index, entry := range rebuilt {
		if entry == nil {
			continue
		}
		entry.ID = 0
		entry.SessionID = sessionID
		entry.Sequence = int64(index + 1)
		if entry.Created == 0 {
			entry.Created = now + int64(index)
		}
		entry.Updated = now
	}
	return rebuilt
}

func (h *SessionHandler) summarizeSelectedMemoryEntries(ctx context.Context, entries []*types.MemoryEntry) (string, error) {
	return h.summarizeMemoryEntriesWithCompactionWorker(ctx, entries, nil)
}

func loadWorkerOccupationPrompt(db *gorm.DB, worker *models.Worker) string {
	if worker == nil || strings.TrimSpace(worker.Occupation) == "" {
		return ""
	}
	occupation, err := database.GetOccupationByCode(db, worker.Occupation)
	if err != nil || occupation == nil {
		return ""
	}
	return strings.TrimSpace(occupation.Prompt)
}

func latestSessionWorkerName(messages []*types.WithParts) string {
	for index := len(messages) - 1; index >= 0; index-- {
		msg := messages[index]
		if msg == nil || msg.Info == nil {
			continue
		}
		if worker := strings.TrimSpace(msg.Info.Worker); worker != "" {
			return worker
		}
	}
	return ""
}

func buildSessionEnvPrompt(session *types.Info) string {
	workDir := strings.TrimSpace(session.Directory)
	isGit := "no"
	if workDir != "" {
		if _, err := os.Stat(filepath.Join(workDir, ".git")); err == nil {
			isGit = "yes"
		}
	}
	lines := []string{
		"Working directory: " + workDir,
		"Platform: " + strings.TrimSpace(runtime.GOOS),
		"Is directory a git repo: " + isGit,
	}
	return strings.Join(lines, "\n")
}

func buildInstalledSkillsSummary() string {
	skills, err := skillfs.ListInstalledSkills()
	if err != nil || len(skills) == 0 {
		return ""
	}
	lines := []string{
		"Installed skills available for on-demand loading:",
		"The items below only include skill introductions, not the full skill content.",
		"If a task needs one of these skills, call the `load_skill` tool with the exact skill name.",
		"A successful `load_skill` returns the full skill document in the tool output; it is stored in conversation history and is not added to the static prompt prefix.",
	}
	for _, skill := range skills {
		line := "- " + strings.TrimSpace(skill.Name)
		if desc := strings.TrimSpace(skill.Description); desc != "" {
			line += ": " + desc
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func buildWorkerToolPrompt(worker *models.Worker, project *models.Project) (string, []gin.H) {
	opts := agenttool.VisibleCatalogOptions{}
	if worker != nil {
		enabledSet, hasEnabledSet, err := models.ParseEnabledTools(worker.EnabledTools)
		if err == nil {
			opts.WorkerEnabledTools = enabledSet
			opts.HasWorkerEnabledTools = hasEnabledSet
		}
	}
	if project != nil {
		opts.ProjectYoloMode = project.YoloMode
		if perms, err := models.ParseProjectToolPermissions(project.ToolPermissions); err == nil {
			opts.ProjectToolPermissions = perms
		}
	}
	toolInfos := agenttool.CatalogVisible(opts)
	lines := make([]string, 0, len(toolInfos))
	out := make([]gin.H, 0, len(toolInfos))
	for _, info := range toolInfos {
		item := gin.H{
			"name":        info.Name,
			"description": info.Description,
		}
		if info.VerbosName != "" {
			item["verbosName"] = info.VerbosName
		}
		if info.IsMcp {
			item["isMcp"] = true
			if info.McpServer != "" {
				item["mcpServer"] = info.McpServer
			}
		}
		out = append(out, item)
		lines = append(lines, fmt.Sprintf("%s: %s", info.Name, strings.TrimSpace(info.Description)))
	}
	return strings.Join(lines, "\n"), out
}

func summarizeLoadedSkills(session *types.Info) ([]gin.H, int) {
	skills := []gin.H{}
	seen := map[string]struct{}{}
	totalBytes := 0
	if session == nil {
		return skills, totalBytes
	}
	for _, enabled := range session.EnabledSkills {
		name := strings.TrimSpace(enabled)
		if name == "" {
			continue
		}
		if _, exists := seen[strings.ToLower(name)]; exists {
			continue
		}
		skill, content, err := skillfs.LoadInstalledSkillContent(name)
		if err != nil {
			continue
		}
		skills = append(skills, gin.H{
			"name":         skill.Name,
			"description":  strings.TrimSpace(skill.Description),
			"relativePath": strings.TrimSpace(skill.RelativePath),
		})
		totalBytes += len([]byte(strings.TrimSpace(content)))
		seen[strings.ToLower(name)] = struct{}{}
	}
	return skills, totalBytes
}

func summarizeRegularMemoryBytes(entries []*types.MemoryEntry) int {
	total := 0
	for _, entry := range entries {
		if entry == nil {
			continue
		}
		if entry.ToolName == "load_skill" {
			continue
		}
		total += len([]byte(strings.TrimSpace(entry.SerializeText())))
	}
	return total
}

func buildContextComponent(key, label, text string) gin.H {
	text = strings.TrimSpace(text)
	return gin.H{
		"key":   key,
		"label": label,
		"bytes": len([]byte(text)),
	}
}

func buildContextComponentWithBytes(key, label string, bytes int) gin.H {
	return gin.H{
		"key":   key,
		"label": label,
		"bytes": bytes,
	}
}

func parseProjectID(projectID string) (uint, error) {
	var parsed uint
	_, err := fmt.Sscanf(projectID, "%d", &parsed)
	return parsed, err
}

func upsertPreviewPart(parts []*types.Part, part *types.Part) []*types.Part {
	next := append([]*types.Part(nil), parts...)
	for index, existing := range next {
		if existing != nil && existing.ID == part.ID {
			next[index] = part
			return next
		}
	}
	return append(next, part)
}

func totalSessionMemoryTokens(entries []*types.MemoryEntry) int {
	total := 0
	for _, entry := range entries {
		if entry == nil {
			continue
		}
		total += entry.TokenCount
	}
	return total
}

func renderMemoryEntriesTranscript(entries []*types.MemoryEntry) string {
	lines := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry == nil {
			continue
		}
		content := strings.TrimSpace(entry.RenderContent())
		if content == "" {
			continue
		}
		lines = append(lines, fmt.Sprintf("[%s] %s", strings.TrimSpace(entry.Role), content))
	}
	return strings.Join(lines, "\n")
}
