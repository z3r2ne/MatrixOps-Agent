package session

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"matrixops-agent/llm"
	"matrixops-agent/types"
	"matrixops-agent/util"
	coreagent "matrixops.local/core_agent"
	database "pkgs/db"
	"pkgs/db/models"
	"pkgs/db/storage"
)

const (
	defaultMemoryCompactionMaxOutputTokens = 2048

	// MemoryCompactionUserInstruction is appended to the compaction task prompt at the end of the user message.
	MemoryCompactionUserInstruction = "优先保留用户目标、已完成事项、当前状态、关键文件/改动、约束、技术决策、待办与风险。"
)

type memoryCompactionEvent struct {
	Kind               string
	Strategy           string
	Scope              string
	Status             string
	SummaryStreaming   bool
	CompressedCount int
	BeforeCount     int
	AfterCount      int
	OriginalCount   int
	BeforeBytes     int
	AfterBytes      int
	Summary         string
	InputPreview    string
	ResultPreview   string
	RequestPrompt   string
	ResponseFinish  string
	ResponseModel   string
	Error           string
	BatchIndex      int
	BatchTotal      int
}

type compactionPromptInfo struct {
	SystemPrompt string
	UserPrompt   string
}

func (p compactionPromptInfo) Combined() string {
	parts := make([]string, 0, 2)
	if strings.TrimSpace(p.SystemPrompt) != "" {
		parts = append(parts, "=== system ===\n"+p.SystemPrompt)
	}
	if strings.TrimSpace(p.UserPrompt) != "" {
		parts = append(parts, "=== user ===\n"+p.UserPrompt)
	}
	return strings.Join(parts, "\n\n")
}

func (r *AgentRunner) updateSessionMemoryTokens(sessionID string, entries []*types.MemoryEntry) error {
	updatedTokens := &MessageTokens{
		Input:     totalMemoryTokens(entries),
		Output:    0,
		Reasoning: 0,
		Cache:     TokenCache{},
	}
	return storage.UpdateSessionTokens(r.db, sessionID, updatedTokens)
}

func totalMemoryBytes(entries []*types.MemoryEntry) int {
	total := 0
	for _, entry := range entries {
		if entry == nil {
			continue
		}

		content := strings.TrimSpace(entry.SerializeText())
		if content == "" {
			content = strings.TrimSpace(entry.RenderContent())
		}
		total += len([]byte(content))
	}
	return total
}

func buildCompressedMemoryEntries(sessionID string, startSequence, startCreated int64, summary string, remaining []*types.MemoryEntry) []*types.MemoryEntry {
	now := time.Now().UnixMilli()
	summaryUserEntry := &types.MemoryEntry{
		SessionID:        sessionID,
		EntryKind:        "summary_user",
		Role:             "user",
		Content:          "总结一下之前都做了什么。",
		Synthetic:        true,
		CompressionLevel: 2,
		Sequence:         startSequence,
		Created:          startCreated,
		Updated:          now,
	}
	summaryUserEntry.TokenCount = estimateMemoryEntryTokenCount(summaryUserEntry)

	syntheticEntries := []*types.MemoryEntry{
		summaryUserEntry,
		buildCompressedAssistantEntry(sessionID, startSequence+1, startCreated+1, summary),
	}

	return append(syntheticEntries, remaining...)
}

func buildCompressedAssistantEntry(sessionID string, startSequence, startCreated int64, summary string) *types.MemoryEntry {
	entry := &types.MemoryEntry{
		SessionID:        sessionID,
		EntryKind:        "summary_assistant",
		Role:             "assistant",
		Content:          strings.TrimSpace(summary),
		Synthetic:        true,
		CompressionLevel: 2,
		Sequence:         startSequence,
		Created:          startCreated,
		Updated:          time.Now().UnixMilli(),
	}
	entry.TokenCount = estimateMemoryEntryTokenCount(entry)
	return entry
}

func previewCompactionInput(input string) string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return ""
	}
	const maxPreviewBytes = 2000
	bytes := []byte(trimmed)
	if len(bytes) <= maxPreviewBytes {
		return trimmed
	}
	return string(bytes[:maxPreviewBytes]) + "\n..."
}

func formatCompactionBytes(bytes int) string {
	if bytes >= 1024*1024 {
		return fmt.Sprintf("%.1fMB", float64(bytes)/(1024*1024))
	}
	if bytes >= 1024 {
		return fmt.Sprintf("%.1fKB", float64(bytes)/1024)
	}
	return fmt.Sprintf("%dB", bytes)
}

func buildMemoryCompactionDescription(event memoryCompactionEvent) string {
	strategyLabel := "消息压缩"
	switch {
	case event.Kind == "file":
		strategyLabel = "文件压缩"
	case event.Strategy == "prompt_builder":
		strategyLabel = "记忆整理"
	case event.Strategy == "size":
		strategyLabel = "消息内容压缩"
	case event.Strategy == "count":
		strategyLabel = "消息压缩"
	}
	if strings.TrimSpace(event.Status) == "error" {
		return strategyLabel + "失败"
	}

	scopeSuffix := ""
	if event.BatchTotal > 1 {
		scopeSuffix = fmt.Sprintf(" %d/%d", event.BatchIndex, event.BatchTotal)
	}

	return fmt.Sprintf(
		"%s%s · %d条 · %s → %s",
		strategyLabel,
		scopeSuffix,
		event.CompressedCount,
		formatCompactionBytes(event.BeforeBytes),
		formatCompactionBytes(event.AfterBytes),
	)
}

func (r *AgentRunner) emitMemoryCompactionPart(runtimeConfig *RuntimeConfig, sessionID string, event memoryCompactionEvent) error {
	if runtimeConfig == nil || runtimeConfig.Assistant == nil || runtimeConfig.Assistant.ID == "" {
		return nil
	}

	now := time.Now().UnixMilli()
	metadata := buildMemoryCompactionMetadata(event)

	_, err := r.emitter.UpdatePart(&Part{
		ID:          util.Ascending("part"),
		MessageID:   runtimeConfig.Assistant.ID,
		SessionID:   sessionID,
		Type:        types.PartTypeCompaction,
		Description: buildMemoryCompactionDescription(event),
		Metadata:    metadata,
		Time:        &PartTime{Start: now, End: now},
	})
	return err
}

func buildMemoryCompactionMetadata(event memoryCompactionEvent) map[string]interface{} {
	metadata := map[string]interface{}{
		"kind":     event.Kind,
		"strategy": event.Strategy,
		"scope":    event.Scope,
		"status": func() string {
			if strings.TrimSpace(event.Status) != "" {
				return strings.TrimSpace(event.Status)
			}
			return "completed"
		}(),
		"compressedCount": event.CompressedCount,
		"beforeCount":     event.BeforeCount,
		"afterCount":      event.AfterCount,
		"originalCount":   event.OriginalCount,
		"beforeBytes":     event.BeforeBytes,
		"afterBytes":      event.AfterBytes,
		"summary":           event.Summary,
		"summaryStreaming": event.SummaryStreaming,
	}
	if event.InputPreview != "" {
		metadata["inputPreview"] = event.InputPreview
	}
	if event.ResultPreview != "" {
		metadata["resultPreview"] = event.ResultPreview
	}
	if event.RequestPrompt != "" {
		metadata["requestPrompt"] = event.RequestPrompt
	}
	if event.ResponseFinish != "" {
		metadata["responseFinish"] = event.ResponseFinish
	}
	if event.ResponseModel != "" {
		metadata["responseModel"] = event.ResponseModel
	}
	if event.Error != "" {
		metadata["error"] = event.Error
	}
	if event.BatchTotal > 0 {
		metadata["batchIndex"] = event.BatchIndex
		metadata["batchTotal"] = event.BatchTotal
	}
	return metadata
}

func (r *AgentRunner) newMemoryCompactionPart(runtimeConfig *RuntimeConfig, sessionID string, event memoryCompactionEvent) *Part {
	if runtimeConfig == nil || runtimeConfig.Assistant == nil || runtimeConfig.Assistant.ID == "" {
		return nil
	}
	now := time.Now().UnixMilli()
	part := &Part{
		ID:          util.Ascending("part"),
		MessageID:   runtimeConfig.Assistant.ID,
		SessionID:   sessionID,
		Type:        types.PartTypeCompaction,
		Description: buildMemoryCompactionDescription(event),
		Metadata:    buildMemoryCompactionMetadata(event),
		Time:        &PartTime{Start: now},
	}
	if strings.TrimSpace(event.Status) == "running" {
		part.Time.End = 0
	} else {
		part.Time.End = now
	}
	return part
}

func (r *AgentRunner) updateMemoryCompactionPart(part *Part, event memoryCompactionEvent) error {
	if part == nil {
		return nil
	}
	part.Description = buildMemoryCompactionDescription(event)
	part.Metadata = buildMemoryCompactionMetadata(event)
	if part.Time == nil {
		part.Time = &PartTime{Start: time.Now().UnixMilli()}
	}
	if strings.TrimSpace(event.Status) == "running" {
		part.Time.End = 0
	} else {
		part.Time.End = time.Now().UnixMilli()
	}
	_, err := r.emitter.UpdatePart(part)
	return err
}

func buildCompactionResultPreview(entries []*types.MemoryEntry) string {
	return previewCompactionInput(renderMemoryTranscript(entries))
}

// MemoryCompactionSamplingParams returns sampling params for memory compaction requests.
// We intentionally omit temperature/top_p for compaction so provider-specific summary
// requests stay compatible with models that reject sampling controls.
func MemoryCompactionSamplingParams() (float64, float64) {
	return 0, 0
}

func MemoryCompactionMaxOutputTokens() int {
	return defaultMemoryCompactionMaxOutputTokens
}

func taskWorkerModelRequest(runtimeConfig *RuntimeConfig) (string, float64, float64, int) {
	model := ""
	temperature, topP := MemoryCompactionSamplingParams()
	if runtimeConfig != nil {
		model = strings.TrimSpace(runtimeConfig.Model)
		if runtimeConfig.Worker != nil {
			if workerModel := strings.TrimSpace(runtimeConfig.Worker.Model); workerModel != "" {
				model = workerModel
			}
			if runtimeConfig.Worker.Temperature != nil {
				temperature = *runtimeConfig.Worker.Temperature
			}
		}
		topP = models.EffectiveTopP(runtimeConfig.ModelSettings)
	}
	return model, temperature, topP, MemoryCompactionMaxOutputTokens()
}

func memoryCompactionModelRequest(runtime *MemoryCompactionRuntime) (string, float64, float64, int) {
	model := ""
	temperature, topP := MemoryCompactionSamplingParams()
	maxOutputTokens := MemoryCompactionMaxOutputTokens()
	if runtime != nil {
		model = runtime.ModelName()
		if runtime.Worker != nil {
			if runtime.Worker.Temperature != nil {
				temperature = *runtime.Worker.Temperature
			}
		}
		topP = models.EffectiveTopP(runtime.ModelSettings)
		if runtime.ModelSettings != nil && runtime.ModelSettings.OutputLimit > 0 {
			maxOutputTokens = runtime.ModelSettings.OutputLimit
		}
	}
	return model, temperature, topP, maxOutputTokens
}

func (r *AgentRunner) summarizeMemoryWithPromptBuilder(runtimeConfig *RuntimeConfig, memory *types.Memory, userInput string, onSummaryDelta func(string)) (string, compactionPromptInfo, MemoryCompactionStreamResult, error) {
	if r == nil || r.db == nil {
		return "", compactionPromptInfo{}, MemoryCompactionStreamResult{}, fmt.Errorf("database is not configured")
	}
	compactionRuntime, err := ResolveMemoryCompactionRuntime(r.db)
	if err != nil {
		return "", compactionPromptInfo{}, MemoryCompactionStreamResult{}, err
	}
	ctx := context.Background()
	if runtimeConfig != nil && runtimeConfig.Ctx != nil {
		ctx = runtimeConfig.Ctx
	}
	var contextInfo *coreagent.ContextInfo
	if runtimeConfig != nil {
		contextInfo = buildPromptContextInfo(runtimeConfig, memory, userInput)
	}
	return RunMemoryCompactionWorkerChat(MemoryCompactionWorkerChatOptions{
		Context:        ctx,
		DB:             r.db,
		HTTPClient:     r.ensureCompactionHTTPClient(compactionRuntime),
		Memory:         memory,
		UserInput:      userInput,
		ContextInfo:    contextInfo,
		OnSummaryDelta: onSummaryDelta,
	})
}

func (r *AgentRunner) summarizeMemoryWithStream(runtimeConfig *RuntimeConfig, request llm.ChatRequest, onSummaryDelta func(string)) (MemoryCompactionStreamResult, error) {
	if runtimeConfig == nil {
		return MemoryCompactionStreamResult{}, fmt.Errorf("llm runtime is not configured")
	}
	return StreamMemoryCompactionSummary(runtimeConfig.LLMClient, request, MemoryCompactionStreamOptions{
		ModelSettings:  runtimeConfig.ModelSettings,
		HTTPClient:     r.ensureLLMHTTPClient(runtimeConfig),
		OnSummaryDelta: onSummaryDelta,
	})
}

func renderMemoryTranscript(entries []*types.MemoryEntry) string {
	if len(entries) == 0 {
		return "[]"
	}

	return (&types.Memory{Entries: entries}).PromptContent()
}

func (r *AgentRunner) logMemoryCompressionCycleStart(sessionID string, strategy string, compressedCount int, originalCount int, currentCount int, currentBytes int, inputTranscript string, beforeEntries []*types.MemoryEntry) uint {
	if r.db == nil {
		return 0
	}

	args, _ := json.Marshal([]string{
		fmt.Sprintf("session=%s", sessionID),
		fmt.Sprintf("strategy=%s", strategy),
		fmt.Sprintf("compressed_count=%d", compressedCount),
		fmt.Sprintf("original_count=%d", originalCount),
		fmt.Sprintf("current_count=%d", currentCount),
		fmt.Sprintf("current_bytes=%d", currentBytes),
	})

	logEntry := &models.CommandLog{
		Source:     "memory_compaction",
		SourceName: fmt.Sprintf("Session %s", sessionID),
		Command:    "compress_memory",
		Args:       string(args),
		WorkDir:    r.GetDirectory(),
		StdinData:  inputTranscript,
		Fields: models.MergeCommandLogFields(
			models.LegacyCommandLogFields(inputTranscript, "", "", ""),
			memoryCompactionSerializationFields(beforeEntries, nil)...,
		),
		Status:    string(models.TaskStatusRunning),
		CreatedAt: time.Now(),
	}
	if r.task != nil {
		logEntry.SourceID = &r.task.ID
	}
	if err := database.CreateCommandLog(r.db, logEntry); err != nil {
		return 0
	}
	return logEntry.ID
}

func MemoryCompactionSerializationFields(beforeEntries, afterEntries []*types.MemoryEntry) []models.CommandLogField {
	return memoryCompactionSerializationFields(beforeEntries, afterEntries)
}

func memoryCompactionSerializationFields(beforeEntries, afterEntries []*types.MemoryEntry) []models.CommandLogField {
	fields := make([]models.CommandLogField, 0, 2)
	if beforeEntries != nil {
		fields = append(fields, models.CommandLogField{
			Key:   "memoryBefore",
			Label: "压缩前 Memory",
			Value: types.SerializeMemoryEntriesJSON(beforeEntries),
			Tone:  "default",
		})
	}
	if afterEntries != nil {
		fields = append(fields, models.CommandLogField{
			Key:   "memoryAfter",
			Label: "压缩后 Memory",
			Value: types.SerializeMemoryEntriesJSON(afterEntries),
			Tone:  "default",
		})
	}
	return fields
}

func currentCompressionRate(originalCount int, currentCount int) float64 {
	if originalCount <= 0 {
		return 0
	}
	return float64(originalCount-currentCount) * 100 / float64(originalCount)
}

func (r *AgentRunner) logMemoryCompressionCycleFinish(logID uint, strategy string, beforeCount int, afterCount int, originalCount int, beforeBytes int, afterBytes int, inputTranscript string, summary string, responseFinish string, responseModel string, levelLogsJSON string, beforeEntries, afterEntries []*types.MemoryEntry, err error) {
	if r.db == nil || logID == 0 {
		return
	}
	if afterEntries == nil {
		afterEntries = beforeEntries
	}

	now := time.Now()
	currentRate := currentCompressionRate(originalCount, afterCount)
	status := "success"
	errorText := ""
	if err != nil {
		status = "failed"
		errorText = err.Error()
	}

	stdout := fmt.Sprintf(
		"压缩策略: %s\n减少条数: %d\n压缩前对话数量: %d\n压缩后对话数量: %d\n当前会话对话数量: %d\n当前压缩率: %.2f%%\n压缩前消息大小: %d bytes\n压缩后消息大小: %d bytes\n\n输入:\n%s\n\n输出:\n%s",
		strategy,
		beforeCount-afterCount,
		beforeCount,
		afterCount,
		afterCount,
		currentRate,
		beforeBytes,
		afterBytes,
		inputTranscript,
		summary,
	)

	updates := map[string]interface{}{
		"stdout":      stdout,
		"finished_at": now,
		"duration":    0,
		"status":      status,
		"error":       errorText,
	}

	existingFields := make([]models.CommandLogField, 0)
	if logEntry, logErr := database.GetCommandLogByID(r.db, logID); logErr == nil {
		updates["duration"] = now.Sub(logEntry.CreatedAt).Milliseconds()
		existingFields = append(existingFields, logEntry.Fields...)
	}
	extraFields := models.BuildCommandLogFields(
		models.NewCommandLogField("requestPrompt", "请求 Prompt", inputTranscript, "default"),
		models.NewCommandLogField("summary", "摘要结果", summary, "default"),
		models.NewCommandLogField("responseFinish", "流结束原因", responseFinish, "default"),
		models.NewCommandLogField("responseModel", "响应模型", responseModel, "default"),
		models.NewCommandLogField("levelLogs", "分级压缩日志", levelLogsJSON, "default"),
	)
	mergedFields := models.MergeCommandLogFields(
		existingFields,
		append(
			append(models.LegacyCommandLogFields("", stdout, "", errorText), extraFields...),
			memoryCompactionSerializationFields(beforeEntries, afterEntries)...,
		)...,
	)
	if encoded, encodeErr := models.EncodeCommandLogFields(mergedFields); encodeErr == nil {
		updates["fields"] = encoded
	}
	_ = database.UpdateCommandLogFields(r.db, logID, updates)
}
