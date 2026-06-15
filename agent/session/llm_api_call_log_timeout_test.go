package session

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestLLMTracingHTTPClientConnectTimeoutTriggersOnSlowHeader 验证 ResponseHeaderTimeout 生效。
// Mock server 收到请求后延迟发送 response headers，超过 connectTimeout。
func TestLLMTracingHTTPClientConnectTimeoutTriggersOnSlowHeader(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 延迟发送 headers，模拟慢响应
		time.Sleep(500 * time.Millisecond)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: ok\n\n"))
	}))
	defer server.Close()

	// connectTimeout 设为 200ms，server 500ms 后才发 header，应该触发超时
	client := newLLMTracingHTTPClient(nil, llmTracingHooks{}, 0, 200*time.Millisecond)

	start := time.Now()
	resp, err := client.Get(server.URL)
	elapsed := time.Since(start)

	if err == nil {
		if resp != nil {
			_ = resp.Body.Close()
		}
		t.Fatal("expected timeout error, got nil")
	}

	if !strings.Contains(err.Error(), "timeout") && !strings.Contains(err.Error(), "deadline exceeded") {
		t.Fatalf("expected timeout error, got: %v", err)
	}

	// 应该在 200-300ms 内失败，而不是等到 500ms 之后
	if elapsed > 300*time.Millisecond {
		t.Fatalf("timeout took too long: %v (expected < 300ms)", elapsed)
	}
}

// TestLLMTracingHTTPClientRequestTimeoutTriggersOnSlowBody 验证 http.Client.Timeout（总超时）生效。
// Mock server 立即返回 headers，但 body 读取中途 sleep 很长时间。
func TestLLMTracingHTTPClientRequestTimeoutTriggersOnSlowBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		flusher, _ := w.(http.Flusher)

		// 立即 flush 第一个 chunk
		_, _ = w.Write([]byte("data: hello\n\n"))
		flusher.Flush()

		// sleep 很长时间，超过 requestTimeout
		time.Sleep(500 * time.Millisecond)

		_, _ = w.Write([]byte("data: world\n\n"))
		flusher.Flush()
	}))
	defer server.Close()

	// requestTimeout 设为 200ms，在 body 读取阶段触发
	client := newLLMTracingHTTPClient(nil, llmTracingHooks{}, 200*time.Millisecond, 0)

	resp, err := client.Get(server.URL)
	if err != nil {
		// 也可能在 Do() 阶段就超时（如果整体耗时超过 200ms）
		if strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "deadline exceeded") {
			return
		}
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	// 读取第一段
	buf := make([]byte, 64)
	n, err := resp.Body.Read(buf)
	if err != nil {
		t.Fatalf("unexpected error reading first chunk: %v", err)
	}
	if !strings.Contains(string(buf[:n]), "hello") {
		t.Fatalf("expected hello chunk, got: %s", string(buf[:n]))
	}

	// 第二段读取应该超时（server 在 sleep 500ms 后才发 world）
	start := time.Now()
	_, err = resp.Body.Read(buf)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected timeout error on second read, got nil")
	}
	errText := err.Error()
	if !strings.Contains(errText, "timeout") &&
		!strings.Contains(errText, "deadline exceeded") &&
		!strings.Contains(errText, "request canceled") {
		t.Fatalf("expected timeout error, got: %v", err)
	}
	if elapsed > 300*time.Millisecond {
		t.Fatalf("timeout took too long: %v (expected < 300ms)", elapsed)
	}
}

// TestLLMTracingHTTPClientNoTimeoutWithZeroValues 验证两个 timeout 都为 0 时不限制。
func TestLLMTracingHTTPClientNoTimeoutWithZeroValues(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: ok\n\n"))
	}))
	defer server.Close()

	client := newLLMTracingHTTPClient(nil, llmTracingHooks{}, 0, 0)

	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("unexpected error with zero timeouts: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status: %d", resp.StatusCode)
	}
}
