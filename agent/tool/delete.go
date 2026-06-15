package tool

import (
	"errors"
	"fmt"
	"os"
)

type DeleteTool struct {
	appendFileRecord func(*FileOpRecord)
}

func NewDeleteTool(appendFileRecord func(*FileOpRecord)) *DeleteTool {
	return &DeleteTool{appendFileRecord: appendFileRecord}
}

func (d *DeleteTool) Name() string {
	return "delete"
}

func (d *DeleteTool) VerbosName() string {
	return "删除文件"
}

func (d *DeleteTool) Description() string {
	return "删除文件，或删除空目录"
}

func (d *DeleteTool) Schema() map[string]interface{} {
	return ObjectParamSchema(map[string]interface{}{
		"path": map[string]interface{}{
			"type":        "string",
			"description": "The path to the file or empty directory to delete",
		},
	}, []string{"path"})
}

func (d *DeleteTool) Execute(ctx Context, input map[string]interface{}) (Result, error) {
	path, ok := input["path"].(string)
	if !ok || path == "" {
		return Result{IsError: true}, errors.New("delete: missing path")
	}

	resolved := resolvePath(ctx.Directory, path)
	info, err := os.Stat(resolved)
	if err != nil {
		if os.IsNotExist(err) {
			return Result{IsError: true}, fmt.Errorf("delete: path not found: %s", path)
		}
		return Result{IsError: true}, err
	}

	if info.IsDir() {
		entries, readErr := os.ReadDir(resolved)
		if readErr != nil {
			return Result{IsError: true}, readErr
		}
		if len(entries) > 0 {
			return Result{IsError: true}, fmt.Errorf("delete: directory is not empty: %s", path)
		}
		if err := os.Remove(resolved); err != nil {
			return Result{IsError: true}, err
		}
	} else {
		if err := os.Remove(resolved); err != nil {
			return Result{IsError: true}, err
		}
	}

	recordFileOp(d.appendFileRecord, &FileOpRecord{
		Path:   path,
		Action: FileOpRecordActionDelete,
	})
	return Result{Content: "ok"}, nil
}
