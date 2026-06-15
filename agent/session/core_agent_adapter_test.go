package session

import (
	"testing"

	coreagent "matrixops.local/core_agent"
	"pkgs/db/models"
)

func TestResolveSystemPromptPlacementUsesModelSettings(t *testing.T) {
	runtimeConfig := &RuntimeConfig{
		LLMConfig: &models.LLMConfig{
			SystemPromptPlacement: "system",
		},
		ModelSettings: &models.ModelSettings{
			SystemPromptPlacement: "instruction",
		},
	}

	if got := resolveSystemPromptPlacement(runtimeConfig); got != coreagent.SystemPromptPlacementInstruction {
		t.Fatalf("expected model settings placement %q, got %q", coreagent.SystemPromptPlacementInstruction, got)
	}
}

func TestResolveSystemPromptPlacementFallsBackToModelSettings(t *testing.T) {
	runtimeConfig := &RuntimeConfig{
		LLMConfig: &models.LLMConfig{},
		ModelSettings: &models.ModelSettings{
			SystemPromptPlacement: "instruction",
		},
	}

	if got := resolveSystemPromptPlacement(runtimeConfig); got != coreagent.SystemPromptPlacementInstruction {
		t.Fatalf("expected model settings placement %q, got %q", coreagent.SystemPromptPlacementInstruction, got)
	}
}
