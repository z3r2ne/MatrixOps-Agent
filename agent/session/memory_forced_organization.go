package session

import (
	"fmt"
	"strings"

	agenttoken "matrixops-agent/token"
	"matrixops-agent/types"
	database "pkgs/db"
	"pkgs/db/storage"

	sessionmemory "matrixops-agent/session/memory"
)

func (r *AgentRunner) memoryCompactionTriggerThresholdPercent() int {
	return database.GetMemoryCompactionTriggerThresholdPercent(r.db)
}

func (r *AgentRunner) memoryCompactionTargetPercent() int {
	return database.GetMemoryCompactionTargetPercent(r.db)
}

func (r *AgentRunner) memoryCompactionL2ScopePercent() int {
	return database.GetMemoryCompactionL2ScopePercent(r.db)
}

func (r *AgentRunner) forceOrganizeProcessV2MemoryIfNeeded(runtimeConfig *RuntimeConfig) error {
	return r.forceOrganizeProcessV2Memory(runtimeConfig, false)
}

func (r *AgentRunner) forceOrganizeProcessV2MemoryNow(runtimeConfig *RuntimeConfig) error {
	return r.forceOrganizeProcessV2Memory(runtimeConfig, true)
}

func (r *AgentRunner) reloadProcessV2MemoryEntriesFromDB(runtimeConfig *RuntimeConfig) error {
	if r == nil || runtimeConfig == nil || r.db == nil {
		return nil
	}
	sessionID := r.GetSessionID()
	if strings.TrimSpace(sessionID) == "" {
		return nil
	}
	entries, err := storage.ListMemoryEntriesBySession(r.db, sessionID)
	if err != nil {
		return fmt.Errorf("reload session memory entries: %w", err)
	}
	replaceProcessV2MemoryEntries(runtimeConfig, entries)
	return nil
}

// shouldPersistGraduatedCompactionResult reports whether compaction changed entries
// enough to warrant rewriting session memory in DB and runtime state.
func shouldPersistGraduatedCompactionResult(before, after []*types.MemoryEntry, result GraduatedMemoryCompactionResult) bool {
	if result.Skipped {
		return false
	}
	if len(result.LevelsExecuted) > 0 || len(result.Logs) > 0 {
		return true
	}
	if len(before) != len(after) {
		return true
	}
	return totalMemoryTokens(before) != totalMemoryTokens(after)
}

func (r *AgentRunner) forceOrganizeProcessV2Memory(runtimeConfig *RuntimeConfig, force bool) error {
	if r == nil || runtimeConfig == nil || r.db == nil {
		return nil
	}
	if _, err := ResolveMemoryCompactionRuntime(r.db); err != nil {
		return nil
	}

	if err := r.reloadProcessV2MemoryEntriesFromDB(runtimeConfig); err != nil {
		return err
	}

	memory, err := r.buildProcessV2Memory(runtimeConfig)
	if err != nil {
		return err
	}
	if memory == nil {
		return nil
	}
	entries := memory.TranscriptSourceEntries()
	if len(entries) == 0 {
		return nil
	}

	limitTokens := memoryCompactionTokenLimit(runtimeConfig)
	if limitTokens <= 0 && !force {
		return nil
	}
	if limitTokens <= 0 {
		limitTokens = totalMemoryTokens(entries) + agenttoken.Estimate(strings.TrimSpace(runtimeConfig.UserInput))
	}
	currentTokens := currentContextTokensFromUsage(runtimeConfig)
	if currentTokens <= 0 {
		// 首轮尚无 LLM usage 时，用记忆条目 token 估算占用，与 shouldRunGraduatedMemoryCompaction 一致。
		// 否则工具密集阶段（如 explore 连续 read）永远不会触发压缩，上下文会无限膨胀。
		currentTokens = totalMemoryTokens(entries) + agenttoken.Estimate(strings.TrimSpace(runtimeConfig.UserInput))
	}
	if currentTokens <= 0 {
		return nil
	}

	return r.runGraduatedMemoryCompaction(runtimeConfig, memory, limitTokens, currentTokens, force)
}

func (r *AgentRunner) runGraduatedMemoryCompaction(runtimeConfig *RuntimeConfig, memory *types.Memory, limitTokens, currentTokens int, force bool) error {
	if memory == nil {
		return nil
	}
	entries := memory.TranscriptSourceEntries()
	if len(entries) == 0 {
		return nil
	}

	sessionID := r.GetSessionID()
	beforeEntries := sessionmemory.CloneMemoryEntries(entries)
	compactionInput := GraduatedMemoryCompactionInput{
		SessionID:     sessionID,
		Entries:       entries,
		LimitTokens:   limitTokens,
		CurrentTokens: currentTokens,
		Config: GraduatedMemoryCompactionConfig{
			TriggerThresholdPercent: r.memoryCompactionTriggerThresholdPercent(),
			TargetPercent:           r.memoryCompactionTargetPercent(),
			L2ScopePercent:          r.memoryCompactionL2ScopePercent(),
			Force:                   force,
		},
		UserPrompt: memoryCompactionUserPrompt(runtimeConfig),
		Summarize: func(scopeMemory *types.Memory, userPrompt string, delta func(string)) (string, compactionPromptInfo, error) {
			summary, promptInfo, _, summarizeErr := r.summarizeMemoryWithPromptBuilder(runtimeConfig, scopeMemory, userPrompt, delta)
			return summary, promptInfo, summarizeErr
		},
	}
	if shouldRun, _ := shouldRunGraduatedMemoryCompaction(compactionInput); !shouldRun {
		return nil
	}

	var compactionLogID uint
	if r.db != nil && sessionID != "" {
		compactionLogID = r.logMemoryCompressionCycleStart(
			sessionID,
			"graduated",
			0,
			len(entries),
			len(entries),
			totalMemoryBytes(entries),
			renderMemoryTranscript(entries),
			beforeEntries,
		)
	}

	beforePreview := buildCompactionResultPreview(beforeEntries)
	runningEvent := memoryCompactionEvent{
		Kind:          "memory",
		Strategy:      "graduated",
		Scope:         "l2",
		Status:        "running",
		BeforeCount:   len(entries),
		AfterCount:    len(entries),
		OriginalCount: len(entries),
		BeforeBytes:   totalMemoryBytes(entries),
		AfterBytes:    totalMemoryBytes(entries),
		InputPreview:  beforePreview,
	}
	part := r.newMemoryCompactionPart(runtimeConfig, sessionID, runningEvent)
	var onSummaryDelta func(string)
	if part != nil {
		_ = r.updateMemoryCompactionPart(part, runningEvent)
		baseEvent := runningEvent
		onSummaryDelta = func(summary string) {
			streamEvent := baseEvent
			streamEvent.Status = "running"
			streamEvent.Summary = summary
			streamEvent.SummaryStreaming = true
			_ = r.updateMemoryCompactionPart(part, streamEvent)
		}
	}
	compactionInput.OnSummaryDelta = onSummaryDelta

	emitFailure := func(promptInfo compactionPromptInfo, after []*types.MemoryEntry, err error) error {
		if err == nil {
			return nil
		}
		if after == nil {
			after = beforeEntries
		}
		if compactionLogID != 0 {
			r.logMemoryCompressionCycleFinish(
				compactionLogID,
				"graduated",
				len(beforeEntries),
				len(after),
				len(beforeEntries),
				totalMemoryBytes(beforeEntries),
				totalMemoryBytes(after),
				renderMemoryTranscript(beforeEntries),
				"",
				"",
				"",
				formatCompactionLevelLogs(nil),
				beforeEntries,
				after,
				err,
			)
		}
		event := runningEvent
		event.Status = "error"
		event.RequestPrompt = promptInfo.Combined()
		event.Error = err.Error()
		if part != nil {
			_ = r.updateMemoryCompactionPart(part, event)
		} else {
			_ = r.emitMemoryCompactionPart(runtimeConfig, sessionID, event)
		}
		return err
	}

	compactionResult, err := RunGraduatedMemoryCompaction(compactionInput)
	if err != nil {
		return emitFailure(compactionPromptInfo{UserPrompt: compactionResult.RequestPrompt}, beforeEntries, err)
	}
	if compactionResult.Skipped {
		if part != nil {
			skipEvent := runningEvent
			skipEvent.Status = "completed"
			skipEvent.Summary = memoryCompactionSkipReasonLabel(compactionResult.SkipReason)
			skipEvent.ResultPreview = beforePreview
			if err := r.updateMemoryCompactionPart(part, skipEvent); err != nil {
				return err
			}
		}
		if compactionLogID != 0 {
			r.logMemoryCompressionCycleFinish(
				compactionLogID,
				"graduated",
				len(beforeEntries),
				len(beforeEntries),
				len(beforeEntries),
				totalMemoryBytes(beforeEntries),
				totalMemoryBytes(beforeEntries),
				renderMemoryTranscript(beforeEntries),
				memoryCompactionSkipReasonLabel(compactionResult.SkipReason),
				"",
				"",
				formatCompactionLevelLogs(nil),
				beforeEntries,
				beforeEntries,
				nil,
			)
		}
		return nil
	}

	nextEntries := compactionResult.Entries
	if shouldPersistGraduatedCompactionResult(beforeEntries, nextEntries, compactionResult) {
		replaceProcessV2MemoryEntries(runtimeConfig, nextEntries)
		syncRuntimeMemoryTokens(runtimeConfig, nextEntries)

		if r.db != nil && sessionID != "" {
			if err := storage.ReplaceSessionMemoryWithEntries(r.db, sessionID, nextEntries); err != nil {
				return emitFailure(compactionPromptInfo{}, nextEntries, err)
			}
			if err := r.updateSessionMemoryTokens(sessionID, nextEntries); err != nil {
				return emitFailure(compactionPromptInfo{}, nextEntries, err)
			}
		}
	} else if err := r.reloadProcessV2MemoryEntriesFromDB(runtimeConfig); err != nil {
		return err
	}

	if onSummaryDelta != nil && strings.TrimSpace(compactionResult.Summary) != "" {
		onSummaryDelta(compactionResult.Summary)
	}

	if compactionLogID != 0 {
		r.logMemoryCompressionCycleFinish(
			compactionLogID,
			"graduated",
			len(beforeEntries),
			len(nextEntries),
			len(beforeEntries),
			totalMemoryBytes(beforeEntries),
			totalMemoryBytes(nextEntries),
			renderMemoryTranscript(beforeEntries),
			compactionResult.Summary,
			"",
			"",
			formatCompactionLevelLogs(compactionResult.Logs),
			beforeEntries,
			nextEntries,
			nil,
		)
	}

	event := runningEvent
	event.Status = "completed"
	event.SummaryStreaming = false
	event.CompressedCount = len(entries) - len(nextEntries)
	if event.CompressedCount < 0 {
		event.CompressedCount = 0
	}
	event.AfterCount = len(nextEntries)
	event.AfterBytes = totalMemoryBytes(nextEntries)
	event.Summary = compactionResult.Summary
	event.RequestPrompt = compactionResult.RequestPrompt
	event.InputPreview = beforePreview
	event.ResultPreview = buildCompactionResultPreview(nextEntries)
	if len(compactionResult.LevelsExecuted) > 0 {
		event.Scope = fmt.Sprintf("levels:%v", compactionResult.LevelsExecuted)
	}
	if part != nil {
		if err := r.updateMemoryCompactionPart(part, event); err != nil {
			return err
		}
	} else if err := r.emitMemoryCompactionPart(runtimeConfig, sessionID, event); err != nil {
		return err
	}
	return nil
}

func syncRuntimeMemoryTokens(runtimeConfig *RuntimeConfig, entries []*types.MemoryEntry) {
	if runtimeConfig == nil {
		return
	}
	input := totalMemoryTokens(entries)
	if input < 0 {
		input = 0
	}
	updated := &MessageTokens{
		Input:     input,
		Output:    0,
		Reasoning: 0,
		Cache:     TokenCache{},
	}
	runtimeConfig.SessionTokens = updated
	if runtimeConfig.Assistant != nil {
		runtimeConfig.Assistant.Tokens = updated
	}
}

func memoryCompactionUserPrompt(runtimeConfig *RuntimeConfig) string {
	if runtimeConfig != nil && strings.TrimSpace(runtimeConfig.ManualMemoryCompactionPrompt) != "" {
		return strings.TrimSpace(runtimeConfig.ManualMemoryCompactionPrompt)
	}
	return ""
}

func memoryCompactionSkipReasonLabel(reason string) string {
	switch strings.TrimSpace(reason) {
	case "below_trigger_threshold":
		return "当前上下文占用未达压缩阈值，无需压缩"
	case "already_meets_target":
		return "当前上下文占用已满足目标，无需压缩"
	case "empty_entries":
		return "没有可压缩的历史记忆"
	case "missing_limit":
		return "缺少可用上下文窗口，无法压缩"
	default:
		if strings.TrimSpace(reason) == "" {
			return "无需压缩"
		}
		return "无需压缩：" + reason
	}
}

func memoryCompactionTokenLimit(runtimeConfig *RuntimeConfig) int {
	if runtimeConfig == nil {
		return 0
	}
	if runtimeConfig.AutoCompressionLimitTokens > 0 {
		return runtimeConfig.AutoCompressionLimitTokens
	}
	if runtimeConfig.ModelSettings == nil {
		return 0
	}
	if runtimeConfig.ModelSettings.ContextLimit <= 0 {
		return 0
	}
	outputLimit := runtimeConfig.ModelSettings.OutputLimit
	if outputLimit < 0 {
		outputLimit = 0
	}
	limit := runtimeConfig.ModelSettings.ContextLimit - outputLimit
	if limit > 0 {
		return limit
	}
	return runtimeConfig.ModelSettings.ContextLimit
}

func replaceProcessV2MemoryEntries(runtimeConfig *RuntimeConfig, entries []*types.MemoryEntry) {
	if runtimeConfig == nil {
		return
	}
	if runtimeConfig.MemoryState != nil {
		runtimeConfig.MemoryState.ReplaceEntries(entries)
		return
	}
	if runtimeConfig.BaseMemory == nil {
		return
	}
	clonedEntries := sessionmemory.CloneMemoryEntries(entries)
	runtimeConfig.BaseMemory.Entries = clonedEntries
	runtimeConfig.BaseMemory.History = memoryEntriesToChatHistory(clonedEntries)
	runtimeConfig.BaseMemory.LatestToolCall = buildLatestToolCall(runtimeConfig.BaseMemory.History)
}

// RunGraduatedMemoryCompactionForSession runs the shared L2 compaction pipeline for HTTP handlers.
func RunGraduatedMemoryCompactionForSession(input GraduatedMemoryCompactionInput) (GraduatedMemoryCompactionResult, error) {
	return RunGraduatedMemoryCompaction(input)
}
