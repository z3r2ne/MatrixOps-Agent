package database

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"pkgs/db/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openSkillsInitTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.SkillSource{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestInitBuiltInSkillsInstallsMissingSkill(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("MATRIXOPS_HOME", tmp)

	db := openSkillsInitTestDB(t)
	content := []byte("---\nname: research\ndescription: test skill\n---\n# Research\n")
	if err := InitBuiltInSkills(db, map[string][]byte{
		"research/SKILL.md": content,
	}); err != nil {
		t.Fatalf("InitBuiltInSkills: %v", err)
	}

	var source models.SkillSource
	if err := db.Where("repo_url = ?", builtInSkillSourceRepoURL).First(&source).Error; err != nil {
		t.Fatalf("find built-in source: %v", err)
	}
	if source.SkillCount != 1 {
		t.Fatalf("skill count = %d, want 1", source.SkillCount)
	}

	installedPath, err := InstalledSkillPath(source.ID, "research")
	if err != nil {
		t.Fatalf("InstalledSkillPath: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(installedPath, "SKILL.md"))
	if err != nil {
		t.Fatalf("read installed skill: %v", err)
	}
	if string(data) != string(content) {
		t.Fatalf("installed content mismatch")
	}

	cachePath := filepath.Join(tmp, "skill-sources", fmt.Sprintf("source-%d", source.ID), "research", "SKILL.md")
	cacheData, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatalf("read cached skill: %v", err)
	}
	if string(cacheData) != string(content) {
		t.Fatalf("cached content mismatch")
	}
}

func TestInitBuiltInSkillsDoesNotOverwriteExistingSkill(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("MATRIXOPS_HOME", tmp)

	db := openSkillsInitTestDB(t)
	original := []byte("---\nname: research\ndescription: original\n---\n# Original\n")
	updated := []byte("---\nname: research\ndescription: updated\n---\n# Updated\n")

	if err := InitBuiltInSkills(db, map[string][]byte{
		"research/SKILL.md": original,
	}); err != nil {
		t.Fatalf("InitBuiltInSkills first: %v", err)
	}
	if err := InitBuiltInSkills(db, map[string][]byte{
		"research/SKILL.md": updated,
	}); err != nil {
		t.Fatalf("InitBuiltInSkills second: %v", err)
	}

	var source models.SkillSource
	if err := db.Where("repo_url = ?", builtInSkillSourceRepoURL).First(&source).Error; err != nil {
		t.Fatalf("find built-in source: %v", err)
	}
	installedPath, err := InstalledSkillPath(source.ID, "research")
	if err != nil {
		t.Fatalf("InstalledSkillPath: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(installedPath, "SKILL.md"))
	if err != nil {
		t.Fatalf("read installed skill: %v", err)
	}
	if string(data) != string(original) {
		t.Fatalf("expected original skill to remain, got %q", string(data))
	}
}

func TestRestoreBuiltInSkillsOverwritesExistingSkill(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("MATRIXOPS_HOME", tmp)

	db := openSkillsInitTestDB(t)
	original := []byte("---\nname: research\ndescription: original\n---\n# Original\n")
	updated := []byte("---\nname: research\ndescription: updated\n---\n# Updated\n")

	if err := InitBuiltInSkills(db, map[string][]byte{
		"research/SKILL.md": original,
	}); err != nil {
		t.Fatalf("InitBuiltInSkills: %v", err)
	}
	if err := RestoreBuiltInSkills(db, map[string][]byte{
		"research/SKILL.md": updated,
	}); err != nil {
		t.Fatalf("RestoreBuiltInSkills: %v", err)
	}

	var source models.SkillSource
	if err := db.Where("repo_url = ?", builtInSkillSourceRepoURL).First(&source).Error; err != nil {
		t.Fatalf("find built-in source: %v", err)
	}
	installedPath, err := InstalledSkillPath(source.ID, "research")
	if err != nil {
		t.Fatalf("InstalledSkillPath: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(installedPath, "SKILL.md"))
	if err != nil {
		t.Fatalf("read installed skill: %v", err)
	}
	if string(data) != string(updated) {
		t.Fatalf("expected restored skill, got %q", string(data))
	}
}
