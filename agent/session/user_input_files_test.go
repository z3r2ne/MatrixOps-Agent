package session

import (
	"os"
	"path/filepath"
	"testing"

	"matrixops-agent/types"
)

func TestBuildUnifiedLLMContentParts_TextAndImage(t *testing.T) {
	absPath, err := SaveTempUserInputFile("a.png", []byte("fakepng"))
	if err != nil {
		t.Fatalf("SaveTempUserInputFile: %v", err)
	}
	parts, err := BuildUnifiedLLMContentParts([]*Part{
		{Type: types.PartTypeText, Text: "hello"},
		{Type: "file", Path: absPath, Mime: "image/png", Filename: "a.png"},
	}, "")
	if err != nil {
		t.Fatalf("BuildUnifiedLLMContentParts: %v", err)
	}
	if len(parts) != 2 {
		t.Fatalf("expected 2 parts, got %d", len(parts))
	}
	if parts[0].Type != "text" || parts[0].Text != "hello" {
		t.Fatalf("unexpected text part: %#v", parts[0])
	}
	if parts[1].Type != "image_url" || parts[1].ImageURL == nil || parts[1].ImageURL.URL == "" {
		t.Fatalf("unexpected image part: %#v", parts[1])
	}
}

func TestUserInputTextOnlySkipsFilePlaceholders(t *testing.T) {
	absPath, err := SaveTempUserInputFile("a.png", []byte("fakepng"))
	if err != nil {
		t.Fatalf("SaveTempUserInputFile: %v", err)
	}
	got := UserInputTextOnly([]*Part{
		{Type: types.PartTypeText, Text: "question"},
		{Type: "file", Path: absPath, Mime: "image/png", Filename: "a.png"},
	})
	if got != "question" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveUserInputPathRejectsEscape(t *testing.T) {
	dir := t.TempDir()
	if _, err := ResolveUserInputPath(dir, "../outside.txt"); err == nil {
		t.Fatal("expected escape error")
	}
}

func TestResolveStoredUserInputFilePathAcceptsAbsoluteTempPath(t *testing.T) {
	absPath, err := SaveTempUserInputFile("note.txt", []byte("hi"))
	if err != nil {
		t.Fatalf("SaveTempUserInputFile: %v", err)
	}
	got, err := ResolveStoredUserInputFilePath(absPath, "")
	if err != nil {
		t.Fatalf("ResolveStoredUserInputFilePath: %v", err)
	}
	if got != absPath {
		t.Fatalf("got %q, want %q", got, absPath)
	}
}

func TestMaterializeUserInputPart(t *testing.T) {
	absPath, err := SaveTempUserInputFile("note.txt", []byte("hi"))
	if err != nil {
		t.Fatalf("SaveTempUserInputFile: %v", err)
	}
	part, err := MaterializeUserInputPart("", &Part{Type: "file", Path: absPath, Mime: "text/plain", Filename: "note.txt"})
	if err != nil {
		t.Fatalf("MaterializeUserInputPart: %v", err)
	}
	if part.Path != absPath {
		t.Fatalf("path = %q, want %q", part.Path, absPath)
	}
	if part.URL == "" {
		t.Fatal("expected file url")
	}
	data, err := os.ReadFile(absPath)
	if err != nil || string(data) != "hi" {
		t.Fatalf("file content mismatch: %v", err)
	}
}

func TestTempUserInputFileAPIURLUsesAbsolutePath(t *testing.T) {
	absPath, err := SaveTempUserInputFile("a.png", []byte("x"))
	if err != nil {
		t.Fatalf("SaveTempUserInputFile: %v", err)
	}
	url := TempUserInputFileAPIURL(absPath)
	if url == "" || !filepath.IsAbs(absPath) {
		t.Fatalf("unexpected url %q for %q", url, absPath)
	}
}
