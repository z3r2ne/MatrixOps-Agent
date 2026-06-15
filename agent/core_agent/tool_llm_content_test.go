package coreagent

import (
	"strings"
	"testing"

	agentmemory "matrixops.local/memory"
	agentprovider "matrixops-agent/provider"
)

func TestBuildToolLLMContent_ReadStyle(t *testing.T) {
	parts, ok := BuildToolLLMContent(
		"5 lines read from file starting from line 1. Total lines in file: 5. End of file reached.",
		"     1\talpha\n     2\tbeta",
		false,
		"",
	).([]agentprovider.CommonContentPart)
	if !ok || len(parts) != 2 {
		t.Fatalf("expected 2 parts, got %#v", BuildToolLLMContent("", "", false, ""))
	}
	if !strings.Contains(parts[0].Text, "<system>") || !strings.Contains(parts[0].Text, "5 lines read") {
		t.Fatalf("system part = %q", parts[0].Text)
	}
	if parts[1].Text != "     1\talpha\n     2\tbeta" {
		t.Fatalf("body part = %q", parts[1].Text)
	}
}

func TestBuildToolLLMContentFromHistoryToolItem_ReadStyle(t *testing.T) {
	parts, ok := BuildToolLLMContentFromHistoryToolItem(&agentmemory.ChatHistoryItem{
		Role:              "tool",
		ToolCallID:        "call-1",
		Content:           "     1\talpha\n     2\tbeta",
		ToolSystemMessage: "2 lines read from file starting from line 1. Total lines in file: 2. End of file reached.",
	}).([]agentprovider.CommonContentPart)
	if !ok || len(parts) != 2 {
		t.Fatalf("expected 2 parts, got %#v", BuildToolLLMContentFromHistoryToolItem(nil))
	}
	if !strings.Contains(parts[0].Text, "<system>") || !strings.Contains(parts[0].Text, "2 lines read") {
		t.Fatalf("system part = %q", parts[0].Text)
	}
	if parts[1].Text != "     1\talpha\n     2\tbeta" {
		t.Fatalf("body part = %q", parts[1].Text)
	}
}

func TestBuildToolLLMContent_EmptySuccess(t *testing.T) {
	parts, ok := BuildToolLLMContent("", "", false, "").([]agentprovider.CommonContentPart)
	if !ok || len(parts) != 1 {
		t.Fatalf("expected 1 part, got %#v", BuildToolLLMContent("", "", false, ""))
	}
	if parts[0].Text != FormatToolSystemTag(emptyToolSystemMessage) {
		t.Fatalf("got %q", parts[0].Text)
	}
}
