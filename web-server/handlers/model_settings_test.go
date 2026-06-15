package handlers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	database "pkgs/db"
	"pkgs/db/models"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupModelSettingsHandlerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&models.ModelSettings{}); err != nil {
		t.Fatalf("migrate model settings: %v", err)
	}
	return db
}

func TestUpdateModelSettingRejectsRenamingDefault(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupModelSettingsHandlerTestDB(t)
	if err := db.Create(&models.ModelSettings{Name: database.DefaultModelSettingsName}).Error; err != nil {
		t.Fatalf("create default model settings: %v", err)
	}

	router := gin.New()
	handler := NewModelSettingsHandler(db)
	router.PUT("/model-settings/:name", handler.UpdateModelSetting)

	req := httptest.NewRequest(http.MethodPut, "/model-settings/"+database.DefaultModelSettingsName, bytes.NewBufferString(`{"name":"renamed_default"}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", resp.Code, resp.Body.String())
	}
}

func TestUpdateModelSettingAllowsClearingOptionalFields(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupModelSettingsHandlerTestDB(t)
	topP := 0.7
	topK := 50
	frequencyPenalty := 0.2
	enableThinking := true
	reasoningEffort := "high"
	if err := db.Create(&models.ModelSettings{
		Name:             "custom",
		ContextLimit:     300000,
		OutputLimit:      100000,
		TopP:             &topP,
		TopK:             &topK,
		FrequencyPenalty: &frequencyPenalty,
		EnableThinking:   &enableThinking,
		ReasoningEffort:  &reasoningEffort,
	}).Error; err != nil {
		t.Fatalf("create model settings: %v", err)
	}

	router := gin.New()
	handler := NewModelSettingsHandler(db)
	router.PUT("/model-settings/:name", handler.UpdateModelSetting)

	req := httptest.NewRequest(http.MethodPut, "/model-settings/custom", bytes.NewBufferString(`{"contextLimit":null,"outputLimit":null,"topP":null,"topK":null,"frequencyPenalty":null,"enableThinking":null,"reasoningEffort":null}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.Code, resp.Body.String())
	}

	var updated models.ModelSettings
	if err := db.First(&updated, "name = ?", "custom").Error; err != nil {
		t.Fatalf("reload updated model settings: %v", err)
	}
	if updated.ContextLimit != 0 || updated.OutputLimit != 0 {
		t.Fatalf("expected limits to be cleared, got %#v", updated)
	}
	if updated.TopP != nil || updated.TopK != nil || updated.FrequencyPenalty != nil || updated.EnableThinking != nil || updated.ReasoningEffort != nil {
		t.Fatalf("expected optional fields to be nil, got %#v", updated)
	}
}

func TestNormalizeReasoningEffortAcceptsXHigh(t *testing.T) {
	value := "xhigh"
	normalized := normalizeReasoningEffort(&value)
	if normalized == nil {
		t.Fatalf("expected xhigh to be accepted")
	}
	if *normalized != "xhigh" {
		t.Fatalf("expected xhigh, got %q", *normalized)
	}
}

func TestNormalizeReasoningEffortAcceptsNoneAndMax(t *testing.T) {
	for _, want := range []string{"none", "max"} {
		v := want
		normalized := normalizeReasoningEffort(&v)
		if normalized == nil {
			t.Fatalf("expected %q to be accepted", want)
		}
		if *normalized != want {
			t.Fatalf("expected %q, got %q", want, *normalized)
		}
	}
}

func TestNormalizeTextVerbosityAcceptsXHigh(t *testing.T) {
	value := "xhigh"
	normalized := normalizeTextVerbosity(&value)
	if normalized == nil {
		t.Fatalf("expected xhigh to be accepted")
	}
	if *normalized != "xhigh" {
		t.Fatalf("expected xhigh, got %q", *normalized)
	}
}
