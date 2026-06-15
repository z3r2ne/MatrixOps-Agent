package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"matrixops-agent/llm"
	"pkgs/db/models"
)

// TestGenericClientStreamChatTimesOutOnSlowBody 验证 http.Client.Timeout（总超时）生效。
// Mock server 立即返回 headers，但流式输出中途 sleep 很长时间，超过 client.Timeout。
func TestGenericClientStreamChatTimesOutOnSlowBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("response writer does not support flushing")
		}

		// 立即返回第一个 chunk
		fmt.Fprint(w, "event: response.output_text.delta\ndata: {\"delta\":\"hello\"}\n\n")
		flusher.Flush()

		// sleep 足够长，让 Timeout 触发（> 300ms）
		time.Sleep(500 * time.Millisecond)

		fmt.Fprint(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	defer server.Close()

	client := NewGenericClient()
	// 设置极短的总超时（200ms），第一个 chunk 后立即超时
	client.Timeout = 200 * time.Millisecond

	stream, err := client.StreamChatWithOptions(llm.ChatRequest{
		Context: context.Background(),
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
		// 总超时可能在 client.Do(req) 阶段就触发（如果连接建立+首字符耗时超过 200ms），
		// 也可能在流式读取阶段触发。两种情况都算 timeout 生效。
		t.Fatalf("StreamChatWithOptions returned error: %v", err)
	}

	// 读取第一个事件
	firstEvent, ok := <-stream
	if !ok {
		t.Fatal("expected first stream event before timeout")
	}
	if firstEvent.Text != "hello" {
		t.Fatalf("expected first chunk hello, got %#v", firstEvent)
	}

	// 等待超时：第二个事件应该是 error，或 channel 被关闭
	timeout := time.After(1 * time.Second)
	var sawTimeoutErr bool
	for {
		select {
		case event, ok := <-stream:
			if !ok {
				if !sawTimeoutErr {
					t.Fatal("stream closed without timeout error")
				}
				return
			}
			if event.Error != nil {
				if strings.Contains(event.Error.Error(), "context deadline exceeded") ||
					strings.Contains(event.Error.Error(), "Client.Timeout") ||
					strings.Contains(event.Error.Error(), "timeout") {
					sawTimeoutErr = true
					// 验证错误消息中包含实际的超时时间
					if !strings.Contains(event.Error.Error(), "timeout: 200ms") {
						t.Fatalf("expected timeout error to include duration, got: %v", event.Error)
					}
					return
				}
				t.Fatalf("unexpected stream error: %v", event.Error)
			}
		case <-timeout:
			t.Fatal("expected timeout error, but stream did not error within 1s")
		}
	}
}

// TestGenericClientStreamChatDoesNotTimeOutWithZeroTimeout 验证 Timeout=0 时不限制。
func TestGenericClientStreamChatDoesNotTimeOutWithZeroTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("response writer does not support flushing")
		}

		fmt.Fprint(w, "event: response.output_text.delta\ndata: {\"delta\":\"hello\"}\n\n")
		flusher.Flush()

		// sleep 500ms，但由于 Timeout=0，不会触发超时
		time.Sleep(500 * time.Millisecond)

		fmt.Fprint(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	defer server.Close()

	client := NewGenericClient()
	client.Timeout = 0 // 不限制

	stream, err := client.StreamChatWithOptions(llm.ChatRequest{
		Context: context.Background(),
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

	var sawHello bool
	var sawDone bool
	timeout := time.After(2 * time.Second)
	for {
		select {
		case event, ok := <-stream:
			if !ok {
				if sawHello && sawDone {
					return
				}
				t.Fatalf("stream closed unexpectedly, sawHello=%v sawDone=%v", sawHello, sawDone)
			}
			if event.Error != nil {
				t.Fatalf("unexpected error with zero timeout: %v", event.Error)
			}
			if event.Text == "hello" {
				sawHello = true
			}
			if event.Type == string(llm.GeneratorMessageTypeFinish) {
				sawDone = true
			}
		case <-timeout:
			t.Fatal("stream did not complete within 2s with zero timeout")
		}
	}
}
