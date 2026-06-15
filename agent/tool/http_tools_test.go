package tool

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPHeadTool(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead {
			http.Error(w, "wrong method", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("X-Test", "1")
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(ts.Close)

	tool := HTTPHeadTool{}
	res, err := tool.Execute(Context{Context: context.Background()}, map[string]interface{}{
		"url": ts.URL + "/r",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("unexpected error result: %s", res.Content)
	}
	if !strings.Contains(res.Content, "204") {
		t.Fatalf("expected 204 in output, got %q", res.Content)
	}
	if !strings.Contains(res.Content, "X-Test: 1") {
		t.Fatalf("expected header in output, got %q", res.Content)
	}
}

func TestHTTPDeleteTool(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "wrong method", http.StatusMethodNotAllowed)
			return
		}
		body, _ := io.ReadAll(r.Body)
		_ = r.Body.Close()
		if string(body) != `{"ok":true}` {
			http.Error(w, "bad body", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"deleted":1}`))
	}))
	t.Cleanup(ts.Close)

	tool := HTTPDeleteTool{}
	res, err := tool.Execute(Context{Context: context.Background()}, map[string]interface{}{
		"url": ts.URL + "/item",
		"headers": map[string]interface{}{
			"Content-Type": "application/json",
		},
		"body": `{"ok":true}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("unexpected error result: %s", res.Content)
	}
	if !strings.Contains(res.Content, `"deleted":1`) {
		t.Fatalf("expected json body in output, got %q", res.Content)
	}
}

func TestHTTPToolRejectsNonHTTPScheme(t *testing.T) {
	tool := HTTPHeadTool{}
	_, err := tool.Execute(Context{Context: context.Background()}, map[string]interface{}{
		"url": "file:///etc/passwd",
	})
	if err == nil {
		t.Fatal("expected error for file:// url")
	}
}
