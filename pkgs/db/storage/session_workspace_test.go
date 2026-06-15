package storage

import (
	"path/filepath"
	"strings"
	"testing"

	"pkgs/db/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestBackfillMissingSessionWorkspaceInfo(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("MATRIXOPS_HOME", tmp)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := InitStorage(db); err != nil {
		t.Fatalf("init storage: %v", err)
	}
	if err := db.AutoMigrate(&models.Project{}); err != nil {
		t.Fatalf("migrate project: %v", err)
	}

	project := &models.Project{
		ID:           1,
		Name:         "demo-project",
		Path:         "/tmp/demo-project",
		WorktreePath: "/tmp/demo-project",
	}
	if err := db.Create(project).Error; err != nil {
		t.Fatalf("create project: %v", err)
	}

	session := &models.Session{
		ID:        "session-1",
		ProjectID: "1",
		Directory: "/tmp/demo-project",
		Title:     "test",
		Version:   "1",
		Created:   1,
		Updated:   1,
	}
	if err := db.Create(session).Error; err != nil {
		t.Fatalf("create session: %v", err)
	}

	if err := backfillMissingSessionWorkspaceInfo(db); err != nil {
		t.Fatalf("backfillMissingSessionWorkspaceInfo: %v", err)
	}

	info, err := GetSession(db, session.ID)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if strings.TrimSpace(info.WorkspaceRoot) == "" {
		t.Fatalf("workspace root is empty")
	}
	if strings.TrimSpace(info.WorkspacePath) == "" {
		t.Fatalf("workspace path is empty")
	}
	if filepath.Base(info.WorkspacePath) != "workspace" {
		t.Fatalf("workspace path = %q", info.WorkspacePath)
	}

	var refreshed models.Session
	if err := db.First(&refreshed, "id = ?", session.ID).Error; err != nil {
		t.Fatalf("reload session: %v", err)
	}
	if strings.TrimSpace(refreshed.WorkspacePath) == "" {
		t.Fatalf("stored workspace path is empty")
	}
}
