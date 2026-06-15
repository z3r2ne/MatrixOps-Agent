package skillmarket

import (
	"os"
	"path/filepath"
	"testing"

	database "pkgs/db"
	"pkgs/db/models"
)

func TestScanSourceSkillsAndInstallLifecycle(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("MATRIXOPS_HOME", tmp)

	sourceRoot := filepath.Join(tmp, "source")
	skillsRoot := filepath.Join(sourceRoot, "skills")
	firstSkill := filepath.Join(skillsRoot, "skill-a")
	secondSkill := filepath.Join(skillsRoot, "nested", "skill-b")
	for _, dir := range []string{firstSkill, secondSkill} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	if err := os.WriteFile(filepath.Join(firstSkill, "SKILL.md"), []byte("---\nname: Skill A\ndescription: First skill\n---\n"), 0o644); err != nil {
		t.Fatalf("write skill a: %v", err)
	}
	if err := os.WriteFile(filepath.Join(secondSkill, "SKILL.md"), []byte("# Skill B\nSecond skill"), 0o644); err != nil {
		t.Fatalf("write skill b: %v", err)
	}

	service := NewService()
	source := models.SkillSource{
		ID:         7,
		Name:       "Demo Source",
		RepoURL:    "https://example.com/demo.git",
		SkillsPath: "skills",
		Enabled:    true,
		LocalPath:  sourceRoot,
	}

	skills, err := service.scanSourceSkills(source)
	if err != nil {
		t.Fatalf("scanSourceSkills: %v", err)
	}
	if len(skills) != 2 {
		t.Fatalf("skill count = %d, want 2", len(skills))
	}

	installedPath, err := service.InstallSkill(source, "skill-a")
	if err != nil {
		t.Fatalf("InstallSkill: %v", err)
	}
	if _, err := os.Stat(filepath.Join(installedPath, "SKILL.md")); err != nil {
		t.Fatalf("stat installed skill: %v", err)
	}

	listed, err := service.ListSkills([]models.SkillSource{source}, false)
	if err != nil {
		t.Fatalf("ListSkills: %v", err)
	}
	var installedFound bool
	for _, skill := range listed {
		if skill.RelativePath == "skill-a" {
			installedFound = skill.Installed
		}
	}
	if !installedFound {
		t.Fatalf("expected installed skill to be marked installed")
	}

	targetPath, err := database.InstalledSkillPath(source.ID, "skill-a")
	if err != nil {
		t.Fatalf("InstalledSkillPath: %v", err)
	}
	if err := service.UninstallSkill(source.ID, "skill-a"); err != nil {
		t.Fatalf("UninstallSkill: %v", err)
	}
	if _, err := os.Stat(targetPath); !os.IsNotExist(err) {
		t.Fatalf("expected installed skill dir removed, stat err = %v", err)
	}
}

func TestSyncSourceSupportsLocalDirectory(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("MATRIXOPS_HOME", tmp)

	localSourceRoot := filepath.Join(tmp, "local-skills")
	firstSkill := filepath.Join(localSourceRoot, "skill-a")
	if err := os.MkdirAll(firstSkill, 0o755); err != nil {
		t.Fatalf("mkdir local skill: %v", err)
	}
	if err := os.WriteFile(filepath.Join(firstSkill, "SKILL.md"), []byte("# Local Skill\nLoaded from local path"), 0o644); err != nil {
		t.Fatalf("write local skill: %v", err)
	}

	service := NewService()
	source := models.SkillSource{
		ID:         9,
		Name:       "Local Source",
		RepoURL:    localSourceRoot,
		SkillsPath: ".",
		Enabled:    true,
	}

	skills, err := service.SyncSource(&source)
	if err != nil {
		t.Fatalf("SyncSource: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("skill count = %d, want 1", len(skills))
	}
	if source.LocalPath == "" {
		t.Fatal("expected local cache path to be set")
	}
	if filepath.Clean(source.LocalPath) == filepath.Clean(localSourceRoot) {
		t.Fatalf("expected local source to be copied into cache, got source.LocalPath = %s", source.LocalPath)
	}
	if _, err := os.Stat(filepath.Join(source.LocalPath, "skill-a", "SKILL.md")); err != nil {
		t.Fatalf("stat cached local skill: %v", err)
	}
}
