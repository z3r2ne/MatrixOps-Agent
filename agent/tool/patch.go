package tool

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type PatchTool struct {
	appendFileRecord func(*FileOpRecord)
}

func NewPatchTool(appendFileRecord func(*FileOpRecord)) *PatchTool {
	return &PatchTool{appendFileRecord: appendFileRecord}
}

func (p *PatchTool) Name() string {
	return "patch"
}

func (p *PatchTool) VerbosName() string {
	return "应用补丁"
}

func (p *PatchTool) Description() string {
	return "应用补丁更新文件"
}

func (p *PatchTool) Schema() map[string]interface{} {
	return ObjectParamSchema(map[string]interface{}{
		"patch": map[string]interface{}{
			"type":        "string",
			"description": "The unified diff patch content",
		},
	}, []string{"patch"})
}

func (p *PatchTool) Execute(ctx Context, input map[string]interface{}) (Result, error) {
	patchText, ok := input["patch"].(string)
	if !ok || patchText == "" {
		return Result{IsError: true}, errors.New("patch: missing patch text")
	}
	stats, err := applyPatchWithStats(ctx.Directory, patchText)
	if err != nil {
		return Result{IsError: true}, err
	}
	patchFileRecord := &FileOpRecord{
		Patch:  patchText,
		Action: FileOpRecordActionPatch,
	}
	recordFileOp(p.appendFileRecord, patchFileRecord)
	return Result{
		Content: "ok",
		Metadata: map[string]interface{}{
			"filesChanged": stats.filesChanged,
			"linesAdded":   stats.linesAdded,
			"linesRemoved": stats.linesRemoved,
		},
	}, nil
}

type patchStats struct {
	filesChanged int
	linesAdded   int
	linesRemoved int
}

func applyPatchWithStats(baseDir string, patchText string) (patchStats, error) {
	var stats patchStats
	lines := splitLines(patchText)
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "*** Begin Patch" {
		return stats, errors.New("patch: missing Begin Patch")
	}
	if strings.TrimSpace(lines[len(lines)-1]) != "*** End Patch" {
		return stats, errors.New("patch: missing End Patch")
	}
	lines = lines[1 : len(lines)-1]

	var files []patchFile
	var current *patchFile
	flush := func() {
		if current != nil {
			files = append(files, *current)
			current = nil
		}
	}

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if strings.HasPrefix(line, "*** Add File: ") {
			flush()
			current = &patchFile{action: "add", path: strings.TrimSpace(strings.TrimPrefix(line, "*** Add File: "))}
			continue
		}
		if strings.HasPrefix(line, "*** Delete File: ") {
			flush()
			files = append(files, patchFile{action: "delete", path: strings.TrimSpace(strings.TrimPrefix(line, "*** Delete File: "))})
			continue
		}
		if strings.HasPrefix(line, "*** Update File: ") {
			flush()
			current = &patchFile{action: "update", path: strings.TrimSpace(strings.TrimPrefix(line, "*** Update File: "))}
			continue
		}
		if strings.HasPrefix(line, "*** Move to: ") {
			if current == nil {
				return stats, errors.New("patch: move without update")
			}
			current.moveTo = strings.TrimSpace(strings.TrimPrefix(line, "*** Move to: "))
			continue
		}
		if current == nil {
			continue
		}
		current.lines = append(current.lines, line)
	}
	flush()

	for _, file := range files {
		switch file.action {
		case "add":
			if err := applyAdd(baseDir, file); err != nil {
				return stats, err
			}
			stats.filesChanged++
			for _, line := range file.lines {
				if strings.HasPrefix(line, "+") {
					stats.linesAdded++
				}
			}
		case "delete":
			path := resolvePath(baseDir, file.path)
			data, _ := os.ReadFile(path)
			if err := applyDelete(baseDir, file); err != nil {
				return stats, err
			}
			stats.filesChanged++
			if data != nil {
				stats.linesRemoved += len(splitLines(string(data)))
			}
		case "update":
			path := resolvePath(baseDir, file.path)
			data, err := os.ReadFile(path)
			if err != nil {
				return stats, err
			}
			oldLines := splitLines(string(data))
			updated, err := applyHunks(file.path, string(data), file.lines)
			if err != nil {
				return stats, err
			}
			if file.moveTo != "" {
				movePath := resolvePath(baseDir, file.moveTo)
				if err := os.MkdirAll(filepath.Dir(movePath), 0o755); err != nil {
					return stats, err
				}
				if err := os.WriteFile(movePath, []byte(updated), 0o600); err != nil {
					return stats, err
				}
				if err := os.Remove(path); err != nil {
					return stats, err
				}
			} else {
				if err := os.WriteFile(path, []byte(updated), 0o600); err != nil {
					return stats, err
				}
			}
			newLines := splitLines(updated)
			stats.filesChanged++
			stats.linesAdded += maxInt(0, len(newLines)-len(oldLines))
			stats.linesRemoved += maxInt(0, len(oldLines)-len(newLines))
		default:
			return stats, errors.New("patch: unknown action")
		}
	}
	return stats, nil
}

func applyPatch(baseDir string, patchText string) error {
	_, err := applyPatchWithStats(baseDir, patchText)
	return err
}

type patchFile struct {
	action string
	path   string
	moveTo string
	lines  []string
}

func applyAdd(baseDir string, file patchFile) error {
	path := resolvePath(baseDir, file.path)
	if _, err := os.Stat(path); err == nil {
		return errors.New("patch: file already exists")
	}
	content := []string{}
	for _, line := range file.lines {
		if strings.HasPrefix(line, "+") {
			content = append(content, line[1:])
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(strings.Join(content, "\n")), 0o600)
}

func applyDelete(baseDir string, file patchFile) error {
	path := resolvePath(baseDir, file.path)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func applyUpdate(baseDir string, file patchFile) error {
	path := resolvePath(baseDir, file.path)
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	updated, err := applyHunks(file.path, string(data), file.lines)
	if err != nil {
		return err
	}
	if file.moveTo != "" {
		movePath := resolvePath(baseDir, file.moveTo)
		if err := os.MkdirAll(filepath.Dir(movePath), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(movePath, []byte(updated), 0o600); err != nil {
			return err
		}
		return os.Remove(path)
	}
	return os.WriteFile(path, []byte(updated), 0o600)
}

type hunk struct {
	context     []string
	replacement []string
}

func applyHunks(path string, content string, lines []string) (string, error) {
	hunks := parseHunks(lines)
	parts := splitLines(content)
	start := 0
	for hunkIndex, h := range hunks {
		if len(h.context) == 0 {
			continue
		}
		index := findMatch(parts, h.context, start)
		if index == -1 {
			return "", fmt.Errorf(
				"patch: context not found in %s hunk #%d\nexpected context:\n%s\nnearby file excerpt:\n%s",
				path,
				hunkIndex+1,
				formatPatchContextPreview(h.context),
				findNearbyPatchExcerpt(parts, h.context, start),
			)
		}
		parts = append(append(parts[:index], h.replacement...), parts[index+len(h.context):]...)
		start = index + len(h.replacement)
	}
	return strings.Join(parts, "\n"), nil
}

func parseHunks(lines []string) []hunk {
	var hunks []hunk
	current := hunk{}
	flush := func() {
		if len(current.context) > 0 || len(current.replacement) > 0 {
			hunks = append(hunks, current)
		}
		current = hunk{}
	}
	for _, line := range lines {
		if strings.HasPrefix(line, "@@") {
			flush()
			continue
		}
		if len(line) == 0 {
			continue
		}
		switch line[0] {
		case ' ':
			current.context = append(current.context, line[1:])
			current.replacement = append(current.replacement, line[1:])
		case '-':
			current.context = append(current.context, line[1:])
		case '+':
			current.replacement = append(current.replacement, line[1:])
		}
	}
	flush()
	return hunks
}

func findMatch(lines []string, context []string, start int) int {
	for i := start; i+len(context) <= len(lines); i++ {
		match := true
		for j := 0; j < len(context); j++ {
			if lines[i+j] != context[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

func formatPatchContextPreview(lines []string) string {
	if len(lines) == 0 {
		return "(empty)"
	}
	limit := minInt(len(lines), 8)
	preview := make([]string, 0, limit+1)
	for i := 0; i < limit; i++ {
		preview = append(preview, "  "+lines[i])
	}
	if len(lines) > limit {
		preview = append(preview, "  ...")
	}
	return strings.Join(preview, "\n")
}

func findNearbyPatchExcerpt(lines []string, context []string, start int) string {
	if len(lines) == 0 {
		return "(file is empty)"
	}

	anchor := start
	if anchor < 0 {
		anchor = 0
	}
	if len(context) > 0 {
		first := context[0]
		for i := 0; i < len(lines); i++ {
			if lines[i] == first {
				anchor = i
				break
			}
		}
	}

	begin := anchor - 2
	if begin < 0 {
		begin = 0
	}
	end := anchor + maxInt(len(context)+2, 5)
	if end > len(lines) {
		end = len(lines)
	}

	out := make([]string, 0, end-begin)
	for i := begin; i < end; i++ {
		out = append(out, "  "+strconv.Itoa(i+1)+"| "+lines[i])
	}
	return strings.Join(out, "\n")
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func splitLines(text string) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.TrimRight(text, "\n")
	if text == "" {
		return []string{}
	}
	return strings.Split(text, "\n")
}
