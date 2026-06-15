package memory

import (
	"context"
	"path/filepath"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestInMemoryStore_SaveLoadDelete(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()
	value := &Memory{GlobalPrompt: "hello"}
	if err := store.Save(ctx, "s1", value); err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, err := store.Load(ctx, "s1")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.GlobalPrompt != "hello" {
		t.Fatalf("expected loaded prompt hello, got %q", loaded.GlobalPrompt)
	}
	if err := store.Delete(ctx, "s1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	loaded, err = store.Load(ctx, "s1")
	if err != nil {
		t.Fatalf("load after delete: %v", err)
	}
	if loaded.GlobalPrompt != "" {
		t.Fatalf("expected empty memory after delete, got %+v", loaded)
	}
}

func TestJSONFileStore_SaveLoad(t *testing.T) {
	store := NewJSONFileStore(filepath.Join(t.TempDir(), "memory.json"))
	ctx := context.Background()
	value := &Memory{ProjectPrompt: "proj"}
	if err := store.Save(ctx, "s1", value); err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, err := store.Load(ctx, "s1")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.ProjectPrompt != "proj" {
		t.Fatalf("expected loaded prompt proj, got %q", loaded.ProjectPrompt)
	}
}

func TestDBStore_SaveLoad(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "memory.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	store := NewDBStore(db)
	ctx := context.Background()
	value := &Memory{WorkerPrompt: "worker"}
	if err := store.Save(ctx, "s1", value); err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, err := store.Load(ctx, "s1")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.WorkerPrompt != "worker" {
		t.Fatalf("expected loaded prompt worker, got %q", loaded.WorkerPrompt)
	}
}
