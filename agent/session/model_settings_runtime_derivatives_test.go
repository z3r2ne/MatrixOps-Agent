package session

import (
	"testing"

	"pkgs/db/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func boolPtr(b bool) *bool { return &b }

func strPtr(s string) *string { return &s }

// TestComputeModelSettingsRuntimeDerivativesEachField 逐项校验 ModelSettings 各字段是否反映到任务循环派生值。
func TestComputeModelSettingsRuntimeDerivativesEachField(t *testing.T) {
	t.Run("ReasoningEffort_nil", func(t *testing.T) {
		d := computeModelSettingsRuntimeDerivatives(&models.ModelSettings{}, 0, nil)
		if d.ReasoningEffort != "" {
			t.Fatalf("ReasoningEffort = %q, want empty", d.ReasoningEffort)
		}
	})
	t.Run("ReasoningEffort_high", func(t *testing.T) {
		d := computeModelSettingsRuntimeDerivatives(&models.ModelSettings{ReasoningEffort: strPtr("high")}, 0, nil)
		if d.ReasoningEffort != "high" {
			t.Fatalf("ReasoningEffort = %q, want high", d.ReasoningEffort)
		}
	})
	t.Run("ReasoningEffort_invalid", func(t *testing.T) {
		d := computeModelSettingsRuntimeDerivatives(&models.ModelSettings{ReasoningEffort: strPtr("not-a-level")}, 0, nil)
		if d.ReasoningEffort != "" {
			t.Fatalf("ReasoningEffort = %q, want empty for invalid", d.ReasoningEffort)
		}
	})

	t.Run("TextVerbosity_nil", func(t *testing.T) {
		d := computeModelSettingsRuntimeDerivatives(&models.ModelSettings{}, 0, nil)
		if d.TextVerbosity != "" {
			t.Fatalf("TextVerbosity = %q, want empty", d.TextVerbosity)
		}
	})
	t.Run("TextVerbosity_low", func(t *testing.T) {
		d := computeModelSettingsRuntimeDerivatives(&models.ModelSettings{TextVerbosity: strPtr("low")}, 0, nil)
		if d.TextVerbosity != "low" {
			t.Fatalf("TextVerbosity = %q, want low", d.TextVerbosity)
		}
	})
	t.Run("TextVerbosity_invalid", func(t *testing.T) {
		d := computeModelSettingsRuntimeDerivatives(&models.ModelSettings{TextVerbosity: strPtr("not-a-verbosity")}, 0, nil)
		if d.TextVerbosity != "" {
			t.Fatalf("TextVerbosity = %q, want empty for invalid value", d.TextVerbosity)
		}
	})

	t.Run("EnableEncryptedReasoning_nil", func(t *testing.T) {
		d := computeModelSettingsRuntimeDerivatives(&models.ModelSettings{}, 0, nil)
		if d.EnableEncryptedReasoning {
			t.Fatal("EnableEncryptedReasoning should be false when nil")
		}
	})
	t.Run("EnableEncryptedReasoning_false", func(t *testing.T) {
		d := computeModelSettingsRuntimeDerivatives(&models.ModelSettings{EnableEncryptedReason: boolPtr(false)}, 0, nil)
		if d.EnableEncryptedReasoning {
			t.Fatal("EnableEncryptedReasoning should be false")
		}
	})
	t.Run("EnableEncryptedReasoning_true", func(t *testing.T) {
		d := computeModelSettingsRuntimeDerivatives(&models.ModelSettings{EnableEncryptedReason: boolPtr(true)}, 0, nil)
		if !d.EnableEncryptedReasoning {
			t.Fatal("EnableEncryptedReasoning should be true")
		}
	})

	t.Run("ParallelToolCalls_nil", func(t *testing.T) {
		d := computeModelSettingsRuntimeDerivatives(&models.ModelSettings{}, 0, nil)
		if d.ParallelToolCalls {
			t.Fatal("ParallelToolCalls should be false when nil")
		}
	})
	t.Run("ParallelToolCalls_true", func(t *testing.T) {
		d := computeModelSettingsRuntimeDerivatives(&models.ModelSettings{ParallelToolCalls: boolPtr(true)}, 0, nil)
		if !d.ParallelToolCalls {
			t.Fatal("ParallelToolCalls should be true")
		}
	})

	t.Run("SilentToolWatchdogEnabled_nil", func(t *testing.T) {
		d := computeModelSettingsRuntimeDerivatives(&models.ModelSettings{}, 0, nil)
		if d.SilentToolWatchdogEnabled {
			t.Fatal("SilentToolWatchdogEnabled should be false when nil")
		}
	})
	t.Run("SilentToolWatchdogEnabled_true", func(t *testing.T) {
		d := computeModelSettingsRuntimeDerivatives(&models.ModelSettings{EnableSilentToolWatchdog: boolPtr(true)}, 0, nil)
		if !d.SilentToolWatchdogEnabled {
			t.Fatal("SilentToolWatchdogEnabled should be true")
		}
	})

	t.Run("PromptCacheKey_disabled", func(t *testing.T) {
		db := openDerivativesTestDB(t)
		d := computeModelSettingsRuntimeDerivatives(&models.ModelSettings{
			EnablePromptCacheKey: boolPtr(false),
		}, 1, db)
		if d.PromptCacheKey != "" {
			t.Fatalf("PromptCacheKey = %q, want empty", d.PromptCacheKey)
		}
	})
	t.Run("PromptCacheKey_enabled_with_task", func(t *testing.T) {
		db := openDerivativesTestDB(t)
		p := models.Project{Name: "proj", Path: "/tmp/x"}
		if err := db.Create(&p).Error; err != nil {
			t.Fatalf("create project: %v", err)
		}
		task := models.Task{
			ProjectID:      p.ID,
			Content:        "x",
			PromptCacheKey: "cache-key-xyz",
		}
		if err := db.Create(&task).Error; err != nil {
			t.Fatalf("create task: %v", err)
		}
		d := computeModelSettingsRuntimeDerivatives(&models.ModelSettings{
			EnablePromptCacheKey: boolPtr(true),
		}, task.ID, db)
		if d.PromptCacheKey != "cache-key-xyz" {
			t.Fatalf("PromptCacheKey = %q", d.PromptCacheKey)
		}
	})
	t.Run("PromptCacheKey_enabled_taskID_zero", func(t *testing.T) {
		db := openDerivativesTestDB(t)
		d := computeModelSettingsRuntimeDerivatives(&models.ModelSettings{
			EnablePromptCacheKey: boolPtr(true),
		}, 0, db)
		if d.PromptCacheKey != "" {
			t.Fatalf("PromptCacheKey = %q, want empty when taskID=0", d.PromptCacheKey)
		}
	})

	t.Run("ThinkingType_empty", func(t *testing.T) {
		d := computeModelSettingsRuntimeDerivatives(&models.ModelSettings{}, 0, nil)
		if d.ThinkingType != "" {
			t.Fatalf("ThinkingType = %q, want empty", d.ThinkingType)
		}
	})
	// EnableThinking 与 ThinkingType 独立；派生里只有 ThinkingType 进入 thinking.type，enable_thinking 由 RuntimeConfig 直传请求体。
	t.Run("EnableThinking_does_not_change_deriv_ThinkingType", func(t *testing.T) {
		d := computeModelSettingsRuntimeDerivatives(&models.ModelSettings{
			ThinkingType:   "",
			EnableThinking: boolPtr(true),
		}, 0, nil)
		if d.ThinkingType != "" {
			t.Fatalf("ThinkingType = %q, want empty when only EnableThinking set", d.ThinkingType)
		}
	})
	t.Run("ThinkingType_enabled", func(t *testing.T) {
		d := computeModelSettingsRuntimeDerivatives(&models.ModelSettings{ThinkingType: "enabled"}, 0, nil)
		if d.ThinkingType != models.LLMThinkingTypeEnabled {
			t.Fatalf("ThinkingType = %q", d.ThinkingType)
		}
	})
	t.Run("ThinkingType_disabled", func(t *testing.T) {
		d := computeModelSettingsRuntimeDerivatives(&models.ModelSettings{ThinkingType: "DISABLED"}, 0, nil)
		if d.ThinkingType != models.LLMThinkingTypeDisabled {
			t.Fatalf("ThinkingType = %q", d.ThinkingType)
		}
	})
	t.Run("BudgetTokens_passthrough", func(t *testing.T) {
		v := 4096
		d := computeModelSettingsRuntimeDerivatives(&models.ModelSettings{BudgetTokens: &v}, 0, nil)
		if d.BudgetTokens == nil || *d.BudgetTokens != v {
			t.Fatalf("BudgetTokens = %#v, want %d", d.BudgetTokens, v)
		}
	})

	t.Run("AutoCompressionLimitTokens_zero_context", func(t *testing.T) {
		d := computeModelSettingsRuntimeDerivatives(&models.ModelSettings{ContextLimit: 0}, 0, nil)
		if d.AutoCompressionLimitTokens != 0 {
			t.Fatalf("AutoCompressionLimitTokens = %d, want 0", d.AutoCompressionLimitTokens)
		}
	})
	t.Run("AutoCompressionLimitTokens_positive", func(t *testing.T) {
		d := computeModelSettingsRuntimeDerivatives(&models.ModelSettings{ContextLimit: 128000}, 0, nil)
		if d.AutoCompressionLimitTokens != 128000 {
			t.Fatalf("AutoCompressionLimitTokens = %d, want 128000", d.AutoCompressionLimitTokens)
		}
	})

	t.Run("nil_model_settings", func(t *testing.T) {
		d := computeModelSettingsRuntimeDerivatives(nil, 99, nil)
		if d.ReasoningEffort != "" || d.TextVerbosity != "" || d.EnableEncryptedReasoning || d.ParallelToolCalls ||
			d.SilentToolWatchdogEnabled || d.PromptCacheKey != "" || d.ThinkingType != "" || d.BudgetTokens != nil ||
			d.AutoCompressionLimitTokens != 0 {
			t.Fatalf("expected zero derivatives, got %+v", d)
		}
	})
}

func openDerivativesTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.Project{}, &models.Task{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}
