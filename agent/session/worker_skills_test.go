package session

import (
	"os"
	"path/filepath"
	"testing"

	"pkgs/db/models"
)

func TestBuildWorkerSkillPromptsLoadsInstalledSkills(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("MATRIXOPS_HOME", tmp)

	root := filepath.Join(tmp, "skills", "source-1", "research")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	content := "---\nname: research\ndescription: test\n---\n# Research\nDo research.\n"
	if err := os.WriteFile(filepath.Join(root, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}

	raw := models.NormalizeEnabledSkillsJSON([]string{"research"})
	prompts, err := buildWorkerSkillPrompts(raw)
	if err != nil {
		t.Fatalf("buildWorkerSkillPrompts: %v", err)
	}
	if len(prompts) != 1 {
		t.Fatalf("prompt count = %d, want 1", len(prompts))
	}
	if prompts[0].Path != "skill://research" {
		t.Fatalf("path = %q", prompts[0].Path)
	}
	if prompts[0].Prompt != content {
		t.Fatalf("unexpected prompt content: %q", prompts[0].Prompt)
	}
}
