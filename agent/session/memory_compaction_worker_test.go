package session

import (
	"strings"
	"testing"

	"matrixops-agent/types"
	coreagent "matrixops.local/core_agent"
	"matrixops.local/core_agent/workersv2/generic"
	"pkgs/db/models"
)

func floatPtr(f float64) *float64 {
	return &f
}

func TestMemoryCompactionPromptBuilderEmbedsTranscriptInSingleUserPrompt(t *testing.T) {
	memory := &types.Memory{
		Entries: []*types.MemoryEntry{
			{Role: "user", Content: "fix the bug", Sequence: 1},
			{Role: "assistant", Content: "done", Sequence: 2},
		},
	}

	builder := memoryCompactionPromptBuilder(memory, nil, "keep migrations")
	text, err := builder(&coreagent.RunState{UserInput: MemoryCompactionUserInstruction})
	if err != nil {
		t.Fatalf("memoryCompactionPromptBuilder: %v", err)
	}

	idxTranscript := strings.Index(text, "MsgID: 1")
	idxTask := strings.Index(text, "压缩优先级")
	if idxTranscript < 0 {
		t.Fatalf("expected transcript in prompt, got:\n%s", text)
	}
	if idxTask < 0 {
		t.Fatalf("expected compaction task in prompt, got:\n%s", text)
	}
	if idxTask <= idxTranscript {
		t.Fatalf("expected task prompt after transcript, transcript@%d task@%d", idxTranscript, idxTask)
	}
	if !strings.Contains(text, "keep migrations") {
		t.Fatalf("expected worker extra prompt in user message, got:\n%s", text)
	}
	if !strings.Contains(text, MemoryCompactionUserInstruction) {
		t.Fatalf("expected user instruction in user message, got:\n%s", text)
	}
}

func TestMemoryCompactionWorkerBuildMemoryReturnsEmptyHistory(t *testing.T) {
	fullMemory := &types.Memory{
		Entries: []*types.MemoryEntry{
			{Role: "user", Content: "hello", Sequence: 1},
		},
	}
	if len(coreagent.BuildMemoryMessages(fullMemory)) == 0 {
		t.Fatal("expected memory entries to expand into history messages")
	}

	compactionHistoryMemory := &types.Memory{}
	messages := coreagent.BuildMemoryMessages(compactionHistoryMemory)
	if len(messages) != 0 {
		t.Fatalf("expected compaction worker to send no history messages, got %d", len(messages))
	}
}

func TestMemoryCompactionRuntimeSystemPromptPlacementPrefersModelSettings(t *testing.T) {
	runtime := &MemoryCompactionRuntime{
		ModelSettings: &models.ModelSettings{SystemPromptPlacement: "system"},
		LLMConfig:     &models.LLMConfig{SystemPromptPlacement: "instruction"},
	}
	if got := runtime.SystemPromptPlacement(); got != coreagent.SystemPromptPlacementSystem {
		t.Fatalf("placement = %q, want %q", got, coreagent.SystemPromptPlacementSystem)
	}
}

func TestMemoryCompactionRuntimeSystemPromptPlacementFallsBackToLLMConfig(t *testing.T) {
	runtime := &MemoryCompactionRuntime{
		LLMConfig: &models.LLMConfig{SystemPromptPlacement: "instruction"},
	}
	if got := runtime.SystemPromptPlacement(); got != coreagent.SystemPromptPlacementInstruction {
		t.Fatalf("placement = %q, want %q", got, coreagent.SystemPromptPlacementInstruction)
	}
}

func TestMemoryCompactionProviderOptionsOptionUsesConfiguredPlacement(t *testing.T) {
	runtime := &MemoryCompactionRuntime{
		ModelSettings: &models.ModelSettings{
			SystemPromptPlacement: "system",
			OutputLimit:           4096,
		},
		Worker: &models.Worker{
			Model:       "compaction-model",
			Temperature: floatPtr(0.1),
			TopP:        0.9,
		},
		LLMConfig: &models.LLMConfig{
			ID:                    2,
			Name:                  "compaction-config",
			BaseURL:               "http://compaction.example",
			APIKey:                "compaction",
			Model:                 "compaction-model",
			SystemPromptPlacement: "instruction",
		},
	}

	cfg := &generic.AgentConfig{}
	if err := memoryCompactionProviderOptionsOption(runtime)(cfg); err != nil {
		t.Fatalf("memoryCompactionProviderOptionsOption: %v", err)
	}
	if cfg.Runtime.SystemPromptPlacement != coreagent.SystemPromptPlacementSystem {
		t.Fatalf("placement = %q, want %q", cfg.Runtime.SystemPromptPlacement, coreagent.SystemPromptPlacementSystem)
	}
}

func TestMemoryCompactionProviderOptionsOptionUsesCompactionRuntime(t *testing.T) {
	runtime := &MemoryCompactionRuntime{
		Worker: &models.Worker{
			Model:       "compaction-model",
			Temperature: floatPtr(0.1),
			TopP:        0.9,
		},
		LLMConfig: &models.LLMConfig{
			ID:      2,
			Name:    "compaction-config",
			BaseURL: "http://compaction.example",
			APIKey:  "compaction",
			Model:   "compaction-model",
		},
		ModelSettings: &models.ModelSettings{OutputLimit: 4096},
	}

	cfg := &generic.AgentConfig{}
	if err := memoryCompactionProviderOptionsOption(runtime)(cfg); err != nil {
		t.Fatalf("memoryCompactionProviderOptionsOption: %v", err)
	}
	if cfg.LLM.Model != "compaction-model" {
		t.Fatalf("model = %q, want compaction-model", cfg.LLM.Model)
	}
	if cfg.LLM.Temperature != 0.1 || cfg.LLM.TopP != 0.9 {
		t.Fatalf("unexpected sampling: temp=%v top_p=%v", cfg.LLM.Temperature, cfg.LLM.TopP)
	}
	if cfg.LLM.MaxOutputTokens != 4096 {
		t.Fatalf("max output tokens = %d, want 4096", cfg.LLM.MaxOutputTokens)
	}
	providerOptions, ok := cfg.LLM.ProviderOptions.(*models.LLMConfig)
	if !ok || providerOptions == nil {
		t.Fatalf("expected compaction provider options, got %#v", cfg.LLM.ProviderOptions)
	}
	if providerOptions.BaseURL != "http://compaction.example" {
		t.Fatalf("base url = %q", providerOptions.BaseURL)
	}
}
