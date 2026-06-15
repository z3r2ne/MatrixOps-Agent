package builtin

import (
	"strings"
	"testing"
)

func TestReadAllIncludesResearchSkill(t *testing.T) {
	files := ReadAll()
	content, ok := files["research/SKILL.md"]
	if !ok {
		t.Fatalf("research/SKILL.md not found in ReadAll()")
	}
	text := string(content)
	if !strings.Contains(text, "name: research") {
		t.Fatalf("expected research skill frontmatter")
	}
	if !strings.Contains(text, "不要相信记忆") {
		t.Fatalf("expected core research principle in skill content")
	}
}
