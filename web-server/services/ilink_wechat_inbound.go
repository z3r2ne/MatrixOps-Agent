package services

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"matrixops-agent/ilink"
	"matrixops-agent/types"
)

const wechatInboundSubdir = ".wechat-inbound"

type savedWechatAttachment struct {
	Path     string
	Filename string
	Kind     string
	Mime     string
}

// prepareWechatInboundAttachments 将微信附件写入任务工作区并生成 file:// Part。
// 目录：{workDir}/.wechat-inbound/{botID}-{messageID}/<filename>
func prepareWechatInboundAttachments(workDir, botID string, messageID int64, attachments []ilink.InboundAttachment) ([]*types.Part, []savedWechatAttachment, error) {
	if len(attachments) == 0 {
		return nil, nil, nil
	}
	workDir = strings.TrimSpace(workDir)
	if workDir == "" {
		return wechatAttachmentsToInputParts(attachments), nil, nil
	}

	dir := filepath.Join(workDir, wechatInboundSubdir, fmt.Sprintf("%s-%d", sanitizeWechatPathSegment(botID), messageID))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, nil, fmt.Errorf("create wechat inbound dir: %w", err)
	}
	ensureWechatInboundGitignore(workDir)

	parts := make([]*types.Part, 0, len(attachments))
	saved := make([]savedWechatAttachment, 0, len(attachments))
	usedNames := make(map[string]int)

	for i, attachment := range attachments {
		name := uniqueWechatFilename(attachment.Filename, attachment.Kind, i, usedNames)
		absPath := filepath.Join(dir, name)
		if err := os.WriteFile(absPath, attachment.Data, 0o600); err != nil {
			return nil, nil, fmt.Errorf("write wechat attachment %q: %w", name, err)
		}

		mimeType := strings.TrimSpace(attachment.MimeType)
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}
		fileURL := fileURLFromAbsolutePath(absPath)
		parts = append(parts, &types.Part{
			Type:     "file",
			URL:      fileURL,
			Mime:     mimeType,
			Filename: name,
		})
		saved = append(saved, savedWechatAttachment{
			Path:     absPath,
			Filename: strings.TrimSpace(attachment.Filename),
			Kind:     attachment.Kind,
			Mime:     mimeType,
		})
	}
	return parts, saved, nil
}

func buildWechatInboundContent(text string, saved []savedWechatAttachment) string {
	text = strings.TrimSpace(text)
	if len(saved) == 0 {
		return text
	}

	var b strings.Builder
	if text != "" {
		b.WriteString(text)
		b.WriteString("\n\n")
	}
	b.WriteString("微信附件已保存到工作区，可用 read 或 message 工具的 filePath 访问：\n")
	for _, item := range saved {
		b.WriteString("- ")
		b.WriteString(item.Path)
		if name := strings.TrimSpace(item.Filename); name != "" && name != filepath.Base(item.Path) {
			b.WriteString(" (")
			b.WriteString(name)
			b.WriteString(")")
		}
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}

func fileURLFromAbsolutePath(absPath string) string {
	absPath = filepath.Clean(absPath)
	return (&url.URL{Scheme: "file", Path: absPath}).String()
}

func sanitizeWechatPathSegment(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "bot"
	}
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|', ' ', '\t', '\n':
			b.WriteRune('_')
		default:
			b.WriteRune(r)
		}
	}
	out := strings.Trim(b.String(), "._")
	if out == "" {
		return "bot"
	}
	return out
}

func sanitizeWechatFilename(name string) string {
	name = strings.TrimSpace(name)
	name = filepath.Base(name)
	if name == "" || name == "." || name == ".." {
		return ""
	}
	var b strings.Builder
	for _, r := range name {
		if r == '/' || r == '\\' || r == 0 {
			b.WriteRune('_')
			continue
		}
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}

func uniqueWechatFilename(original, kind string, index int, used map[string]int) string {
	base := sanitizeWechatFilename(original)
	if base == "" {
		switch kind {
		case "image":
			base = "image.jpg"
		case "video":
			base = "video.mp4"
		default:
			base = fmt.Sprintf("attachment-%d", index+1)
		}
	}
	name := base
	if count, ok := used[base]; ok {
		ext := filepath.Ext(base)
		stem := strings.TrimSuffix(base, ext)
		if stem == "" {
			stem = base
			ext = ""
		}
		name = fmt.Sprintf("%s_%d%s", stem, count+1, ext)
	}
	used[base]++
	return name
}

func ensureWechatInboundGitignore(workDir string) {
	root := filepath.Join(workDir, wechatInboundSubdir)
	path := filepath.Join(root, ".gitignore")
	if _, err := os.Stat(path); err == nil {
		return
	}
	_ = os.MkdirAll(root, 0o755)
	_ = os.WriteFile(path, []byte("*\n!.gitignore\n"), 0o644)
}
