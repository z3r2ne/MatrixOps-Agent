//go:build semanticregression

package semantic_regression

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	builtinworkers "matrixops.local/core_agent/workersv2/builtin"
	builtinskills "matrixops-agent/skills/builtin"
	taskrunner "matrixops/services/task_runner"
	database "pkgs/db"
	"pkgs/db/models"
	"pkgs/llmheaders"
	"pkgs/semreg"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestSemanticRegressionBehavior(t *testing.T) {
	if !semregEnabled() {
		t.Skip("set SEMREG_ENABLE=1 to run behavior regression")
	}

	scenarios, err := semreg.LoadScenariosDir(scenariosDir())
	if err != nil {
		t.Fatal(err)
	}
	scenarios = semreg.FilterScenarios(scenarios, semreg.TierL1)
	if len(scenarios) == 0 {
		t.Skip("no L1 behavior scenarios configured")
	}

	db, err := openSemregDB()
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	for _, scenario := range scenarios {
		if scenario.Kind != semreg.KindBehavior {
			continue
		}
		t.Run(scenario.ID, func(t *testing.T) {
			runBehaviorScenario(t, db, scenario)
		})
	}
}

func runBehaviorScenario(t *testing.T, db *gorm.DB, scenario semreg.Scenario) {
	t.Helper()
	spec := scenario.Behavior
	if spec == nil {
		t.Fatal("behavior spec is nil")
	}

	prompt := strings.TrimSpace(spec.Prompt)
	if prompt == "" {
		path := resolveScenarioPath(scenariosDir(), spec.PromptFile)
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read prompt file: %v", err)
		}
		prompt = strings.TrimSpace(string(raw))
	}

	baselinePath := resolveScenarioPath(baselinesDir(), spec.BaselineFile)
	baseline, err := semreg.LoadTraceDocument(baselinePath)
	if err != nil {
		t.Fatalf("load baseline: %v", err)
	}

	workDir := envOr(firstNonEmpty(spec.WorkDirEnv, "SEMREG_WORK_DIR"), "")
	workspaceID, ok := envUint(firstNonEmpty(spec.WorkspaceEnv, "SEMREG_WORKSPACE_ID"))
	if !ok {
		t.Fatalf("set %s for behavior regression", firstNonEmpty(spec.WorkspaceEnv, "SEMREG_WORKSPACE_ID"))
	}
	projectID := envOr(firstNonEmpty(spec.ProjectEnv, "SEMREG_PROJECT_ID"), "0")

	timeout := time.Duration(spec.TimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 20 * time.Minute
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	worker := strings.TrimSpace(spec.Worker)
	if worker == "" {
		worker = "explore"
	}

	hub := &traceHub{}
	task, err := taskrunner.CreateAndRunTask(
		taskrunner.WithDB(db),
		taskrunner.WithWSHub(hub),
		taskrunner.WithCtx(ctx),
		taskrunner.WithWorkspaceID(fmt.Sprintf("%d", workspaceID)),
		taskrunner.WithProjectID(projectID),
		taskrunner.WithWorkDir(workDir),
		taskrunner.WithContent(prompt),
		taskrunner.WithToWorker(worker),
		taskrunner.WithTaskName("[semreg] "+scenario.Name),
	)
	if err != nil {
		t.Fatalf("CreateAndRunTask: %v", err)
	}

	waitTaskDone(t, db, task.ID, timeout)
	refreshed, _ := database.GetTaskByID(db, task.ID)
	if refreshed != nil && models.TaskStatus(refreshed.Status) == models.TaskStatusFailed {
		t.Fatalf("task failed: %s", refreshed.Status)
	}

	actualSummary := semreg.BuildTraceSummary(hub.snapshot())
	result := semreg.CompareTraceSummary(actualSummary, baseline.Summary, baseline.Tolerances)
	if !result.Passed {
		t.Fatalf("trace regression: %s", strings.Join(result.Errors, "; "))
	}
}

func openSemregDB() (*gorm.DB, error) {
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func waitTaskDone(t *testing.T, db *gorm.DB, taskID uint, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		task, err := database.GetTaskByID(db, taskID)
		if err != nil {
			t.Fatalf("GetTaskByID: %v", err)
		}
		switch models.TaskStatus(task.Status) {
		case models.TaskStatusDone, models.TaskStatusFailed, models.TaskStatusCancelled:
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("task %d did not complete within %s", taskID, timeout)
}
