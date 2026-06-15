package openai_native

import (
	"net/http"
	"strings"
	"testing"

	agentprovider "matrixops-agent/provider"
	"matrixops.local/core_agent/streamtypes"
	"pkgs/db/models"
)

func TestOpenAINativeNoEventStreamErrorIsRetryableForBurpHTML(t *testing.T) {
	err := openAINativeNoEventStreamError(
		&models.LLMConfig{Name: "openai"},
		http.StatusOK,
		map[string]string{"content-type": "text/event-stream"},
		`<html><head><title>Burp Suite Community Edition</title></head><body><h1>Error</h1><p>Stream failed to close correctly</p></body></html>`,
	)
	if err == nil {
		t.Fatal("expected retryable error")
	}

	apiErr, ok := err.(*agentprovider.APIError)
	if !ok {
		t.Fatalf("expected APIError, got %T", err)
	}
	if !apiErr.IsRetryable {
		t.Fatal("expected APIError.IsRetryable=true")
	}
	if !strings.Contains(apiErr.ResponseBody, "Burp Suite Community Edition") {
		t.Fatalf("expected response body to contain Burp html, got %q", apiErr.ResponseBody)
	}
	if !strings.Contains(apiErr.Message, "proxy/html error page") {
		t.Fatalf("expected message to mention proxy/html error page, got %q", apiErr.Message)
	}
}

func TestOpenAINativeRetryableStreamErrorForContentTypeMismatch(t *testing.T) {
	err := openAINativeRetryableStreamError(
		&models.LLMConfig{Name: "openai"},
		http.StatusOK,
		map[string]string{"content-type": "text/html; charset=utf-8", "retry-after-ms": "1"},
		`unexpected streaming content-type "text/html; charset=utf-8"`,
		`<html><body>proxy error</body></html>`,
	)
	if err == nil {
		t.Fatal("expected retryable error")
	}

	apiErr, ok := err.(*agentprovider.APIError)
	if !ok {
		t.Fatalf("expected APIError, got %T", err)
	}
	if !apiErr.IsRetryable {
		t.Fatal("expected APIError.IsRetryable=true")
	}
	if apiErr.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", apiErr.StatusCode)
	}
	if apiErr.ResponseHeaders["retry-after-ms"] != "1" {
		t.Fatalf("expected retry-after-ms header to be preserved, got %#v", apiErr.ResponseHeaders)
	}
	if !streamtypes.StreamShouldRetryError(apiErr) {
		t.Fatal("expected streamShouldRetryError to accept synthesized APIError")
	}
}
