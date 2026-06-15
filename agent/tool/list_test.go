package tool

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListToolSinglePath(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "alpha"), 0o755); err != nil {
		t.Fatalf("mkdir alpha: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "beta.txt"), []byte("x"), 0o600); err != nil {
		t.Fatalf("write beta.txt: %v", err)
	}

	result, err := ListTool{}.Execute(Context{Directory: dir}, map[string]interface{}{
		"path": ".",
	})
	if err != nil {
		t.Fatalf("list execute: %v", err)
	}
	if !strings.Contains(result.Content, "alpha/") || !strings.Contains(result.Content, "beta.txt") {
		t.Fatalf("unexpected list content: %q", result.Content)
	}
	if strings.Contains(result.Content, "# .") {
		t.Fatalf("single path should not include section header, got %q", result.Content)
	}
}

func TestListToolMultiplePaths(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "first")
	second := filepath.Join(dir, "second")
	if err := os.MkdirAll(first, 0o755); err != nil {
		t.Fatalf("mkdir first: %v", err)
	}
	if err := os.MkdirAll(second, 0o755); err != nil {
		t.Fatalf("mkdir second: %v", err)
	}
	if err := os.WriteFile(filepath.Join(first, "a.txt"), []byte("a"), 0o600); err != nil {
		t.Fatalf("write first/a.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(second, "b.txt"), []byte("b"), 0o600); err != nil {
		t.Fatalf("write second/b.txt: %v", err)
	}

	result, err := ListTool{}.Execute(Context{Directory: dir}, map[string]interface{}{
		"paths": []interface{}{"first", "second"},
	})
	if err != nil {
		t.Fatalf("list execute: %v", err)
	}
	if !strings.Contains(result.Content, "# first\na.txt") {
		t.Fatalf("expected first section, got %q", result.Content)
	}
	if !strings.Contains(result.Content, "# second\nb.txt") {
		t.Fatalf("expected second section, got %q", result.Content)
	}
}

func TestListToolPathsPreferOverPath(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "chosen"), 0o755); err != nil {
		t.Fatalf("mkdir chosen: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "chosen", "only.txt"), []byte("x"), 0o600); err != nil {
		t.Fatalf("write only.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ignored.txt"), []byte("y"), 0o600); err != nil {
		t.Fatalf("write ignored.txt: %v", err)
	}

	result, err := ListTool{}.Execute(Context{Directory: dir}, map[string]interface{}{
		"path":  ".",
		"paths": []interface{}{"chosen"},
	})
	if err != nil {
		t.Fatalf("list execute: %v", err)
	}
	if !strings.Contains(result.Content, "only.txt") {
		t.Fatalf("expected chosen directory listing, got %q", result.Content)
	}
	if strings.Contains(result.Content, "ignored.txt") {
		t.Fatalf("path should be ignored when paths is set, got %q", result.Content)
	}
}
