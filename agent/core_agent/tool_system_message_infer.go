package coreagent

import (
	"encoding/json"
	"strings"

	agentmemory "matrixops.local/memory"
	agenttool "matrixops-agent/tool"
)

func resolveToolSystemMessage(toolName, systemMessage, metadataJSON string) string {
	if msg := strings.TrimSpace(systemMessage); msg != "" {
		return msg
	}
	name := strings.TrimSpace(toolName)
	if name != "read" && name != "read_whole" {
		return ""
	}
	meta := parseToolMetadataMap(metadataJSON)
	if len(meta) == 0 {
		return ""
	}
	return agenttool.BuildReadToolSystemMessageFromMetadata(meta)
}

func parseToolMetadataMap(metadataJSON string) map[string]interface{} {
	raw := strings.TrimSpace(metadataJSON)
	if raw == "" {
		return nil
	}
	var meta map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &meta); err != nil {
		return nil
	}
	return meta
}

func resolveToolSystemMessageFromEntry(entry *agentmemory.MemoryEntry) string {
	if entry == nil {
		return ""
	}
	return resolveToolSystemMessage(entry.ToolName, entry.ToolSystemMessage, entry.ToolMetadataJSON)
}

func resolveToolSystemMessageFromHistoryItem(item *agentmemory.ChatHistoryItem) string {
	if item == nil {
		return ""
	}
	return resolveToolSystemMessage(item.ToolName, item.ToolSystemMessage, "")
}
