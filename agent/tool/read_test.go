package tool

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadToolExecuteWithPaginationMetadata(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.txt")
	var b strings.Builder
	for i := 0; i < 1200; i++ {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString("line")
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	result, err := NewReadTool(nil).Execute(Context{Directory: dir}, map[string]interface{}{
		"path": "sample.txt",
	})
	if err != nil {
		t.Fatalf("read execute: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result: %s", result.Content)
	}
	if !result.PreserveFullOutput {
		t.Fatal("read tool should preserve its own bounded output")
	}
	if result.Metadata["total_lines"] != 1200 {
		t.Fatalf("total_lines: got %v", result.Metadata["total_lines"])
	}
	if result.Metadata["has_more"] != true {
		t.Fatalf("has_more: got %v", result.Metadata["has_more"])
	}
	if result.Metadata["next_offset"] != maxReadLines {
		t.Fatalf("next_offset: got %v want %d", result.Metadata["next_offset"], maxReadLines)
	}
}

func TestReadToolExecuteSmallFileWholeRead(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "small.txt"), []byte("a\nb\nc\n"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	result, err := NewReadTool(nil).Execute(Context{Directory: dir}, map[string]interface{}{
		"path": "small.txt",
	})
	if err != nil {
		t.Fatalf("read execute: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result: %s", result.Content)
	}
	if result.Metadata["has_more"] != false {
		t.Fatalf("has_more: got %v", result.Metadata["has_more"])
	}
	if !strings.Contains(result.Content, "1\ta") || !strings.Contains(result.Content, "3\tc") {
		t.Fatalf("expected whole file content, got %q", result.Content)
	}
}

func TestReadToolOffsetBeyondEnd(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "tiny.txt"), []byte("a\nb\nc\n"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	result, err := NewReadTool(nil).Execute(Context{Directory: dir}, map[string]interface{}{
		"path":   "tiny.txt",
		"offset": 99,
	})
	if err != nil {
		t.Fatalf("read execute: %v", err)
	}
	if result.IsError {
		t.Fatalf("offset beyond end should not be tool error: %s", result.Content)
	}
	if result.Metadata["beyond_end"] != true {
		t.Fatalf("beyond_end: got %v", result.Metadata["beyond_end"])
	}
	if result.Metadata["total_lines"] != 4 {
		t.Fatalf("total_lines: got %v want 4", result.Metadata["total_lines"])
	}
}
