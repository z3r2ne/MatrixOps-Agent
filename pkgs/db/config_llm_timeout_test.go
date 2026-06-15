package database

import (
	"testing"
	"time"

	"pkgs/db/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestGetLLMHTTPConnectTimeout(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&models.GlobalConfig{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if got := GetLLMHTTPConnectTimeout(db); got != 0 {
		t.Fatalf("missing key: got %v", got)
	}
	if err := UpsertGlobalConfig(db, models.ConfigKeyLLMHTTPConnectTimeoutSeconds, "not-a-number"); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if got := GetLLMHTTPConnectTimeout(db); got != 0 {
		t.Fatalf("invalid value: got %v", got)
	}
	if err := UpsertGlobalConfig(db, models.ConfigKeyLLMHTTPConnectTimeoutSeconds, "0"); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if got := GetLLMHTTPConnectTimeout(db); got != 0 {
		t.Fatalf("zero: got %v", got)
	}
	if err := UpsertGlobalConfig(db, models.ConfigKeyLLMHTTPConnectTimeoutSeconds, "120"); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if got := GetLLMHTTPConnectTimeout(db); got != 120*time.Second {
		t.Fatalf("got %v", got)
	}
}

func TestGetLLMHTTPClientTimeout(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&models.GlobalConfig{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if got := GetLLMHTTPClientTimeout(db); got != 0 {
		t.Fatalf("missing key: got %v", got)
	}
	if err := UpsertGlobalConfig(db, models.ConfigKeyLLMHTTPTimeoutSeconds, "not-a-number"); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if got := GetLLMHTTPClientTimeout(db); got != 0 {
		t.Fatalf("invalid value: got %v", got)
	}
	if err := UpsertGlobalConfig(db, models.ConfigKeyLLMHTTPTimeoutSeconds, "0"); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if got := GetLLMHTTPClientTimeout(db); got != 0 {
		t.Fatalf("zero: got %v", got)
	}
	if err := UpsertGlobalConfig(db, models.ConfigKeyLLMHTTPTimeoutSeconds, "120"); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if got := GetLLMHTTPClientTimeout(db); got != 120*time.Second {
		t.Fatalf("got %v", got)
	}
}
