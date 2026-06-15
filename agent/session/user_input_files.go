package session

import (
	"encoding/base64"
	"fmt"
	"mime"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"matrixops-agent/types"
)

const UserInputSubdir = ".user-input"

const (
	UserInputSourcePaste  = "paste"
	UserInputSourcePicker = "picker"
	UserInputSourceDrop   = "drop"
)

func ResolveUserInputPath(workDir, relativePath string) (string, error) {
	workDir = strings.TrimSpace(workDir)
	relativePath = strings.TrimSpace(relativePath)
	if workDir == "" || relativePath == "" {
		return "", fmt.Errorf("workDir and path are required")
	}
	relativePath = filepath.Clean(filepath.FromSlash(relativePath))
	if strings.HasPrefix(relativePath, "..") {
		return "", fmt.Errorf("invalid path")
	}
	abs := filepath.Join(workDir, relativePath)
	abs = filepath.Clean(abs)
	workRoot := filepath.Clean(workDir)
	if abs != workRoot && !strings.HasPrefix(abs, workRoot+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes workDir")
	}
	userRoot := filepath.Join(workRoot, UserInputSubdir)
	if abs != userRoot && !strings.HasPrefix(abs, userRoot+string(filepath.Separator)) {
		return "", fmt.Errorf("path must be under %s", UserInputSubdir)
	}
	return abs, nil
}

// UserInputFileAPIURL 返回用户附件预览 URL（兼容旧调用方，统一走 temp-uploads）。
func UserInputFileAPIURL(_ uint, path string) string {
	return TempUserInputFileAPIURL(path)
}

// MaterializeUserInputPart 将带 path 的用户附件解析为绝对路径与 file:// URL，供后续 read/LLM 使用。
func MaterializeUserInputPart(workDir string, part *Part) (*Part, error) {
	if part == nil {
		return nil, fmt.Errorf("part is nil")
	}
	if strings.TrimSpace(part.Type) != "file" {
		return part, nil
	}
	rawPath := strings.TrimSpace(part.Path)
	if rawPath == "" {
		if strings.TrimSpace(part.URL) != "" {
			return part, nil
		}
		return nil, fmt.Errorf("file part requires path or url")
	}
	abs, err := ResolveStoredUserInputFilePath(rawPath, workDir)
	if err != nil {
		return nil, err
	}
	out := *part
	out.Path = abs
	out.URL = fileURLFromAbsolutePath(abs)
	if strings.TrimSpace(out.Filename) == "" {
		out.Filename = filepath.Base(abs)
	}
	if strings.TrimSpace(out.Mime) == "" {
		out.Mime = mime.TypeByExtension(filepath.Ext(abs))
		if out.Mime == "" {
			out.Mime = "application/octet-stream"
		}
	}
	return &out, nil
}

// UserInputTextOnly 仅合并文本 part，不包含文件占位符。
func UserInputTextOnly(parts []*Part) string {
	var builder strings.Builder
	for _, part := range parts {
		if part == nil || part.Type != types.PartTypeText {
			continue
		}
		if text := strings.TrimSpace(part.Text); text != "" {
			if builder.Len() > 0 {
				builder.WriteString("\n\n")
			}
			builder.WriteString(text)
		}
	}
	return strings.TrimSpace(builder.String())
}

// BuildUnifiedLLMContentParts 将用户 parts 统一转换为 LLM 多模态 content parts。
func BuildUnifiedLLMContentParts(parts []*Part, workDir string) ([]types.ChatHistoryContentPart, error) {
	if len(parts) == 0 {
		return nil, nil
	}
	out := make([]types.ChatHistoryContentPart, 0, len(parts))
	for _, part := range parts {
		if part == nil {
			continue
		}
		switch strings.TrimSpace(part.Type) {
		case types.PartTypeText:
			text := strings.TrimSpace(part.Text)
			if text == "" || part.Synthetic {
				continue
			}
			out = append(out, types.ChatHistoryContentPart{Type: "text", Text: text})
		case "file":
			contentParts, err := filePartToLLMContentParts(part, workDir)
			if err != nil {
				return nil, err
			}
			out = append(out, contentParts...)
		}
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

func filePartToLLMContentParts(part *Part, workDir string) ([]types.ChatHistoryContentPart, error) {
	materialized, err := MaterializeUserInputPart(workDir, part)
	if err != nil {
		return nil, err
	}
	mimeType := strings.TrimSpace(materialized.Mime)
	filename := strings.TrimSpace(materialized.Filename)
	if filename == "" {
		filename = "attachment"
	}
	absPath, err := resolvePartAbsolutePath(workDir, materialized)
	if err != nil {
		return nil, err
	}
	if strings.HasPrefix(mimeType, "image/") {
		dataURL, err := imageDataURLFromAbsolutePath(absPath, mimeType)
		if err != nil {
			return nil, err
		}
		return []types.ChatHistoryContentPart{{
			Type:     "image_url",
			ImageURL: &types.ChatHistoryImageURL{URL: dataURL},
		}}, nil
	}
	if mimeType == "text/plain" || mimeType == "application/x-directory" {
		text, err := readTextLikeFileForLLM(workDir, materialized, absPath)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(text) == "" {
			return nil, nil
		}
		return []types.ChatHistoryContentPart{{Type: "text", Text: text}}, nil
	}
	return []types.ChatHistoryContentPart{{
		Type: "text",
		Text: fmt.Sprintf("[Attached file: %s at %s]", filename, absPath),
	}}, nil
}

func resolvePartAbsolutePath(workDir string, part *Part) (string, error) {
	if part == nil {
		return "", fmt.Errorf("part is nil")
	}
	if rawPath := strings.TrimSpace(part.Path); rawPath != "" {
		return ResolveStoredUserInputFilePath(rawPath, workDir)
	}
	parsed, err := url.Parse(strings.TrimSpace(part.URL))
	if err != nil {
		return "", err
	}
	if parsed.Scheme != "file" {
		return "", fmt.Errorf("unsupported file url scheme %q", parsed.Scheme)
	}
	path := parsed.Path
	if path == "" {
		return "", fmt.Errorf("empty file path")
	}
	path, _ = url.PathUnescape(path)
	if !filepath.IsAbs(path) {
		path = filepath.Join(string(filepath.Separator), path)
	}
	return filepath.Clean(path), nil
}

func readTextLikeFileForLLM(workDir string, part *Part, absPath string) (string, error) {
	if part != nil && part.Mime == "application/x-directory" {
		entries, err := os.ReadDir(absPath)
		if err != nil {
			return "", err
		}
		names := make([]string, 0, len(entries))
		for _, entry := range entries {
			names = append(names, entry.Name())
		}
		return fmt.Sprintf("Directory listing for %s:\n%s", absPath, strings.Join(names, "\n")), nil
	}
	data, err := os.ReadFile(absPath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func imageDataURLFromAbsolutePath(absPath, mimeType string) (string, error) {
	data, err := os.ReadFile(absPath)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(mimeType) == "" {
		mimeType = mime.TypeByExtension(filepath.Ext(absPath))
	}
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	return "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}

func sanitizeUserInputFilename(name string) string {
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

func fileURLFromAbsolutePath(absPath string) string {
	absPath = filepath.Clean(absPath)
	return (&url.URL{Scheme: "file", Path: absPath}).String()
}

func enrichUserFilePartURLs(runner *AgentRunner, parts []*Part) {
	if runner == nil || len(parts) == 0 {
		return
	}
	workDir := runner.GetDirectory()
	for _, part := range parts {
		if part == nil || part.Type != "file" || strings.TrimSpace(part.Path) == "" {
			continue
		}
		materialized, err := MaterializeUserInputPart(workDir, part)
		if err != nil {
			continue
		}
		part.Path = materialized.Path
		part.Mime = materialized.Mime
		part.Filename = materialized.Filename
		part.URL = TempUserInputFileAPIURL(materialized.Path)
	}
}
