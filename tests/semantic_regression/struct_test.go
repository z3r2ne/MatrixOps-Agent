package semantic_regression

import (
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	coreagent "matrixops.local/core_agent"
	"matrixops/services"
	"matrixops/services/task_runner"
	agentmemory "matrixops.local/memory"
	"pkgs/semreg"
	testutil "tests/testutil"
	utils "tests/utils"

	database "pkgs/db"
	"pkgs/db/models"

	provider "matrixops-agent/provider"
	llm "matrixops-agent/llm"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func TestSemanticRegressionStruct(t *testing.T) {
	scenarios, err := semreg.LoadScenariosDir(scenariosDir())
	if err != nil {
		t.Fatal(err)
	}
	scenarios = semreg.FilterScenarios(scenarios, semreg.TierL0)
	if len(scenarios) == 0 {
		t.Fatal("no L0 scenarios found")
	}

	for _, scenario := range scenarios {
		t.Run(scenario.ID, func(t *testing.T) {
			switch scenario.Kind {
			case semreg.KindPromptRender:
				runPromptRenderScenario(t, scenario)
			case semreg.KindTaskRunner:
				runTaskRunnerScenario(t, scenario)
			default:
				t.Fatalf("unsupported kind %q", scenario.Kind)
			}
		})
	}
}

func runPromptRenderScenario(t *testing.T, scenario semreg.Scenario) {
	t.Helper()
	spec := scenario.PromptRender
	if spec == nil {
		t.Fatal("prompt_render spec is nil")
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
		t.Fatalf("RenderV2TaskPrompt: %v", err)
	}

	result := semreg.EvaluateStructAssertions(promptText, spec.UserInput, scenario.Assert)
	if !result.Passed {
		t.Fatalf("assertions failed: %s", strings.Join(result.Errors, "; "))
	}
}

func runTaskRunnerScenario(t *testing.T, scenario semreg.Scenario) {
	t.Helper()
	spec := scenario.TaskRunner
	if spec == nil {
		t.Fatal("task_runner spec is nil")
	}

	kit := newStructTestKit(t)
	inputText := strings.TrimSpace(spec.TaskInput)
	outputText := uuid.New().String()
	llmReq := make([]llm.ChatRequest, 0)

	kit.clientCallback = func(request llm.ChatRequest) (<-chan llm.StreamEvent, error) {
		llmReq = append(llmReq, request)
		return testutil.MockAnswerActionStream(outputText), nil
	}

	task, err := task_runner.CreateAndRunTask(
		task_runner.WithDB(kit.db),
		task_runner.WithWSHub(kit.hub),
		task_runner.WithWorkspaceID(strconv.FormatUint(uint64(kit.workspace.ID), 10)),
		task_runner.WithProjectID(kit.project.GetID()),
		task_runner.WithWorkDir(kit.project.Path),
		task_runner.WithLLMClient(kit.client),
		task_runner.WithContent(inputText),
		task_runner.WithToWorker(kit.worker.Name),
	)
	if err != nil {
		t.Fatalf("CreateAndRunTask: %v", err)
	}
	defer kit.hub.Close()
	waitTaskDone(t, kit.db, task.ID)

	mainReq := testutil.FindChatRequestWithUserInput(llmReq, inputText)
	if mainReq == nil {
		t.Fatalf("expected LLM request with input %q", inputText)
	}
	systemPrompt := testutil.FirstSystemMessageContent(mainReq)
	userInput := testutil.UserMessageContent(mainReq)
	result := semreg.EvaluateStructAssertions(systemPrompt, userInput, scenario.Assert)
	if !result.Passed {
		t.Fatalf("assertions failed: %s", strings.Join(result.Errors, "; "))
	}

	if scenario.Assert.TaskCompletes {
		task, err = database.GetTaskByID(kit.db, task.ID)
		if err != nil {
			t.Fatalf("GetTaskByID: %v", err)
		}
		if string(task.Status) != string(models.TaskStatusDone) {
			t.Fatalf("task status = %q, want done", task.Status)
		}
	}
}

type structTestKit struct {
	db             *gorm.DB
	hub            *services.GlobalWSHub
	client         *provider.MockClient
	workspace      *models.Workspace
	project        *models.Project
	worker         *models.Worker
	clientCallback func(request llm.ChatRequest) (<-chan llm.StreamEvent, error)
}

func waitTaskDone(t *testing.T, db *gorm.DB, taskID uint) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		task, err := database.GetTaskByID(db, taskID)
		if err != nil {
			t.Fatalf("GetTaskByID: %v", err)
		}
		switch string(task.Status) {
		case string(models.TaskStatusDone), string(models.TaskStatusFailed), string(models.TaskStatusCancelled):
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("task %d did not complete within 30s", taskID)
}

func newStructTestKit(t *testing.T) *structTestKit {
	t.Helper()
	kit := &structTestKit{}
	db := utils.SetupTestDB(t)
	hub, _ := utils.NewMockWsHub(db, func(message []byte) {})

	projectPath := t.TempDir()
	project := &models.Project{
		Name:         "semreg-project",
		Path:         projectPath,
		WorktreePath: projectPath,
		Status:       "active",
	}
	if err := database.CreateProject(db, project); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	workspace := &models.Workspace{
		Name:       "semreg-workspace",
		Path:       t.TempDir(),
		ProjectIDs: []uint{project.ID},
	}
	if err := database.CreateWorkspace(db, workspace); err != nil {
		t.Fatalf("CreateWorkspace: %v", err)
	}

	llmConfig := &models.LLMConfig{
		Name:                  "mock",
		Type:                  string(models.LLMAPITypeChat),
		APIKey:                "test-key",
		Model:                 "mock-model",
		NativeOpenAIToolCalls: false,
	}
	if err := database.CreateLLMConfig(db, llmConfig); err != nil {
		t.Fatalf("CreateLLMConfig: %v", err)
	}

	modelSettings, err := utils.CreateTestModelSettings(t, db, "semreg_model", "")
	if err != nil {
		t.Fatalf("CreateTestModelSettings: %v", err)
	}
	worker := &models.Worker{
		Name:              "semreg-worker",
		Provider:          "mock",
		Model:             "mock-model",
		Description:       "semantic regression worker",
		Occupation:        "analyst",
		Temperature:       floatPtr(0.7),
		SystemPrompt:      "worker prompt",
		LLMConfigID:       &llmConfig.ID,
		ModelSettingsName: modelSettings.Name,
	}
	if err := database.CreateWorker(db, worker); err != nil {
		t.Fatalf("CreateWorker: %v", err)
	}

	mockClient := provider.NewMockClientWithStreamCallback(func(request llm.ChatRequest) (<-chan llm.StreamEvent, error) {
		if kit.clientCallback != nil {
			return kit.clientCallback(request)
		}
		return nil, errors.New("mock client not configured")
	})

	kit.db = db
	kit.hub = hub
	kit.client = mockClient
	kit.workspace = workspace
	kit.project = project
	kit.worker = worker
	return kit
}

func floatPtr(v float64) *float64 { return &v }
