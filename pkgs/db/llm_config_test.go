package database

import (
	"testing"

	defaultconfig "matrixops-agent/default_config"
	"pkgs/db/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openLLMConfigTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&models.ModelSettings{}); err != nil {
		t.Fatalf("migrate model settings: %v", err)
	}
	return db
}

func TestBuildProviderModelSettingsKey(t *testing.T) {
	if got := BuildProviderModelSettingsKey("openai", "gpt-5.4"); got != "openai/gpt-5.4" {
		t.Fatalf("unexpected key: %q", got)
	}
	if got := BuildProviderModelSettingsKey("", "gpt-5.4"); got != "gpt-5.4" {
		t.Fatalf("unexpected fallback key: %q", got)
	}
}

func TestGetModelSettingsByProviderAndModelPrefersProviderSpecificKey(t *testing.T) {
	db := openLLMConfigTestDB(t)

	if err := db.Create(&models.ModelSettings{Name: "gpt-5.4", Prompt: "legacy"}).Error; err != nil {
		t.Fatalf("create legacy model settings: %v", err)
	}
	if err := db.Create(&models.ModelSettings{Name: "openai/gpt-5.4", Prompt: "provider-specific"}).Error; err != nil {
		t.Fatalf("create provider model settings: %v", err)
	}

	setting, err := GetModelSettingsByProviderAndModel(db, "openai", "gpt-5.4")
	if err != nil {
		t.Fatalf("get settings: %v", err)
	}
	if setting.Prompt != "provider-specific" {
		t.Fatalf("prompt = %q, want %q", setting.Prompt, "provider-specific")
	}
}

func TestGetModelSettingsByProviderAndModelFallsBackToLegacyModelName(t *testing.T) {
	db := openLLMConfigTestDB(t)

	if err := db.Create(&models.ModelSettings{Name: "gpt-5.4", Prompt: "legacy"}).Error; err != nil {
		t.Fatalf("create legacy model settings: %v", err)
	}

	setting, err := GetModelSettingsByProviderAndModel(db, "openai", "gpt-5.4")
	if err != nil {
		t.Fatalf("get settings: %v", err)
	}
	if setting.Prompt != "legacy" {
		t.Fatalf("prompt = %q, want %q", setting.Prompt, "legacy")
	}
}

func TestEnsureDefaultModelSettingsInitIfMissing(t *testing.T) {
	db := openLLMConfigTestDB(t)
	original := defaultconfig.GetDefaultModelConfigApplyMode()
	defer defaultconfig.SetDefaultModelConfigApplyMode(original)
	defaultconfig.SetDefaultModelConfigApplyMode(defaultconfig.DefaultModelConfigApplyModeInitIfMissing)

	if err := EnsureDefaultModelSettings(db); err != nil {
		t.Fatalf("EnsureDefaultModelSettings: %v", err)
	}
	setting, err := GetDefaultModelSettings(db)
	if err != nil {
		t.Fatalf("GetDefaultModelSettings: %v", err)
	}
	if setting.Name != DefaultModelSettingsName {
		t.Fatalf("unexpected default name: %q", setting.Name)
	}
	gpt5Setting, err := GetModelSettingsByName(db, "gpt-5")
	if err != nil {
		t.Fatalf("GetModelSettingsByName(gpt-5): %v", err)
	}
	if gpt5Setting.ContextLimit != 300000 {
		t.Fatalf("unexpected gpt-5 contextLimit: %d", gpt5Setting.ContextLimit)
	}
}

func TestEnsureDefaultModelSettingsForceOverwrite(t *testing.T) {
	db := openLLMConfigTestDB(t)
	original := defaultconfig.GetDefaultModelConfigApplyMode()
	defer defaultconfig.SetDefaultModelConfigApplyMode(original)

	if err := db.Create(&models.ModelSettings{
		Name:                  DefaultModelSettingsName,
		Prompt:                "custom prompt",
		SystemPromptPlacement: "user_input",
		NativeOpenAIToolCalls: false,
	}).Error; err != nil {
		t.Fatalf("create existing default model settings: %v", err)
	}

	defaultconfig.SetDefaultModelConfigApplyMode(defaultconfig.DefaultModelConfigApplyModeForceOverwrite)
	if err := EnsureDefaultModelSettings(db); err != nil {
		t.Fatalf("EnsureDefaultModelSettings: %v", err)
	}
	setting, err := GetDefaultModelSettings(db)
	if err != nil {
		t.Fatalf("GetDefaultModelSettings: %v", err)
	}
	if setting.Prompt != "" {
		t.Fatalf("expected prompt to be overwritten from yaml, got %q", setting.Prompt)
	}
	if setting.SystemPromptPlacement != "system" {
		t.Fatalf("expected systemPromptPlacement to be overwritten to system, got %q", setting.SystemPromptPlacement)
	}
	if !setting.NativeOpenAIToolCalls {
		t.Fatal("expected nativeOpenAIToolCalls to be overwritten to true")
	}
	gpt5Setting, err := GetModelSettingsByName(db, "gpt-5")
	if err != nil {
		t.Fatalf("GetModelSettingsByName(gpt-5): %v", err)
	}
	if err := db.Save(&models.ModelSettings{
		Name:                  "gpt-5",
		ContextLimit:          1,
		OutputLimit:           3,
		Prompt:                "custom",
		SystemPromptPlacement: "user_input",
		NativeOpenAIToolCalls: false,
	}).Error; err != nil {
		t.Fatalf("overwrite gpt-5 seed: %v", err)
	}

	defaultconfig.SetDefaultModelConfigApplyMode(defaultconfig.DefaultModelConfigApplyModeForceOverwrite)
	if err := EnsureDefaultModelSettings(db); err != nil {
		t.Fatalf("EnsureDefaultModelSettings second run: %v", err)
	}
	gpt5Setting, err = GetModelSettingsByName(db, "gpt-5")
	if err != nil {
		t.Fatalf("GetModelSettingsByName(gpt-5) second run: %v", err)
	}
	if gpt5Setting.ContextLimit != 300000 {
		t.Fatalf("expected gpt-5 contextLimit to be overwritten to 300000, got %d", gpt5Setting.ContextLimit)
	}
	if gpt5Setting.SystemPromptPlacement != "instruction" {
		t.Fatalf("expected gpt-5 systemPromptPlacement to be overwritten to instruction, got %q", gpt5Setting.SystemPromptPlacement)
	}
	if !gpt5Setting.NativeOpenAIToolCalls {
		t.Fatal("expected gpt-5 nativeOpenAIToolCalls to be overwritten to true")
	}
}
