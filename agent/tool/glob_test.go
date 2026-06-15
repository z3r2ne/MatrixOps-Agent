package tool

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGlobToolBraceExpansion(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.ts"), []byte("a"), 0o600); err != nil {
		t.Fatalf("write a.ts: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.tsx"), []byte("b"), 0o600); err != nil {
		t.Fatalf("write b.tsx: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "c.go"), []byte("c"), 0o600); err != nil {
		t.Fatalf("write c.go: %v", err)
	}

	result, err := GlobTool{}.Execute(Context{Directory: dir}, map[string]interface{}{
		"pattern": "*.{ts,tsx}",
	})
	if err != nil {
		t.Fatalf("glob execute: %v", err)
	}
	if !strings.Contains(result.Content, "a.ts") || !strings.Contains(result.Content, "b.tsx") {
		t.Fatalf("expected ts/tsx matches, got %q", result.Content)
	}
	if strings.Contains(result.Content, "c.go") {
		t.Fatalf("go file should not match, got %q", result.Content)
	}
}

func TestGlobToolRecursive(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "nested")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sub, "deep.go"), []byte("x"), 0o600); err != nil {
		t.Fatalf("write deep.go: %v", err)
	}

	result, err := GlobTool{}.Execute(Context{Directory: dir}, map[string]interface{}{
		"pattern": "**/*.go",
	})
	if err != nil {
		t.Fatalf("glob execute: %v", err)
	}
	if !strings.Contains(result.Content, "deep.go") {
		t.Fatalf("expected nested go file, got %q", result.Content)
	}
}

func TestGlobToolSkipsDirectories(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "pkg"), 0o755); err != nil {
		t.Fatalf("mkdir pkg: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("m"), 0o600); err != nil {
		t.Fatalf("write main.go: %v", err)
	}

	result, err := GlobTool{}.Execute(Context{Directory: dir}, map[string]interface{}{
		"pattern": "*",
	})
	if err != nil {
		t.Fatalf("glob execute: %v", err)
	}
	if strings.Contains(result.Content, string(filepath.Separator)+"pkg") && !strings.Contains(result.Content, "main.go") {
		t.Fatalf("directory-only match unexpected: %q", result.Content)
	}
	if !strings.Contains(result.Content, "main.go") {
		t.Fatalf("expected main.go, got %q", result.Content)
	}
}
