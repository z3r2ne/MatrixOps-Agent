package session

import (
	"strings"
	"testing"

	database "pkgs/db"
	"pkgs/db/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestResolveMemoryCompactionRuntimeRequiresCompactionWorker(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&models.Worker{}, &models.LLMConfig{}); err != nil {
		t.Fatalf("migrate tables: %v", err)
	}

	llmConfig := &models.LLMConfig{Name: "chat-config", BaseURL: "http://chat.example", APIKey: "chat"}
	if err := db.Create(llmConfig).Error; err != nil {
		t.Fatalf("create chat config: %v", err)
	}
	if err := db.Create(&models.Worker{
		Name:        "chat",
		Provider:    "llm",
		Model:       "chat-model",
		LLMConfigID: &llmConfig.ID,
	}).Error; err != nil {
		t.Fatalf("create chat worker: %v", err)
	}

	_, err = ResolveMemoryCompactionRuntime(db)
	if err == nil {
		t.Fatal("expected error when compaction worker is missing")
	}
	if !strings.Contains(err.Error(), models.WorkerCompaction) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveMemoryCompactionRuntimePrefersCompactionWorker(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&models.Worker{}, &models.LLMConfig{}, &models.ModelSettings{}); err != nil {
		t.Fatalf("migrate tables: %v", err)
	}

	chatConfig := &models.LLMConfig{Name: "chat-config", BaseURL: "http://chat.example", APIKey: "chat"}
	compactionConfig := &models.LLMConfig{Name: "compaction-config", BaseURL: "http://compaction.example", APIKey: "compaction"}
	if err := db.Create(chatConfig).Error; err != nil {
		t.Fatalf("create chat config: %v", err)
	}
	if err := db.Create(compactionConfig).Error; err != nil {
		t.Fatalf("create compaction config: %v", err)
	}

	if err := db.Create(&models.Worker{
		Name:        "chat",
		Provider:    "llm",
		Model:       "chat-model",
		LLMConfigID: &chatConfig.ID,
	}).Error; err != nil {
		t.Fatalf("create chat worker: %v", err)
	}
	if err := db.Create(&models.Worker{
		Name:              models.WorkerCompaction,
		Provider:          "llm",
		Model:             "compaction-model",
		SystemPrompt:      "保留 API 变更",
		LLMConfigID:       &compactionConfig.ID,
		ModelSettingsName: database.DefaultModelSettingsName,
	}).Error; err != nil {
		t.Fatalf("create compaction worker: %v", err)
	}

	runtime, err := ResolveMemoryCompactionRuntime(db)
	if err != nil {
		t.Fatalf("ResolveMemoryCompactionRuntime: %v", err)
	}
	if runtime.Worker == nil || runtime.Worker.Name != models.WorkerCompaction {
		t.Fatalf("unexpected worker: %+v", runtime.Worker)
	}
	if runtime.LLMConfig == nil || runtime.LLMConfig.ID != compactionConfig.ID {
		t.Fatalf("unexpected llm config: %+v", runtime.LLMConfig)
	}
	if runtime.ModelName() != "compaction-model" {
		t.Fatalf("model = %q, want compaction-model", runtime.ModelName())
	}
	if runtime.WorkerExtraPrompt() != "保留 API 变更" {
		t.Fatalf("extra prompt = %q", runtime.WorkerExtraPrompt())
	}
}
