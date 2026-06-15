package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"matrixops-agent/llm"
	"pkgs/db/models"
)

func TestGenericClientStreamChatStopsOnContextCancel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("response writer does not support flushing")
		}

		fmt.Fprint(w, "event: response.output_text.delta\ndata: {\"delta\":\"hello\"}\n\n")
		flusher.Flush()

		time.Sleep(300 * time.Millisecond)

		fmt.Fprint(w, "event: response.output_text.delta\ndata: {\"delta\":\" world\"}\n\n")
		flusher.Flush()

		time.Sleep(300 * time.Millisecond)

		fmt.Fprint(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	defer server.Close()

	client := NewGenericClient()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stream, err := client.StreamChatWithOptions(llm.ChatRequest{
		Context: ctx,
		Model:   "gpt-test",
		Messages: []*llm.ModelMessage{
			{Role: "user", Content: "hello"},
		},
		ProviderOptions: &models.LLMConfig{
			Name:    "openai",
			BaseURL: server.URL,
			APIKey:  "test-key",
		},
	})
	if err != nil {
		t.Fatalf("StreamChatWithOptions returned error: %v", err)
	}

	firstEvent, ok := <-stream
	if !ok {
		t.Fatal("expected first stream event before cancellation")
	}
	if firstEvent.Text != "hello" {
		t.Fatalf("expected first chunk hello, got %#v", firstEvent)
	}

	cancel()

	timeout := time.After(200 * time.Millisecond)
	for {
		select {
		case event, ok := <-stream:
			if !ok {
				return
			}
			if event.Type == string(llm.GeneratorMessageTypeTextDelta) && event.Text != "" {
				t.Fatalf("received text after cancellation: %#v", event)
			}
		case <-timeout:
			t.Fatal("stream did not stop shortly after cancellation")
		}
	}
}
