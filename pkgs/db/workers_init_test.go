package database

import (
	"testing"

	"pkgs/db/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func floatPtr(f float64) *float64 {
	return &f
}

const testBuiltInWorkerYAML = `
name: chat
provider: openai
model: gpt-5.4
description: builtin description
temperature: 0.2
workerPrompt: builtin prompt
occupation: analyst
enabledTools:
  - read
`

func openWorkerInitTestDB(t *testing.T) (*gorm.DB, *models.LLMConfig) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&models.LLMConfig{}, &models.Worker{}); err != nil {
		t.Fatalf("migrate worker tables: %v", err)
	}

	provider := &models.LLMConfig{
		Name:   "default-openai",
		Type:   "openai",
		APIKey: "test-key",
		Model:  "gpt-5.4",
	}
	if err := db.Create(provider).Error; err != nil {
		t.Fatalf("create llm config: %v", err)
	}

	return db, provider
}

func TestInitBuiltInWorkersCreatesMissingWorker(t *testing.T) {
	db, provider := openWorkerInitTestDB(t)

	if err := InitBuiltInWorkers(db, "gpt-5.4", provider, map[string][]byte{
		"chat.yaml": []byte(testBuiltInWorkerYAML),
	}); err != nil {
		t.Fatalf("InitBuiltInWorkers: %v", err)
	}

	worker, err := GetWorkerByName(db, "chat")
	if err != nil {
		t.Fatalf("GetWorkerByName: %v", err)
	}
	if worker.Description != "builtin description" {
		t.Fatalf("description = %q, want %q", worker.Description, "builtin description")
	}
	if worker.SystemPrompt != "builtin prompt" {
		t.Fatalf("system prompt = %q, want %q", worker.SystemPrompt, "builtin prompt")
	}
}

func TestInitBuiltInWorkersDoesNotOverwriteExistingWorker(t *testing.T) {
	db, provider := openWorkerInitTestDB(t)

	existing := &models.Worker{
		Name:         "chat",
		Provider:     "custom-provider",
		Model:        "custom-model",
		Description:  "custom description",
		Temperature:  floatPtr(0.9),
		SystemPrompt: "custom prompt",
		Occupation:   "general",
		LLMConfigID:  &provider.ID,
	}
	if err := db.Create(existing).Error; err != nil {
		t.Fatalf("create existing worker: %v", err)
	}

	if err := InitBuiltInWorkers(db, "gpt-5.4", provider, map[string][]byte{
		"chat.yaml": []byte(testBuiltInWorkerYAML),
	}); err != nil {
		t.Fatalf("InitBuiltInWorkers: %v", err)
	}

	worker, err := GetWorkerByName(db, "chat")
	if err != nil {
		t.Fatalf("GetWorkerByName: %v", err)
	}
	if worker.Provider != "custom-provider" {
		t.Fatalf("provider = %q, want %q", worker.Provider, "custom-provider")
	}
	if worker.Description != "custom description" {
		t.Fatalf("description = %q, want %q", worker.Description, "custom description")
	}
	if worker.SystemPrompt != "custom prompt" {
		t.Fatalf("system prompt = %q, want %q", worker.SystemPrompt, "custom prompt")
	}
}

func TestRestoreBuiltInWorkersOverwritesExistingWorker(t *testing.T) {
	db, provider := openWorkerInitTestDB(t)

	existing := &models.Worker{
		Name:         "chat",
		Provider:     "custom-provider",
		Model:        "custom-model",
		Description:  "custom description",
		Temperature:  floatPtr(0.9),
		SystemPrompt: "custom prompt",
		Occupation:   "general",
		LLMConfigID:  &provider.ID,
	}
	if err := db.Create(existing).Error; err != nil {
		t.Fatalf("create existing worker: %v", err)
	}

	if err := RestoreBuiltInWorkers(db, "gpt-5.4", provider, map[string][]byte{
		"chat.yaml": []byte(testBuiltInWorkerYAML),
	}); err != nil {
		t.Fatalf("RestoreBuiltInWorkers: %v", err)
	}

	worker, err := GetWorkerByName(db, "chat")
	if err != nil {
		t.Fatalf("GetWorkerByName: %v", err)
	}
	if worker.Provider != provider.Name {
		t.Fatalf("provider = %q, want %q", worker.Provider, provider.Name)
	}
	if worker.Description != "builtin description" {
		t.Fatalf("description = %q, want %q", worker.Description, "builtin description")
	}
	if worker.SystemPrompt != "builtin prompt" {
		t.Fatalf("system prompt = %q, want %q", worker.SystemPrompt, "builtin prompt")
	}
}
