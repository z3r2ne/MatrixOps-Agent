package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	agentsession "matrixops-agent/session"
	"matrixops-agent/types"
	"pkgs/db/models"
	"pkgs/db/storage"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupSessionHandlerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	if err := storage.InitStorage(db); err != nil {
		t.Fatalf("init storage: %v", err)
	}

	return db
}

func TestResolveMemoryCompactionRuntimeRequiresCompactionWorker(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupSessionHandlerTestDB(t)
	if err := db.AutoMigrate(&models.Worker{}, &models.LLMConfig{}, &models.GlobalConfig{}, &models.ModelSettings{}); err != nil {
		t.Fatalf("migrate runtime config tables: %v", err)
	}

	llmConfig := &models.LLMConfig{Name: "test-config", BaseURL: "http://example.com", APIKey: "test"}
	if err := db.Create(llmConfig).Error; err != nil {
		t.Fatalf("create llm config: %v", err)
	}

	if err := db.Create(&models.Worker{
		Name:        "chat",
		Provider:    "llm",
		Model:       "gpt-5.4",
		LLMConfigID: &llmConfig.ID,
	}).Error; err != nil {
		t.Fatalf("create chat worker: %v", err)
	}

	_, err := agentsession.ResolveMemoryCompactionRuntime(db)
	if err == nil {
		t.Fatal("expected error when compaction worker is missing")
	}
	if !strings.Contains(err.Error(), models.WorkerCompaction) {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = context.Background()
}

func TestGetSessionMemoryReturnsHistory(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupSessionHandlerTestDB(t)
	sessionID := "session-test"

	session := &models.Session{
		ID:        sessionID,
		ProjectID: "project-1",
		Title:     "memory session",
		Version:   "1",
		Created:   1,
		Updated:   1,
	}
	if err := db.Create(session).Error; err != nil {
		t.Fatalf("create session: %v", err)
	}

	if err := storage.ReplaceSessionMemoryWithEntries(db, sessionID, []*types.MemoryEntry{
		{
			SessionID:  sessionID,
			EntryKind:  "history",
			Role:       "user",
			Content:    "remember this",
			Sequence:   1,
			TokenCount: 2,
			Created:    100,
			Updated:    100,
		},
	}); err != nil {
		t.Fatalf("replace session memory: %v", err)
	}

	router := gin.New()
	handler := NewSessionHandler(db)
	router.GET("/sessions/:id/memory", handler.GetSessionMemory)

	req := httptest.NewRequest(http.MethodGet, "/sessions/"+sessionID+"/memory", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.Code, resp.Body.String())
	}

	var body struct {
		Session struct {
			ID string `json:"id"`
		} `json:"session"`
		MemoryEntries []struct {
			Content string `json:"content"`
		} `json:"memoryEntries"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if body.Session.ID != sessionID {
		t.Fatalf("session id = %q, want %q", body.Session.ID, sessionID)
	}
	if len(body.MemoryEntries) != 1 || body.MemoryEntries[0].Content != "remember this" {
		t.Fatalf("unexpected memory entries: %+v", body.MemoryEntries)
	}
}
