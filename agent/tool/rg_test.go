package tool

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRGToolExecute(t *testing.T) {
	if _, err := exec.LookPath("rg"); err != nil {
		t.Skip("rg executable is not installed")
	}

	dir := t.TempDir()
	goFile := filepath.Join(dir, "main.go")
	txtFile := filepath.Join(dir, "notes.txt")

	if err := os.WriteFile(goFile, []byte("package main\n\nfunc main() {\n\tprintln(\"hello\")\n}\n"), 0o600); err != nil {
		t.Fatalf("write go fixture: %v", err)
	}
	if err := os.WriteFile(txtFile, []byte("func main should not be returned\n"), 0o600); err != nil {
		t.Fatalf("write txt fixture: %v", err)
	}

	result, err := RGTool{}.Execute(Context{Directory: dir}, map[string]interface{}{
		"pattern": "func main",
		"path":    ".",
		"glob":    []interface{}{"*.go"},
	})
	if err != nil {
		t.Fatalf("rg execute: %v", err)
	}

	expected := filepath.Join(dir, "main.go") + ":3:func main() {"
	if result.Content != expected {
		t.Fatalf("unexpected rg content:\nexpected: %q\nactual:   %q", expected, result.Content)
	}
}

func TestRGToolNoMatchesReturnsEmptyContent(t *testing.T) {
	if _, err := exec.LookPath("rg"); err != nil {
		t.Skip("rg executable is not installed")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "main.go")
	if err := os.WriteFile(path, []byte("package main\n"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	result, err := RGTool{}.Execute(Context{Directory: dir}, map[string]interface{}{
		"pattern": "not_found",
		"path":    ".",
	})
	if err != nil {
		t.Fatalf("rg execute with no matches: %v", err)
	}
	if result.Content != "" {
		t.Fatalf("expected empty result, got %q", result.Content)
	}
}

func TestRGToolMissingBinary(t *testing.T) {
	original := rgLookPath
	originalCandidates := rgPathCandidates
	rgLookPath = func(string) (string, error) {
		return "", exec.ErrNotFound
	}
	defer func() {
		rgLookPath = original
		rgPathCandidates = originalCandidates
	}()
	rgPathCandidates = func() []string { return nil }

	result, err := RGTool{}.Execute(Context{Directory: t.TempDir()}, map[string]interface{}{
		"pattern": "main",
		"path":    ".",
	})
	if err == nil {
		t.Fatal("expected missing binary error")
	}
	if !result.IsError {
		t.Fatal("expected result to be marked as error")
	}
	if !strings.Contains(err.Error(), "ripgrep executable not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRGToolUsesFallbackPathWhenPATHMissesRG(t *testing.T) {
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "rg")
	script := "#!/bin/sh\n" +
		"printf '%s\\n' '{\"type\":\"match\",\"data\":{\"path\":{\"text\":\"main.go\"},\"lines\":{\"text\":\"func main() {}\\n\"},\"line_number\":3}}'\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o700); err != nil {
		t.Fatalf("write fallback rg script: %v", err)
	}

	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "main.go"), []byte("package main\n"), 0o600); err != nil {
		t.Fatalf("write workspace fixture: %v", err)
	}

	original := rgLookPath
	originalCandidates := rgPathCandidates
	rgLookPath = func(string) (string, error) {
		return "", exec.ErrNotFound
	}
	rgPathCandidates = func() []string {
		return []string{scriptPath}
	}
	defer func() {
		rgLookPath = original
		rgPathCandidates = originalCandidates
	}()

	result, err := RGTool{}.Execute(Context{Directory: workspace}, map[string]interface{}{
		"pattern": "func main",
		"path":    ".",
	})
	if err != nil {
		t.Fatalf("rg execute with fallback path: %v", err)
	}

	expected := filepath.Join(workspace, "main.go") + ":3:func main() {}"
	if result.Content != expected {
		t.Fatalf("unexpected rg fallback content:\nexpected: %q\nactual:   %q", expected, result.Content)
	}
}

func TestParseRGOutputHandlesLargeJSONEvent(t *testing.T) {
	dir := t.TempDir()
	longLine := strings.Repeat("a", 11*1024*1024)

	raw, err := json.Marshal(rgJSONEvent{
		Type: "match",
		Data: mustMarshalRaw(t, rgMatchEvent{
			Path:       &rgTextValue{Text: "large.txt"},
			Lines:      &rgTextValue{Text: longLine + "\n"},
			LineNumber: 7,
		}),
	})
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}

	content, err := parseRGOutput(bytes.NewReader(raw), dir)
	if err != nil {
		t.Fatalf("parse rg output: %v", err)
	}

	expectedPrefix := filepath.Join(dir, "large.txt") + ":7:"
	if !strings.HasPrefix(content, expectedPrefix) {
		t.Fatalf("expected prefix %q, got %q", expectedPrefix, content[:min(len(content), len(expectedPrefix)+16)])
	}
	if len(content) != len(expectedPrefix)+len(longLine) {
		t.Fatalf("unexpected content length: got %d want %d", len(content), len(expectedPrefix)+len(longLine))
	}
}

func mustMarshalRaw(t *testing.T, value interface{}) json.RawMessage {
	t.Helper()

	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal raw json: %v", err)
	}
	return raw
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
