package session

import (
	"reflect"
	"testing"

	"pkgs/db/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestLoadCallableWorkerNamesFiltersHiddenWorkers(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&models.Worker{}); err != nil {
		t.Fatalf("migrate workers: %v", err)
	}

	fixtures := []models.Worker{
		{Name: "leader", Provider: "test", Model: "gpt-5.4"},
		{Name: "chat", Provider: "test", Model: "gpt-5.4"},
		{Name: "frontend_engineer", Provider: "test", Model: "gpt-5.4"},
		{Name: "compaction", Provider: "test", Model: "gpt-5.4", Hidden: true},
	}
	for _, worker := range fixtures {
		item := worker
		if err := db.Create(&item).Error; err != nil {
			t.Fatalf("create worker %s: %v", worker.Name, err)
		}
	}

	names := loadCallableWorkerNames(db)
	if !reflect.DeepEqual(names, []string{"chat", "frontend_engineer", "leader"}) {
		t.Fatalf("unexpected worker whitelist: %+v", names)
	}
}
