package tool

import "matrixops-agent/truncate"

// ExecuteWithOutputTruncation runs a tool and applies the shared output truncation
// policy to its result content.
func ExecuteWithOutputTruncation(toolInstance Tool, ctx Context, input map[string]interface{}) (Result, error) {
	if err := CheckContext(ctx); err != nil {
		return Result{IsError: true, Name: toolInstance.Name()}, err
	}
	result, err := toolInstance.Execute(ctx, input)
	if result.Name == "" {
		result.Name = toolInstance.Name()
	}
	if result.FullContent == "" && result.Content != "" {
		result.FullContent = result.Content
	}
	if result.MemoryMetadata == nil && result.Metadata != nil {
		result.MemoryMetadata = cloneMap(result.Metadata)
	}

	if result.Content != "" && !result.Truncated && !result.PreserveFullOutput && result.OutputPath == "" {
		truncated, truncateErr := TruncateResult(result)
		if truncateErr != nil {
			return Result{Name: result.Name, IsError: true}, truncateErr
		}
		result = truncated
	}

	if err != nil {
		return result, err
	}

	return result, nil
}

// TruncateResult applies the shared truncation policy to tool results so every
// tool output is processed consistently outside the tool implementation itself.
func TruncateResult(result Result) (Result, error) {
	if result.Content == "" || result.Truncated || result.PreserveFullOutput || result.OutputPath != "" {
		return result, nil
	}

	truncated, err := truncate.Output(result.Content, truncate.Options{HasTaskTool: false})
	if err != nil {
		return Result{Name: result.Name, IsError: true}, err
	}

	result.Content = truncated.Content
	result.Truncated = truncated.Truncated
	result.OutputPath = truncated.OutputPath
	return result, nil
}

// PrepareFileOpRecordForStorage truncates file content before it is persisted so
// file memory stays bounded even though tools return raw output.
func PrepareFileOpRecordForStorage(record *FileOpRecord) (*FileOpRecord, error) {
	if record == nil {
		return nil, nil
	}

	prepared := *record
	if prepared.Content == "" {
		return &prepared, nil
	}

	truncated, err := truncate.Output(prepared.Content, truncate.Options{HasTaskTool: false})
	if err != nil {
		return nil, err
	}

	prepared.Content = truncated.Content
	return &prepared, nil
}

func buildReadToolMetadata(record *FileOpRecord) map[string]interface{} {
	if record == nil {
		return nil
	}

	prepared, err := PrepareFileOpRecordForStorage(record)
	if err != nil || prepared == nil {
		prepared = record
	}

	fileRead := map[string]interface{}{
		"path":     prepared.Path,
		"offset":   prepared.Offset,
		"limit":    prepared.Limit,
		"is_whole": prepared.IsWhole,
		"action":   string(prepared.Action),
		"content":  prepared.Content,
	}

	return map[string]interface{}{
		"fileRead": fileRead,
	}
}

func cloneMap(input map[string]interface{}) map[string]interface{} {
	if input == nil {
		return nil
	}
	out := make(map[string]interface{}, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}
