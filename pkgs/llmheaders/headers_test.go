package llmheaders

import (
	"net/http"
	"testing"
)

func TestSetFromJSONApply(t *testing.T) {
	t.Cleanup(func() { _ = SetFromJSON("") })

	if err := SetFromJSON(`{"X-A":"1","X-B":"two"}`); err != nil {
		t.Fatal(err)
	}
	h := http.Header{}
	h.Set("Content-Type", "application/json")
	Apply(h)
	if got := h.Get("X-A"); got != "1" {
		t.Fatalf("X-A: %q", got)
	}
	if got := h.Get("X-B"); got != "two" {
		t.Fatalf("X-B: %q", got)
	}
}

func TestValidateJSONInvalid(t *testing.T) {
	if err := ValidateJSON("not-json"); err == nil {
		t.Fatal("expected error")
	}
}

func TestEmptyClears(t *testing.T) {
	_ = SetFromJSON(`{"X-T":"y"}`)
	_ = SetFromJSON("")
	h := http.Header{}
	Apply(h)
	if h.Get("X-T") != "" {
		t.Fatal("expected cleared")
	}
}
