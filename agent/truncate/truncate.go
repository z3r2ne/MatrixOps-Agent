package truncate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"matrixops-agent/global"
)

const (
	MaxLines = 300
	MaxBytes = 100 * 1024
)

type Result struct {
	Content    string
	Truncated  bool
	OutputPath string
}

type Options struct {
	MaxLines    int
	MaxBytes    int
	Direction   string // "head" or "tail"
	HasTaskTool bool
}

func Dir() string {
	_ = global.Init()
	return filepath.Join(global.Path.Data, "tool-output")
}

func Glob() string {
	return filepath.Join(Dir(), "*")
}

func Output(text string, options Options) (Result, error) {
	if err := global.Init(); err != nil {
		return Result{}, err
	}
	if err := os.MkdirAll(Dir(), 0o755); err != nil {
		return Result{}, err
	}
	maxLines := options.MaxLines
	if maxLines <= 0 {
		maxLines = MaxLines
	}
	maxBytes := options.MaxBytes
	if maxBytes <= 0 {
		maxBytes = MaxBytes
	}
	direction := options.Direction
	if direction == "" {
		direction = "head"
	}

	lines := strings.Split(text, "\n")
	totalBytes := len([]byte(text))
	if len(lines) <= maxLines && totalBytes <= maxBytes {
		return Result{Content: text, Truncated: false}, nil
	}

	var (
		out      []string
		bytes    int
		hitBytes bool
	)
	if direction == "head" {
		out, bytes, hitBytes = collectHeadPreview(lines, maxLines, maxBytes)
	} else {
		out, bytes, hitBytes = collectTailPreview(lines, maxLines, maxBytes)
	}

	removed := 0
	unit := "lines"
	if hitBytes {
		removed = totalBytes - bytes
		unit = "bytes"
	} else {
		removed = len(lines) - len(out)
	}

	outputPath := filepath.Join(Dir(), fmt.Sprintf("tool_%d", time.Now().UnixNano()))
	if err := os.WriteFile(outputPath, []byte(text), 0o600); err != nil {
		return Result{}, err
	}

	hint := "Use Grep to search the full content or Read with offset/limit to view specific sections."
	if options.HasTaskTool {
		hint = "Use the Task tool to have explore agent process this file with Grep and Read (with offset/limit). Do NOT read the full file yourself - delegate to save context."
	}

	preview := strings.Join(out, "\n")
	var message string
	if direction == "head" {
		message = fmt.Sprintf("%s\n\n...%d %s truncated...\n\nThe tool call succeeded but the output was truncated. Full output saved to: %s\n%s",
			preview, removed, unit, outputPath, hint)
	} else {
		message = fmt.Sprintf("...%d %s truncated...\n\nThe tool call succeeded but the output was truncated. Full output saved to: %s\n%s\n\n%s",
			removed, unit, outputPath, hint, preview)
	}

	return Result{Content: message, Truncated: true, OutputPath: outputPath}, nil
}

func collectHeadPreview(lines []string, maxLines, maxBytes int) ([]string, int, bool) {
	var out []string
	bytes := 0
	hitBytes := false
	for i := 0; i < len(lines) && i < maxLines; i++ {
		line := lines[i]
		lineLen := len([]byte(line))
		sep := 0
		if i > 0 {
			sep = 1
		}
		if bytes+sep+lineLen <= maxBytes {
			out = append(out, line)
			bytes += sep + lineLen
			continue
		}
		hitBytes = true
		remaining := maxBytes - bytes - sep
		if remaining > 0 {
			if partial := truncateBytesPrefix(line, remaining); partial != "" {
				out = append(out, partial)
				bytes += sep + len([]byte(partial))
			}
		}
		break
	}
	return out, bytes, hitBytes
}

func collectTailPreview(lines []string, maxLines, maxBytes int) ([]string, int, bool) {
	var out []string
	bytes := 0
	hitBytes := false
	for i := len(lines) - 1; i >= 0 && len(out) < maxLines; i-- {
		line := lines[i]
		lineLen := len([]byte(line))
		sep := 0
		if len(out) > 0 {
			sep = 1
		}
		if bytes+sep+lineLen <= maxBytes {
			out = append([]string{line}, out...)
			bytes += sep + lineLen
			continue
		}
		hitBytes = true
		remaining := maxBytes - bytes - sep
		if remaining > 0 {
			if partial := truncateBytesSuffix(line, remaining); partial != "" {
				out = append([]string{partial}, out...)
				bytes += sep + len([]byte(partial))
			}
		}
		break
	}
	return out, bytes, hitBytes
}

func truncateBytesPrefix(text string, maxBytes int) string {
	if maxBytes <= 0 {
		return ""
	}
	raw := []byte(text)
	if len(raw) <= maxBytes {
		return text
	}
	return string(raw[:maxBytes])
}

func truncateBytesSuffix(text string, maxBytes int) string {
	if maxBytes <= 0 {
		return ""
	}
	raw := []byte(text)
	if len(raw) <= maxBytes {
		return text
	}
	return string(raw[len(raw)-maxBytes:])
}
