package tests

import (
	"context"
	"encoding/json"
	"io"
	"runtime"
	"strings"
	"testing"
	"time"

	"matrixops-agent/llm"
	"matrixops-agent/provider"
	"matrixops-agent/session"
	coreagent "matrixops.local/core_agent"
)

type regressionControlSink struct {
	ch chan *coreagent.ActionOutput
}

func newRegressionControlSink() (*regressionControlSink, coreagent.CompatibleControlHandler) {
	s := &regressionControlSink{ch: make(chan *session.ActionOutput, 8)}
	return s, func(action *coreagent.ActionOutput) error {
		s.ch <- action
		return nil
	}
}

func (s *regressionControlSink) recv(t *testing.T, timeout time.Duration) *coreagent.ActionOutput {
	t.Helper()
	select {
	case action := <-s.ch:
		return action
	case <-time.After(timeout):
		t.Fatal("timeout waiting for compatible control action")
		return nil
	}
}

func TestStreamV2_ParsesActionWhenActionFieldComesLast(t *testing.T) {
	client := provider.NewMockClientWithStreamCallback(func(request llm.ChatRequest) (<-chan llm.StreamEvent, error) {
		events := make(chan llm.StreamEvent, 3)
		go func() {
			defer close(events)
			events <- llm.StreamEvent{
				Type: string(llm.GeneratorMessageTypeTextDelta),
				Text: `{"data":"后置 action",`,
			}
			events <- llm.StreamEvent{
				Type: string(llm.GeneratorMessageTypeTextDelta),
				Text: `"@action":"answer"}`,
			}
			events <- llm.StreamEvent{
				Type:  string(llm.GeneratorMessageTypeFinish),
				Usage: &llm.Usage{InputTokens: 10, OutputTokens: 5},
			}
		}()
		return events, nil
	})

	sink, controlHandler := newRegressionControlSink()
	output, err := session.StreamV2(session.StreamInputV2{
		Context:                  context.Background(),
		Model:                    "mock-model",
		Prompt:                   "say hi",
		CompatibleControlHandler: controlHandler,
	}, client)
	if err != nil {
		t.Fatalf("StreamV2 returned error: %v", err)
	}
	action := sink.recv(t, 2*time.Second)
	if action.Action != "answer" {
		t.Fatalf("expected action=answer, got %q", action.Action)
	}

	payloadBytes, err := io.ReadAll(action.Data)
	if err != nil {
		t.Fatalf("read action data: %v", err)
	}
	if err := output.Wait(); err != nil {
		t.Fatalf("wait stream: %v", err)
	}

	var answer string
	if err := json.Unmarshal(payloadBytes, &answer); err != nil {
		t.Fatalf("unmarshal answer payload: %v", err)
	}
	if answer != "后置 action" {
		t.Fatalf("unexpected answer payload: %q", answer)
	}
}

func TestStreamV2_LargeToolPayloadDoesNotClosePipeEarly(t *testing.T) {
	oldProcs := runtime.GOMAXPROCS(1)
	defer runtime.GOMAXPROCS(oldProcs)

	blob := strings.Repeat("x", 1<<20)
	response := `{"@action":"call_tool","data":{"name":"lookup_weather","params":{"blob":"` + blob + `"}}}`

	client := provider.NewMockClientWithStreamCallback(func(request llm.ChatRequest) (<-chan llm.StreamEvent, error) {
		events := make(chan llm.StreamEvent, 2)
		go func() {
			defer close(events)
			events <- llm.StreamEvent{
				Type: string(llm.GeneratorMessageTypeTextDelta),
				Text: response,
			}
			events <- llm.StreamEvent{
				Type:  string(llm.GeneratorMessageTypeFinish),
				Usage: &llm.Usage{InputTokens: 10, OutputTokens: 10},
			}
		}()
		return events, nil
	})

	output, err := session.StreamV2(session.StreamInputV2{
		Context: context.Background(),
		Model:   "mock-model",
		Prompt:  "call tool",
	}, client)
	if err != nil {
		t.Fatalf("StreamV2 returned error: %v", err)
	}
	req, ok := <-output.ToolCalls
	if !ok || req == nil {
		t.Fatal("expected tool call to be parsed")
	}
	if req.Name != "call_tool" {
		t.Fatalf("expected tool call name=call_tool, got %q", req.Name)
	}

	payloadBytes, err := io.ReadAll(req.Arguments)
	if err != nil {
		t.Fatalf("read tool payload: %v", err)
	}
	if err := output.Wait(); err != nil {
		t.Fatalf("wait stream: %v", err)
	}

	var payload struct {
		Name   string                 `json:"name"`
		Params map[string]interface{} `json:"params"`
	}
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		t.Fatalf("unmarshal tool payload: %v", err)
	}
	if payload.Name != "lookup_weather" {
		t.Fatalf("unexpected tool name: %q", payload.Name)
	}
	gotBlob, _ := payload.Params["blob"].(string)
	if len(gotBlob) != len(blob) {
		t.Fatalf("unexpected blob length: got=%d want=%d", len(gotBlob), len(blob))
	}
}

func TestStreamV2_ParsesMultipleConcatenatedActions(t *testing.T) {
	client := provider.NewMockClientWithStreamCallback(func(request llm.ChatRequest) (<-chan llm.StreamEvent, error) {
		events := make(chan llm.StreamEvent, 2)
		go func() {
			defer close(events)
			events <- llm.StreamEvent{
				Type: string(llm.GeneratorMessageTypeTextDelta),
				Text: `{"@action":"message","data":{"message":"先看一下","next_step":"读取文件"}}{"@action":"call_tool","data":{"name":"read","params":{"path":"README.md"}}}`,
			}
			events <- llm.StreamEvent{
				Type: string(llm.GeneratorMessageTypeFinish),
			}
		}()
		return events, nil
	})

	sink, controlHandler := newRegressionControlSink()
	output, err := session.StreamV2(session.StreamInputV2{
		Context:                  context.Background(),
		Model:                    "mock-model",
		Prompt:                   "inspect",
		CompatibleControlHandler: controlHandler,
	}, client)
	if err != nil {
		t.Fatalf("StreamV2 returned error: %v", err)
	}

	first := sink.recv(t, 2*time.Second)
	second, ok := <-output.ToolCalls
	if !ok || second == nil {
		t.Fatal("expected second tool call")
	}
	if _, ok := <-output.ToolCalls; ok {
		t.Fatal("expected exactly 1 tool call")
	}

	if first.Action != "message" {
		t.Fatalf("expected first action=message, got %q", first.Action)
	}
	if second.Name != "call_tool" {
		t.Fatalf("expected second tool call name=call_tool, got %q", second.Name)
	}
	if _, err := io.ReadAll(first.Data); err != nil {
		t.Fatalf("read first action data: %v", err)
	}
	toolPayload, err := io.ReadAll(second.Arguments)
	if err != nil {
		t.Fatalf("read second tool args: %v", err)
	}
	if !strings.Contains(string(toolPayload), `"name":"read"`) {
		t.Fatalf("expected read tool in payload, got %q", string(toolPayload))
	}
	if !strings.Contains(first.RawJSON, `"@action":"message"`) {
		t.Fatalf("expected first raw JSON to preserve @action, got %q", first.RawJSON)
	}

	if err := output.Wait(); err != nil {
		t.Fatalf("wait stream: %v", err)
	}
	rawText, err := io.ReadAll(output.RawTextReader)
	if err != nil {
		t.Fatalf("read raw text: %v", err)
	}
	if !strings.Contains(string(rawText), `"@action":"call_tool"`) {
		t.Fatalf("expected raw stream to preserve call_tool envelope, got %q", string(rawText))
	}
}
