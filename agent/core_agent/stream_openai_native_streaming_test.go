package coreagent

import (
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"pkgs/db/models"
)

type nativeStreamingRoundTripper struct {
	fn func(*http.Request) (*http.Response, error)
}

func (rt nativeStreamingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return rt.fn(req)
}

type streamingGate struct {
	ch chan struct{}
}

func newStreamingGate() *streamingGate {
	return &streamingGate{ch: make(chan struct{}, 1)}
}

func (g *streamingGate) allow() {
	g.ch <- struct{}{}
}

func (g *streamingGate) wait() {
	<-g.ch
}

func TestStreamV2OpenAINativeResponsesStreamsReasonContentAndTwoActions(t *testing.T) {
	gate := newStreamingGate()
	httpClient := &http.Client{
		Transport: nativeStreamingRoundTripper{
			fn: func(req *http.Request) (*http.Response, error) {
				pr, pw := io.Pipe()
				go func() {
					_, _ = io.WriteString(pw, "event: response.reasoning.delta\ndata: {\"type\":\"response.reasoning.delta\",\"reasoning_content\":\"rea\"}\n\n")
					gate.wait()
					_, _ = io.WriteString(pw, "event: response.reasoning.delta\ndata: {\"type\":\"response.reasoning.delta\",\"reasoning_content\":\"son\"}\n\n")
					gate.wait()
					_, _ = io.WriteString(pw, "event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"con\",\"output_index\":0,\"item_id\":\"msg_1\"}\n\n")
					gate.wait()
					_, _ = io.WriteString(pw, "event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"tent\",\"output_index\":0,\"item_id\":\"msg_1\"}\n\n")
					gate.wait()
					_, _ = io.WriteString(pw, "event: response.output_item.added\ndata: {\"type\":\"response.output_item.added\",\"item\":{\"id\":\"fc_1\",\"type\":\"function_call\",\"status\":\"in_progress\",\"arguments\":\"\",\"call_id\":\"call_1\",\"name\":\"answer\"},\"output_index\":1}\n\n")
					_, _ = io.WriteString(pw, "event: response.function_call_arguments.delta\ndata: {\"type\":\"response.function_call_arguments.delta\",\"delta\":\"{\\\"text\\\":\\\"he\",\"item_id\":\"fc_1\",\"output_index\":1}\n\n")
					gate.wait()
					_, _ = io.WriteString(pw, "event: response.function_call_arguments.done\ndata: {\"type\":\"response.function_call_arguments.done\",\"arguments\":\"{\\\"text\\\":\\\"hello\\\"}\",\"item_id\":\"fc_1\",\"output_index\":1}\n\n")
					gate.wait()
					_, _ = io.WriteString(pw, "event: response.output_item.added\ndata: {\"type\":\"response.output_item.added\",\"item\":{\"id\":\"fc_2\",\"type\":\"function_call\",\"status\":\"in_progress\",\"arguments\":\"\",\"call_id\":\"call_2\",\"name\":\"call_tool\"},\"output_index\":2}\n\n")
					_, _ = io.WriteString(pw, "event: response.function_call_arguments.delta\ndata: {\"type\":\"response.function_call_arguments.delta\",\"delta\":\"{\\\"command\\\":\\\"echo \",\"item_id\":\"fc_2\",\"output_index\":2}\n\n")
					gate.wait()
					_, _ = io.WriteString(pw, "event: response.function_call_arguments.done\ndata: {\"type\":\"response.function_call_arguments.done\",\"arguments\":\"{\\\"command\\\":\\\"echo hi\\\"}\",\"item_id\":\"fc_2\",\"output_index\":2}\n\n")
					gate.wait()
					_, _ = io.WriteString(pw, "event: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{\"stop_reason\":\"tool_calls\",\"usage\":{\"input_tokens\":1,\"output_tokens\":1,\"total_tokens\":2,\"input_tokens_details\":{\"cached_tokens\":0},\"output_tokens_details\":{\"reasoning_tokens\":0}}}}\n\n")
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

	output, err := StreamV2OpenAINative(StreamInput{
		Context: context.Background(),
		Model:   "gpt-test",
		Prompt:  "hello",
		Tools: []ToolDefinition{
			{Name: "answer", Schema: map[string]any{"type": "object"}},
			{Name: "call_tool", Schema: map[string]any{"type": "object"}},
		},
		HTTPClient: httpClient,
		ProviderOptions: &models.LLMConfig{
			Name:       "openai",
			Type:       "openai",
			BaseURL:    "https://example.com/v1",
			APIKey:     "test-key",
			APIType:    models.LLMAPITypeResponse,
			MaxRetries: 1,
		},
	})
	if err != nil {
		t.Fatalf("StreamV2OpenAINative returned error: %v", err)
	}

	assertReadWithin(t, output.ReasonReader, "rea")
	gate.allow()
	assertReadWithin(t, output.ReasonReader, "son")
	gate.allow()
	assertReadWithin(t, output.ContentReader, "con")
	gate.allow()
	assertReadWithin(t, output.ContentReader, "tent")
	gate.allow()

	var first *CallToolRequest
	select {
	case first = <-output.ToolCalls:
	case <-time.After(streamingReadTimeout):
		t.Fatal("expected first native tool call within streaming timeout")
	}
	if first == nil || first.Name != "answer" {
		t.Fatalf("unexpected first tool call: %#v", first)
	}
	assertReadWithin(t, first.Arguments, `{"text":"he`)
	gate.allow()
	assertReadAllEquals(t, first.Arguments, `{"text":"he`, "", `{"text":"hello"}`)
	gate.allow()

	var second *CallToolRequest
	select {
	case second = <-output.ToolCalls:
	case <-time.After(streamingReadTimeout):
		t.Fatal("expected second native tool call within streaming timeout")
	}
	if second == nil || second.Name != "call_tool" {
		t.Fatalf("unexpected second tool call: %#v", second)
	}
	assertReadWithin(t, second.Arguments, `{"command":"echo `)
	gate.allow()
	assertReadAllEquals(t, second.Arguments, `{"command":"echo `, "", `{"command":"echo hi"}`)
	gate.allow()

	if err := output.Wait(); err != nil {
		t.Fatalf("Wait returned error: %v", err)
	}
}
