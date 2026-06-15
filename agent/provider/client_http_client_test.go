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

type recordingRoundTripper struct {
	called      bool
	requestURL  string
	requestBody string
}

func (rt *recordingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	rt.called = true
	rt.requestURL = req.URL.String()

	reqBody, _ := io.ReadAll(req.Body)
	rt.requestBody = string(reqBody)

	respBody := "event: response.output_text.delta\ndata: {\"delta\":\"hello\"}\n\ndata: [DONE]\n\n"
	if strings.HasSuffix(req.URL.Path, "/chat/completions") {
		respBody = "data: {\"id\":\"chatcmpl-1\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"hello\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n"
	}

	return &http.Response{
		StatusCode: 200,
		Header: http.Header{
			"Content-Type": []string{"text/event-stream"},
		},
		Body: io.NopCloser(strings.NewReader(respBody)),
	}, nil
}

func TestGenericClientStreamChatUsesInjectedHTTPClient(t *testing.T) {
	roundTripper := &recordingRoundTripper{}
	httpClient := &http.Client{
		Transport: roundTripper,
	}

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
		llm.WithHTTPClient(httpClient),
	)
	if err != nil {
		t.Fatalf("StreamChatWithOptions returned error: %v", err)
	}

	var sawText bool
	for event := range stream {
		if event.Type == string(llm.GeneratorMessageTypeTextDelta) && event.Text == "hello" {
			sawText = true
		}
		if event.Error != nil {
			t.Fatalf("unexpected stream error: %v", event.Error)
		}
	}

	if !roundTripper.called {
		t.Fatal("expected injected http client to be used")
	}
	if roundTripper.requestURL != "https://example.com/v1/responses" {
		t.Fatalf("unexpected request URL: %s", roundTripper.requestURL)
	}
	if !strings.Contains(roundTripper.requestBody, "\"model\":\"gpt-test\"") {
		t.Fatalf("unexpected request body: %s", roundTripper.requestBody)
	}
	if !sawText {
		t.Fatal("expected text delta from stream")
	}
}

func TestGenericClientStreamChatUsesChatCompletionsWhenAPITypeIsChat(t *testing.T) {
	roundTripper := &recordingRoundTripper{}
	httpClient := &http.Client{
		Transport: roundTripper,
	}

	client := NewGenericClient()
	stream, err := client.StreamChatWithOptions(
		llm.ChatRequest{
			Context: context.Background(),
			Model:   "gpt-test",
			Messages: []*llm.ModelMessage{
				{Role: "user", Content: "hello"},
			},
			ProviderOptions: &models.LLMConfig{
				Name:    "doubao",
				Type:    "custom",
				BaseURL: "https://example.com/v1",
				APIKey:  "test-key",
				APIType: models.LLMAPITypeChat,
			},
		},
		llm.WithHTTPClient(httpClient),
	)
	if err != nil {
		t.Fatalf("StreamChatWithOptions returned error: %v", err)
	}

	for range stream {
	}

	if !roundTripper.called {
		t.Fatal("expected injected http client to be used")
	}
	if roundTripper.requestURL != "https://example.com/v1/chat/completions" {
		t.Fatalf("unexpected request URL: %s", roundTripper.requestURL)
	}
	if !strings.Contains(roundTripper.requestBody, "\"messages\"") {
		t.Fatalf("expected chat completions request body, got %s", roundTripper.requestBody)
	}
}
