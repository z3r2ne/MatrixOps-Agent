package tool

import (
	"errors"
	"os"
	"path/filepath"
)

type WriteTool struct {
	appendFileRecord func(*FileOpRecord)
}

func NewWriteTool(appendFileRecord func(*FileOpRecord)) *WriteTool {
	return &WriteTool{appendFileRecord: appendFileRecord}
}

func (w *WriteTool) Name() string {
	return "write"
}

func (w *WriteTool) VerbosName() string {
	return "写入文件"
}

func (w *WriteTool) Description() string {
	return "创建或覆盖文件内容"
}

func (w *WriteTool) Schema() map[string]interface{} {
	return ObjectParamSchema(map[string]interface{}{
		"path": map[string]interface{}{
			"type":        "string",
			"description": "The path to the file to write",
		},
		"content": map[string]interface{}{
			"type":        "string",
			"description": "The content to write to the file",
		},
	}, []string{"path", "content"})
}

func (w *WriteTool) Execute(ctx Context, input map[string]interface{}) (Result, error) {
	path, ok := input["path"].(string)
	if !ok || path == "" {
		return Result{IsError: true}, errors.New("write: missing path")
	}
	content, ok := input["content"].(string)
	if !ok {
		return Result{IsError: true}, errors.New("write: missing content")
	}
	resolved := resolvePath(ctx.Directory, path)
	var existed bool
	var oldLineCount int
	if data, err := os.ReadFile(resolved); err == nil {
		existed = true
		oldLineCount = len(splitLines(string(data)))
	}
	if err := os.MkdirAll(filepath.Dir(resolved), 0o755); err != nil {
		return Result{IsError: true}, err
	}
	if err := os.WriteFile(resolved, []byte(content), 0o600); err != nil {
		return Result{IsError: true}, err
	}
	newLineCount := len(splitLines(content))
	writeFileRecord := &FileOpRecord{
		Path:    path,
		Content: content,
		Action:  FileOpRecordActionWrite,
	}
	recordFileOp(w.appendFileRecord, writeFileRecord)
	metadata := map[string]interface{}{
		"filesChanged": 1,
		"linesAdded":   newLineCount,
	}
	if existed {
		metadata["linesRemoved"] = oldLineCount
	}
	return Result{
		Content:  "ok",
		Metadata: metadata,
	}, nil
}
