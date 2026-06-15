package database

import (
	"testing"

	"pkgs/db/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestGetDefaultProjectToolPermissionsJSON(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&models.GlobalConfig{}); err != nil {
		t.Fatalf("migrate global config: %v", err)
	}

	if got := GetDefaultProjectToolPermissionsJSON(nil); got != "{}" {
		t.Fatalf("nil db: got %q want {}", got)
	}
	if got := GetDefaultProjectToolPermissionsJSON(db); got != "{}" {
		t.Fatalf("empty db: got %q want {}", got)
	}

	if err := UpsertGlobalConfig(db, models.ConfigKeyDefaultProjectToolPermissions, "invalid"); err != nil {
		t.Fatalf("seed invalid config: %v", err)
	}
	if got := GetDefaultProjectToolPermissionsJSON(db); got != "{}" {
		t.Fatalf("invalid config: got %q want {}", got)
	}

	if err := UpsertGlobalConfig(db, models.ConfigKeyDefaultProjectToolPermissions, `{"bash":"allow","write":"deny","bad":"oops"}`); err != nil {
		t.Fatalf("seed valid config: %v", err)
	}
	got := GetDefaultProjectToolPermissions(db)
	if got["bash"] != models.ProjectToolPermissionAllow {
		t.Fatalf("bash permission = %q", got["bash"])
	}
	if got["write"] != models.ProjectToolPermissionDeny {
		t.Fatalf("write permission = %q", got["write"])
	}
	if _, ok := got["bad"]; ok {
		t.Fatalf("unexpected invalid permission preserved")
	}
}
