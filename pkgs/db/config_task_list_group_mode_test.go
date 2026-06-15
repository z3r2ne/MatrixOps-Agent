package database

import (
	"testing"

	"pkgs/db/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestGetDefaultTaskListGroupMode(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&models.GlobalConfig{}); err != nil {
		t.Fatalf("migrate global config: %v", err)
	}

	if got := GetDefaultTaskListGroupMode(nil); got != models.DefaultTaskListGroupMode {
		t.Fatalf("nil db: got %q want %q", got, models.DefaultTaskListGroupMode)
	}
	if got := GetDefaultTaskListGroupMode(db); got != models.DefaultTaskListGroupMode {
		t.Fatalf("empty db: got %q want %q", got, models.DefaultTaskListGroupMode)
	}

	if err := UpsertGlobalConfig(db, models.ConfigKeyDefaultTaskListGroupMode, "invalid"); err != nil {
		t.Fatalf("seed invalid config: %v", err)
	}
	if got := GetDefaultTaskListGroupMode(db); got != models.DefaultTaskListGroupMode {
		t.Fatalf("invalid config: got %q want %q", got, models.DefaultTaskListGroupMode)
	}

	if err := UpsertGlobalConfig(db, models.ConfigKeyDefaultTaskListGroupMode, string(models.TaskListGroupModeDate)); err != nil {
		t.Fatalf("seed valid config: %v", err)
	}
	if got := GetDefaultTaskListGroupMode(db); got != models.TaskListGroupModeDate {
		t.Fatalf("valid config: got %q want %q", got, models.TaskListGroupModeDate)
	}
}
