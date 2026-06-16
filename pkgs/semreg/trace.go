package semreg

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
)

const TraceSchemaVersion = 1

// TraceDocument 与 explore-compare / collect_trace.py 输出对齐。
type TraceDocument struct {
	Version     int              `json:"version"`
	Project     string           `json:"project,omitempty"`
	Worker      string           `json:"worker,omitempty"`
	SessionID   string           `json:"session_id,omitempty"`
	TaskID      int              `json:"task_id,omitempty"`
	CollectedAt string           `json:"collected_at,omitempty"`
	Summary     TraceSummary     `json:"summary"`
	ToolCalls   []TraceToolCall  `json:"tool_calls,omitempty"`
	Tolerances  *TraceTolerances `json:"tolerances,omitempty"`
}

type TraceSummary struct {
	TotalToolCalls        int                       `json:"total_tool_calls"`
	ToolCounts            map[string]int            `json:"tool_counts"`
	ReadTotal             int                       `json:"read_total"`
	ReadUniquePaths       int                       `json:"read_unique_paths"`
	ReadTopPaths          [][]interface{}           `json:"read_top_paths,omitempty"`
	ReadDuplicateRanges   []TraceReadDuplicateRange `json:"read_duplicate_ranges"`
	TotalReadOutputChars  int                       `json:"total_read_output_chars"`
}

type TraceReadDuplicateRange struct {
	Path   string `json:"path"`
	Offset any    `json:"offset"`
	Limit  any    `json:"limit"`
	Count  int    `json:"count"`
}

type TraceToolCall struct {
	Tool        string         `json:"tool"`
	Status      string         `json:"status,omitempty"`
	Input       map[string]any `json:"input,omitempty"`
	OutputChars int            `json:"output_chars,omitempty"`
	TimeCreated int64          `json:"time_created,omitempty"`
}

// TraceTolerances 定义相对 baseline 允许的浮动（比例，如 0.2 表示 +20%）。
type TraceTolerances struct {
	TotalToolCalls       *float64 `json:"total_tool_calls,omitempty"`
	ReadTotal            *float64 `json:"read_total,omitempty"`
	ReadDuplicateRanges  *int     `json:"read_duplicate_ranges,omitempty"`
	TotalReadOutputChars *float64 `json:"total_read_output_chars,omitempty"`
}

// TraceCompareResult 行为回归对比结果。
type TraceCompareResult struct {
	Passed  bool
	Errors  []string
	Details map[string]string
}

// LoadTraceDocument 读取 trace JSON；旧格式无 version 时自动补 1。
func LoadTraceDocument(path string) (*TraceDocument, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc TraceDocument
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	if doc.Version == 0 {
		doc.Version = 1
	}
	return &doc, nil
}

// CompareTraceSummary 将实际 trace 摘要与 baseline 对比。
func CompareTraceSummary(actual, baseline TraceSummary, tolerances *TraceTolerances) TraceCompareResult {
	result := TraceCompareResult{
		Passed:  true,
		Details: map[string]string{},
	}
	if tolerances == nil {
		tolerances = defaultTraceTolerances()
	}

	compareIntMetric(&result, "total_tool_calls", actual.TotalToolCalls, baseline.TotalToolCalls, tolerances.TotalToolCalls, true)
	compareIntMetric(&result, "read_total", actual.ReadTotal, baseline.ReadTotal, tolerances.ReadTotal, true)
	compareIntMetric(&result, "total_read_output_chars", actual.TotalReadOutputChars, baseline.TotalReadOutputChars, tolerances.TotalReadOutputChars, true)

	actualDup := len(actual.ReadDuplicateRanges)
	baselineDup := len(baseline.ReadDuplicateRanges)
	maxDup := baselineDup
	if tolerances.ReadDuplicateRanges != nil {
		maxDup = baselineDup + *tolerances.ReadDuplicateRanges
	}
	result.Details["read_duplicate_ranges"] = fmt.Sprintf("%d (baseline %d, max %d)", actualDup, baselineDup, maxDup)
	if actualDup > maxDup {
		result.Passed = false
		result.Errors = append(result.Errors, fmt.Sprintf("read_duplicate_ranges %d exceeds max %d", actualDup, maxDup))
	}

	for tool, baselineCount := range baseline.ToolCounts {
		actualCount := actual.ToolCounts[tool]
		result.Details["tool:"+tool] = fmt.Sprintf("%d (baseline %d)", actualCount, baselineCount)
	}
	return result
}

func defaultTraceTolerances() *TraceTolerances {
	total := 0.2
	readDup := 0
	return &TraceTolerances{
		TotalToolCalls:      &total,
		ReadTotal:           &total,
		ReadDuplicateRanges: &readDup,
	}
}

func compareIntMetric(result *TraceCompareResult, name string, actual, baseline int, toleranceRatio *float64, allowIncreaseOnly bool) {
	maxAllowed := baseline
	if toleranceRatio != nil && baseline >= 0 {
		delta := int(math.Ceil(float64(baseline) * *toleranceRatio))
		if allowIncreaseOnly {
			maxAllowed = baseline + delta
		} else {
			maxAllowed = baseline + delta
		}
	}
	result.Details[name] = fmt.Sprintf("%d (baseline %d, max %d)", actual, baseline, maxAllowed)
	if actual > maxAllowed {
		result.Passed = false
		result.Errors = append(result.Errors, fmt.Sprintf("%s %d exceeds max %d", name, actual, maxAllowed))
	}
}

// NormalizeLegacyTrace 将 collect_trace.py 顶层 JSON 转为 TraceDocument。
func NormalizeLegacyTrace(raw map[string]any) (*TraceDocument, error) {
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	var doc TraceDocument
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	if doc.Version == 0 {
		doc.Version = TraceSchemaVersion
	}
	if doc.Project == "" {
		if project, ok := raw["project"].(string); ok {
			doc.Project = strings.TrimSpace(project)
		}
	}
	return &doc, nil
}

// BuildTraceSummary 从工具调用记录构建 trace 摘要（与 collect_trace.py 对齐）。
func BuildTraceSummary(calls []TraceToolCall) TraceSummary {
	toolCounts := map[string]int{}
	readByPath := map[string]int{}
	readKeys := map[string]int{}
	readTotal := 0
	totalReadChars := 0

	for _, call := range calls {
		toolCounts[call.Tool]++
		if call.Tool != "read" {
			continue
		}
		readTotal++
		totalReadChars += call.OutputChars
		path, _ := call.Input["path"].(string)
		readByPath[path]++
		offset := fmt.Sprint(call.Input["offset"])
		limit := fmt.Sprint(call.Input["limit"])
		readKeys[path+"|"+offset+"|"+limit]++
	}

	type pathCount struct {
		Path  string
		Count int
	}
	topPaths := make([]pathCount, 0, len(readByPath))
	for path, count := range readByPath {
		topPaths = append(topPaths, pathCount{Path: path, Count: count})
	}
	sort.Slice(topPaths, func(i, j int) bool {
		return topPaths[i].Count > topPaths[j].Count
	})
	if len(topPaths) > 15 {
		topPaths = topPaths[:15]
	}

	readTopPaths := make([][]interface{}, 0, len(topPaths))
	for _, item := range topPaths {
		readTopPaths = append(readTopPaths, []interface{}{item.Path, item.Count})
	}

	dupRanges := make([]TraceReadDuplicateRange, 0)
	for key, count := range readKeys {
		if count <= 1 {
			continue
		}
		parts := strings.SplitN(key, "|", 3)
		offset, limit := "", ""
		if len(parts) > 1 {
			offset = parts[1]
		}
		if len(parts) > 2 {
			limit = parts[2]
		}
		dupRanges = append(dupRanges, TraceReadDuplicateRange{
			Path:   parts[0],
			Offset: offset,
			Limit:  limit,
			Count:  count,
		})
	}

	return TraceSummary{
		TotalToolCalls:       len(calls),
		ToolCounts:           toolCounts,
		ReadTotal:            readTotal,
		ReadUniquePaths:      len(readByPath),
		ReadTopPaths:         readTopPaths,
		ReadDuplicateRanges:  dupRanges,
		TotalReadOutputChars: totalReadChars,
	}
}
