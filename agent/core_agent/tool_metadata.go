package coreagent

import "strings"

var toolDisplayMetadataKeys = map[string]struct{}{
	"inputPreview":           {},
	"blockedReason":          {},
	"outputFormat":           {},
	"streamMode":             {},
	"tty":                    {},
	"truncated":              {},
	"outputPath":             {},
	"cancelable":             {},
	"blocked":                {},
	"reason":                 {},
	"subtaskTaskId":          {},
	"subtaskSessionId":       {},
	"subtaskParentTaskId":    {},
	"subtaskWorkerName":      {},
	"subtaskTaskName":        {},
	"subtaskContent":         {},
	"subtaskStatus":          {},
	"subtaskAnswer":          {},
	"subtaskPreviewMessages": {},
	"subtaskSummary":         {},
	"subtaskError":           {},
	"subtaskCancelled":       {},
	"subtaskDurationMs":      {},
	"subtaskWorkDir":         {},
	"subtaskBranch":          {},
	"subtaskBaseBranch":      {},
	"subtaskModifiedFiles":   {},
	"subtaskCreatedFiles":    {},
	"linesAdded":             {},
	"linesRemoved":           {},
	"filesChanged":           {},
}

func mergeToolResultMetadataForCore(part *Part, result ToolResult) {
	if part == nil || part.Tool == nil {
		return
	}

	display := filterToolDisplayMetadata(part.Tool.State.Metadata)
	for key, value := range filterToolDisplayMetadata(result.Metadata) {
		display[key] = value
	}
	if result.Truncated {
		display["truncated"] = true
		if result.OutputPath != "" {
			display["outputPath"] = result.OutputPath
		}
	}
	part.Tool.State.Metadata = emptyMapAsNil(display)

	memoryMetadata := cloneMap(part.Tool.State.MemoryMetadata)
	if len(memoryMetadata) == 0 {
		memoryMetadata = filterOutDisplayMetadata(part.Tool.State.Metadata)
	}
	if result.MemoryMetadata != nil {
		for key, value := range result.MemoryMetadata {
			memoryMetadata[key] = value
		}
	} else if result.Metadata != nil {
		for key, value := range result.Metadata {
			memoryMetadata[key] = value
		}
	}
	if result.Truncated {
		memoryMetadata["truncated"] = true
		if result.OutputPath != "" {
			memoryMetadata["outputPath"] = result.OutputPath
		}
	}
	if fullOutput := strings.TrimSpace(result.FullContent); fullOutput != "" {
		memoryMetadata["fullOutput"] = fullOutput
		part.Tool.State.FullOutput = fullOutput
	}
	part.Tool.State.MemoryMetadata = emptyMapAsNil(memoryMetadata)
}

func filterToolDisplayMetadata(input map[string]interface{}) map[string]interface{} {
	if len(input) == 0 {
		return map[string]interface{}{}
	}
	out := make(map[string]interface{})
	for key, value := range input {
		if _, ok := toolDisplayMetadataKeys[key]; ok {
			out[key] = value
		}
	}
	return out
}

func filterOutDisplayMetadata(input map[string]interface{}) map[string]interface{} {
	if len(input) == 0 {
		return map[string]interface{}{}
	}
	out := make(map[string]interface{})
	for key, value := range input {
		if _, ok := toolDisplayMetadataKeys[key]; ok {
			continue
		}
		out[key] = value
	}
	return out
}

func cloneMap(input map[string]interface{}) map[string]interface{} {
	if len(input) == 0 {
		return map[string]interface{}{}
	}
	out := make(map[string]interface{}, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func emptyMapAsNil(input map[string]interface{}) map[string]interface{} {
	if len(input) == 0 {
		return nil
	}
	return input
}
