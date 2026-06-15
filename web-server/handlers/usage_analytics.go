package handlers

import (
	"encoding/json"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"matrixops-agent/types"
	"pkgs/db/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type UsageAnalyticsHandler struct {
	db *gorm.DB
}

func NewUsageAnalyticsHandler(db *gorm.DB) *UsageAnalyticsHandler {
	return &UsageAnalyticsHandler{db: db}
}

type UsageAnalyticsQuery struct {
	Start  int64  `form:"start"`
	End    int64  `form:"end"`
	Bucket string `form:"bucket"`
	TaskID uint   `form:"taskId"`
}

type UsageAnalyticsResponse struct {
	Range        UsageAnalyticsRange      `json:"range"`
	Summary      UsageAnalyticsSummary    `json:"summary"`
	Task         *UsageAnalyticsTask      `json:"task,omitempty"`
	Providers    []UsageProviderMetric    `json:"providers"`
	Models       []UsageModelMetric       `json:"models"`
	Tools        []UsageToolMetric        `json:"tools"`
	Timeline     []UsageTimelinePoint     `json:"timeline"`
	ToolTimeline []UsageToolTimelinePoint `json:"toolTimeline"`
}

type UsageAnalyticsRange struct {
	Start  int64  `json:"start"`
	End    int64  `json:"end"`
	Bucket string `json:"bucket"`
}

type UsageAnalyticsTask struct {
	ID        uint   `json:"id"`
	Name      string `json:"name"`
	Content   string `json:"content"`
	SessionID string `json:"sessionId"`
}

type UsageAnalyticsSummary struct {
	AssistantMessages int     `json:"assistantMessages"`
	LLMCalls          int     `json:"llmCalls"`
	FailedLLMCalls    int     `json:"failedLLMCalls"`
	ToolCalls         int     `json:"toolCalls"`
	UniqueTools       int     `json:"uniqueTools"`
	InputTokens       int     `json:"inputTokens"`
	OutputTokens      int     `json:"outputTokens"`
	ReasoningTokens   int     `json:"reasoningTokens"`
	CacheReadTokens   int     `json:"cacheReadTokens"`
	CacheWriteTokens  int     `json:"cacheWriteTokens"`
	CacheHitCount     int     `json:"cacheHitCount"`
	CacheHitRate      float64 `json:"cacheHitRate"`
	AvgFirstByteMs    float64 `json:"avgFirstByteMs"`
	P95FirstByteMs    float64 `json:"p95FirstByteMs"`
	AvgTokenSpeed     float64 `json:"avgTokenSpeed"`
}

type UsageProviderMetric struct {
	Provider          string  `json:"provider"`
	Calls             int     `json:"calls"`
	FailedCalls       int     `json:"failedCalls"`
	AssistantMessages int     `json:"assistantMessages"`
	InputTokens       int     `json:"inputTokens"`
	OutputTokens      int     `json:"outputTokens"`
	ReasoningTokens   int     `json:"reasoningTokens"`
	CacheReadTokens   int     `json:"cacheReadTokens"`
	CacheWriteTokens  int     `json:"cacheWriteTokens"`
	CacheHitCount     int     `json:"cacheHitCount"`
	AvgFirstByteMs    float64 `json:"avgFirstByteMs"`
	P95FirstByteMs    float64 `json:"p95FirstByteMs"`
	AvgTokenSpeed     float64 `json:"avgTokenSpeed"`
}

type UsageModelMetric struct {
	Provider        string  `json:"provider"`
	Model           string  `json:"model"`
	Calls           int     `json:"calls"`
	FailedCalls     int     `json:"failedCalls"`
	InputTokens     int     `json:"inputTokens"`
	OutputTokens    int     `json:"outputTokens"`
	CacheReadTokens int     `json:"cacheReadTokens"`
	AvgFirstByteMs  float64 `json:"avgFirstByteMs"`
	AvgTokenSpeed   float64 `json:"avgTokenSpeed"`
}

type UsageToolMetric struct {
	Tool            string  `json:"tool"`
	Calls           int     `json:"calls"`
	CompletedCalls  int     `json:"completedCalls"`
	FailedCalls     int     `json:"failedCalls"`
	AvgDurationMs   float64 `json:"avgDurationMs"`
	TotalDurationMs int64   `json:"totalDurationMs"`
}

type UsageTimelinePoint struct {
	Timestamp       int64   `json:"timestamp"`
	Label           string  `json:"label"`
	LLMCalls        int     `json:"llmCalls"`
	ToolCalls       int     `json:"toolCalls"`
	InputTokens     int     `json:"inputTokens"`
	OutputTokens    int     `json:"outputTokens"`
	CacheReadTokens int     `json:"cacheReadTokens"`
	AvgFirstByteMs  float64 `json:"avgFirstByteMs"`
	AvgTokenSpeed   float64 `json:"avgTokenSpeed"`
}

type UsageToolTimelinePoint struct {
	Timestamp int64  `json:"timestamp"`
	Label     string `json:"label"`
	ToolCalls int    `json:"toolCalls"`
}

type tokenEnvelope struct {
	Input     int `json:"input"`
	Output    int `json:"output"`
	Reasoning int `json:"reasoning"`
	Cache     struct {
		Read  int `json:"read"`
		Write int `json:"write"`
	} `json:"cache"`
}

type toolEnvelope struct {
	Tool  string `json:"tool"`
	State struct {
		Status string `json:"status"`
		Time   struct {
			Start int64 `json:"start"`
			End   int64 `json:"end"`
		} `json:"time"`
	} `json:"state"`
}

type llmCallMeta struct {
	Provider     string
	Model        string
	FirstByteMs  int64
	TotalMs      int64
	GenerationMs int64
	Status       string
	CreatedAtMs  int64
}

type providerAccumulator struct {
	UsageProviderMetric
	firstBytes   []int64
	generationMs int64
}

type modelAccumulator struct {
	UsageModelMetric
	generationMs int64
}

type toolAccumulator struct {
	UsageToolMetric
}

type timelineAccumulator struct {
	UsageTimelinePoint
	firstByteSum   float64
	firstByteCount int
	generationMs   int64
}

func (h *UsageAnalyticsHandler) GetUsageAnalytics(c *gin.Context) {
	var query UsageAnalyticsQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	end := query.End
	if end <= 0 {
		end = time.Now().UnixMilli()
	}
	start := query.Start
	if start <= 0 || start >= end {
		start = end - int64(7*24*time.Hour/time.Millisecond)
	}
	if start >= end {
		c.JSON(http.StatusBadRequest, gin.H{"error": "时间范围错误"})
		return
	}

	bucket := normalizeBucket(query.Bucket, start, end)
	bucketSize := bucketDuration(bucket)

	var taskSummary *UsageAnalyticsTask
	messageDB := h.db
	partDB := h.db
	logDB := h.db
	if query.TaskID > 0 {
		var task models.Task
		if err := h.db.Select("id", "name", "content", "session_id").First(&task, query.TaskID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "读取任务失败"})
			return
		}
		taskSummary = &UsageAnalyticsTask{
			ID:        task.ID,
			Name:      strings.TrimSpace(task.Name),
			Content:   strings.TrimSpace(task.Content),
			SessionID: strings.TrimSpace(task.SessionID),
		}
		if taskSummary.SessionID != "" {
			messageDB = messageDB.Where("session_id = ?", taskSummary.SessionID)
			partDB = partDB.Where("session_id = ?", taskSummary.SessionID)
		} else {
			messageDB = messageDB.Where("1 = 0")
			partDB = partDB.Where("1 = 0")
		}
		logDB = logDB.Where("source_id = ?", query.TaskID)
	}

	var messageRows []models.Message
	if err := messageDB.Select("role", "provider_id", "model_id", "created", "tokens").
		Where("role = ? AND created >= ? AND created <= ?", "assistant", start, end).
		Order("created ASC").
		Find(&messageRows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取消息统计失败"})
		return
	}

	var partRows []models.Part
	if err := partDB.Select("type", "time_created", "time_start", "time_end", "tool").
		Where("type = ? AND time_created >= ? AND time_created <= ?", types.PartTypeTool, start, end).
		Order("time_created ASC").
		Find(&partRows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取工具统计失败"})
		return
	}

	var logRows []models.CommandLog
	startTime := time.UnixMilli(start)
	endTime := time.UnixMilli(end)
	if err := logDB.Select("source", "command", "args", "duration", "status", "created_at").
		Where("source = ? AND command IN ? AND created_at >= ? AND created_at <= ?", "llm_api_call", []string{"stream_chat_response", "stream_chat"}, startTime, endTime).
		Order("created_at ASC").
		Find(&logRows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取调用统计失败"})
		return
	}

	resp := buildUsageAnalyticsResponse(start, end, bucket, bucketSize, taskSummary, messageRows, partRows, logRows)
	c.JSON(http.StatusOK, resp)
}

func buildUsageAnalyticsResponse(start int64, end int64, bucket string, bucketSize time.Duration, task *UsageAnalyticsTask, messages []models.Message, parts []models.Part, logs []models.CommandLog) UsageAnalyticsResponse {
	summary := UsageAnalyticsSummary{}
	providerMap := make(map[string]*providerAccumulator)
	modelMap := make(map[string]*modelAccumulator)
	toolMap := make(map[string]*toolAccumulator)
	timelineMap := make(map[int64]*timelineAccumulator)
	toolTimelineMap := make(map[int64]*UsageToolTimelinePoint)

	getTimeline := func(ts int64) *timelineAccumulator {
		bucketStart := alignBucket(ts, bucketSize)
		point := timelineMap[bucketStart]
		if point == nil {
			point = &timelineAccumulator{
				UsageTimelinePoint: UsageTimelinePoint{
					Timestamp: bucketStart,
					Label:     formatBucketLabel(bucketStart, bucket),
				},
			}
			timelineMap[bucketStart] = point
		}
		return point
	}

	getToolTimeline := func(ts int64) *UsageToolTimelinePoint {
		bucketStart := alignBucket(ts, bucketSize)
		point := toolTimelineMap[bucketStart]
		if point == nil {
			point = &UsageToolTimelinePoint{
				Timestamp: bucketStart,
				Label:     formatBucketLabel(bucketStart, bucket),
			}
			toolTimelineMap[bucketStart] = point
		}
		return point
	}

	for _, row := range messages {
		tokens := parseTokens(row.Tokens.Data)
		provider := normalizeLabel(row.ProviderID, "unknown")
		model := normalizeLabel(row.ModelID, "unknown")

		summary.AssistantMessages++
		summary.InputTokens += tokens.Input
		summary.OutputTokens += tokens.Output
		summary.ReasoningTokens += tokens.Reasoning
		summary.CacheReadTokens += tokens.Cache.Read
		summary.CacheWriteTokens += tokens.Cache.Write
		if tokens.Cache.Read > 0 {
			summary.CacheHitCount++
		}

		providerEntry := ensureProvider(providerMap, provider)
		providerEntry.AssistantMessages++
		providerEntry.InputTokens += tokens.Input
		providerEntry.OutputTokens += tokens.Output
		providerEntry.ReasoningTokens += tokens.Reasoning
		providerEntry.CacheReadTokens += tokens.Cache.Read
		providerEntry.CacheWriteTokens += tokens.Cache.Write
		if tokens.Cache.Read > 0 {
			providerEntry.CacheHitCount++
		}

		modelEntry := ensureModel(modelMap, provider, model)
		modelEntry.InputTokens += tokens.Input
		modelEntry.OutputTokens += tokens.Output
		modelEntry.CacheReadTokens += tokens.Cache.Read

		timeline := getTimeline(row.Created)
		timeline.InputTokens += tokens.Input
		timeline.OutputTokens += tokens.Output
		timeline.CacheReadTokens += tokens.Cache.Read
	}

	for _, row := range parts {
		tool := parseToolName(row.Tool.Data)
		if tool == "" {
			tool = "unknown"
		}
		status := parseToolStatus(row.Tool.Data)
		durationMs := calcToolDurationMs(row)

		summary.ToolCalls++
		entry := ensureTool(toolMap, tool)
		entry.Calls++
		entry.TotalDurationMs += durationMs
		if durationMs > 0 {
			entry.AvgDurationMs = float64(entry.TotalDurationMs) / float64(entry.Calls)
		}
		if status == "completed" || status == "success" {
			entry.CompletedCalls++
		}
		if status == "failed" || status == "error" {
			entry.FailedCalls++
		}

		timestamp := row.TimeCreated
		if timestamp <= 0 {
			timestamp = row.TimeStart
		}
		if timestamp <= 0 {
			timestamp = row.TimeEnd
		}
		getTimeline(timestamp).ToolCalls++
		getToolTimeline(timestamp).ToolCalls++
	}
	summary.UniqueTools = len(toolMap)

	for _, row := range logs {
		meta := parseLLMCallMeta(row)
		if meta.Provider == "" {
			meta.Provider = "unknown"
		}
		if meta.Model == "" {
			meta.Model = "unknown"
		}

		summary.LLMCalls++
		if meta.Status == "failed" {
			summary.FailedLLMCalls++
		}
		if meta.FirstByteMs > 0 {
			summary.AvgFirstByteMs += float64(meta.FirstByteMs)
		}

		providerEntry := ensureProvider(providerMap, meta.Provider)
		providerEntry.Calls++
		if meta.Status == "failed" {
			providerEntry.FailedCalls++
		}
		if meta.FirstByteMs > 0 {
			providerEntry.firstBytes = append(providerEntry.firstBytes, meta.FirstByteMs)
		}
		providerEntry.generationMs += meta.GenerationMs

		modelEntry := ensureModel(modelMap, meta.Provider, meta.Model)
		modelEntry.Calls++
		if meta.Status == "failed" {
			modelEntry.FailedCalls++
		}
		modelEntry.generationMs += meta.GenerationMs
		if meta.FirstByteMs > 0 {
			modelEntry.AvgFirstByteMs += float64(meta.FirstByteMs)
		}

		timeline := getTimeline(meta.CreatedAtMs)
		timeline.LLMCalls++
		if meta.FirstByteMs > 0 {
			timeline.firstByteSum += float64(meta.FirstByteMs)
			timeline.firstByteCount++
		}
		timeline.generationMs += meta.GenerationMs
	}

	if summary.AssistantMessages > 0 {
		summary.CacheHitRate = float64(summary.CacheHitCount) / float64(summary.AssistantMessages)
	}

	allFirstBytes := make([]int64, 0)
	if summary.LLMCalls > 0 {
		firstByteSum := 0.0
		for _, entry := range providerMap {
			if len(entry.firstBytes) > 0 {
				for _, value := range entry.firstBytes {
					allFirstBytes = append(allFirstBytes, value)
					firstByteSum += float64(value)
				}
				entry.AvgFirstByteMs = firstByteSumProvider(entry.firstBytes)
				entry.P95FirstByteMs = percentile(entry.firstBytes, 0.95)
			}
			entry.AvgTokenSpeed = safeTokenSpeed(entry.OutputTokens, entry.generationMs)
		}
		if len(allFirstBytes) > 0 {
			summary.AvgFirstByteMs = firstByteSum / float64(len(allFirstBytes))
			summary.P95FirstByteMs = percentile(allFirstBytes, 0.95)
		}
		summary.AvgTokenSpeed = aggregateTokenSpeed(providerMap)
	}

	providers := make([]UsageProviderMetric, 0, len(providerMap))
	for _, entry := range providerMap {
		providers = append(providers, entry.UsageProviderMetric)
	}
	sort.Slice(providers, func(i, j int) bool {
		if providers[i].Calls == providers[j].Calls {
			return providers[i].OutputTokens > providers[j].OutputTokens
		}
		return providers[i].Calls > providers[j].Calls
	})

	modelsList := make([]UsageModelMetric, 0, len(modelMap))
	for _, entry := range modelMap {
		if entry.Calls > 0 {
			entry.AvgFirstByteMs = entry.AvgFirstByteMs / float64(entry.Calls)
		}
		entry.AvgTokenSpeed = safeTokenSpeed(entry.OutputTokens, entry.generationMs)
		modelsList = append(modelsList, entry.UsageModelMetric)
	}
	sort.Slice(modelsList, func(i, j int) bool {
		if modelsList[i].OutputTokens == modelsList[j].OutputTokens {
			return modelsList[i].Calls > modelsList[j].Calls
		}
		return modelsList[i].OutputTokens > modelsList[j].OutputTokens
	})

	tools := make([]UsageToolMetric, 0, len(toolMap))
	for _, entry := range toolMap {
		if entry.Calls > 0 {
			entry.AvgDurationMs = float64(entry.TotalDurationMs) / float64(entry.Calls)
		}
		tools = append(tools, entry.UsageToolMetric)
	}
	sort.Slice(tools, func(i, j int) bool {
		if tools[i].Calls == tools[j].Calls {
			return tools[i].TotalDurationMs > tools[j].TotalDurationMs
		}
		return tools[i].Calls > tools[j].Calls
	})

	timeline := make([]UsageTimelinePoint, 0, len(timelineMap))
	for ts := alignBucket(start, bucketSize); ts <= end; ts += bucketSize.Milliseconds() {
		point := timelineMap[ts]
		if point == nil {
			point = &timelineAccumulator{
				UsageTimelinePoint: UsageTimelinePoint{
					Timestamp: ts,
					Label:     formatBucketLabel(ts, bucket),
				},
			}
		}
		if point.firstByteCount > 0 {
			point.AvgFirstByteMs = point.firstByteSum / float64(point.firstByteCount)
		}
		point.AvgTokenSpeed = safeTokenSpeed(point.OutputTokens, point.generationMs)
		timeline = append(timeline, point.UsageTimelinePoint)
	}

	toolTimeline := make([]UsageToolTimelinePoint, 0, len(toolTimelineMap))
	for ts := alignBucket(start, bucketSize); ts <= end; ts += bucketSize.Milliseconds() {
		point := toolTimelineMap[ts]
		if point == nil {
			point = &UsageToolTimelinePoint{
				Timestamp: ts,
				Label:     formatBucketLabel(ts, bucket),
			}
		}
		toolTimeline = append(toolTimeline, *point)
	}

	return UsageAnalyticsResponse{
		Range: UsageAnalyticsRange{
			Start:  start,
			End:    end,
			Bucket: bucket,
		},
		Task:         task,
		Summary:      summary,
		Providers:    providers,
		Models:       modelsList,
		Tools:        tools,
		Timeline:     timeline,
		ToolTimeline: toolTimeline,
	}
}

func normalizeBucket(bucket string, start int64, end int64) string {
	switch strings.TrimSpace(bucket) {
	case "hour", "day":
		return strings.TrimSpace(bucket)
	}
	if end-start <= int64(48*time.Hour/time.Millisecond) {
		return "hour"
	}
	return "day"
}

func bucketDuration(bucket string) time.Duration {
	if bucket == "hour" {
		return time.Hour
	}
	return 24 * time.Hour
}

func alignBucket(timestampMs int64, duration time.Duration) int64 {
	step := duration.Milliseconds()
	if step <= 0 {
		return timestampMs
	}
	return (timestampMs / step) * step
}

func formatBucketLabel(timestampMs int64, bucket string) string {
	t := time.UnixMilli(timestampMs)
	if bucket == "hour" {
		return t.Format("01-02 15:00")
	}
	return t.Format("01-02")
}

func parseTokens(data interface{}) tokenEnvelope {
	var tokens tokenEnvelope
	if data == nil {
		return tokens
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return tokens
	}
	_ = json.Unmarshal(raw, &tokens)
	return tokens
}

func parseToolName(data interface{}) string {
	var tool toolEnvelope
	raw, err := json.Marshal(data)
	if err != nil {
		return ""
	}
	_ = json.Unmarshal(raw, &tool)
	return strings.TrimSpace(tool.Tool)
}

func parseToolStatus(data interface{}) string {
	var tool toolEnvelope
	raw, err := json.Marshal(data)
	if err != nil {
		return ""
	}
	_ = json.Unmarshal(raw, &tool)
	return strings.TrimSpace(tool.State.Status)
}

func calcToolDurationMs(row models.Part) int64 {
	start := row.TimeStart
	end := row.TimeEnd
	if start > 0 && end >= start {
		return end - start
	}
	var tool toolEnvelope
	raw, err := json.Marshal(row.Tool.Data)
	if err != nil {
		return 0
	}
	if err := json.Unmarshal(raw, &tool); err != nil {
		return 0
	}
	if tool.State.Time.Start > 0 && tool.State.Time.End >= tool.State.Time.Start {
		return tool.State.Time.End - tool.State.Time.Start
	}
	return 0
}

func parseLLMCallMeta(row models.CommandLog) llmCallMeta {
	meta := llmCallMeta{
		Status:      strings.TrimSpace(row.Status),
		CreatedAtMs: row.CreatedAt.UnixMilli(),
		TotalMs:     row.Duration,
	}
	args := parseArgs(row.Args)
	for _, arg := range args {
		key, value, ok := strings.Cut(arg, "=")
		if !ok {
			continue
		}
		switch key {
		case "provider":
			meta.Provider = strings.TrimSpace(value)
		case "model":
			meta.Model = strings.TrimSpace(value)
		case "first_byte_ms":
			meta.FirstByteMs = parseInt64(value)
		case "total_ms":
			meta.TotalMs = parseInt64(value)
		}
	}
	if meta.TotalMs < row.Duration {
		meta.TotalMs = row.Duration
	}
	meta.GenerationMs = meta.TotalMs - meta.FirstByteMs
	if meta.GenerationMs <= 0 {
		meta.GenerationMs = meta.TotalMs
	}
	return meta
}

func parseArgs(argsJSON string) []string {
	if strings.TrimSpace(argsJSON) == "" {
		return nil
	}
	var args []string
	if err := json.Unmarshal([]byte(argsJSON), &args); err == nil {
		return args
	}
	return nil
}

func parseInt64(value string) int64 {
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return 0
	}
	return parsed
}

func ensureProvider(items map[string]*providerAccumulator, provider string) *providerAccumulator {
	entry := items[provider]
	if entry == nil {
		entry = &providerAccumulator{
			UsageProviderMetric: UsageProviderMetric{Provider: provider},
		}
		items[provider] = entry
	}
	return entry
}

func ensureModel(items map[string]*modelAccumulator, provider string, model string) *modelAccumulator {
	key := provider + "::" + model
	entry := items[key]
	if entry == nil {
		entry = &modelAccumulator{
			UsageModelMetric: UsageModelMetric{
				Provider: provider,
				Model:    model,
			},
		}
		items[key] = entry
	}
	return entry
}

func ensureTool(items map[string]*toolAccumulator, tool string) *toolAccumulator {
	entry := items[tool]
	if entry == nil {
		entry = &toolAccumulator{
			UsageToolMetric: UsageToolMetric{Tool: tool},
		}
		items[tool] = entry
	}
	return entry
}

func normalizeLabel(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func firstByteSumProvider(values []int64) float64 {
	if len(values) == 0 {
		return 0
	}
	total := 0.0
	for _, value := range values {
		total += float64(value)
	}
	return total / float64(len(values))
}

func percentile(values []int64, p float64) float64 {
	if len(values) == 0 {
		return 0
	}
	if p <= 0 {
		return float64(values[0])
	}
	sorted := append([]int64(nil), values...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	index := int(math.Ceil(p*float64(len(sorted)))) - 1
	if index < 0 {
		index = 0
	}
	if index >= len(sorted) {
		index = len(sorted) - 1
	}
	return float64(sorted[index])
}

func safeTokenSpeed(outputTokens int, generationMs int64) float64 {
	if outputTokens <= 0 || generationMs <= 0 {
		return 0
	}
	return float64(outputTokens) / (float64(generationMs) / 1000)
}

func aggregateTokenSpeed(items map[string]*providerAccumulator) float64 {
	totalOutput := 0
	totalDuration := int64(0)
	for _, entry := range items {
		totalOutput += entry.OutputTokens
		totalDuration += entry.generationMs
	}
	return safeTokenSpeed(totalOutput, totalDuration)
}
