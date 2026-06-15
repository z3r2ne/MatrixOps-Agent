package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseMarkdownFrontmatter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.md")
	content := `---
description: Test agent
mode: primary
tools:
  bash: false
  read: true
---

Prompt body`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	md, err := ParseMarkdown(path)
	if err != nil {
		t.Fatalf("parse markdown: %v", err)
	}
	if md.Content != "Prompt body" {
		t.Fatalf("unexpected content: %q", md.Content)
	}
	tools, ok := md.Frontmatter["tools"].(map[string]interface{})
	if !ok {
		t.Fatalf("missing tools")
	}
	if tools["bash"] != false || tools["read"] != true {
		t.Fatalf("unexpected tools: %#v", tools)
	}
}
