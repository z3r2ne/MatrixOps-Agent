package database

import (
	"testing"

	"pkgs/db/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestGetAgentMaxStepsUsesDefaultWhenUnset(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&models.GlobalConfig{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if got := GetAgentMaxSteps(db); got != models.DefaultAgentMaxSteps {
		t.Fatalf("got %d, want %d", got, models.DefaultAgentMaxSteps)
	}
}

func TestGetAgentMaxStepsReadsConfiguredValue(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&models.GlobalConfig{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := db.Create(&models.GlobalConfig{
		Key:   models.ConfigKeyAgentMaxSteps,
		Value: "42",
	}).Error; err != nil {
		t.Fatalf("create config: %v", err)
	}
	if got := GetAgentMaxSteps(db); got != 42 {
		t.Fatalf("got %d, want 42", got)
	}
}
