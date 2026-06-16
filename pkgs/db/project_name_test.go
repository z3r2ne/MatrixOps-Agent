package database

import (
	"errors"
	"testing"

	"pkgs/db/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupProjectNameTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&models.Project{}); err != nil {
		t.Fatalf("migrate project table: %v", err)
	}
	return db
}

func TestEnsureProjectNameAvailable(t *testing.T) {
	db := setupProjectNameTestDB(t)
	project := &models.Project{Name: "MatrixOps", Path: t.TempDir()}
	if err := CreateProject(db, project); err != nil {
		t.Fatalf("create project: %v", err)
	}

	if err := EnsureProjectNameAvailable(db, "matrixops", 0); !errors.Is(err, ErrProjectNameExists) {
		t.Fatalf("expected duplicate name error, got %v", err)
	}
	if err := EnsureProjectNameAvailable(db, "Other", 0); err != nil {
		t.Fatalf("expected available name, got %v", err)
	}
	if err := EnsureProjectNameAvailable(db, "MatrixOps", project.ID); err != nil {
		t.Fatalf("expected same project to pass, got %v", err)
	}
}
