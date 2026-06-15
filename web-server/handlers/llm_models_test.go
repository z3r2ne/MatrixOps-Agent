package handlers

import (
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"pkgs/db/models"
)

func TestExtractLLMModelNamesSupportsMultipleShapes(t *testing.T) {
	modelNames, err := extractLLMModelNames([]byte(`{
		"data": [{"id": "model-a"}, {"id": "model-b"}],
		"models": [{"name": "model-c"}, "model-d", {"id": "model-a"}]
	}`))
	if err != nil {
		t.Fatalf("extractLLMModelNames returned error: %v", err)
	}

	expected := []string{"model-a", "model-b", "model-c", "model-d"}
	if !reflect.DeepEqual(modelNames, expected) {
		t.Fatalf("expected models %v, got %v", expected, modelNames)
	}
}

func TestFetchLLMModelsUsesUnsavedConfigValues(t *testing.T) {
	var authorization string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorization = r.Header.Get("Authorization")
		if r.URL.Path != "/models" {
			t.Fatalf("expected path /models, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":[{"id":"preview-model-1"},{"id":"preview-model-2"}]}`)
	}))
	defer server.Close()

	modelNames, statusCode, err := fetchLLMModels(models.LLMConfig{
		Type:    "custom",
		APIKey:  "preview-secret",
		BaseURL: server.URL,
	}, server.Client())
	if err != nil {
		t.Fatalf("fetchLLMModels returned error: %v", err)
	}
	if statusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", statusCode)
	}

	expected := []string{"preview-model-1", "preview-model-2"}
	if !reflect.DeepEqual(modelNames, expected) {
		t.Fatalf("expected models %v, got %v", expected, modelNames)
	}
	if authorization != "Bearer preview-secret" {
		t.Fatalf("expected Authorization header to use unsaved API key, got %q", authorization)
	}
}
