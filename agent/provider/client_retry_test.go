package provider

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"matrixops-agent/llm"
	"pkgs/db/models"
)

type retryRoundTripper struct {
	attempts int
}

func (rt *retryRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	rt.attempts++

	if rt.attempts <= 2 {
		return &http.Response{
			StatusCode: http.StatusTooManyRequests,
			Header: http.Header{
				"Content-Type": []string{"application/json"},
				"Retry-After":  []string{"0"},
			},
			Body: io.NopCloser(strings.NewReader(`{"error":{"message":"rate limited"}}`)),
		}, nil
	}

	return &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"text/event-stream"},
		},
		Body: io.NopCloser(strings.NewReader("event: response.output_text.delta\ndata: {\"delta\":\"hello\"}\n\ndata: [DONE]\n\n")),
	}, nil
}

func TestGenericClientStreamChatDoesNotRetryRetryableFailures(t *testing.T) {
	roundTripper := &retryRoundTripper{}
	httpClient := &http.Client{Transport: roundTripper}

	client := NewGenericClient()
	var retryAttempts []int

	_, err := client.StreamChatWithOptions(
		llm.ChatRequest{
			Context: context.Background(),
			Model:   "gpt-test",
			Messages: []*llm.ModelMessage{
				{Role: "user", Content: "hello"},
			},
			ProviderOptions: &models.LLMConfig{
				Name:       "openai",
				BaseURL:    "https://example.com/v1",
				APIKey:     "test-key",
				MaxRetries: 2,
			},
		},
		llm.WithHTTPClient(httpClient),
		llm.WithOnRetryError(func(err error, retryAttempt int, maxRetries int, nextDelay time.Duration, attemptDuration time.Duration, rawResponse string) {
			retryAttempts = append(retryAttempts, retryAttempt)
		}),
	)
	if err == nil {
		t.Fatal("expected first retryable failure to be returned by client")
	}
	if roundTripper.attempts != 1 {
		t.Fatalf("expected no client retries, got %d attempts", roundTripper.attempts)
	}
	if len(retryAttempts) != 0 {
		t.Fatalf("client should not emit retry callbacks, got %#v", retryAttempts)
	}
}
