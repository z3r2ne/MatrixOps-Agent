package tool

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
)

type ReadTool struct {
	appendFileRecord func(*FileOpRecord)
}

func NewReadTool(appendFileRecord func(*FileOpRecord)) *ReadTool {
	return &ReadTool{appendFileRecord: appendFileRecord}
}

func (r *ReadTool) Name() string {
	return "read"
}

func (r *ReadTool) VerbosName() string {
	return "读取文件"
}

func (r *ReadTool) Description() string {
	return "读取文件内容；输出为「行号\\t行内容」格式（行号从 1 起算）。省略 offset/limit 时优先读取整个文件，" +
		"单次最多 " + strconv.Itoa(maxReadLines) + " 行或 " + strconv.Itoa(maxReadBytes/1024) +
		"KB（先到者为准）；大文件请用 offset/limit 分页续读；offset 越界时返回总行数与建议 offset，不报错"
}

func (r *ReadTool) Schema() map[string]interface{} {
	return ObjectParamSchema(map[string]interface{}{
		"path": map[string]interface{}{
			"type":        "string",
			"description": "The path to the file to read",
		},
		"offset": map[string]interface{}{
			"type": "number",
			"description": "0-based line index to start reading from; output line numbers are 1-based. " +
				"If beyond file length, returns total line count and suggested offset instead of failing",
		},
		"limit": map[string]interface{}{
			"type": "number",
			"description": "Maximum number of lines to return after offset (capped at " + strconv.Itoa(maxReadLines) +
				"). If 0 or omitted, reads the whole file from offset up to " + strconv.Itoa(maxReadLines) +
				" lines or " + strconv.Itoa(maxReadBytes/1024) + "KB per call (use next_offset from metadata to continue)",
		},
	}, []string{"path"})
}

func (readTool *ReadTool) Execute(ctx Context, input map[string]interface{}) (Result, error) {
	if err := CheckContext(ctx); err != nil {
		return Result{IsError: true, Name: "read"}, err
	}
	path, ok := input["path"].(string)
	if !ok || path == "" {
		return Result{IsError: true}, errors.New("read: missing path")
	}
	resolved := resolvePath(ctx.Directory, path)
	data, err := os.ReadFile(resolved)
	if err != nil {
		return Result{IsError: true}, err
	}
	offset := intFrom(input["offset"])
	limit := intFrom(input["limit"])
	info := readFileContent(string(data), offset, limit)

	readFileRecord := &FileOpRecord{
		Path:    path,
		Offset:  info.OffsetUsed,
		Limit:   info.LimitUsed,
		Content: info.Content,
		Action:  FileOpRecordActionRead,
	}
	recordFileOp(readTool.appendFileRecord, readFileRecord)
	metadata := buildReadToolMetadata(readFileRecord)
	appendReadPaginationMetadata(metadata, info)
	return Result{
		Message:            BuildReadToolSystemMessage(info),
		Content:            info.Content,
		Metadata:           metadata,
		PreserveFullOutput: true,
	}, nil
}

func resolvePath(base string, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(base, path)
}

func intFrom(value interface{}) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case string:
		parsed, _ := strconv.Atoi(v)
		return parsed
	default:
		return 0
	}
}
