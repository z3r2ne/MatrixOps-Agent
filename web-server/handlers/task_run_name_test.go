package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"matrixops/services/task_runner"
	"pkgs/db/models"

	"github.com/gin-gonic/gin"
)

func initTaskHandlerGitRepo(t *testing.T) string {
	t.Helper()

	repoDir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repoDir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test",
			"GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=test",
			"GIT_COMMITTER_EMAIL=test@example.com",
		)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, string(output))
		}
	}

	run("init", "-b", "main")
	if err := os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	run("add", "README.md")
	run("commit", "-m", "init")
	return repoDir
}

func TestRunTaskPassesTaskNameToRuntime(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupSessionHandlerTestDB(t)
	if err := db.AutoMigrate(&models.Workspace{}, &models.Project{}, &models.Task{}); err != nil {
		t.Fatalf("migrate task tables: %v", err)
	}

	repoDir := initTaskHandlerGitRepo(t)
	project := &models.Project{
		Name:         "demo",
		Path:         repoDir,
		WorktreePath: t.TempDir(),
	}
	if err := db.Create(project).Error; err != nil {
		t.Fatalf("create project: %v", err)
	}
	workspace := &models.Workspace{
		Name:       "demo-workspace",
		Path:       t.TempDir(),
		ProjectIDs: []uint{project.ID},
	}
	if err := db.Create(workspace).Error; err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	originalCreateAndRunTask := createAndRunTask
	defer func() {
		createAndRunTask = originalCreateAndRunTask
	}()

	var capturedConfig *task_runner.TaskRuntimeConfig
	createAndRunTask = func(opts ...task_runner.TaskRuntimeConfigOption) (*models.Task, error) {
		capturedConfig = task_runner.NewTaskRuntimeConfig(opts...)
		return &models.Task{
			ID:          1,
			WorkspaceID: workspace.ID,
			Name:        capturedConfig.TaskName,
			Content:     capturedConfig.Content,
			WorkerName:  capturedConfig.ToWorker,
			Status:      "queue",
		}, nil
	}

	body, err := json.Marshal(map[string]interface{}{
		"name":       "修复标题生成",
		"content":    "修复标题生成",
		"workerName": "chat",
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	resp := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(resp)
	ctx.Params = gin.Params{{Key: "id", Value: "1"}}
	ctx.Request = httptest.NewRequest(http.MethodPost, "/workspaces/1/tasks/run", bytes.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")

	handler := NewTaskHandler(db)
	handler.RunWorkspaceTask(ctx)

	if resp.Code != http.StatusCreated {
		t.Fatalf("unexpected status %d: %s", resp.Code, resp.Body.String())
	}
	if capturedConfig == nil {
		t.Fatal("expected createAndRunTask to be called")
	}
	if capturedConfig.TaskName != "修复标题生成" {
		t.Fatalf("expected task name to be passed through, got %q", capturedConfig.TaskName)
	}
	if capturedConfig.Content != "修复标题生成" {
		t.Fatalf("unexpected task content: %q", capturedConfig.Content)
	}
}
