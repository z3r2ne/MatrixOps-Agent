package skillfs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListInstalledSkillsAndLoadByName(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("MATRIXOPS_HOME", tmp)

	root := filepath.Join(tmp, "skills", "source-1", "skill-a")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	content := "---\nname: Skill A\ndescription: Helpful skill\n---\n# Skill A\nUse me.\n"
	if err := os.WriteFile(filepath.Join(root, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}

	skills, err := ListInstalledSkills()
	if err != nil {
		t.Fatalf("ListInstalledSkills: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("skill count = %d, want 1", len(skills))
	}
	if skills[0].Name != "Skill A" || skills[0].Description != "Helpful skill" {
		t.Fatalf("unexpected skill metadata: %+v", skills[0])
	}

	skill, loaded, err := LoadInstalledSkillContent("skill a")
	if err != nil {
		t.Fatalf("LoadInstalledSkillContent: %v", err)
	}
	if skill.Name != "Skill A" {
		t.Fatalf("skill name = %q", skill.Name)
	}
	if loaded != content {
		t.Fatalf("loaded content mismatch: %q", loaded)
	}
}
