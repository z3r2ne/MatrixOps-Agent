package search

import "testing"

func TestResolveSearchEndpoint(t *testing.T) {
	tests := []struct {
		baseURL  string
		expected string
	}{
		{"", "https://agent-gw.kimi.com/coding/v1/search"},
		{"https://agent-gw.kimi.com/coding", "https://agent-gw.kimi.com/coding/v1/search"},
		{"https://api.kimi.com/coding/v1", "https://api.kimi.com/coding/v1/search"},
		{"https://api.kimi.com/coding/v1/search", "https://api.kimi.com/coding/v1/search"},
	}

	for _, tc := range tests {
		if got := ResolveSearchEndpoint(tc.baseURL); got != tc.expected {
			t.Fatalf("ResolveSearchEndpoint(%q) = %q, want %q", tc.baseURL, got, tc.expected)
		}
	}
}

func TestFormatResultsEmpty(t *testing.T) {
	if got := FormatResults(nil); got == "" {
		t.Fatalf("expected non-empty message for empty results")
	}
}
