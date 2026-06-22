package semreg_runner

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	agentmemory "matrixops.local/memory"
	agentprovider "matrixops-agent/provider"
	llm "matrixops-agent/llm"
	coreagent "matrixops.local/core_agent"
	builtinworkers "matrixops.local/core_agent/workersv2/builtin"
	builtinskills "matrixops-agent/skills/builtin"
	taskrunner "web-server/services/task_runner"
	"pkgs/semreg"
	"pkgs/testrunner"

	database "pkgs/db"
	"pkgs/db/models"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Runner struct {
	scenariosDir string
	baselinesDir string
	wsHub        taskrunner.WSHub
}

func NewRunner(wsHub taskrunner.WSHub) (*Runner, error) {
	sd, bd, err := resolveSemanticRegressionDirs()
	if err != nil {
		return nil, err
	}
	return &Runner{scenariosDir: sd, baselinesDir: bd, wsHub: wsHub}, nil
}

func (r *Runner) ScenariosDir() string { return r.scenariosDir }
func (r *Runner) BaselinesDir() string { return r.baselinesDir }

func (r *Runner) ListScenarios() ([]ScenarioInfo, error) {
	scenarios, err := semreg.LoadScenariosDir(r.scenariosDir)
	if err != nil {
		return nil, err
	}
	out := make([]ScenarioInfo, 0, len(scenarios))
	for _, scenario := range scenarios {
		info := ScenarioInfo{
			ID:          scenario.ID,
			Name:        scenario.Name,
			Description: scenario.Description,
			Tier:        string(scenario.Tier),
			Kind:        string(scenario.Kind),
			RequiresLLM: scenario.Tier == semreg.TierL1 || scenario.Tier == semreg.TierL2,
		}
		if scenario.Behavior != nil && strings.TrimSpace(scenario.Behavior.BaselineFile) != "" {
			baselinePath := resolveScenarioPath(r.baselinesDir, scenario.Behavior.BaselineFile)
			if _, statErr := os.Stat(baselinePath); statErr == nil {
				info.HasBaseline = true
			}
		}
		out = append(out, info)
	}
	return out, nil
}

func (r *Runner) EnvironmentStatus(cfg RunConfig) EnvironmentStatus {
	status := EnvironmentStatus{
		ScenariosDir: r.scenariosDir,
		BaselinesDir: r.baselinesDir,
		WorkDir:      strings.TrimSpace(cfg.WorkDir),
		WorkspaceID:  cfg.WorkspaceID,
		ProjectID:    strings.TrimSpace(cfg.ProjectID),
	}
	if status.WorkDir == "" {
		status.WorkDir = strings.TrimSpace(os.Getenv("SEMREG_WORK_DIR"))
	}
	if status.WorkspaceID == 0 {
		if id, ok := envUint("SEMREG_WORKSPACE_ID"); ok {
			status.WorkspaceID = id
		}
	}
	if status.ProjectID == "" {
		status.ProjectID = strings.TrimSpace(os.Getenv("SEMREG_PROJECT_ID"))
	}
	status.L1L2Ready = status.WorkDir != "" && status.WorkspaceID > 0 && status.ProjectID != ""
	if !status.L1L2Ready {
		status.Message = "L1/L2 需要配置工作目录、工作区 ID 与项目 ID"
	}
	return status
}

func (r *Runner) RunAll(ctx context.Context, db *gorm.DB, cfg RunConfig) []ScenarioResult {
	scenarios, err := semreg.LoadScenariosDir(r.scenariosDir)
	if err != nil {
		return []ScenarioResult{{
			ScenarioID: "load",
			Name:       "加载场景",
			Status:     "error",
			Errors:     []string{err.Error()},
		}}
	}

	selected := map[string]struct{}{}
	for _, id := range cfg.ScenarioIDs {
		id = strings.TrimSpace(id)
		if id != "" {
			selected[id] = struct{}{}
		}
	}
	tierFilter := map[string]struct{}{}
	for _, tier := range cfg.Tiers {
		tier = strings.TrimSpace(tier)
		if tier != "" {
			tierFilter[tier] = struct{}{}
		}
	}

	env := r.EnvironmentStatus(cfg)
	results := make([]ScenarioResult, 0, len(scenarios))
	for _, scenario := range scenarios {
		if len(selected) > 0 {
			if _, ok := selected[scenario.ID]; !ok {
				continue
			}
		}
		if len(tierFilter) > 0 {
			if _, ok := tierFilter[string(scenario.Tier)]; !ok {
				continue
			}
		}

		if ctx.Err() != nil {
			results = append(results, ScenarioResult{
				ScenarioID: scenario.ID,
				Name:       scenario.Name,
				Tier:       string(scenario.Tier),
				Kind:       string(scenario.Kind),
				Status:     "skipped",
				Errors:     []string{"运行已取消"},
			})
			continue
		}

		if (scenario.Tier == semreg.TierL1 || scenario.Tier == semreg.TierL2) && !env.L1L2Ready {
			results = append(results, ScenarioResult{
				ScenarioID: scenario.ID,
				Name:       scenario.Name,
				Tier:       string(scenario.Tier),
				Kind:       string(scenario.Kind),
				Status:     "skipped",
				Errors:     []string{env.Message},
			})
			continue
		}

		results = append(results, r.runScenario(ctx, db, scenario, cfg))
	}
	return results
}

func (r *Runner) RunScenario(ctx context.Context, db *gorm.DB, scenario semreg.Scenario, cfg RunConfig) ScenarioResult {
	return r.runScenario(ctx, db, scenario, cfg)
}

func (r *Runner) runScenario(ctx context.Context, db *gorm.DB, scenario semreg.Scenario, cfg RunConfig) ScenarioResult {
	started := time.Now()
	result := ScenarioResult{
		ScenarioID: scenario.ID,
		Name:       scenario.Name,
		Tier:       string(scenario.Tier),
		Kind:       string(scenario.Kind),
		Status:     "running",
	}

	var runErr error
	switch scenario.Kind {
	case semreg.KindPromptRender:
		runErr = r.runPromptRender(scenario, &result)
	case semreg.KindTaskRunner:
		runErr = r.runTaskRunner(ctx, scenario, &result)
	case semreg.KindBehavior:
		runErr = r.runBehavior(ctx, db, scenario, cfg, &result)
	case semreg.KindSemantic:
		runErr = r.runSemantic(ctx, db, scenario, cfg, &result)
	default:
		runErr = fmt.Errorf("unsupported kind %q", scenario.Kind)
	}

	result.DurationMs = time.Since(started).Milliseconds()
	if runErr != nil {
		result.Status = "failed"
		result.Errors = append(result.Errors, runErr.Error())
	} else if result.Status == "running" {
		result.Status = "passed"
	}
	return result
}

func (r *Runner) runPromptRender(scenario semreg.Scenario, result *ScenarioResult) error {
	spec := scenario.PromptRender
	if spec == nil {
		return errors.New("prompt_render spec is nil")
	}
	entries := make([]*agentmemory.MemoryEntry, 0, len(spec.History))
	for i, item := range spec.History {
		entries = append(entries, &agentmemory.MemoryEntry{
			Role:     item.Role,
			Content:  item.Content,
			Sequence: int64(i + 1),
		})
	}
	tools := make([]coreagent.ToolDefinition, 0, len(spec.ToolNames))
	for _, name := range spec.ToolNames {
		tools = append(tools, coreagent.ToolDefinition{Name: name, Description: name})
	}
	promptText, err := coreagent.RenderV2TaskPrompt(coreagent.V2TaskPromptData{
		Memory: &agentmemory.Memory{
			GlobalPrompt: spec.GlobalPrompt,
			Entries:      entries,
		},
		Tools:     tools,
		UserInput: spec.UserInput,
		ContextInfo: &coreagent.ContextInfo{
			LimitTokens:   spec.ContextLimit,
			CurrentTokens: spec.ContextCurrentTokens,
			CurrentBytes:  spec.ContextCurrentBytes,
		},
	})
	if err != nil {
		return err
	}
	assertResult := semreg.EvaluateStructAssertions(promptText, spec.UserInput, scenario.Assert)
	result.Details = map[string]interface{}{
		"systemPromptLen": len(promptText),
		"assertions":      assertResult.Details,
	}
	if !assertResult.Passed {
		result.Errors = append(result.Errors, assertResult.Errors...)
		return fmt.Errorf("assertions failed")
	}
	return nil
}

func (r *Runner) runTaskRunner(ctx context.Context, scenario semreg.Scenario, result *ScenarioResult) error {
	spec := scenario.TaskRunner
	if spec == nil {
		return errors.New("task_runner spec is nil")
	}

	tempDir, err := os.MkdirTemp("", "semreg-l0-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "semreg.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return err
	}
	if err := database.InitDB(db, builtinworkers.ReadAll(), builtinskills.ReadAll()); err != nil {
		return err
	}

	projectPath := filepath.Join(tempDir, "project")
	if err := os.MkdirAll(projectPath, 0o755); err != nil {
		return err
	}
	project := &models.Project{
		Name:         "semreg-project",
		Path:         projectPath,
		WorktreePath: projectPath,
		Status:       "active",
	}
	if err := database.CreateProject(db, project); err != nil {
		return err
	}
	workspace := &models.Workspace{
		Name:       "semreg-workspace",
		Path:       projectPath,
		ProjectIDs: []uint{project.ID},
	}
	if err := database.CreateWorkspace(db, workspace); err != nil {
		return err
	}
	llmConfig := &models.LLMConfig{
		Name:   "mock",
		Type:   string(models.LLMAPITypeChat),
		APIKey: "test-key",
		Model:  "mock-model",
	}
	if err := database.CreateLLMConfig(db, llmConfig); err != nil {
		return err
	}
	modelSettings := &models.ModelSettings{
		Name:         "semreg_model",
		ContextLimit: 128000,
		OutputLimit:  8000,
	}
	if err := database.CreateModelSettings(db, modelSettings); err != nil {
		return err
	}
	temp := 0.7
	workerName := strings.TrimSpace(spec.WorkerName)
	if workerName == "" {
		workerName = "semreg-worker"
	}
	worker := &models.Worker{
		Name:              workerName,
		Provider:          "mock",
		Model:             "mock-model",
		Description:       "semantic regression worker",
		Occupation:        "analyst",
		Temperature:       &temp,
		SystemPrompt:      "worker prompt",
		LLMConfigID:       &llmConfig.ID,
		ModelSettingsName: modelSettings.Name,
	}
	if err := database.CreateWorker(db, worker); err != nil {
		return err
	}

	inputText := strings.TrimSpace(spec.TaskInput)
	outputText := uuid.New().String()
	mockClient := agentprovider.NewMockClientWithStreamCallback(func(request llm.ChatRequest) (<-chan llm.StreamEvent, error) {
		return agentprovider.MockAnswerActionStream(outputText), nil
	})
	collector := NewTraceCollector()

	task, err := taskrunner.CreateAndRunTask(
		taskrunner.WithDB(db),
		taskrunner.WithWSHub(collector),
		taskrunner.WithCtx(ctx),
		taskrunner.WithWorkspaceID(strconv.FormatUint(uint64(workspace.ID), 10)),
		taskrunner.WithProjectID(project.GetID()),
		taskrunner.WithWorkDir(project.Path),
		taskrunner.WithLLMClient(mockClient),
		taskrunner.WithContent(inputText),
		taskrunner.WithToWorker(worker.Name),
	)
	if err != nil {
		return err
	}
	if err := waitTaskDone(ctx, db, task.ID, 2*time.Minute); err != nil {
		return err
	}

	mainReq := mockClient.FindRequestWithUserInput(inputText)
	if mainReq == nil {
		return fmt.Errorf("expected LLM request with user input %q", inputText)
	}
	systemPrompt := agentprovider.FirstSystemMessageContent(mainReq)
	userInput := agentprovider.FirstUserMessageContent(mainReq)
	assertResult := semreg.EvaluateStructAssertions(systemPrompt, userInput, scenario.Assert)
	result.Details = map[string]interface{}{
		"taskId":     task.ID,
		"assertions": assertResult.Details,
	}
	if !assertResult.Passed {
		result.Errors = append(result.Errors, assertResult.Errors...)
		return fmt.Errorf("assertions failed")
	}
	if scenario.Assert.TaskCompletes {
		refreshed, err := database.GetTaskByID(db, task.ID)
		if err != nil {
			return err
		}
		if models.TaskStatus(refreshed.Status) != models.TaskStatusDone {
			return fmt.Errorf("task status = %s, want done", refreshed.Status)
		}
		result.Details["taskStatus"] = refreshed.Status
	}
	result.ToolCalls = collector.Snapshot()
	return nil
}

func (r *Runner) runBehavior(ctx context.Context, db *gorm.DB, scenario semreg.Scenario, cfg RunConfig, result *ScenarioResult) error {
	spec := scenario.Behavior
	if spec == nil {
		return errors.New("behavior spec is nil")
	}
	prompt := strings.TrimSpace(spec.Prompt)
	if prompt == "" {
		path := resolveScenarioPath(r.scenariosDir, spec.PromptFile)
		raw, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read prompt file: %w", err)
		}
		prompt = strings.TrimSpace(string(raw))
	}
	baselinePath := resolveScenarioPath(r.baselinesDir, spec.BaselineFile)
	baseline, err := semreg.LoadTraceDocument(baselinePath)
	if err != nil {
		return fmt.Errorf("load baseline: %w", err)
	}

	workDir := strings.TrimSpace(cfg.WorkDir)
	if workDir == "" {
		workDir = strings.TrimSpace(os.Getenv("SEMREG_WORK_DIR"))
	}
	worker := strings.TrimSpace(spec.Worker)
	if worker == "" {
		worker = "explore"
	}
	timeout := time.Duration(spec.TimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 20 * time.Minute
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	collector := NewTraceCollector()
	hub := newCompositeWSHub(collector, r.wsHub)

	task, err := taskrunner.CreateAndRunTask(
		taskrunner.WithDB(db),
		taskrunner.WithWSHub(hub),
		taskrunner.WithCtx(runCtx),
		taskrunner.WithWorkspaceID(fmt.Sprintf("%d", cfg.WorkspaceID)),
		taskrunner.WithProjectID(cfg.ProjectID),
		taskrunner.WithWorkDir(workDir),
		taskrunner.WithContent(prompt),
		taskrunner.WithToWorker(worker),
		taskrunner.WithTaskName("[semreg] "+scenario.Name),
	)
	if err != nil {
		return err
	}
	notifyTaskStarted(cfg, scenario.ID, task.ID, task.SessionID, "main")
	result.ActiveTaskID = task.ID
	result.TaskIDs = []uint{task.ID}
	if err := waitTaskDone(runCtx, db, task.ID, timeout); err != nil {
		return err
	}
	refreshed, _ := database.GetTaskByID(db, task.ID)
	if refreshed != nil && models.TaskStatus(refreshed.Status) == models.TaskStatusFailed {
		return fmt.Errorf("task failed: %s", strings.TrimSpace(refreshed.Error))
	}
	if refreshed != nil {
		result.SessionID = refreshed.SessionID
	}

	actualSummary := semreg.BuildTraceSummary(collector.Snapshot())
	compare := semreg.CompareTraceSummary(actualSummary, baseline.Summary, baseline.Tolerances)
	result.ToolCalls = collector.Snapshot()
	result.Metrics = buildMetricComparisons(compare, actualSummary, baseline.Summary, baseline.Tolerances)
	result.Details = map[string]interface{}{
		"taskId":      task.ID,
		"sessionId":   refreshed.SessionID,
		"compare":     compare.Details,
		"toolCounts":  actualSummary.ToolCounts,
		"baseline":    baseline.Summary,
		"actual":      actualSummary,
		"toolTrace":   collector.SnapshotDetailed(),
	}
	if !compare.Passed {
		result.Errors = append(result.Errors, compare.Errors...)
		return fmt.Errorf("trace regression failed")
	}
	return nil
}

func (r *Runner) runSemantic(ctx context.Context, db *gorm.DB, scenario semreg.Scenario, cfg RunConfig, result *ScenarioResult) error {
	spec := scenario.Semantic
	if spec == nil {
		return errors.New("semantic spec is nil")
	}
	reuseID := strings.TrimSpace(spec.ReuseScenario)
	if reuseID == "" && strings.TrimSpace(spec.TaskInput) == "" {
		return errors.New("missing reuse_scenario or task_input")
	}
	_ = reuseID
	all, err := testrunner.LoadScenariosFromDir(r.scenariosDir)
	if err != nil {
		return err
	}
	testScenario, ok := all[scenario.ID]
	if !ok {
		return fmt.Errorf("scenario %s not found in testrunner", scenario.ID)
	}

	collector := NewTraceCollector()
	hub := newCompositeWSHub(collector, r.wsHub)
	testResult, err := testrunner.ExecuteScenario(
		db,
		hub,
		nil,
		cfg.WorkspaceID,
		testScenario,
		testrunner.WithWorkDir(cfg.WorkDir),
		testrunner.WithProjectID(cfg.ProjectID),
		testrunner.WithOnTaskStarted(func(taskID uint, phase string) {
			notifyTaskStarted(cfg, scenario.ID, taskID, "", phase)
			result.ActiveTaskID = taskID
			result.TaskIDs = appendUniqueUint(result.TaskIDs, taskID)
		}),
	)
	if err != nil {
		return err
	}
	expect := "passed"
	if strings.TrimSpace(spec.ExpectStatus) != "" {
		expect = strings.TrimSpace(spec.ExpectStatus)
	}
	result.ToolCalls = collector.Snapshot()
	if testResult.TaskID > 0 {
		result.TaskIDs = appendUniqueUint(result.TaskIDs, testResult.TaskID)
		result.ActiveTaskID = testResult.TaskID
	}
	if testResult.VerifyTaskID > 0 {
		result.TaskIDs = appendUniqueUint(result.TaskIDs, testResult.VerifyTaskID)
	}
	result.Details = map[string]interface{}{
		"taskId":              testResult.TaskID,
		"verifyTaskId":        testResult.VerifyTaskID,
		"mainTaskOutput":      testResult.MainTaskOutput,
		"verificationOutput":  testResult.VerificationOutput,
		"expectedStatus":    expect,
		"actualStatus":      testResult.Status,
		"toolTrace":         collector.SnapshotDetailed(),
	}
	if testResult.Error != "" {
		result.Errors = append(result.Errors, testResult.Error)
	}
	if testResult.Status != expect {
		return fmt.Errorf("status=%s want=%s", testResult.Status, expect)
	}
	return nil
}

func buildMetricComparisons(compare semreg.TraceCompareResult, actual, baseline semreg.TraceSummary, tolerances *semreg.TraceTolerances) []MetricComparison {
	metrics := []MetricComparison{
		{
			Name:     "total_tool_calls",
			Actual:   actual.TotalToolCalls,
			Baseline: baseline.TotalToolCalls,
			Passed:   !containsError(compare.Errors, "total_tool_calls"),
			Detail:   compare.Details["total_tool_calls"],
		},
		{
			Name:     "read_total",
			Actual:   actual.ReadTotal,
			Baseline: baseline.ReadTotal,
			Passed:   !containsError(compare.Errors, "read_total"),
			Detail:   compare.Details["read_total"],
		},
		{
			Name:     "total_read_output_chars",
			Actual:   actual.TotalReadOutputChars,
			Baseline: baseline.TotalReadOutputChars,
			Passed:   !containsError(compare.Errors, "total_read_output_chars"),
			Detail:   compare.Details["total_read_output_chars"],
		},
		{
			Name:     "read_duplicate_ranges",
			Actual:   len(actual.ReadDuplicateRanges),
			Baseline: len(baseline.ReadDuplicateRanges),
			Passed:   !containsError(compare.Errors, "read_duplicate_ranges"),
			Detail:   compare.Details["read_duplicate_ranges"],
		},
	}
	_ = tolerances
	for tool, baselineCount := range baseline.ToolCounts {
		actualCount := actual.ToolCounts[tool]
		metrics = append(metrics, MetricComparison{
			Name:     "tool:" + tool,
			Actual:   actualCount,
			Baseline: baselineCount,
			Passed:   actualCount <= baselineCount*2, // informational
			Detail:   compare.Details["tool:"+tool],
		})
	}
	return metrics
}

func containsError(errors []string, prefix string) bool {
	for _, item := range errors {
		if strings.HasPrefix(item, prefix) {
			return true
		}
	}
	return false
}

func waitTaskDone(ctx context.Context, db *gorm.DB, taskID uint, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		task, err := database.GetTaskByID(db, taskID)
		if err != nil {
			return err
		}
		switch models.TaskStatus(task.Status) {
		case models.TaskStatusDone, models.TaskStatusFailed, models.TaskStatusCancelled:
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("task %d did not complete within %s", taskID, timeout)
}

func resolveScenarioPath(baseDir, relative string) string {
	relative = strings.TrimSpace(relative)
	if relative == "" {
		return ""
	}
	if filepath.IsAbs(relative) {
		return relative
	}
	return filepath.Clean(filepath.Join(baseDir, relative))
}

func envUint(key string) (uint, bool) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return 0, false
	}
	parsed, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || parsed == 0 {
		return 0, false
	}
	return uint(parsed), true
}

func SummarizeResults(results []ScenarioResult, started time.Time) RunSummary {
	summary := RunSummary{Total: len(results), DurationMs: time.Since(started).Milliseconds()}
	for _, result := range results {
		switch result.Status {
		case "passed":
			summary.Passed++
		case "failed":
			summary.Failed++
		case "skipped":
			summary.Skipped++
		default:
			summary.Error++
		}
	}
	return summary
}

func notifyTaskStarted(cfg RunConfig, scenarioID string, taskID uint, sessionID, phase string) {
	if cfg.OnProgress == nil || taskID == 0 {
		return
	}
	cfg.OnProgress(RunProgressEvent{
		ScenarioID: scenarioID,
		TaskID:     taskID,
		SessionID:  sessionID,
		Phase:      phase,
	})
}

func appendUniqueUint(values []uint, value uint) []uint {
	if value == 0 {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}
