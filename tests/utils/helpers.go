package utils

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"matrixops/services"
	database "pkgs/db"
	"pkgs/db/models"
	pkgtestutil "pkgs/testutil"
	testutil "tests/testutil"

	util "matrixops-agent/util"
	"matrixops-agent/global"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func SetupTestDB(t *testing.T) *gorm.DB {
	return testutil.SetupFullTestDB(t)
}

func CreateTestProject(t *testing.T, db *gorm.DB, path string) (*models.Project, error) {
	t.Helper()
	project := &models.Project{
		Name:         "test-project",
		Path:         path,
		WorktreePath: path,
		Status:       "active",
	}
	if err := database.CreateProject(db, project); err != nil {
		return nil, err
	}
	return project, nil
}

func floatPtr(f float64) *float64 {
	return &f
}

func CreateTestWorker(t *testing.T, db *gorm.DB, systemPrompt string) (*models.Worker, error) {
	llmConfig := &models.LLMConfig{
		Name:   "mock",
		Type:   "openai",
		APIKey: "test-key",
		Model:  "mock-model",
	}
	err := database.CreateLLMConfig(db, llmConfig)
	if err != nil {
		return nil, err
	}

	worker := &models.Worker{
		Name:         "test-worker",
		Provider:     "mock",
		Model:        "mock-model",
		Description:  "Test worker",
		Temperature:  floatPtr(0.7),
		SystemPrompt: systemPrompt,
		LLMConfigID:  &llmConfig.ID,
	}
	err = database.CreateWorker(db, worker)
	if err != nil {
		return nil, err
	}
	return worker, nil
}

func CreateTestModelSettings(t *testing.T, db *gorm.DB, name string, prompt string) (*models.ModelSettings, error) {
	modelSettings := &models.ModelSettings{
		Name:         name,
		ContextLimit: 128000,
		OutputLimit:  8000,
		Prompt:       prompt,
	}
	err := database.CreateModelSettings(db, modelSettings)
	if err != nil {
		return nil, err
	}
	return modelSettings, nil
}

func assertContains(t *testing.T, s, substr string) {
	t.Helper()
	if !contains(s, substr) {
		t.Errorf("Expected string to contain %q, but it doesn't.\nString: %s", substr, s)
	}
}

func assertNotContains(t *testing.T, s, substr string) {
	t.Helper()
	if contains(s, substr) {
		t.Errorf("Expected string not to contain %q, but it does.\nString: %s", substr, s)
	}
}

func assertEqual(t *testing.T, expected, actual interface{}) {
	t.Helper()
	if expected != actual {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
}

func assertNotEqual(t *testing.T, expected, actual interface{}) {
	t.Helper()
	if expected == actual {
		t.Errorf("Expected values to be different, but both are %v", expected)
	}
}

func assertError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Error("Expected an error, but got nil")
	}
}

func assertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Errorf("Expected no error, but got: %v", err)
	}
}

func assertFileExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("Expected file to exist: %s", path)
	}
}

func assertFileNotExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err == nil {
		t.Errorf("Expected file not to exist: %s", path)
	}
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && strings.Contains(s, substr)
}

func NewMockWsHub(db *gorm.DB, msgHandle func(message []byte)) (*services.GlobalWSHub, func()) {
	client := services.NewGlobalWSClient(uuid.New().String())

	hub := services.NewGlobalWSHub(db)
	hub.Register(client)
	hub.RunAsync()

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for message := range client.Send {
			msgHandle(message)
		}
	}()

	return hub, func() {
		close(client.Send)
		wg.Wait()
	}
}

func SetupTestAgentConfig(t *testing.T, db *gorm.DB, projectPath string) error {
	agentConfigDir := filepath.Join(projectPath, global.ConfigDirName)
	if err := os.MkdirAll(agentConfigDir, 0755); err != nil {
		return err
	}

	agentConfigYAML := `agents:
  default:
    name: default
    description: Default agent
    model:
      provider: mock
      model: mock-model
    tools:
      - read_file
      - write
      - list_dir
      - grep
`
	agentConfigFile := filepath.Join(projectPath, global.AppName+".yaml")
	if err := os.WriteFile(agentConfigFile, []byte(agentConfigYAML), 0644); err != nil {
		return err
	}

	t.Logf("Created agent config at %s", agentConfigFile)
	return nil
}

func GenerateID() string {
	return util.Ascending("test")
}

// OpenTaskTestDB 复用 pkgs/testutil 的最小任务库（供需要轻量 DB 的测试）。
func OpenTaskTestDB(t *testing.T) *gorm.DB {
	return pkgtestutil.OpenTaskTestDB(t)
}
