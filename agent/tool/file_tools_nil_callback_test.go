package tool

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileToolsWorkWithoutAppendFileRecord(t *testing.T) {
	t.Run("read", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "sample.txt")
		if err := os.WriteFile(path, []byte("hello\nworld"), 0o600); err != nil {
			t.Fatalf("write fixture: %v", err)
		}

		result, err := NewReadTool(nil).Execute(Context{Directory: dir}, map[string]interface{}{
			"path": "sample.txt",
		})
		if err != nil {
			t.Fatalf("read execute: %v", err)
		}
		if result.Content != "1\thello\n2\tworld" {
			t.Fatalf("unexpected read content: %q", result.Content)
		}
	})

	t.Run("read_whole", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "sample.txt")
		if err := os.WriteFile(path, []byte("whole file"), 0o600); err != nil {
			t.Fatalf("write fixture: %v", err)
		}

		result, err := NewReadWholeTool(nil).Execute(Context{Directory: dir}, map[string]interface{}{
			"path": "sample.txt",
		})
		if err != nil {
			t.Fatalf("read_whole execute: %v", err)
		}
		if result.Content != "1\twhole file" {
			t.Fatalf("unexpected read_whole content: %q", result.Content)
		}
	})

	t.Run("write", func(t *testing.T) {
		dir := t.TempDir()

		if _, err := NewWriteTool(nil).Execute(Context{Directory: dir}, map[string]interface{}{
			"path":    "written.txt",
			"content": "created by write tool",
		}); err != nil {
			t.Fatalf("write execute: %v", err)
		}

		data, err := os.ReadFile(filepath.Join(dir, "written.txt"))
		if err != nil {
			t.Fatalf("read written file: %v", err)
		}
		if string(data) != "created by write tool" {
			t.Fatalf("unexpected write content: %q", string(data))
		}
	})

	t.Run("delete file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "delete.txt")
		if err := os.WriteFile(path, []byte("remove me"), 0o600); err != nil {
			t.Fatalf("write fixture: %v", err)
		}

		if _, err := NewDeleteTool(nil).Execute(Context{Directory: dir}, map[string]interface{}{
			"path": "delete.txt",
		}); err != nil {
			t.Fatalf("delete execute: %v", err)
		}

		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("expected deleted file to be absent, got err=%v", err)
		}
	})

	t.Run("delete empty directory", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "empty-dir")
		if err := os.MkdirAll(target, 0o755); err != nil {
			t.Fatalf("mkdir fixture: %v", err)
		}

		if _, err := NewDeleteTool(nil).Execute(Context{Directory: dir}, map[string]interface{}{
			"path": "empty-dir",
		}); err != nil {
			t.Fatalf("delete empty dir execute: %v", err)
		}

		if _, err := os.Stat(target); !os.IsNotExist(err) {
			t.Fatalf("expected deleted directory to be absent, got err=%v", err)
		}
	})

	t.Run("delete non-empty directory returns error", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "non-empty")
		if err := os.MkdirAll(target, 0o755); err != nil {
			t.Fatalf("mkdir fixture: %v", err)
		}
		if err := os.WriteFile(filepath.Join(target, "keep.txt"), []byte("keep"), 0o600); err != nil {
			t.Fatalf("write fixture: %v", err)
		}

		_, err := NewDeleteTool(nil).Execute(Context{Directory: dir}, map[string]interface{}{
			"path": "non-empty",
		})
		if err == nil {
			t.Fatal("expected delete non-empty directory to fail")
		}
		if !strings.Contains(err.Error(), "directory is not empty") {
			t.Fatalf("unexpected delete error: %v", err)
		}
	})

	t.Run("edit", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "edit.txt")
		if err := os.WriteFile(path, []byte("before text"), 0o600); err != nil {
			t.Fatalf("write fixture: %v", err)
		}

		if _, err := NewEditTool(nil).Execute(Context{Directory: dir}, map[string]interface{}{
			"path": "edit.txt",
			"old":  "before",
			"new":  "after",
		}); err != nil {
			t.Fatalf("edit execute: %v", err)
		}

		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read edited file: %v", err)
		}
		if string(data) != "after text" {
			t.Fatalf("unexpected edit content: %q", string(data))
		}
	})

	t.Run("patch", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "patch.txt")
		if err := os.WriteFile(path, []byte("old line\nkeep line\n"), 0o600); err != nil {
			t.Fatalf("write fixture: %v", err)
		}

		patch := "*** Begin Patch\n" +
			"*** Update File: patch.txt\n" +
			"@@\n" +
			"-old line\n" +
			"+new line\n" +
			" keep line\n" +
			"*** End Patch\n"

		if _, err := NewPatchTool(nil).Execute(Context{Directory: dir}, map[string]interface{}{
			"patch": patch,
		}); err != nil {
			t.Fatalf("patch execute: %v", err)
		}

		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read patched file: %v", err)
		}
		if string(data) != "new line\nkeep line" {
			t.Fatalf("unexpected patch content: %q", string(data))
		}
	})

	t.Run("patch context not found includes diagnostics", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "patch.txt")
		if err := os.WriteFile(path, []byte("alpha\nbeta\ngamma\n"), 0o600); err != nil {
			t.Fatalf("write fixture: %v", err)
		}

		patch := "*** Begin Patch\n" +
			"*** Update File: patch.txt\n" +
			"@@\n" +
			"-missing line\n" +
			"+new line\n" +
			" beta\n" +
			"*** End Patch\n"

		_, err := NewPatchTool(nil).Execute(Context{Directory: dir}, map[string]interface{}{
			"patch": patch,
		})
		if err == nil {
			t.Fatal("expected patch context error")
		}
		if !strings.Contains(err.Error(), "expected context:") {
			t.Fatalf("expected diagnostics in error, got %v", err)
		}
		if !strings.Contains(err.Error(), "nearby file excerpt:") {
			t.Fatalf("expected nearby excerpt in error, got %v", err)
		}
	})
}

func TestDefaultRegistryWithQuestionUsesNoopAppendFileRecord(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.txt")
	if err := os.WriteFile(path, []byte("registry read"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	registry := NewDefaultRegistryWithQuestion(&DefaultRegistryOptions{})
	readTool, err := registry.Get("read")
	if err != nil {
		t.Fatalf("get read tool: %v", err)
	}

	result, err := readTool.Execute(Context{Directory: dir}, map[string]interface{}{
		"path": "sample.txt",
	})
	if err != nil {
		t.Fatalf("execute read: %v", err)
	}
	if result.Content != "1\tregistry read" {
		t.Fatalf("unexpected registry read content: %q", result.Content)
	}
}
