package tool

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type RGTool struct{}

var (
	rgLookPath       = exec.LookPath
	rgCommand        = exec.Command
	rgPathCandidates = defaultRGPathCandidates
)

func (RGTool) Name() string {
	return "rg"
}

func (RGTool) VerbosName() string {
	return "Ripgrep 搜索"
}

func (RGTool) Description() string {
	return "使用 ripgrep 搜索文件内容；适合针对已缩小范围后的内容检索与局部核实。"
}

func (RGTool) Schema() map[string]interface{} {
	return ObjectParamSchema(map[string]interface{}{
		"pattern": map[string]interface{}{
			"type":        "string",
			"description": "The regex pattern to search for",
		},
		"path": map[string]interface{}{
			"type":        "string",
			"description": "The file or directory to search in",
		},
		"glob": map[string]interface{}{
			"type":        "array",
			"description": "Optional include globs passed to ripgrep",
			"items": map[string]interface{}{
				"type": "string",
			},
		},
		"ignore_case": map[string]interface{}{
			"type":        "boolean",
			"description": "Whether to search case-insensitively",
		},
		"fixed_strings": map[string]interface{}{
			"type":        "boolean",
			"description": "Whether to treat pattern as a literal string",
		},
		"hidden": map[string]interface{}{
			"type":        "boolean",
			"description": "Whether to include hidden files",
		},
		"max_count": map[string]interface{}{
			"type":        "number",
			"description": "Maximum number of matches to return",
		},
	}, []string{"pattern"})
}

func (RGTool) Execute(ctx Context, input map[string]interface{}) (Result, error) {
	pattern, ok := input["pattern"].(string)
	if !ok || strings.TrimSpace(pattern) == "" {
		return Result{IsError: true}, errors.New("rg: missing pattern")
	}

	rgPath, err := findRGExecutable()
	if err != nil {
		return Result{IsError: true}, errors.New("rg: ripgrep executable not found in PATH")
	}

	target := ctx.Directory
	if path, ok := input["path"].(string); ok && strings.TrimSpace(path) != "" {
		target = resolvePath(ctx.Directory, path)
	}
	if target == "" {
		target = "."
	}

	if _, err := os.Stat(target); err != nil {
		return Result{IsError: true}, fmt.Errorf("rg: %w", err)
	}

	args := buildRGArgs(pattern, target, input)
	cmd := rgCommand(rgPath, args...)
	cmd.Dir = ctx.Directory

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return Result{IsError: true}, fmt.Errorf("rg: failed to capture stdout: %w", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return Result{IsError: true}, fmt.Errorf("rg: %w", err)
	}

	content, parseErr := parseRGOutput(stdout, ctx.Directory)

	err = cmd.Wait()
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
			message := strings.TrimSpace(stderr.String())
			if message == "" {
				message = err.Error()
			}
			return Result{IsError: true}, fmt.Errorf("rg: %s", message)
		}
	}

	if parseErr != nil {
		return Result{IsError: true}, fmt.Errorf("rg: %w", parseErr)
	}

	return Result{Content: content}, nil
}

func buildRGArgs(pattern string, target string, input map[string]interface{}) []string {
	args := []string{
		"--json",
		"--line-number",
		"--color", "never",
		"--with-filename",
		"--no-messages",
		"--sort", "path",
	}

	if boolFrom(input["ignore_case"]) {
		args = append(args, "--ignore-case")
	}
	if boolFrom(input["fixed_strings"]) {
		args = append(args, "--fixed-strings")
	}
	if boolFrom(input["hidden"]) {
		args = append(args, "--hidden")
	}
	if maxCount := intFrom(input["max_count"]); maxCount > 0 {
		args = append(args, "--max-count", itoa(maxCount))
	}
	for _, glob := range stringSliceFrom(input["glob"]) {
		if strings.TrimSpace(glob) == "" {
			continue
		}
		args = append(args, "--glob", glob)
	}

	args = append(args, "-e", pattern, target)
	return args
}

func findRGExecutable() (string, error) {
	if path, err := rgLookPath("rg"); err == nil && strings.TrimSpace(path) != "" {
		return path, nil
	}

	for _, candidate := range rgPathCandidates() {
		if strings.TrimSpace(candidate) == "" {
			continue
		}
		info, err := os.Stat(candidate)
		if err != nil || info.IsDir() {
			continue
		}
		if runtime.GOOS != "windows" && info.Mode()&0o111 == 0 {
			continue
		}
		return candidate, nil
	}

	return "", exec.ErrNotFound
}

func defaultRGPathCandidates() []string {
	paths := []string{}
	add := func(items ...string) {
		for _, item := range items {
			if strings.TrimSpace(item) == "" {
				continue
			}
			paths = append(paths, item)
		}
	}

	switch runtime.GOOS {
	case "darwin":
		add(
			"/opt/homebrew/bin/rg",
			"/usr/local/bin/rg",
			"/opt/local/bin/rg",
			"/usr/bin/rg",
			"/bin/rg",
		)
	case "linux":
		add(
			"/usr/local/bin/rg",
			"/usr/bin/rg",
			"/bin/rg",
			"/snap/bin/rg",
		)
	case "windows":
		add(
			`C:\Program Files\ripgrep\rg.exe`,
			`C:\Program Files (x86)\ripgrep\rg.exe`,
			`C:\tools\ripgrep\rg.exe`,
		)
	}

	return paths
}

func parseRGOutput(raw io.Reader, baseDir string) (string, error) {
	decoder := json.NewDecoder(raw)
	lines := make([]string, 0)
	for {
		var event rgJSONEvent
		if err := decoder.Decode(&event); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return "", fmt.Errorf("failed to decode json event: %w", err)
		}
		if event.Type != "match" {
			continue
		}

		var match rgMatchEvent
		if err := json.Unmarshal(event.Data, &match); err != nil {
			return "", fmt.Errorf("failed to decode match event: %w", err)
		}

		path, err := decodeRGText(match.Path)
		if err != nil {
			return "", fmt.Errorf("failed to decode match path: %w", err)
		}
		text, err := decodeRGText(match.Lines)
		if err != nil {
			return "", fmt.Errorf("failed to decode match line: %w", err)
		}

		if path != "" && !filepath.IsAbs(path) {
			path = filepath.Join(baseDir, path)
		}
		text = strings.TrimRight(text, "\r\n")

		lines = append(lines, fmt.Sprintf("%s:%d:%s", path, match.LineNumber, text))
	}

	return strings.Join(lines, "\n"), nil
}

func decodeRGText(value *rgTextValue) (string, error) {
	if value == nil {
		return "", nil
	}
	if value.Text != "" {
		return value.Text, nil
	}
	if value.Bytes == "" {
		return "", nil
	}
	decoded, err := base64.StdEncoding.DecodeString(value.Bytes)
	if err != nil {
		return "", err
	}
	return string(decoded), nil
}

func boolFrom(value interface{}) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		return strings.EqualFold(v, "true")
	default:
		return false
	}
}

func stringSliceFrom(value interface{}) []string {
	switch v := value.(type) {
	case string:
		return []string{v}
	case []string:
		return append([]string(nil), v...)
	case []interface{}:
		result := make([]string, 0, len(v))
		for _, item := range v {
			text, ok := item.(string)
			if ok {
				result = append(result, text)
			}
		}
		return result
	default:
		return nil
	}
}

type rgJSONEvent struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

type rgMatchEvent struct {
	Path       *rgTextValue `json:"path"`
	Lines      *rgTextValue `json:"lines"`
	LineNumber int          `json:"line_number"`
}

type rgTextValue struct {
	Text  string `json:"text"`
	Bytes string `json:"bytes"`
}
