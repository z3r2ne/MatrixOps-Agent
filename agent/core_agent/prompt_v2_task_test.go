package coreagent

import (
	"strings"
	"testing"

	agentmemory "matrixops.local/memory"
)

func TestRenderV2TaskPrompt_RendersBasicSections(t *testing.T) {
	promptText, err := RenderV2TaskPrompt(V2TaskPromptData{
		Memory:      &agentmemory.Memory{GlobalPrompt: "global", Entries: []*agentmemory.MemoryEntry{{Role: "user", Content: "hello", Sequence: 1}}},
		Tools:       []ToolDefinition{{Name: "read_file", Description: "read file"}},
		UserInput:   "do it",
		ContextInfo: &ContextInfo{LimitTokens: 1000, CurrentTokens: 120, CurrentBytes: 512},
	})
	if err != nil {
		t.Fatalf("RenderV2TaskPrompt: %v", err)
	}
	for _, expected := range []string{"<system_prompt>", "<global_prompt>", "<context_window>"} {
		if !strings.Contains(promptText, expected) {
			t.Fatalf("expected prompt to contain %q, got:\n%s", expected, promptText)
		}
	}
}
