package provider

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"matrixops-agent/llm"
	"pkgs/db/models"
)

func TestResolveOptionsReturnsConfiguredValues(t *testing.T) {
	cfg := &models.LLMConfig{
		Name:    "openai",
		APIKey:  "cfg-key",
		BaseURL: "https://cfg.local/v1",
		Proxy:   "https://cfg-proxy",
	}

	apiKey, baseURL, proxy := resolveOptions(cfg)
	if apiKey != "cfg-key" {
		t.Fatalf("expected apiKey cfg-key, got %q", apiKey)
	}
	if baseURL != "https://cfg.local/v1" {
		t.Fatalf("expected baseURL https://cfg.local/v1, got %q", baseURL)
	}
	if proxy != "https://cfg-proxy" {
		t.Fatalf("expected proxy https://cfg-proxy, got %q", proxy)
	}
}

func TestResolveOptionsDefaultsOpenAIBaseURL(t *testing.T) {
	cfg := &models.LLMConfig{Name: "openai"}
	_, baseURL, _ := resolveOptions(cfg)
	if baseURL != "https://api.openai.com/v1" {
		t.Fatalf("expected default openai baseURL, got %q", baseURL)
	}
}

func TestResolveOptionsDefaultsAnthropicBaseURL(t *testing.T) {
	cfg := &models.LLMConfig{Name: "anthropic"}
	_, baseURL, _ := resolveOptions(cfg)
	if baseURL != "https://api.anthropic.com/v1" {
		t.Fatalf("expected default anthropic baseURL, got %q", baseURL)
	}
}

type genericRoundTripper struct {
	url  string
	body string
}

func (rt *genericRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	rt.url = req.URL.String()
	payload, _ := io.ReadAll(req.Body)
	rt.body = string(payload)
	return &http.Response{
		StatusCode: 200,
		Header: http.Header{
			"Content-Type": []string{"text/event-stream"},
		},
		Body: io.NopCloser(strings.NewReader("event: response.output_text.delta\ndata: {\"delta\":\"hello\"}\n\ndata: [DONE]\n\n")),
	}, nil
}

func TestGenericClientStreamChatRoutesToResponsesEndpoint(t *testing.T) {
	rt := &genericRoundTripper{}
	client := NewGenericClient()
	stream, err := client.StreamChatWithOptions(
		llm.ChatRequest{
			Context: context.Background(),
			Model:   "gpt-test",
			Messages: []*llm.ModelMessage{
				{Role: "user", Content: "hello"},
			},
			ProviderOptions: &models.LLMConfig{
				Name:    "openai",
				BaseURL: "https://example.com/v1",
				APIKey:  "test-key",
				APIType: models.LLMAPITypeResponse,
			},
		},
		llm.WithHTTPClient(&http.Client{Transport: rt}),
	)
	if err != nil {
		t.Fatalf("StreamChatWithOptions returned error: %v", err)
	}
	for range stream {
	}
	if rt.url != "https://example.com/v1/responses" {
		t.Fatalf("unexpected request URL: %s", rt.url)
	}
	if !strings.Contains(rt.body, "\"model\":\"gpt-test\"") {
		t.Fatalf("unexpected request body: %s", rt.body)
	}
}
