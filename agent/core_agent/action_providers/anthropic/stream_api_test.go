package anthropic

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"matrixops.local/core_agent/streamtypes"
	"pkgs/db/models"
)

type anthropicRoundTripper struct {
	fn func(*http.Request) (*http.Response, error)
}

func (rt *anthropicRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return rt.fn(req)
}

func TestStreamV2Anthropic_MessageStopEndsWaitBeforeTransportEOF(t *testing.T) {
	t.Parallel()

	releaseBody := make(chan struct{})
	httpClient := &http.Client{
		Transport: &anthropicRoundTripper{
			fn: func(req *http.Request) (*http.Response, error) {
				pr, pw := io.Pipe()
				go func() {
					_, _ = io.WriteString(pw, "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{}}\n\n")
					_, _ = io.WriteString(pw, "event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"thinking\",\"thinking\":\"\"}}\n\n")
					_, _ = io.WriteString(pw, "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"thinking_delta\",\"thinking\":\"plan\"}}\n\n")
					_, _ = io.WriteString(pw, "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"signature_delta\",\"signature\":\"sig-1\"}}\n\n")
					_, _ = io.WriteString(pw, "event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":0}\n\n")
					_, _ = io.WriteString(pw, "event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n")
					<-releaseBody
					_ = pw.Close()
				}()
				return &http.Response{
					StatusCode: http.StatusOK,
					Header: http.Header{
						"Content-Type": []string{"text/event-stream"},
					},
					Body:    pr,
					Request: req,
				}, nil
			},
		},
	}

	output, err := StreamV2AnthropicOnce(streamtypes.StreamInput{
		Context:         context.Background(),
		Model:           "claude-test",
		MaxOutputTokens: 128,
		ProviderOptions: &models.LLMConfig{
			Name:       "anthropic",
			BaseURL:    "https://example.com",
			APIKey:     "test-key",
			MaxRetries: 0,
		},
		HTTPClient:     httpClient,
		EnableThinking: testBoolPtr(true),
		BudgetTokens:   testIntPtr(32),
	})
	if err != nil {
		t.Fatalf("StreamV2AnthropicOnce returned error: %v", err)
	}

	waitDone := make(chan error, 1)
	go func() {
		waitDone <- output.Wait()
	}()

	select {
	case err := <-waitDone:
		if err != nil {
			t.Fatalf("Wait returned error: %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Wait blocked after message_stop; expected it to finish before transport EOF")
	}

	reasonBytes, err := io.ReadAll(output.ReasonReader)
	if err != nil {
		t.Fatalf("ReadAll ReasonReader: %v", err)
	}
	if string(reasonBytes) != "plan" {
		t.Fatalf("ReasonReader = %q, want %q", string(reasonBytes), "plan")
	}
	if got := output.AnthropicThinkingSignature(); got != "sig-1" {
		t.Fatalf("AnthropicThinkingSignature = %q, want %q", got, "sig-1")
	}

	close(releaseBody)
}

func TestStreamV2Anthropic_ContentAndReasonReadersAggregateAcrossBlocks(t *testing.T) {
	t.Parallel()

	body := strings.Join([]string{
		"event: message_start",
		"data: {\"type\":\"message_start\",\"message\":{}}",
		"",
		"event: content_block_start",
		"data: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}",
		"",
		"event: content_block_delta",
		"data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"hello \"}}",
		"",
		"event: content_block_stop",
		"data: {\"type\":\"content_block_stop\",\"index\":0}",
		"",
		"event: content_block_start",
		"data: {\"type\":\"content_block_start\",\"index\":1,\"content_block\":{\"type\":\"thinking\",\"thinking\":\"\"}}",
		"",
		"event: content_block_delta",
		"data: {\"type\":\"content_block_delta\",\"index\":1,\"delta\":{\"type\":\"thinking_delta\",\"thinking\":\"step-1 \"}}",
		"",
		"event: content_block_stop",
		"data: {\"type\":\"content_block_stop\",\"index\":1}",
		"",
		"event: content_block_start",
		"data: {\"type\":\"content_block_start\",\"index\":2,\"content_block\":{\"type\":\"tool_use\",\"id\":\"toolu_1\",\"name\":\"lookup\",\"input\":{}}}",
		"",
		"event: content_block_delta",
		"data: {\"type\":\"content_block_delta\",\"index\":2,\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":\"{\\\"q\\\":\\\"weather\\\"}\"}}",
		"",
		"event: content_block_stop",
		"data: {\"type\":\"content_block_stop\",\"index\":2}",
		"",
		"event: content_block_start",
		"data: {\"type\":\"content_block_start\",\"index\":3,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}",
		"",
		"event: content_block_delta",
		"data: {\"type\":\"content_block_delta\",\"index\":3,\"delta\":{\"type\":\"text_delta\",\"text\":\"world\"}}",
		"",
		"event: content_block_stop",
		"data: {\"type\":\"content_block_stop\",\"index\":3}",
		"",
		"event: content_block_start",
		"data: {\"type\":\"content_block_start\",\"index\":4,\"content_block\":{\"type\":\"thinking\",\"thinking\":\"\"}}",
		"",
		"event: content_block_delta",
		"data: {\"type\":\"content_block_delta\",\"index\":4,\"delta\":{\"type\":\"thinking_delta\",\"thinking\":\"step-2\"}}",
		"",
		"event: content_block_delta",
		"data: {\"type\":\"content_block_delta\",\"index\":4,\"delta\":{\"type\":\"signature_delta\",\"signature\":\"sig-2\"}}",
		"",
		"event: content_block_stop",
		"data: {\"type\":\"content_block_stop\",\"index\":4}",
		"",
		"event: message_stop",
		"data: {\"type\":\"message_stop\"}",
		"",
	}, "\n")

	httpClient := &http.Client{
		Transport: &anthropicRoundTripper{
			fn: func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header: http.Header{
						"Content-Type": []string{"text/event-stream"},
					},
					Body:    io.NopCloser(strings.NewReader(body)),
					Request: req,
				}, nil
			},
		},
	}

	output, err := StreamV2AnthropicOnce(streamtypes.StreamInput{
		Context: context.Background(),
		Model:   "claude-test",
		Tools: []streamtypes.ToolDefinition{
			{Name: "lookup", Description: "lookup weather", Schema: map[string]any{"type": "object"}},
		},
		MaxOutputTokens: 128,
		ProviderOptions: &models.LLMConfig{
			Name:       "anthropic",
			BaseURL:    "https://example.com",
			APIKey:     "test-key",
			MaxRetries: 0,
		},
		HTTPClient:     httpClient,
		EnableThinking: testBoolPtr(true),
		BudgetTokens:   testIntPtr(32),
	})
	if err != nil {
		t.Fatalf("StreamV2AnthropicOnce returned error: %v", err)
	}

	if err := output.Wait(); err != nil {
		t.Fatalf("Wait returned error: %v", err)
	}

	var actions []*streamtypes.CallToolRequest
	for req := range output.ToolCalls {
		actions = append(actions, req)
	}

	if len(actions) != 1 {
		t.Fatalf("actions len = %d, want 1", len(actions))
	}
	if actions[0] == nil || actions[0].Name != "lookup" {
		t.Fatalf("unexpected tool call: %#v", actions[0])
	}
	actionPayload, err := io.ReadAll(actions[0].Arguments)
	if err != nil {
		t.Fatalf("read tool args: %v", err)
	}
	if string(actionPayload) != `{"q":"weather"}` {
		t.Fatalf("tool args payload = %q, want %q", string(actionPayload), `{"q":"weather"}`)
	}
	if actions[0].RawJSON != fmt.Sprintf(`{"@action":"call_tool","data":{"name":%q,"params":%s}}`, "lookup", `{"q":"weather"}`) {
		t.Fatalf("unexpected tool call raw json: %q", actions[0].RawJSON)
	}

	contentBytes, err := io.ReadAll(output.ContentReader)
	if err != nil {
		t.Fatalf("ReadAll ContentReader: %v", err)
	}
	if string(contentBytes) != "hello world" {
		t.Fatalf("ContentReader = %q, want %q", string(contentBytes), "hello world")
	}

	reasonBytes, err := io.ReadAll(output.ReasonReader)
	if err != nil {
		t.Fatalf("ReadAll ReasonReader: %v", err)
	}
	if string(reasonBytes) != "step-1 step-2" {
		t.Fatalf("ReasonReader = %q, want %q", string(reasonBytes), "step-1 step-2")
	}
	if got := output.AnthropicThinkingSignature(); got != "sig-2" {
		t.Fatalf("AnthropicThinkingSignature = %q, want %q", got, "sig-2")
	}
}

func TestStreamV2AnthropicStreamsReasonContentAndTwoActions(t *testing.T) {
	t.Parallel()

	type gate struct{ ch chan struct{} }
	newGate := func() *gate { return &gate{ch: make(chan struct{}, 1)} }
	allow := func(g *gate) { g.ch <- struct{}{} }
	wait := func(g *gate) { <-g.ch }

	g := newGate()
	httpClient := &http.Client{
		Transport: &anthropicRoundTripper{
			fn: func(req *http.Request) (*http.Response, error) {
				pr, pw := io.Pipe()
				go func() {
					_, _ = io.WriteString(pw, "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{}}\n\n")
					_, _ = io.WriteString(pw, "event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"thinking\",\"thinking\":\"\"}}\n\n")
					_, _ = io.WriteString(pw, "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"thinking_delta\",\"thinking\":\"rea\"}}\n\n")
					wait(g)
					_, _ = io.WriteString(pw, "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"thinking_delta\",\"thinking\":\"son\"}}\n\n")
					_, _ = io.WriteString(pw, "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"signature_delta\",\"signature\":\"sig-1\"}}\n\n")
					_, _ = io.WriteString(pw, "event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":0}\n\n")
					wait(g)
					_, _ = io.WriteString(pw, "event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":1,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n")
					_, _ = io.WriteString(pw, "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":1,\"delta\":{\"type\":\"text_delta\",\"text\":\"con\"}}\n\n")
					wait(g)
					_, _ = io.WriteString(pw, "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":1,\"delta\":{\"type\":\"text_delta\",\"text\":\"tent\"}}\n\n")
					_, _ = io.WriteString(pw, "event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":1}\n\n")
					wait(g)
					_, _ = io.WriteString(pw, "event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":2,\"content_block\":{\"type\":\"tool_use\",\"id\":\"toolu_1\",\"name\":\"answer\",\"input\":{}}}\n\n")
					_, _ = io.WriteString(pw, "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":2,\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":\"{\\\"text\\\":\\\"he\"}}\n\n")
					wait(g)
					_, _ = io.WriteString(pw, "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":2,\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":\"llo\\\"}\"}}\n\n")
					_, _ = io.WriteString(pw, "event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":2}\n\n")
					wait(g)
					_, _ = io.WriteString(pw, "event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":3,\"content_block\":{\"type\":\"tool_use\",\"id\":\"toolu_2\",\"name\":\"call_tool\",\"input\":{}}}\n\n")
					_, _ = io.WriteString(pw, "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":3,\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":\"{\\\"command\\\":\\\"echo \"}}\n\n")
					wait(g)
					_, _ = io.WriteString(pw, "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":3,\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":\"hi\\\"}\"}}\n\n")
					_, _ = io.WriteString(pw, "event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":3}\n\n")
					_, _ = io.WriteString(pw, "event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n")
					_ = pw.Close()
				}()
				return &http.Response{
					StatusCode: http.StatusOK,
					Header: http.Header{
						"Content-Type": []string{"text/event-stream"},
					},
					Body:    pr,
					Request: req,
				}, nil
			},
		},
	}

	output, err := StreamV2Anthropic(streamtypes.StreamInput{
		Context: context.Background(),
		Model:   "claude-test",
		Tools: []streamtypes.ToolDefinition{
			{Name: "answer", Schema: map[string]any{"type": "object"}},
			{Name: "call_tool", Schema: map[string]any{"type": "object"}},
		},
		MaxOutputTokens: 128,
		ProviderOptions: &models.LLMConfig{
			Name:       "anthropic",
			BaseURL:    "https://example.com",
			APIKey:     "test-key",
			Type:       "claude",
			MaxRetries: 1,
		},
		HTTPClient:     httpClient,
		EnableThinking: testBoolPtr(true),
		BudgetTokens:   testIntPtr(32),
	})
	if err != nil {
		t.Fatalf("StreamV2Anthropic returned error: %v", err)
	}

	assertAnthropicReadWithin(t, output.ReasonReader, "rea")
	allow(g)
	assertAnthropicReadWithin(t, output.ReasonReader, "son")
	allow(g)
	assertAnthropicReadWithin(t, output.ContentReader, "con")
	allow(g)
	assertAnthropicReadWithin(t, output.ContentReader, "tent")
	allow(g)

	var first *streamtypes.CallToolRequest
	select {
	case first = <-output.ToolCalls:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected first anthropic tool call within streaming timeout")
	}
	if first == nil || first.Name != "answer" {
		t.Fatalf("unexpected first tool call: %#v", first)
	}
	assertAnthropicReadWithin(t, first.Arguments, `{"text":"he`)
	allow(g)
	if rest, err := io.ReadAll(first.Arguments); err != nil {
		t.Fatalf("ReadAll first tool call failed: %v", err)
	} else if got := `{"text":"he` + string(rest); got != `{"text":"hello"}` {
		t.Fatalf("first tool call stream = %q, want %q", got, `{"text":"hello"}`)
	}
	allow(g)

	var second *streamtypes.CallToolRequest
	select {
	case second = <-output.ToolCalls:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected second anthropic tool call within streaming timeout")
	}
	if second == nil || second.Name != "call_tool" {
		t.Fatalf("unexpected second tool call: %#v", second)
	}
	assertAnthropicReadWithin(t, second.Arguments, `{"command":"echo `)
	allow(g)
	if rest, err := io.ReadAll(second.Arguments); err != nil {
		t.Fatalf("ReadAll second tool call failed: %v", err)
	} else if got := `{"command":"echo ` + string(rest); got != `{"command":"echo hi"}` {
		t.Fatalf("second tool call stream = %q, want %q", got, `{"command":"echo hi"}`)
	}

	if err := output.Wait(); err != nil {
		t.Fatalf("Wait returned error: %v", err)
	}
}

func testBoolPtr(v bool) *bool { return &v }

func testIntPtr(v int) *int { return &v }

func assertAnthropicReadWithin(t *testing.T, r io.Reader, want string) {
	t.Helper()
	dataCh := make(chan []byte, 1)
	errCh := make(chan error, 1)
	go func() {
		buf := make([]byte, len(want)+16)
		n, err := r.Read(buf)
		dataCh <- append([]byte(nil), buf[:n]...)
		errCh <- err
	}()
	select {
	case got := <-dataCh:
		if string(got) != want {
			t.Fatalf("streamed chunk = %q, want %q", string(got), want)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("timed out waiting for streamed chunk %q", want)
	}
	if err := <-errCh; err != nil {
		t.Fatalf("unexpected streamed read error: %v", err)
	}
}

func TestStreamV2Anthropic_EmptyEndTurnIsRetryable(t *testing.T) {
	t.Parallel()

	httpClient := &http.Client{
		Transport: &anthropicRoundTripper{
			fn: func(req *http.Request) (*http.Response, error) {
				pr, pw := io.Pipe()
				go func() {
					_, _ = io.WriteString(pw, "event: message_start\n")
					_, _ = io.WriteString(pw, `data: {"type":"message_start","message":{"id":"msg_test","type":"message","role":"assistant","content":[],"model":"kimi-for-coding","stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":11045,"output_tokens":0}}}`+"\n\n")
					_, _ = io.WriteString(pw, "event: message_delta\n")
					_, _ = io.WriteString(pw, `data: {"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null},"usage":{"input_tokens":293,"output_tokens":1,"cache_read_input_tokens":10752}}`+"\n\n")
					_, _ = io.WriteString(pw, "event: message_stop\n")
					_, _ = io.WriteString(pw, `data: {"type":"message_stop"}`+"\n\n")
					_ = pw.Close()
				}()
				return &http.Response{
					StatusCode: http.StatusOK,
					Header: http.Header{
						"Content-Type": []string{"text/event-stream"},
					},
					Body:    pr,
					Request: req,
				}, nil
			},
		},
	}

	output, err := StreamV2AnthropicOnce(streamtypes.StreamInput{
		Context:         context.Background(),
		Model:           "kimi-for-coding",
		MaxOutputTokens: 128,
		ProviderOptions: &models.LLMConfig{
			Name:    "anthropic",
			BaseURL: "https://example.com",
			APIKey:  "test-key",
		},
		HTTPClient: httpClient,
	})
	if err != nil {
		t.Fatalf("StreamV2AnthropicOnce returned error: %v", err)
	}

	waitErr := output.Wait()
	if waitErr == nil {
		t.Fatal("expected retryable empty stream error")
	}
	if !streamtypes.StreamShouldRetryError(waitErr) {
		t.Fatalf("expected retryable error, got %v", waitErr)
	}
	if !strings.Contains(waitErr.Error(), "end_turn") {
		t.Fatalf("error = %v, want end_turn hint", waitErr)
	}
}
