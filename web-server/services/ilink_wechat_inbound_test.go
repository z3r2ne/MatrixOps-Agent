package services

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"matrixops-agent/ilink"
)

func TestPrepareWechatInboundAttachments_SavesFilesWithPaths(t *testing.T) {
	dir := t.TempDir()
	attachments := []ilink.InboundAttachment{
		{
			Filename: "photo.png",
			MimeType: "image/png",
			Data:     []byte{0x89, 0x50, 0x4e, 0x47},
			Kind:     "image",
		},
	}

	parts, saved, err := prepareWechatInboundAttachments(dir, "bot-1", 42, attachments)
	if err != nil {
		t.Fatalf("prepareWechatInboundAttachments: %v", err)
	}
	if len(parts) != 1 || len(saved) != 1 {
		t.Fatalf("parts=%d saved=%d", len(parts), len(saved))
	}
	if !strings.HasPrefix(parts[0].URL, "file://") {
		t.Fatalf("url = %q, want file://", parts[0].URL)
	}
	if _, err := os.Stat(saved[0].Path); err != nil {
		t.Fatalf("saved file missing: %v", err)
	}
	if !strings.Contains(saved[0].Path, ".wechat-inbound") {
		t.Fatalf("path = %q, want .wechat-inbound segment", saved[0].Path)
	}

	content := buildWechatInboundContent("[用户发送图片]", saved)
	if !strings.Contains(content, saved[0].Path) {
		t.Fatalf("content should include path, got %q", content)
	}
	if !strings.Contains(content, "filePath") {
		t.Fatalf("content should mention filePath hint, got %q", content)
	}
}

func TestPrepareWechatInboundAttachments_FallbackWhenNoWorkDir(t *testing.T) {
	attachments := []ilink.InboundAttachment{
		{Filename: "a.txt", MimeType: "text/plain", Data: []byte("hi"), Kind: "file"},
	}
	parts, saved, err := prepareWechatInboundAttachments("", "bot", 1, attachments)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(saved) != 0 {
		t.Fatalf("expected no saved paths")
	}
	if len(parts) != 1 || !strings.HasPrefix(parts[0].URL, "data:") {
		t.Fatalf("expected data URL fallback, got %#v", parts[0])
	}
}

func TestUniqueWechatFilename_Dedupes(t *testing.T) {
	used := map[string]int{}
	a := uniqueWechatFilename("doc.pdf", "file", 0, used)
	b := uniqueWechatFilename("doc.pdf", "file", 1, used)
	if a == b {
		t.Fatalf("expected unique names, both %q", a)
	}
	if filepath.Ext(b) != ".pdf" {
		t.Fatalf("ext = %q", filepath.Ext(b))
	}
}
