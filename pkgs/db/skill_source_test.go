package database

import (
	"os"
	"path/filepath"
	"testing"

	"pkgs/db/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestEnsureDefaultSkillSources(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&models.SkillSource{}); err != nil {
		t.Fatalf("migrate skill sources: %v", err)
	}

	if err := EnsureDefaultSkillSources(db); err != nil {
		t.Fatalf("EnsureDefaultSkillSources: %v", err)
	}
	if err := EnsureDefaultSkillSources(db); err != nil {
		t.Fatalf("EnsureDefaultSkillSources second run: %v", err)
	}

	var sources []models.SkillSource
	if err := db.Order("repo_url ASC").Find(&sources).Error; err != nil {
		t.Fatalf("list skill sources: %v", err)
	}
	if len(sources) != 2 {
		t.Fatalf("skill source count = %d, want 2", len(sources))
	}

	expected := map[string]string{
		"~/.agents/skills": ".",
		"~/.codex/skills":  ".",
	}
	for _, source := range sources {
		if source.Enabled {
			t.Fatalf("expected default source %s to be disabled", source.RepoURL)
		}
		wantPath, ok := expected[source.RepoURL]
		if !ok {
			t.Fatalf("unexpected default source repo_url = %s", source.RepoURL)
		}
		if source.SkillsPath != wantPath {
			t.Fatalf("skillsPath for %s = %s, want %s", source.RepoURL, source.SkillsPath, wantPath)
		}
	}
}

func TestEnsureDefaultSkillSourcesAvoidsDuplicateExpandedLocalPath(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&models.SkillSource{}); err != nil {
		t.Fatalf("migrate skill sources: %v", err)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("user home dir: %v", err)
	}
	existing := models.SkillSource{
		Name:       "Existing Agents",
		RepoURL:    filepath.Join(homeDir, ".agents", "skills"),
		SkillsPath: ".",
		Enabled:    true,
	}
	if err := db.Create(&existing).Error; err != nil {
		t.Fatalf("create existing skill source: %v", err)
	}

	if err := EnsureDefaultSkillSources(db); err != nil {
		t.Fatalf("EnsureDefaultSkillSources: %v", err)
	}

	var count int64
	if err := db.Model(&models.SkillSource{}).Count(&count).Error; err != nil {
		t.Fatalf("count skill sources: %v", err)
	}
	if count != 2 {
		t.Fatalf("skill source count = %d, want 2", count)
	}
}
