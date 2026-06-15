package coreagent

import (
	"strings"
	"testing"

	agentmemory "matrixops.local/memory"
	agentprovider "matrixops-agent/provider"
)

func TestResolveToolSystemMessageFromEntry_ReadMetadataFallback(t *testing.T) {
	entry := &agentmemory.MemoryEntry{
		ToolName: "read",
		ToolOutput: "     1\talpha\n     2\tbeta",
		ToolMetadataJSON: `{"total_lines":2,"lines_returned":2,"start_line":1,"has_more":false}`,
	}
	systemMessage := resolveToolSystemMessageFromEntry(entry)
	if !strings.Contains(systemMessage, "2 lines read") {
		t.Fatalf("expected inferred read summary, got %q", systemMessage)
	}

	parts, ok := BuildToolLLMContentFromEntry(entry).([]agentprovider.CommonContentPart)
	if !ok || len(parts) != 2 {
		t.Fatalf("expected 2 content parts, got %#v", BuildToolLLMContentFromEntry(entry))
	}
	if !strings.Contains(parts[0].Text, "<system>") {
		t.Fatalf("expected system tag in first part, got %q", parts[0].Text)
	}
}
