package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"pkgs/db/models"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupWorkerHandlerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&models.ProviderSetting{}, &models.LLMConfig{}, &models.ModelSettings{}, &models.Worker{}); err != nil {
		t.Fatalf("migrate worker-related tables: %v", err)
	}
	return db
}

func TestBulkApplyConfigUpdatesSelectedWorkers(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupWorkerHandlerTestDB(t)
	llmConfig := models.LLMConfig{
		Name:  "team-openai",
		Type:  "openai",
		APIKey: "test-key",
		Model: "gpt-5.4,gpt-5.5",
	}
	if err := db.Create(&llmConfig).Error; err != nil {
		t.Fatalf("create llm config: %v", err)
	}
	if err := db.Create(&models.ModelSettings{Name: "default_model_config"}).Error; err != nil {
		t.Fatalf("create default model settings: %v", err)
	}

	workers := []models.Worker{
		{Name: "worker-a", Provider: "legacy", Model: "old-a", ModelSettingsName: "legacy_config"},
		{Name: "worker-b", Provider: "legacy", Model: "old-b", ModelSettingsName: "legacy_config"},
		{Name: "worker-c", Provider: "legacy", Model: "old-c", ModelSettingsName: "legacy_config"},
	}
	for i := range workers {
		if err := db.Create(&workers[i]).Error; err != nil {
			t.Fatalf("create worker %d: %v", i, err)
		}
	}

	router := gin.New()
	handler := NewWorkerHandler(db)
	router.POST("/workers/bulk-apply-config", handler.BulkApplyConfig)

	body, err := json.Marshal(gin.H{
		"workerIds":         []uint{workers[0].ID, workers[1].ID},
		"provider":          "team-openai",
		"model":             "gpt-5.5",
		"modelSettingsName": "default_model_config",
		"llmConfigId":       llmConfig.ID,
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/workers/bulk-apply-config", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.Code, resp.Body.String())
	}

	var updatedA models.Worker
	if err := db.First(&updatedA, workers[0].ID).Error; err != nil {
		t.Fatalf("reload worker-a: %v", err)
	}
	if updatedA.Provider != "team-openai" || updatedA.Model != "gpt-5.5" || updatedA.ModelSettingsName != "default_model_config" {
		t.Fatalf("worker-a not updated as expected: %+v", updatedA)
	}
	if updatedA.LLMConfigID == nil || *updatedA.LLMConfigID != llmConfig.ID {
		t.Fatalf("worker-a llm config not updated: %+v", updatedA)
	}

	var updatedB models.Worker
	if err := db.First(&updatedB, workers[1].ID).Error; err != nil {
		t.Fatalf("reload worker-b: %v", err)
	}
	if updatedB.Provider != "team-openai" || updatedB.Model != "gpt-5.5" || updatedB.ModelSettingsName != "default_model_config" {
		t.Fatalf("worker-b not updated as expected: %+v", updatedB)
	}

	var untouched models.Worker
	if err := db.First(&untouched, workers[2].ID).Error; err != nil {
		t.Fatalf("reload worker-c: %v", err)
	}
	if untouched.Provider != "legacy" || untouched.Model != "old-c" || untouched.ModelSettingsName != "legacy_config" {
		t.Fatalf("worker-c should remain unchanged: %+v", untouched)
	}
}
