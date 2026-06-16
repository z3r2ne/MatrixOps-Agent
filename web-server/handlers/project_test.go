package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	database "pkgs/db"
	"pkgs/db/models"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupProjectHandlerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&models.Workspace{}, &models.Project{}, &models.GlobalConfig{}); err != nil {
		t.Fatalf("migrate project tables: %v", err)
	}
	return db
}

func TestCreateProjectUsesDefaultToolPermissionsWhenOmitted(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupProjectHandlerTestDB(t)
	workspace := &models.Workspace{
		Name:      "demo-workspace",
		Path:      t.TempDir(),
		GroupMode: models.TaskListGroupModeProject,
	}
	if err := db.Create(workspace).Error; err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	if err := database.UpsertGlobalConfig(db, models.ConfigKeyDefaultProjectToolPermissions, `{"bash":"allow","write":"deny"}`); err != nil {
		t.Fatalf("seed default project tool permissions: %v", err)
	}

	projectPath := t.TempDir()

	router := gin.New()
	handler := NewProjectHandler(db)
	router.POST("/workspaces/:id/projects", handler.CreateProject)

	body := bytes.NewBufferString(`{"name":"demo-project","path":"` + projectPath + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/workspaces/1/projects", body)
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", resp.Code, resp.Body.String())
	}

	var created models.ProjectResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	values, err := models.ParseProjectToolPermissions(created.ToolPermissions)
	if err != nil {
		t.Fatalf("parse tool permissions: %v", err)
	}
	if values["bash"] != models.ProjectToolPermissionAllow {
		t.Fatalf("bash permission = %q", values["bash"])
	}
	if values["write"] != models.ProjectToolPermissionDeny {
		t.Fatalf("write permission = %q", values["write"])
	}
}

func TestCreateStandaloneProjectRejectsDuplicateName(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupProjectHandlerTestDB(t)
	firstPath := t.TempDir()
	secondPath := t.TempDir()

	router := gin.New()
	handler := NewProjectHandler(db)
	router.POST("/projects", handler.CreateStandaloneProject)

	create := func(name, path string) int {
		body := bytes.NewBufferString(`{"name":"` + name + `","path":"` + path + `"}`)
		req := httptest.NewRequest(http.MethodPost, "/projects", body)
		req.Header.Set("Content-Type", "application/json")
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)
		return resp.Code
	}

	if status := create("MatrixOps", firstPath); status != http.StatusCreated {
		t.Fatalf("first create status = %d", status)
	}
	if status := create("matrixops", secondPath); status != http.StatusConflict {
		t.Fatalf("duplicate create status = %d, want %d", status, http.StatusConflict)
	}
}
