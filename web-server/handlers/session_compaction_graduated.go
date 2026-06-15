package handlers

import (
	"encoding/json"
	"fmt"
	"strings"

	agentsession "matrixops-agent/session"
	"matrixops-agent/types"
	database "pkgs/db"
	"pkgs/db/models"
	"pkgs/db/storage"

	"matrixops/services/task_runner"
)

func (h *SessionHandler) memoryCompactionTargetPercent() int {
	return database.GetMemoryCompactionTargetPercent(h.db)
}

func (h *SessionHandler) memoryCompactionL2ScopePercent() int {
	return database.GetMemoryCompactionL2ScopePercent(h.db)
}

func (h *SessionHandler) memoryCompactionScopePercent() int {
	return h.memoryCompactionL2ScopePercent()
}

func (h *SessionHandler) memoryCompactionLimitTokens() int {
	runtime, err := agentsession.ResolveMemoryCompactionRuntime(h.db)
	if err != nil || runtime == nil || runtime.ModelSettings == nil {
		return 0
	}
	settings := runtime.ModelSettings
	if settings.ContextLimit <= 0 {
		return 0
	}
	outputLimit := settings.OutputLimit
	if outputLimit < 0 {
		outputLimit = 0
	}
	limit := settings.ContextLimit - outputLimit
	if limit > 0 {
		return limit
	}
	return settings.ContextLimit
}

func (h *SessionHandler) runGraduatedSessionCompaction(sessionID string, entries []*types.MemoryEntry, force bool, userPrompt string, onDelta func(string)) (agentsession.GraduatedMemoryCompactionResult, error) {
	return agentsession.RunSessionGraduatedCompaction(agentsession.SessionGraduatedCompactionOptions{
		DB:             h.db,
		SessionID:      sessionID,
		Entries:        entries,
		LimitTokens:    h.memoryCompactionLimitTokens(),
		Force:          force,
		UserPrompt:     userPrompt,
		OnSummaryDelta: onDelta,
	})
}

func (h *SessionHandler) persistGraduatedCompaction(sessionID string, allEntries []*types.MemoryEntry, result agentsession.GraduatedMemoryCompactionResult, command string) error {
	if result.Skipped {
		return fmt.Errorf("当前上下文无需压缩")
	}
	rebuilt := result.Entries
	logger := task_runner.GetCommandLogger(h.db)
	logID := logger.LogCommand(models.CommandLogCreate{
		Source:     "memory_compaction",
		SourceName: fmt.Sprintf("Session %s", sessionID),
		Command:    command,
		Args: []string{
			fmt.Sprintf("levels=%v", result.LevelsExecuted),
			fmt.Sprintf("target_percent=%d", h.memoryCompactionTargetPercent()),
			fmt.Sprintf("l2_scope_percent=%d", h.memoryCompactionL2ScopePercent()),
		},
		StdinData: renderMemoryEntriesTranscript(allEntries),
		Fields: models.MergeCommandLogFields(
			models.BuildCommandLogFields(
				models.NewCommandLogField("levelLogs", "分级压缩日志", formatCompactionLevelLogsPublic(result.Logs), "default"),
				models.NewCommandLogField("summary", "压缩摘要", result.Summary, "default"),
			),
			agentsession.MemoryCompactionSerializationFields(allEntries, rebuilt)...,
		),
	})

	if err := storage.ReplaceSessionMemoryWithEntries(h.db, sessionID, rebuilt); err != nil {
		exitCode := 1
		logger.UpdateCommandResultWithFields(logID, task_runner.CommandResultUpdate{
			Fields:   []models.CommandLogField{{Key: "error", Label: "错误信息", Value: err.Error(), Tone: "error"}},
			ExitCode: &exitCode,
			Error:    err,
		})
		return err
	}
	if err := storage.UpdateSessionTokens(h.db, sessionID, &types.MessageTokens{
		Input: totalSessionMemoryTokens(rebuilt),
	}); err != nil {
		return err
	}
	exitCode := 0
	logger.UpdateCommandResultWithFields(logID, task_runner.CommandResultUpdate{
		Fields: append([]models.CommandLogField{
			{Key: "stats", Label: "压缩统计", Value: fmt.Sprintf("%d -> %d", len(allEntries), len(rebuilt)), Tone: "default"},
		}, agentsession.MemoryCompactionSerializationFields(allEntries, rebuilt)...),
		ExitCode: &exitCode,
	})
	return nil
}

func formatCompactionLevelLogsPublic(logs []agentsession.MemoryCompactionLevelLog) string {
	if len(logs) == 0 {
		return ""
	}
	raw, err := json.Marshal(logs)
	if err != nil {
		return ""
	}
	return string(raw)
}

func (h *SessionHandler) buildSessionCompactionPreview(allEntries []*types.MemoryEntry, result agentsession.GraduatedMemoryCompactionResult) sessionMemoryCompactionPreview {
	rebuilt := result.Entries
	if len(rebuilt) == 0 {
		rebuilt = allEntries
	}
	currentRate := 0.0
	if len(allEntries) > 0 {
		currentRate = float64(len(allEntries)-len(rebuilt)) * 100 / float64(len(allEntries))
	}
	return sessionMemoryCompactionPreview{
		Message:         "记忆压缩完成",
		Count:           len(allEntries) - len(rebuilt),
		ScopePercent:    h.memoryCompactionL2ScopePercent(),
		TargetPercent:   h.memoryCompactionTargetPercent(),
		L2ScopePercent:  h.memoryCompactionL2ScopePercent(),
		LevelsExecuted:  append([]int(nil), result.LevelsExecuted...),
		BeforeCount:     len(allEntries),
		AfterCount:      len(rebuilt),
		CompressionRate: currentRate,
		BeforePreview:   previewTranscriptText(renderMemoryEntriesTranscript(allEntries[:minInt(len(allEntries), 3)])),
		AfterPreview:    previewTranscriptText(renderMemoryEntriesTranscript(rebuilt[:minInt(len(rebuilt), 3)])),
		Summary:         strings.TrimSpace(result.Summary),
	}
}
