package session

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"matrixops-agent/llm"
	"matrixops-agent/provider"
	"matrixops-agent/types"
	database "pkgs/db"
	"pkgs/db/models"
	"pkgs/db/storage"

	"gorm.io/gorm"
)

const memoryAnalysisSummaryMaxRunes = 150

const memoryAnalysisSystemInstruction = `你是一位会话记忆分析助手。请阅读用户提供的会话记忆内容，提取关键词并撰写简短总结。

输出要求：
- 严格只输出 JSON，格式如下：
{"keywords":["关键词1","关键词2"],"summary":"简短总结"}
- keywords：3-8 个中文关键词，反映会话主题、技术栈、核心对象或任务方向
- summary：用一段话概括会话当前的核心内容与状态，不超过150个汉字
- 只基于给定记忆内容总结，不要编造未出现的信息
- 不要输出 JSON 以外的任何文字、Markdown 或解释`

type SessionMemoryAnalysisOptions struct {
	DB        *gorm.DB
	SessionID string
}

func RunSessionMemoryAnalysis(options SessionMemoryAnalysisOptions) (*types.MemoryAnalysis, error) {
	if options.DB == nil {
		return nil, fmt.Errorf("database is not configured")
	}
	sessionID := strings.TrimSpace(options.SessionID)
	if sessionID == "" {
		return nil, fmt.Errorf("session id is required")
	}

	if _, err := storage.GetSession(options.DB, sessionID); err != nil {
		return nil, fmt.Errorf("session not found: %w", err)
	}

	entries, err := storage.ListMemoryEntriesBySession(options.DB, sessionID)
	if err != nil {
		return nil, fmt.Errorf("load session memory failed: %w", err)
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("当前会话没有可分析的记忆内容")
	}

	workerCtx, err := resolveSessionMemoryAnalysisWorkerContext(options.DB, sessionID)
	if err != nil {
		return nil, err
	}

	transcript := renderMemoryTranscript(entries)
	model := strings.TrimSpace(workerCtx.Worker.Model)
	if model == "" {
		model = strings.TrimSpace(workerCtx.LLMConfig.Model)
	}
	if model == "" {
		return nil, fmt.Errorf("no model configured for memory analysis")
	}

	temperature := 0.0
	if workerCtx.Worker.Temperature != nil {
		temperature = *workerCtx.Worker.Temperature
	}
	topP := models.EffectiveTopP(workerCtx.ModelSettings)
	maxOutput := database.EffectiveLLMMaxOutputTokens(options.DB, workerCtx.ModelSettings.OutputLimit)
	if maxOutput <= 0 {
		maxOutput = 512
	}
	if maxOutput > 1024 {
		maxOutput = 1024
	}

	client := provider.NewGenericClient()
	response, err := client.Chat(llm.ChatRequest{
		Context: context.Background(),
		Messages: []*llm.ModelMessage{
			{
				Role:    "user",
				Content: buildMemoryAnalysisUserPrompt(transcript),
			},
		},
		ProviderOptions: workerCtx.LLMConfig,
		Model:           model,
		Temperature:     temperature,
		TopP:            topP,
		MaxOutputTokens: maxOutput,
		ExtraOptions: map[string]interface{}{
			"instructions": memoryAnalysisSystemInstruction,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("memory analysis llm call failed: %w", err)
	}

	analysis, err := parseMemoryAnalysisResponse(renderContent(response.Message.Content))
	if err != nil {
		return nil, err
	}
	analysis.UpdatedAt = time.Now().UnixMilli()

	updated, err := storage.UpdateSessionByCallback(options.DB, sessionID, func(info *types.Info) error {
		info.MemoryAnalysis = analysis
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("save memory analysis failed: %w", err)
	}
	if updated == nil || updated.MemoryAnalysis == nil {
		return analysis, nil
	}
	return updated.MemoryAnalysis, nil
}

type sessionMemoryAnalysisWorkerContext struct {
	Worker        *models.Worker
	LLMConfig     *models.LLMConfig
	ModelSettings *models.ModelSettings
}

func resolveSessionMemoryAnalysisWorkerContext(db *gorm.DB, sessionID string) (*sessionMemoryAnalysisWorkerContext, error) {
	workerName := "chat"
	if task, err := database.GetTaskBySessionID(db, sessionID); err == nil && task != nil {
		if name := strings.TrimSpace(task.WorkerName); name != "" {
			workerName = name
		}
	}

	loaded, err := database.LoadWorkerModelContext(db, workerName)
	if err != nil {
		return nil, fmt.Errorf("load worker %q failed: %w", workerName, err)
	}
	if loaded.LLMConfig == nil {
		return nil, fmt.Errorf("worker %q has no llm config", workerName)
	}
	modelSettings := loaded.ModelSettings
	if modelSettings == nil {
		modelSettings = &models.ModelSettings{Name: database.DefaultModelSettingsName}
	}
	return &sessionMemoryAnalysisWorkerContext{
		Worker:        loaded.Worker,
		LLMConfig:     loaded.LLMConfig,
		ModelSettings: modelSettings,
	}, nil
}

func buildMemoryAnalysisUserPrompt(transcript string) string {
	return strings.Join([]string{
		"请分析以下会话记忆，并按模板输出 JSON。",
		"",
		"【记忆内容】",
		transcript,
	}, "\n")
}

type memoryAnalysisPayload struct {
	Keywords []string `json:"keywords"`
	Summary  string   `json:"summary"`
}

func parseMemoryAnalysisResponse(raw string) (*types.MemoryAnalysis, error) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var payload memoryAnalysisPayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil, fmt.Errorf("parse memory analysis response: %w", err)
	}

	keywords := make([]string, 0, len(payload.Keywords))
	seen := map[string]struct{}{}
	for _, keyword := range payload.Keywords {
		keyword = strings.TrimSpace(keyword)
		if keyword == "" {
			continue
		}
		if _, ok := seen[keyword]; ok {
			continue
		}
		seen[keyword] = struct{}{}
		keywords = append(keywords, keyword)
	}
	if len(keywords) == 0 {
		return nil, fmt.Errorf("memory analysis keywords are empty")
	}

	summary := strings.TrimSpace(payload.Summary)
	if summary == "" {
		return nil, fmt.Errorf("memory analysis summary is empty")
	}
	if utf8.RuneCountInString(summary) > memoryAnalysisSummaryMaxRunes {
		summary = truncateRunes(summary, memoryAnalysisSummaryMaxRunes)
	}

	return &types.MemoryAnalysis{
		Keywords: keywords,
		Summary:  summary,
	}, nil
}

func truncateRunes(value string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	return string(runes[:max])
}
