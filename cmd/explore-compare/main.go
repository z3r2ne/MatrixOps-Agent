package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	agenttypes "matrixops-agent/types"
	builtinworkers "matrixops.local/core_agent/workersv2/builtin"
	builtinskills "matrixops-agent/skills/builtin"
	taskr "matrixops/services/task_runner"
	wstypes "matrixops/types"
	database "pkgs/db"
	"pkgs/db/models"
	"pkgs/llmheaders"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type traceHub struct {
	mu    sync.Mutex
	tools []toolCallRecord
}

type toolCallRecord struct {
	Tool        string                 `json:"tool"`
	Status      string                 `json:"status"`
	Input       map[string]interface{} `json:"input"`
	OutputChars int                    `json:"output_chars"`
	PartID      string                 `json:"part_id"`
}

func toolInputMap(input interface{}) map[string]interface{} {
	switch v := input.(type) {
	case map[string]interface{}:
		return v
	case nil:
		return map[string]interface{}{}
	default:
		return map[string]interface{}{"raw": v}
	}
}

func (h *traceHub) recordPart(part *agenttypes.Part) {
	if part == nil || part.Type != agenttypes.PartTypeTool || part.Tool == nil {
		return
	}
	name := strings.TrimSpace(part.Tool.Name)
	if name == "" {
		return
	}
	input := toolInputMap(part.Tool.State.Input)
	h.mu.Lock()
	defer h.mu.Unlock()
	for i := range h.tools {
		if h.tools[i].PartID == part.ID {
			h.tools[i].Status = strings.TrimSpace(part.Tool.State.Status)
			h.tools[i].OutputChars = len(strings.TrimSpace(part.Tool.State.Output))
			if len(input) > 0 {
				h.tools[i].Input = input
			}
			return
		}
	}
	h.tools = append(h.tools, toolCallRecord{
		Tool:        name,
		Status:      strings.TrimSpace(part.Tool.State.Status),
		Input:       input,
		OutputChars: len(strings.TrimSpace(part.Tool.State.Output)),
		PartID:      part.ID,
	})
}

func (h *traceHub) BroadcastToTask(_ uint, msg wstypes.WSOutgoingMessage) {
	if msg.Type != wstypes.WSTypeMessageV2 {
		return
	}
	switch value := msg.Data.(type) {
	case *agenttypes.WithParts:
		for _, part := range value.Parts {
			h.recordPart(part)
		}
	case agenttypes.WithParts:
		for _, part := range value.Parts {
			h.recordPart(part)
		}
	}
}

func (h *traceHub) BroadcastTaskMessage(_ uint, _ *models.TaskMessage) {}
func (h *traceHub) BroadcastNormalizedEntry(_ uint, _ *models.NormalizedEntry) {}
func (h *traceHub) BroadcastTaskStatus(_ uint, _ models.TaskStatus, _ string, _ string) {}
func (h *traceHub) BroadcastIsWorking(_ uint)                                       {}
func (h *traceHub) BroadcastIsNotWorking(_ uint)                                    {}
func (h *traceHub) BroadcastStartLoading(_ uint)                                    {}
func (h *traceHub) BroadcastEndLoading(_ uint)                                      {}
func (h *traceHub) BroadcastError(_ uint, _ string)                                   {}
func (h *traceHub) BroadcastSessionTitle(_ uint, _ string)                          {}
func (h *traceHub) BroadcastRetry(_ uint)                                           {}
func (h *traceHub) BroadcastTaskQueue(_ uint, _ []models.TaskMessageQueueItem)      {}
func (h *traceHub) BroadcastTaskPlan(_ uint, _ any)                                   {}
func (h *traceHub) BroadcastWaitUserInput(_ uint, _ string, ack func(map[string]interface{}), _ map[string]interface{}) {
	if ack != nil {
		ack(map[string]interface{}{"decision": "allow"})
	}
}

func (h *traceHub) snapshot() []toolCallRecord {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]toolCallRecord, len(h.tools))
	copy(out, h.tools)
	return out
}

func openDB() (*gorm.DB, error) {
	dbPath, err := database.DBPath()
	if err != nil {
		return nil, err
	}
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, err
	}
	if err := database.InitDB(db, builtinworkers.ReadAll(), builtinskills.ReadAll()); err != nil {
		return nil, err
	}
	llmheaders.InitFromDatabase(db)
	return db, nil
}

func readPrompt(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(raw)), nil
}

func summarize(calls []toolCallRecord) map[string]interface{} {
	toolCounts := map[string]int{}
	readTotal := 0
	readByPath := map[string]int{}
	readKeys := map[string]int{}
	totalReadChars := 0

	for _, c := range calls {
		toolCounts[c.Tool]++
		if c.Tool != "read" {
			continue
		}
		readTotal++
		totalReadChars += c.OutputChars
		path, _ := c.Input["path"].(string)
		readByPath[path]++
		offset := fmt.Sprint(c.Input["offset"])
		limit := fmt.Sprint(c.Input["limit"])
		readKeys[path+"|"+offset+"|"+limit]++
	}

	type pathCount struct {
		Path  string `json:"path"`
		Count int    `json:"count"`
	}
	topPaths := make([]pathCount, 0, len(readByPath))
	for path, count := range readByPath {
		topPaths = append(topPaths, pathCount{Path: path, Count: count})
	}
	for i := 0; i < len(topPaths); i++ {
		for j := i + 1; j < len(topPaths); j++ {
			if topPaths[j].Count > topPaths[i].Count {
				topPaths[i], topPaths[j] = topPaths[j], topPaths[i]
			}
		}
	}
	if len(topPaths) > 15 {
		topPaths = topPaths[:15]
	}

	dupRanges := make([]map[string]interface{}, 0)
	for key, count := range readKeys {
		if count <= 1 {
			continue
		}
		parts := strings.SplitN(key, "|", 3)
		dupRanges = append(dupRanges, map[string]interface{}{
			"path": parts[0], "offset": parts[1], "limit": parts[2], "count": count,
		})
	}

	return map[string]interface{}{
		"total_tool_calls":      len(calls),
		"tool_counts":             toolCounts,
		"read_total":              readTotal,
		"read_unique_paths":       len(readByPath),
		"read_top_paths":          topPaths,
		"read_duplicate_ranges":   dupRanges,
		"total_read_output_chars": totalReadChars,
	}
}

func main() {
	log.SetOutput(io.Discard)

	workDir := envOr("WORK_DIR", "/Users/patrick/Code/matrixops")
	workspaceID := envOr("WORKSPACE_ID", "7")
	projectID := envOr("PROJECT_ID", "8")
	promptFile := envOr("PROMPT_FILE", filepath.Join("tests", "explore_comparison", "prompt.txt"))
	output := envOr("OUTPUT", filepath.Join("tests", "explore_comparison", "output", fmt.Sprintf("matrixops_trace_%s.json", time.Now().Format("20060102_150405"))))

	prompt, err := readPrompt(promptFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "读取 prompt 失败: %v\n", err)
		os.Exit(1)
	}

	db, err := openDB()
	if err != nil {
		fmt.Fprintf(os.Stderr, "打开数据库失败: %v\n", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	hub := &traceHub{}
	fmt.Fprintf(os.Stderr, "workdir=%s workspace=%s project=%s\n", workDir, workspaceID, projectID)

	task, err := taskr.CreateAndRunTask(
		taskr.WithDB(db),
		taskr.WithWSHub(hub),
		taskr.WithCtx(ctx),
		taskr.WithWorkspaceID(workspaceID),
		taskr.WithProjectID(projectID),
		taskr.WithWorkDir(workDir),
		taskr.WithContent(prompt),
		taskr.WithToWorker("explore"),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "CreateAndRunTask: %v\n", err)
		os.Exit(1)
	}

	if err := taskr.WaitTask(task.ID); err != nil {
		fmt.Fprintf(os.Stderr, "WaitTask: %v\n", err)
	}

	refreshed, _ := database.GetTaskByID(db, task.ID)
	calls := hub.snapshot()
	trace := map[string]interface{}{
		"project":     "matrixops",
		"worker":      "explore",
		"task_id":     task.ID,
		"session_id":  strings.TrimSpace(task.SessionID),
		"work_dir":    workDir,
		"status":      "",
		"collected_at":  time.Now().UTC().Format(time.RFC3339),
		"summary":     summarize(calls),
		"tool_calls":  calls,
	}
	if refreshed != nil {
		trace["status"] = refreshed.Status
		trace["session_id"] = refreshed.SessionID
	}

	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "创建输出目录失败: %v\n", err)
		os.Exit(1)
	}
	encoded, _ := json.MarshalIndent(trace, "", "  ")
	if err := os.WriteFile(output, encoded, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "写入 trace 失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "task_id=%d session_id=%s trace=%s tool_calls=%d\n", task.ID, trace["session_id"], output, len(calls))
	if refreshed != nil && models.TaskStatus(refreshed.Status) == models.TaskStatusFailed {
		os.Exit(1)
	}
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
