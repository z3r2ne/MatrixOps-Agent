package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	database "pkgs/db"
	"pkgs/db/models"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupWorkspaceHandlerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&models.Workspace{}, &models.GlobalConfig{}); err != nil {
		t.Fatalf("migrate workspace tables: %v", err)
	}
	return db
}

func TestCreateWorkspaceUsesConfiguredDefaultGroupMode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("MATRIXOPS_HOME", t.TempDir())

	db := setupWorkspaceHandlerTestDB(t)
	if err := database.UpsertGlobalConfig(db, models.ConfigKeyDefaultTaskListGroupMode, string(models.TaskListGroupModeDate)); err != nil {
		t.Fatalf("seed default group mode: %v", err)
	}

	router := gin.New()
	handler := NewWorkspaceHandler(db)
	router.POST("/workspaces", handler.CreateWorkspace)

	req := httptest.NewRequest(http.MethodPost, "/workspaces", bytes.NewBufferString(`{"name":"demo"}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", resp.Code, resp.Body.String())
	}

	var created models.WorkspaceResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if created.GroupMode != models.TaskListGroupModeDate {
		t.Fatalf("group mode = %q, want %q", created.GroupMode, models.TaskListGroupModeDate)
	}
}

func TestUpdateWorkspacePersistsGroupMode(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupWorkspaceHandlerTestDB(t)
	workspace := &models.Workspace{
		Name:      "demo",
		Path:      filepath.Join(t.TempDir(), "workspace"),
		GroupMode: models.TaskListGroupModeProject,
	}
	if err := db.Create(workspace).Error; err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	router := gin.New()
	handler := NewWorkspaceHandler(db)
	router.PUT("/workspaces/:id", handler.UpdateWorkspace)

	req := httptest.NewRequest(http.MethodPut, "/workspaces/1", bytes.NewBufferString(`{"groupMode":"none"}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.Code, resp.Body.String())
	}

	var updated models.Workspace
	if err := db.First(&updated, workspace.ID).Error; err != nil {
		t.Fatalf("reload workspace: %v", err)
	}
	if updated.GroupMode != models.TaskListGroupModeNone {
		t.Fatalf("group mode = %q, want %q", updated.GroupMode, models.TaskListGroupModeNone)
	}
}

func TestUpdateWorkspaceRejectsInvalidGroupMode(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupWorkspaceHandlerTestDB(t)
	workspace := &models.Workspace{
		Name:      "demo",
		Path:      filepath.Join(t.TempDir(), "workspace"),
		GroupMode: models.TaskListGroupModeProject,
	}
	if err := db.Create(workspace).Error; err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	router := gin.New()
	handler := NewWorkspaceHandler(db)
	router.PUT("/workspaces/:id", handler.UpdateWorkspace)

	req := httptest.NewRequest(http.MethodPut, "/workspaces/1", bytes.NewBufferString(`{"groupMode":"invalid"}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", resp.Code, resp.Body.String())
	}
}
