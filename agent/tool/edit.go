package tool

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

type EditTool struct {
	appendFileRecord func(*FileOpRecord)
}

func NewEditTool(appendFileRecord func(*FileOpRecord)) *EditTool {
	return &EditTool{appendFileRecord: appendFileRecord}
}

func (e *EditTool) Name() string {
	return "edit"
}

func (e *EditTool) VerbosName() string {
	return "编辑文件"
}

func (e *EditTool) Description() string {
	return "对文件进行局部编辑"
}

func (e *EditTool) Schema() map[string]interface{} {
	return ObjectParamSchema(map[string]interface{}{
		"path": map[string]interface{}{
			"type":        "string",
			"description": "The path to the file to edit",
		},
		"old": map[string]interface{}{
			"type":        "string",
			"description": "The old text to replace",
		},
		"new": map[string]interface{}{
			"type":        "string",
			"description": "The new text to replace with",
		},
	}, []string{"path", "old", "new"})
}

func (e *EditTool) Execute(ctx Context, input map[string]interface{}) (Result, error) {
	path, ok := input["path"].(string)
	if !ok || path == "" {
		return Result{IsError: true}, errors.New("edit: missing path")
	}
	oldText, ok := input["old"].(string)
	if !ok || oldText == "" {
		return Result{IsError: true}, errors.New("edit: missing old text")
	}
	newText, ok := input["new"].(string)
	if !ok {
		return Result{IsError: true}, errors.New("edit: missing new text")
	}
	resolved := resolvePath(ctx.Directory, path)
	data, err := os.ReadFile(resolved)
	if err != nil {
		return Result{IsError: true}, err
	}
	content := string(data)
	if !strings.Contains(content, oldText) {
		return Result{IsError: true}, errors.New("edit: old text not found")
	}
	updated := strings.Replace(content, oldText, newText, 1)
	if err := os.MkdirAll(filepath.Dir(resolved), 0o755); err != nil {
		return Result{IsError: true}, err
	}
	if err := os.WriteFile(resolved, []byte(updated), 0o600); err != nil {
		return Result{IsError: true}, err
	}
	oldLines := len(splitLines(oldText))
	newLines := len(splitLines(newText))
	editFileRecord := &FileOpRecord{
		Path:    path,
		Old:     oldText,
		New:     newText,
		Action:  FileOpRecordActionEdit,
		Content: updated,
	}
	recordFileOp(e.appendFileRecord, editFileRecord)
	return Result{
		Content: "ok",
		Metadata: map[string]interface{}{
			"filesChanged": 1,
			"linesAdded":   maxInt(0, newLines-oldLines),
			"linesRemoved": maxInt(0, oldLines-newLines),
		},
	}, nil
}
