package database

import (
	"testing"

	defaultconfig "matrixops-agent/default_config"
	"pkgs/db/models"
)

func TestNormalizeModelSettingsEachField(t *testing.T) {
	t.Run("SystemPromptPlacement_keeps_instruction", func(t *testing.T) {
		s := &models.ModelSettings{SystemPromptPlacement: "instruction"}
		normalizeModelSettings(s)
		if s.SystemPromptPlacement != "instruction" {
			t.Fatalf("got %q", s.SystemPromptPlacement)
		}
	})
	t.Run("SystemPromptPlacement_keeps_user_input", func(t *testing.T) {
		s := &models.ModelSettings{SystemPromptPlacement: "user_input"}
		normalizeModelSettings(s)
		if s.SystemPromptPlacement != "user_input" {
			t.Fatalf("got %q", s.SystemPromptPlacement)
		}
	})
	t.Run("SystemPromptPlacement_invalid_becomes_system", func(t *testing.T) {
		s := &models.ModelSettings{SystemPromptPlacement: "bogus"}
		normalizeModelSettings(s)
		if s.SystemPromptPlacement != "system" {
			t.Fatalf("got %q, want system", s.SystemPromptPlacement)
		}
	})

	t.Run("ReasoningEffort_valid_kept", func(t *testing.T) {
		v := "xhigh"
		s := &models.ModelSettings{ReasoningEffort: &v}
		normalizeModelSettings(s)
		if s.ReasoningEffort == nil || *s.ReasoningEffort != "xhigh" {
			t.Fatalf("got %#v", s.ReasoningEffort)
		}
	})
	t.Run("ReasoningEffort_invalid_cleared", func(t *testing.T) {
		v := "not-valid"
		s := &models.ModelSettings{ReasoningEffort: &v}
		normalizeModelSettings(s)
		if s.ReasoningEffort != nil {
			t.Fatalf("expected nil, got %#v", s.ReasoningEffort)
		}
	})

	t.Run("TextVerbosity_valid_kept", func(t *testing.T) {
		v := "medium"
		s := &models.ModelSettings{TextVerbosity: &v}
		normalizeModelSettings(s)
		if s.TextVerbosity == nil || *s.TextVerbosity != "medium" {
			t.Fatalf("got %#v", s.TextVerbosity)
		}
	})
	t.Run("TextVerbosity_invalid_cleared", func(t *testing.T) {
		v := "none"
		s := &models.ModelSettings{TextVerbosity: &v}
		normalizeModelSettings(s)
		if s.TextVerbosity != nil {
			t.Fatalf("expected nil, got %#v", s.TextVerbosity)
		}
	})

	t.Run("ThinkingType_normalized", func(t *testing.T) {
		s := &models.ModelSettings{ThinkingType: "ENABLED"}
		normalizeModelSettings(s)
		if s.ThinkingType != models.LLMThinkingTypeEnabled {
			t.Fatalf("got %q", s.ThinkingType)
		}
	})
	t.Run("BudgetTokens_non_positive_cleared", func(t *testing.T) {
		v := 0
		s := &models.ModelSettings{BudgetTokens: &v}
		normalizeModelSettings(s)
		if s.BudgetTokens != nil {
			t.Fatalf("expected nil, got %#v", s.BudgetTokens)
		}
	})
	t.Run("BudgetTokens_positive_kept", func(t *testing.T) {
		v := 4096
		s := &models.ModelSettings{BudgetTokens: &v}
		normalizeModelSettings(s)
		if s.BudgetTokens == nil || *s.BudgetTokens != v {
			t.Fatalf("got %#v", s.BudgetTokens)
		}
	})
}

// TestEnsureModelSettingsEachEmbeddedFieldRoundTrip 逐项校验内置 DefaultModelSettings 各字段经 ensureModelSettings 写入 DB 后读回一致。
func TestEnsureModelSettingsEachEmbeddedFieldRoundTrip(t *testing.T) {
	db := openLLMConfigTestDB(t)
	origMode := defaultconfig.GetDefaultModelConfigApplyMode()
	defer defaultconfig.SetDefaultModelConfigApplyMode(origMode)
	defaultconfig.SetDefaultModelConfigApplyMode(defaultconfig.DefaultModelConfigApplyModeForceOverwrite)

	enc := true
	par := false
	pcache := true
	src := &defaultconfig.DefaultModelSettings{
		Name:                  "all_fields_embedded_roundtrip",
		ContextLimit:          111222,
		OutputLimit:           33344,
		Prompt:                "model-specific system fragment",
		SystemPromptPlacement: "instruction",
		NativeOpenAIToolCalls: true,
		ReasoningEffort:       "high",
		TextVerbosity:         "low",
		EnableEncryptedReason: &enc,
		ParallelToolCalls:     &par,
		EnablePromptCacheKey:  &pcache,
	}
	if err := ensureModelSettings(db, src); err != nil {
		t.Fatalf("ensureModelSettings (create): %v", err)
	}
	got, err := GetModelSettingsByName(db, src.Name)
	if err != nil {
		t.Fatalf("GetModelSettingsByName: %v", err)
	}

	t.Run("Name", func(t *testing.T) {
		if got.Name != src.Name {
			t.Fatalf("Name = %q, want %q", got.Name, src.Name)
		}
	})
	t.Run("ContextLimit", func(t *testing.T) {
		if got.ContextLimit != src.ContextLimit {
			t.Fatalf("ContextLimit = %d, want %d", got.ContextLimit, src.ContextLimit)
		}
	})
	t.Run("OutputLimit", func(t *testing.T) {
		if got.OutputLimit != src.OutputLimit {
			t.Fatalf("OutputLimit = %d, want %d", got.OutputLimit, src.OutputLimit)
		}
	})
	t.Run("Prompt", func(t *testing.T) {
		if got.Prompt != src.Prompt {
			t.Fatalf("Prompt mismatch")
		}
	})
	t.Run("SystemPromptPlacement", func(t *testing.T) {
		if got.SystemPromptPlacement != src.SystemPromptPlacement {
			t.Fatalf("SystemPromptPlacement = %q, want %q", got.SystemPromptPlacement, src.SystemPromptPlacement)
		}
	})
	t.Run("NativeOpenAIToolCalls", func(t *testing.T) {
		if got.NativeOpenAIToolCalls != src.NativeOpenAIToolCalls {
			t.Fatalf("NativeOpenAIToolCalls = %v, want %v", got.NativeOpenAIToolCalls, src.NativeOpenAIToolCalls)
		}
	})
	t.Run("ReasoningEffort", func(t *testing.T) {
		if got.ReasoningEffort == nil || *got.ReasoningEffort != src.ReasoningEffort {
			t.Fatalf("ReasoningEffort = %#v, want ptr %q", got.ReasoningEffort, src.ReasoningEffort)
		}
	})
	t.Run("TextVerbosity", func(t *testing.T) {
		if got.TextVerbosity == nil || *got.TextVerbosity != src.TextVerbosity {
			t.Fatalf("TextVerbosity = %#v, want ptr %q", got.TextVerbosity, src.TextVerbosity)
		}
	})
	t.Run("EnableEncryptedReasoning", func(t *testing.T) {
		if got.EnableEncryptedReason == nil || *got.EnableEncryptedReason != *src.EnableEncryptedReason {
			t.Fatalf("EnableEncryptedReason = %#v", got.EnableEncryptedReason)
		}
	})
	t.Run("ParallelToolCalls", func(t *testing.T) {
		if got.ParallelToolCalls == nil || *got.ParallelToolCalls != *src.ParallelToolCalls {
			t.Fatalf("ParallelToolCalls = %#v", got.ParallelToolCalls)
		}
	})
	t.Run("EnablePromptCacheKey", func(t *testing.T) {
		if got.EnablePromptCacheKey == nil || *got.EnablePromptCacheKey != *src.EnablePromptCacheKey {
			t.Fatalf("EnablePromptCacheKey = %#v", got.EnablePromptCacheKey)
		}
	})

	// 覆盖路径：修改嵌入源后再次 ensure，逐项确认被更新。
	src.ContextLimit = 999001
	src.OutputLimit = 888002
	src.Prompt = "updated prompt"
	src.SystemPromptPlacement = "user_input"
	src.NativeOpenAIToolCalls = false
	src.ReasoningEffort = "low"
	src.TextVerbosity = "xhigh"
	enc2, par2, pc2 := false, true, false
	src.EnableEncryptedReason = &enc2
	src.ParallelToolCalls = &par2
	src.EnablePromptCacheKey = &pc2

	if err := ensureModelSettings(db, src); err != nil {
		t.Fatalf("ensureModelSettings (overwrite): %v", err)
	}
	got2, err := GetModelSettingsByName(db, src.Name)
	if err != nil {
		t.Fatalf("GetModelSettingsByName after overwrite: %v", err)
	}

	t.Run("overwrite_ContextLimit", func(t *testing.T) {
		if got2.ContextLimit != 999001 {
			t.Fatalf("ContextLimit = %d", got2.ContextLimit)
		}
	})
	t.Run("overwrite_OutputLimit", func(t *testing.T) {
		if got2.OutputLimit != 888002 {
			t.Fatalf("OutputLimit = %d", got2.OutputLimit)
		}
	})
	t.Run("overwrite_Prompt", func(t *testing.T) {
		if got2.Prompt != "updated prompt" {
			t.Fatalf("Prompt = %q", got2.Prompt)
		}
	})
	t.Run("overwrite_SystemPromptPlacement", func(t *testing.T) {
		if got2.SystemPromptPlacement != "user_input" {
			t.Fatalf("SystemPromptPlacement = %q", got2.SystemPromptPlacement)
		}
	})
	t.Run("overwrite_NativeOpenAIToolCalls", func(t *testing.T) {
		if got2.NativeOpenAIToolCalls {
			t.Fatal("expected NativeOpenAIToolCalls false")
		}
	})
	t.Run("overwrite_ReasoningEffort", func(t *testing.T) {
		if got2.ReasoningEffort == nil || *got2.ReasoningEffort != "low" {
			t.Fatalf("ReasoningEffort = %#v", got2.ReasoningEffort)
		}
	})
	t.Run("overwrite_TextVerbosity", func(t *testing.T) {
		if got2.TextVerbosity == nil || *got2.TextVerbosity != "xhigh" {
			t.Fatalf("TextVerbosity = %#v", got2.TextVerbosity)
		}
	})
	t.Run("overwrite_EnableEncryptedReasoning", func(t *testing.T) {
		if got2.EnableEncryptedReason == nil || *got2.EnableEncryptedReason {
			t.Fatalf("EnableEncryptedReason = %#v", got2.EnableEncryptedReason)
		}
	})
	t.Run("overwrite_ParallelToolCalls", func(t *testing.T) {
		if got2.ParallelToolCalls == nil || !*got2.ParallelToolCalls {
			t.Fatalf("ParallelToolCalls = %#v", got2.ParallelToolCalls)
		}
	})
	t.Run("overwrite_EnablePromptCacheKey", func(t *testing.T) {
		if got2.EnablePromptCacheKey == nil || *got2.EnablePromptCacheKey {
			t.Fatalf("EnablePromptCacheKey = %#v", got2.EnablePromptCacheKey)
		}
	})
}

// TestModelSettingsExtendedDBFieldsRoundTrip 校验仅存在于 DB 模型、不由内置 YAML 提供的字段在读写后保持一致。
func TestModelSettingsExtendedDBFieldsRoundTrip(t *testing.T) {
	db := openLLMConfigTestDB(t)
	topP := 0.88
	topK := 42
	freq := 0.12
	enThink := true
	ms := &models.ModelSettings{
		Name:                  "extended_db_only_fields",
		ContextLimit:          1000,
		OutputLimit:           500,
		TopP:                  &topP,
		TopK:                  &topK,
		FrequencyPenalty:      &freq,
		EnableThinking:        &enThink,
		ThinkingType:          "disabled",
		Prompt:                "",
		SystemPromptPlacement: "system",
	}
	if err := CreateModelSettings(db, ms); err != nil {
		t.Fatalf("CreateModelSettings: %v", err)
	}
	got, err := GetModelSettingsByName(db, ms.Name)
	if err != nil {
		t.Fatalf("GetModelSettingsByName: %v", err)
	}

	t.Run("TopP", func(t *testing.T) {
		if got.TopP == nil || *got.TopP != topP {
			t.Fatalf("TopP = %#v", got.TopP)
		}
	})
	t.Run("TopK", func(t *testing.T) {
		if got.TopK == nil || *got.TopK != topK {
			t.Fatalf("TopK = %#v", got.TopK)
		}
	})
	t.Run("FrequencyPenalty", func(t *testing.T) {
		if got.FrequencyPenalty == nil || *got.FrequencyPenalty != freq {
			t.Fatalf("FrequencyPenalty = %#v", got.FrequencyPenalty)
		}
	})
	t.Run("EnableThinking", func(t *testing.T) {
		if got.EnableThinking == nil || !*got.EnableThinking {
			t.Fatalf("EnableThinking = %#v", got.EnableThinking)
		}
	})
	t.Run("ThinkingType_after_normalize_on_read", func(t *testing.T) {
		if got.ThinkingType != models.LLMThinkingTypeDisabled {
			t.Fatalf("ThinkingType = %q", got.ThinkingType)
		}
	})
}
