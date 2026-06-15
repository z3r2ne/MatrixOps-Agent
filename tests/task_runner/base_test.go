package task_runner

import (
	"encoding/json"
	"errors"
	"pkgs/db/models"
	"strconv"
	"strings"
	"testing"

	"matrixops/services"
	"matrixops/services/task_runner"
	"matrixops/types"

	utils "tests/utils"
	testutil "tests/testutil"

	database "pkgs/db"

	provider "matrixops-agent/provider"

	llm "matrixops-agent/llm"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func floatPtr(f float64) *float64 {
	return &f
}

func TestCreateAndRunTask(t *testing.T) {
	agentKit := newAgentTestKit()
	agentKit.build(t)

	hubErrors := []string{}
	taskStatus := []string{}
	llmReq := []llm.ChatRequest{}
	workerSystemPrompt := uuid.New().String()
	globalPrompt := uuid.New().String()
	occupationPrompt := uuid.New().String()
	inputText := uuid.New().String()
	outputText := uuid.New().String()

	agentKit.hubCallback = func(message []byte) {
		var msg services.WSOutgoingMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			t.Fatalf("failed to unmarshal message: %v", err)
		}
		if msg.Type == types.WSTypeError {
			hubErrors = append(hubErrors, msg.Error)
		}
		if msg.Type == types.WSTypeTaskStatus {
			taskStatus = append(taskStatus, msg.Status)
		}
	}

	agentKit.clientCallback = func(request llm.ChatRequest) (<-chan llm.StreamEvent, error) {
		llmReq = append(llmReq, request)
		return testutil.MockAnswerActionStream(outputText), nil
	}

	agentKit.worker.SystemPrompt = workerSystemPrompt
	database.UpdateWorkerByName(agentKit.db, agentKit.worker.Name, agentKit.worker)
	database.SetGlobalPrompt(agentKit.db, globalPrompt)
	occupation, err := database.GetOccupationByCode(agentKit.db, agentKit.worker.Occupation)
	if err != nil {
		t.Fatalf("GetOccupationByName returned error: %v", err)
	}
	occupation.Prompt = occupationPrompt
	database.UpdateOccupationInstance(agentKit.db, occupation)

	task, err := task_runner.CreateAndRunTask(
		task_runner.WithDB(agentKit.db),
		task_runner.WithWSHub(agentKit.hub),
		task_runner.WithWorkspaceID(strconv.FormatUint(uint64(agentKit.workspace.ID), 10)),
		task_runner.WithProjectID(agentKit.project.GetID()),
		task_runner.WithWorkDir(agentKit.project.Path),
		task_runner.WithLLMClient(agentKit.client),
		task_runner.WithContent(inputText),
		task_runner.WithToWorker(agentKit.worker.Name),
	)
	if err != nil {
		t.Fatalf("CreateAndRunTask returned error: %v", err)
	}
	err = task_runner.WaitTask(task.ID)
	if err != nil {
		t.Fatalf("WaitTask returned error: %v", err)
	}
	agentKit.waitHub()

	mainReq := testutil.FindChatRequestWithUserInput(llmReq, inputText)
	if mainReq == nil {
		t.Fatalf("expected an LLM request containing user input %q, got %d requests", inputText, len(llmReq))
	}
	systemMsg := testutil.FirstSystemMessageContent(mainReq)
	assert.Contains(t, systemMsg, workerSystemPrompt)
	assert.Contains(t, systemMsg, occupationPrompt)
	assert.Contains(t, systemMsg, globalPrompt)
	assert.Contains(t, systemMsg, "<system_prompt>")
	assert.Equal(t, inputText, testutil.UserMessageContent(mainReq))

	assert.Empty(t, hubErrors)
	assert.Equal(t, []string{string(models.TaskStatusRunning), string(models.TaskStatusRunning), string(models.TaskStatusDone)}, taskStatus)
}

func TestCreateAndRunTaskFailed(t *testing.T) {
	agentKit := newAgentTestKit()
	agentKit.build(t)

	hubErrors := []string{}
	taskStatus := []string{}
	workerSystemPrompt := uuid.New().String()
	globalPrompt := uuid.New().String()
	occupationPrompt := uuid.New().String()
	inputText := uuid.New().String()
	errorMsg := uuid.New().String()
	agentKit.hubCallback = func(message []byte) {
		var msg services.WSOutgoingMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			t.Fatalf("failed to unmarshal message: %v", err)
		}
		if msg.Type == types.WSTypeError {
			hubErrors = append(hubErrors, msg.Error)
		}
		if msg.Type == types.WSTypeTaskStatus {
			taskStatus = append(taskStatus, msg.Status)
		}
	}

	agentKit.clientCallback = func(request llm.ChatRequest) (<-chan llm.StreamEvent, error) {
		return nil, errors.New(errorMsg)
	}

	agentKit.worker.SystemPrompt = workerSystemPrompt
	database.UpdateWorkerByName(agentKit.db, agentKit.worker.Name, agentKit.worker)
	database.SetGlobalPrompt(agentKit.db, globalPrompt)
	occupation, err := database.GetOccupationByCode(agentKit.db, agentKit.worker.Occupation)
	if err != nil {
		t.Fatalf("GetOccupationByName returned error: %v", err)
	}
	occupation.Prompt = occupationPrompt
	database.UpdateOccupationInstance(agentKit.db, occupation)

	task, err := task_runner.CreateAndRunTask(
		task_runner.WithDB(agentKit.db),
		task_runner.WithWSHub(agentKit.hub),
		task_runner.WithWorkspaceID(strconv.FormatUint(uint64(agentKit.workspace.ID), 10)),
		task_runner.WithProjectID(agentKit.project.GetID()),
		task_runner.WithWorkDir(agentKit.project.Path),
		task_runner.WithLLMClient(agentKit.client),
		task_runner.WithContent(inputText),
		task_runner.WithToWorker(agentKit.worker.Name),
	)
	if err != nil {
		t.Fatalf("CreateAndRunTask returned error: %v", err)
	}
	err = task_runner.WaitTask(task.ID)
	if err != nil {
		t.Fatalf("WaitTask returned error: %v", err)
	}
	agentKit.waitHub()
	assert.Contains(t, strings.Join(hubErrors, ","), errorMsg)
	assert.Equal(t, []string{string(models.TaskStatusRunning), string(models.TaskStatusRunning), string(models.TaskStatusFailed)}, taskStatus)
}

func TestRunTaskWithLLMConfig(t *testing.T) {
	agentKit := newAgentTestKit()
	agentKit.build(t)

	hubErrors := []string{}
	taskStatus := []string{}
	llmReq := []llm.ChatRequest{}
	workerSystemPrompt := uuid.New().String()
	globalPrompt := uuid.New().String()
	occupationPrompt := uuid.New().String()
	inputText := uuid.New().String()
	outputText := uuid.New().String()

	agentKit.hubCallback = func(message []byte) {
		var msg services.WSOutgoingMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			t.Fatalf("failed to unmarshal message: %v", err)
		}
		if msg.Type == types.WSTypeError {
			hubErrors = append(hubErrors, msg.Error)
		}
		if msg.Type == types.WSTypeTaskStatus {
			taskStatus = append(taskStatus, msg.Status)
		}
	}

	agentKit.clientCallback = func(request llm.ChatRequest) (<-chan llm.StreamEvent, error) {
		llmReq = append(llmReq, request)
		return testutil.MockAnswerActionStream(outputText), nil
	}

	agentKit.worker.SystemPrompt = workerSystemPrompt
	database.UpdateWorkerByName(agentKit.db, agentKit.worker.Name, agentKit.worker)
	database.SetGlobalPrompt(agentKit.db, globalPrompt)
	occupation, err := database.GetOccupationByCode(agentKit.db, agentKit.worker.Occupation)
	if err != nil {
		t.Fatalf("GetOccupationByName returned error: %v", err)
	}
	occupation.Prompt = occupationPrompt
	database.UpdateOccupationInstance(agentKit.db, occupation)

	task := &models.Task{
		ProjectID:  agentKit.project.ID,
		WorkerID:   &agentKit.worker.ID,
		WorkerName: agentKit.worker.Name,
		Status:     string(models.TaskStatusQueue),
		Branch:     "main",
		WorkDir:    agentKit.project.Path,
		Content:    inputText,
	}

	err = database.CreateTask(agentKit.db, task)
	if err != nil {
		t.Fatalf("CreateTask returned error: %v", err)
	}

	err = task_runner.RunTask(
		task.ID,
		task_runner.WithDB(agentKit.db),
		task_runner.WithWSHub(agentKit.hub),
		task_runner.WithLLMClient(agentKit.client),
		task_runner.WithContent(inputText),
		task_runner.WithBaseBranch("main"),
	)
	if err != nil {
		t.Fatalf("CreateAndRunTask returned error: %v", err)
	}
	err = task_runner.WaitTask(task.ID)
	if err != nil {
		t.Fatalf("WaitTask returned error: %v", err)
	}
	agentKit.waitHub()

	mainReq := testutil.FindChatRequestWithUserInput(llmReq, inputText)
	if mainReq == nil {
		t.Fatalf("expected an LLM request containing user input %q, got %d requests", inputText, len(llmReq))
	}
	systemMsg := testutil.FirstSystemMessageContent(mainReq)
	assert.Contains(t, systemMsg, workerSystemPrompt)
	assert.Contains(t, systemMsg, occupationPrompt)
	assert.Contains(t, systemMsg, globalPrompt)
	assert.Contains(t, systemMsg, "<system_prompt>")
	assert.Equal(t, inputText, testutil.UserMessageContent(mainReq))

	assert.Empty(t, hubErrors)
	assert.Equal(t, []string{string(models.TaskStatusRunning), string(models.TaskStatusRunning), string(models.TaskStatusDone)}, taskStatus)
}

type agentTestKit struct {
	db     *gorm.DB
	hub    *services.GlobalWSHub
	client *provider.MockClient

	workspace *models.Workspace
	project   *models.Project
	worker    *models.Worker
	llmConfig *models.LLMConfig

	hubCallback     func(message []byte)
	clientCallback  func(request llm.ChatRequest) (<-chan llm.StreamEvent, error)
	waitWsHubListen func()
}

func (agentKit *agentTestKit) waitHub() {
	agentKit.hub.Close()
	agentKit.waitWsHubListen()
}

func newAgentTestKit() *agentTestKit {
	return &agentTestKit{}
}

func (agentKit *agentTestKit) build(t *testing.T) *agentTestKit {
	db := utils.SetupTestDB(t)
	hub, waitWsHubListen := utils.NewMockWsHub(db, func(message []byte) {
		if agentKit.hubCallback != nil {
			agentKit.hubCallback(message)
		}
	})
	agentKit.waitWsHubListen = waitWsHubListen

	// 创建project
	var project *models.Project
	if agentKit.project != nil {
		project = agentKit.project
	} else {
		projectPath := t.TempDir()
		project = &models.Project{
			Name:         "test-project",
			Path:         projectPath,
			WorktreePath: projectPath,
			Status:       "active",
		}
	}
	err := database.CreateProject(db, project)
	if err != nil {
		t.Fatalf("CreateProject returned error: %v", err)
	}

	var workspace *models.Workspace
	if agentKit.workspace != nil {
		workspace = agentKit.workspace
		workspace.ProjectIDs = []uint{project.ID}
	} else {
		workspace = &models.Workspace{
			Name:       "test-workspace",
			Path:       t.TempDir(),
			ProjectIDs: []uint{project.ID},
		}
	}
	if err := database.CreateWorkspace(db, workspace); err != nil {
		t.Fatalf("CreateWorkspace returned error: %v", err)
	}

	// 创建llm config
	var llmConfig *models.LLMConfig
	if agentKit.llmConfig != nil {
		llmConfig = agentKit.llmConfig
	} else {
		llmConfig = &models.LLMConfig{
			Name:                  "mock",
			Type:                  string(models.LLMAPITypeChat),
			APIKey:                "test-key",
			Model:                 "mock-model",
			NativeOpenAIToolCalls: false,
		}
	}
	err = database.CreateLLMConfig(db, llmConfig)
	if err != nil {
		t.Fatalf("CreateLLMConfig returned error: %v", err)
	}

	// 创建worker
	var worker *models.Worker
	if agentKit.worker != nil {
		worker = agentKit.worker
		worker.LLMConfigID = &llmConfig.ID
	} else {
		worker = &models.Worker{
			Name:         "test-worker",
			Provider:     "mock",
			Model:        "mock-model",
			Description:  "Test worker",
			Occupation:   "analyst",
			Temperature:  floatPtr(0.7),
			SystemPrompt: "",
			LLMConfigID:  &llmConfig.ID,
		}
	}
	modelSettings, err := utils.CreateTestModelSettings(t, db, "test_runner_model", "")
	if err != nil {
		t.Fatalf("CreateTestModelSettings returned error: %v", err)
	}
	modelSettings.NativeOpenAIToolCalls = false
	if err := database.UpdateModelSettings(db, modelSettings); err != nil {
		t.Fatalf("UpdateModelSettings returned error: %v", err)
	}
	worker.ModelSettingsName = modelSettings.Name

	err = database.CreateWorker(db, worker)
	if err != nil {
		t.Fatalf("CreateWorker returned error: %v", err)
	}

	mockClient := provider.NewMockClientWithStreamCallback(func(request llm.ChatRequest) (<-chan llm.StreamEvent, error) {
		if agentKit.clientCallback != nil {
			return agentKit.clientCallback(request)
		}
		return nil, errors.New("test error")
	})

	agentKit.db = db
	agentKit.hub = hub
	agentKit.client = mockClient
	agentKit.workspace = workspace
	agentKit.project = project
	agentKit.worker = worker
	agentKit.llmConfig = llmConfig
	return agentKit
}
