package database

import (
	"testing"

	"pkgs/db/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestGetLLMMaxOutputTokens(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.GlobalConfig{}); err != nil {
		t.Fatal(err)
	}

	if got := GetLLMMaxOutputTokens(db); got != models.DefaultLLMMaxOutputTokens {
		t.Fatalf("empty db: got %d want %d", got, models.DefaultLLMMaxOutputTokens)
	}
	if got := GetLLMMaxOutputTokens(nil); got != models.DefaultLLMMaxOutputTokens {
		t.Fatalf("nil db: got %d", got)
	}

	if err := UpsertGlobalConfig(db, models.ConfigKeyLLMMaxOutputTokens, "not-a-number"); err != nil {
		t.Fatal(err)
	}
	if got := GetLLMMaxOutputTokens(db); got != models.DefaultLLMMaxOutputTokens {
		t.Fatalf("invalid value: got %d", got)
	}
	if err := UpsertGlobalConfig(db, models.ConfigKeyLLMMaxOutputTokens, "0"); err != nil {
		t.Fatal(err)
	}
	if got := GetLLMMaxOutputTokens(db); got != models.DefaultLLMMaxOutputTokens {
		t.Fatalf("zero: got %d", got)
	}
	if err := UpsertGlobalConfig(db, models.ConfigKeyLLMMaxOutputTokens, "8192"); err != nil {
		t.Fatal(err)
	}
	if got := GetLLMMaxOutputTokens(db); got != 8192 {
		t.Fatalf("configured: got %d", got)
	}
}

func TestEffectiveLLMMaxOutputTokens(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.GlobalConfig{}); err != nil {
		t.Fatal(err)
	}
	if got := EffectiveLLMMaxOutputTokens(db, 5000); got != 5000 {
		t.Fatalf("model limit: got %d", got)
	}
	if got := EffectiveLLMMaxOutputTokens(db, 0); got != models.DefaultLLMMaxOutputTokens {
		t.Fatalf("fallback global default: got %d", got)
	}
}
