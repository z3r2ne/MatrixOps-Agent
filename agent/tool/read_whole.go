package tool

import (
	"errors"
	"os"
	"strings"
)

type ReadWholeTool struct {
	appendFileRecord func(*FileOpRecord)
}

func NewReadWholeTool(appendFileRecord func(*FileOpRecord)) *ReadWholeTool {
	return &ReadWholeTool{appendFileRecord: appendFileRecord}
}

func (r *ReadWholeTool) Name() string {
	return "read_whole"
}

func (r *ReadWholeTool) VerbosName() string {
	return "读取文件全部内容"
}

func (r *ReadWholeTool) Description() string {
	return "读取文件全部内容；输出为「行号\\t行内容」格式（行号从 1 起算）"
}

func (r *ReadWholeTool) Schema() map[string]interface{} {
	return ObjectParamSchema(map[string]interface{}{
		"path": map[string]interface{}{
			"type":        "string",
			"description": "The path to the file to read",
		},
	}, []string{"path"})
}

func (r *ReadWholeTool) Execute(ctx Context, input map[string]interface{}) (Result, error) {
	path, ok := input["path"].(string)
	if !ok || path == "" {
		return Result{IsError: true}, errors.New("read: missing path")
	}
	resolved := resolvePath(ctx.Directory, path)
	data, err := os.ReadFile(resolved)
	if err != nil {
		return Result{IsError: true}, err
	}
	lines := strings.Split(string(data), "\n")
	if len(lines) > maxReadLines {
		return Result{IsError: true}, errors.New("read_whole: file is too large, please use read tool to read the file")
	}
	formatted, returned, maxBytesReached := formatLinesWithLineNumbersLimited(lines, 1)
	if maxBytesReached {
		return Result{IsError: true}, errors.New("read_whole: file is too large, please use read tool to read the file")
	}
	text := formatted
	info := FileReadInfo{
		Content:       text,
		TotalLines:    len(lines),
		ReturnedLines: returned,
		StartLine:     1,
		OffsetUsed:    0,
		HasMore:       returned < len(lines),
	}

	readFileRecord := &FileOpRecord{
		Path:    path,
		Offset:  0,
		Limit:   0,
		Content: text,
		IsWhole: true,
		Action:  FileOpRecordActionRead,
	}
	recordFileOp(r.appendFileRecord, readFileRecord)
	metadata := buildReadToolMetadata(readFileRecord)
	return Result{
		Message:  BuildReadToolSystemMessage(info),
		Content:  text,
		Metadata: metadata,
	}, nil
}
