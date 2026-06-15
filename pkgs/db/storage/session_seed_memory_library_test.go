package storage

import (
	"testing"

	"pkgs/db/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestNewSessionSeedsProjectMemoryLibraryConversation(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("MATRIXOPS_HOME", tmp)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := InitStorage(db); err != nil {
		t.Fatalf("init storage: %v", err)
	}
	if err := db.AutoMigrate(&models.Project{}, &models.MemoryLibrary{}); err != nil {
		t.Fatalf("migrate project tables: %v", err)
	}

	project := &models.Project{
		ID:               1,
		Name:             "demo-project",
		Path:             tmp,
		WorktreePath:     tmp,
		MemoryLibraryIDs: models.UintSlice{1, 2},
	}
	if err := db.Create(project).Error; err != nil {
		t.Fatalf("create project: %v", err)
	}
	if err := db.Create(&models.MemoryLibrary{ID: 1, Name: "lib-1", Content: "记忆一"}).Error; err != nil {
		t.Fatalf("create library 1: %v", err)
	}
	if err := db.Create(&models.MemoryLibrary{ID: 2, Name: "lib-2", Content: "记忆二"}).Error; err != nil {
		t.Fatalf("create library 2: %v", err)
	}

	session, err := NewSession(db, "1", tmp)
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}

	messages, err := GetSessionMessageParts(db, session.ID)
	if err != nil {
		t.Fatalf("GetSessionMessageParts: %v", err)
	}
	if len(messages) != 4 {
		t.Fatalf("message count = %d, want 4", len(messages))
	}
	if messages[0].Info.Role != "user" || messages[0].Parts[0].Text != "总结一下。" {
		t.Fatalf("unexpected first message: %#v", messages[0])
	}
	if messages[1].Info.Role != "assistant" || messages[1].Parts[0].Text != "记忆一" {
		t.Fatalf("unexpected second message: %#v", messages[1])
	}
	if messages[2].Info.Role != "user" || messages[2].Parts[0].Text != "总结一下。" {
		t.Fatalf("unexpected third message: %#v", messages[2])
	}
	if messages[3].Info.Role != "assistant" || messages[3].Parts[0].Text != "记忆二" {
		t.Fatalf("unexpected fourth message: %#v", messages[3])
	}

	entries, err := ListMemoryEntriesBySession(db, session.ID)
	if err != nil {
		t.Fatalf("ListMemoryEntriesBySession: %v", err)
	}
	if len(entries) != 4 {
		t.Fatalf("memory entry count = %d, want 4", len(entries))
	}
	if entries[0].Role != "user" || entries[1].Role != "assistant" || entries[2].Role != "user" || entries[3].Role != "assistant" {
		t.Fatalf("unexpected memory entry roles: %#v", entries)
	}
}
