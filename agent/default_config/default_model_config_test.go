package defaultconfig

import "testing"

func TestLoadDefaultModelSettings(t *testing.T) {
	settings, err := LoadDefaultModelSettings()
	if err != nil {
		t.Fatalf("LoadDefaultModelSettings: %v", err)
	}
	if settings.Name != "default_model_config" {
		t.Fatalf("unexpected name: %q", settings.Name)
	}
	if settings.SystemPromptPlacement != "system" {
		t.Fatalf("unexpected systemPromptPlacement: %q", settings.SystemPromptPlacement)
	}
	if settings.ContextLimit != 100000 {
		t.Fatalf("unexpected contextLimit: %d", settings.ContextLimit)
	}
	if settings.OutputLimit != 100000 {
		t.Fatalf("unexpected outputLimit: %d", settings.OutputLimit)
	}
	if !settings.NativeOpenAIToolCalls {
		t.Fatalf("expected nativeOpenAIToolCalls=true")
	}
}

func TestLoadBuiltinModelSettings(t *testing.T) {
	settings, err := LoadBuiltinModelSettings()
	if err != nil {
		t.Fatalf("LoadBuiltinModelSettings: %v", err)
	}
	if len(settings) != 3 {
		t.Fatalf("unexpected builtin settings count: %d", len(settings))
	}
	if settings[0].Name != "default_model_config" {
		t.Fatalf("unexpected first builtin name: %q", settings[0].Name)
	}
	if settings[1].Name != "gpt-5" {
		t.Fatalf("unexpected second builtin name: %q", settings[1].Name)
	}
	if settings[1].ContextLimit != 300000 {
		t.Fatalf("unexpected gpt-5 contextLimit: %d", settings[1].ContextLimit)
	}
	if settings[1].ReasoningEffort != "xhigh" {
		t.Fatalf("unexpected gpt-5 reasoningEffort: %q", settings[1].ReasoningEffort)
	}
	if settings[1].TextVerbosity != "low" {
		t.Fatalf("unexpected gpt-5 textVerbosity: %q", settings[1].TextVerbosity)
	}
	if settings[1].ParallelToolCalls == nil || !*settings[1].ParallelToolCalls {
		t.Fatalf("expected gpt-5 parallelToolCalls=true")
	}
	if settings[1].EnableEncryptedReason == nil || !*settings[1].EnableEncryptedReason {
		t.Fatalf("expected gpt-5 enableEncryptedReasoning=true")
	}
	if settings[2].Name != "deepseek-v4" {
		t.Fatalf("unexpected third builtin name: %q", settings[2].Name)
	}
	if settings[2].ContextLimit != 800000 {
		t.Fatalf("unexpected deepseek-v4 contextLimit: %d", settings[2].ContextLimit)
	}
	if settings[2].SystemPromptPlacement != "system" {
		t.Fatalf("unexpected deepseek-v4 systemPromptPlacement: %q", settings[2].SystemPromptPlacement)
	}
	if settings[2].TextVerbosity != "low" {
		t.Fatalf("unexpected deepseek-v4 textVerbosity: %q", settings[2].TextVerbosity)
	}
}

func TestDefaultModelConfigApplyModeCanBeUpdated(t *testing.T) {
	original := GetDefaultModelConfigApplyMode()
	defer SetDefaultModelConfigApplyMode(original)

	SetDefaultModelConfigApplyMode(DefaultModelConfigApplyModeForceOverwrite)
	if got := GetDefaultModelConfigApplyMode(); got != DefaultModelConfigApplyModeForceOverwrite {
		t.Fatalf("unexpected mode after set: %q", got)
	}

	SetDefaultModelConfigApplyMode("unexpected")
	if got := GetDefaultModelConfigApplyMode(); got != DefaultModelConfigApplyModeInitIfMissing {
		t.Fatalf("unexpected fallback mode: %q", got)
	}
}
