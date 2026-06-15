package tool

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func allowPrivateURLsForTest(t *testing.T) {
	t.Helper()
	old := urlSecurityAllowPrivate
	urlSecurityAllowPrivate = true
	t.Cleanup(func() { urlSecurityAllowPrivate = old })
}

func TestFetchURLToolExtractsHTML(t *testing.T) {
	allowPrivateURLsForTest(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!DOCTYPE html><html><head><title>T</title><style>.x{}</style></head>
<body><h1>Hello</h1><p>World</p><script>alert(1)</script></body></html>`))
	}))
	t.Cleanup(ts.Close)

	res, err := FetchURLTool{}.Execute(Context{Context: context.Background()}, map[string]interface{}{
		"url": ts.URL,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("unexpected error: %s", res.Content)
	}
	if !strings.Contains(res.Content, "Hello") || !strings.Contains(res.Content, "World") {
		t.Fatalf("expected extracted text, got %q", res.Content)
	}
	if strings.Contains(res.Content, "alert(1)") {
		t.Fatalf("script content should be stripped, got %q", res.Content)
	}
}

func TestFetchURLToolPlainText(t *testing.T) {
	allowPrivateURLsForTest(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("plain body"))
	}))
	t.Cleanup(ts.Close)

	res, err := FetchURLTool{}.Execute(Context{Context: context.Background()}, map[string]interface{}{
		"url": ts.URL,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Content, "plain body") {
		t.Fatalf("expected plain body, got %q", res.Content)
	}
}

func TestFetchURLToolRejectsPrivateHost(t *testing.T) {
	_, err := FetchURLTool{}.Execute(Context{Context: context.Background()}, map[string]interface{}{
		"url": "http://127.0.0.1/",
	})
	if err == nil {
		t.Fatal("expected error for loopback url")
	}
}

func TestFetchURLToolRejectsOversizedBody(t *testing.T) {
	allowPrivateURLsForTest(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		payload := make([]byte, fetchURLMaxBodyBytes+1)
		_, _ = w.Write(payload)
	}))
	t.Cleanup(ts.Close)

	_, err := FetchURLTool{}.Execute(Context{Context: context.Background()}, map[string]interface{}{
		"url": ts.URL,
	})
	if err == nil {
		t.Fatal("expected error for oversized body")
	}
}

func TestValidatePublicHTTPURLRejectsFileScheme(t *testing.T) {
	_, err := validatePublicHTTPURL("file:///etc/passwd")
	if err == nil {
		t.Fatal("expected error")
	}
}
