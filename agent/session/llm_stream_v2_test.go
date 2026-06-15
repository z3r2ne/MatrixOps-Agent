package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"matrixops-agent/llm"
	"matrixops-agent/provider"
	coreagent "matrixops.local/core_agent"
	"pkgs/db/models"
)

type scriptedStreamChatClient struct {
	stream chan llm.StreamEvent
}

func newScriptedStreamChatClient() *scriptedStreamChatClient {
	return &scriptedStreamChatClient{
		stream: make(chan llm.StreamEvent, 128),
	}
}

func (m *scriptedStreamChatClient) StreamChatWithOptions(req llm.ChatRequest, opts ...llm.StreamChatOption) (<-chan llm.StreamEvent, error) {
	options := llm.NewStreamChatOptions(opts...)
	if options.OnRequest != nil {
		if err := options.OnRequest(&req); err != nil {
			return nil, err
		}
	}
	return m.stream, nil
}

func (m *scriptedStreamChatClient) Chat(req llm.ChatRequest) (llm.ChatResponse, error) {
	return llm.ChatResponse{}, errors.New("not implemented")
}

func (m *scriptedStreamChatClient) sendText(text string) {
	m.stream <- llm.StreamEvent{
		Type: string(llm.GeneratorMessageTypeTextDelta),
		Text: text,
	}
}

func (m *scriptedStreamChatClient) finish() {
	m.stream <- llm.StreamEvent{
		Type:   string(llm.GeneratorMessageTypeFinish),
		Finish: "stop",
		Usage: &llm.Usage{
			InputTokens:  10,
			OutputTokens: 20,
		},
	}
	close(m.stream)
}

func (m *scriptedStreamChatClient) close() {
	close(m.stream)
}

type sessionControlSink struct {
	ch chan *ActionOutput
}

func newSessionControlSink() (*sessionControlSink, coreagent.CompatibleControlHandler) {
	s := &sessionControlSink{ch: make(chan *ActionOutput, 16)}
	return s, s.handle
}

func (s *sessionControlSink) handle(action *ActionOutput) error {
	s.ch <- action
	return nil
}

func (s *sessionControlSink) recv(t *testing.T, timeout time.Duration) *ActionOutput {
	t.Helper()
	select {
	case action := <-s.ch:
		return action
	case <-time.After(timeout):
		t.Fatal("timeout waiting for compatible control action")
		return nil
	}
}

func TestStreamV2_AssemblesSplitChunksIntoOneAction(t *testing.T) {
	client := newScriptedStreamChatClient()
	sink, controlHandler := newSessionControlSink()

	output, err := StreamV2(StreamInputV2{
		Context:                  context.Background(),
		Model:                    "gpt-test",
		Prompt:                   "hello",
		CompatibleControlHandler: controlHandler,
	}, client)
	if err != nil {
		t.Fatalf("StreamV2 returned error: %v", err)
	}

	client.sendText(`{"@action":"answer","data":"hel`)

	action := sink.recv(t, 150*time.Millisecond)
	if action.Action != "answer" {
		t.Fatalf("expected action=answer, got %q", action.Action)
	}
	partBuf := make([]byte, 16)
	n, err := action.Data.Read(partBuf)
	if err != nil && err != io.EOF {
		t.Fatalf("read first action chunk: %v", err)
	}
	if got := string(partBuf[:n]); !strings.HasPrefix(`"hel`, got) {
		t.Fatalf("expected first params chunk prefix %q, got %q", `"hel`, got)
	}
	firstPrefix := string(partBuf[:n])

	client.sendText(`lo"}`)
	client.finish()

	rest, err := io.ReadAll(action.Data)
	if err != nil {
		t.Fatalf("ReadAll action.Data failed: %v", err)
	}
	if got := firstPrefix + string(rest); got != `"hello"` {
		t.Fatalf("expected params data %q, got %q", `"hello"`, got)
	}

	if err := output.Wait(); err != nil {
		t.Fatalf("Wait returned error: %v", err)
	}

	rawText, err := io.ReadAll(output.RawTextReader)
	if err != nil {
		t.Fatalf("ReadAll RawTextReader failed: %v", err)
	}
	if string(rawText) != `{"@action":"answer","data":"hello"}` {
		t.Fatalf("unexpected raw text: %q", string(rawText))
	}
	if action.RawJSON != `{"@action":"answer","data":"hello"}` {
		t.Fatalf("unexpected raw action json: %q", action.RawJSON)
	}
}

func TestStreamV2_StreamsMultipleActionsSequentially(t *testing.T) {
	client := newScriptedStreamChatClient()
	sink, controlHandler := newSessionControlSink()

	output, err := StreamV2(StreamInputV2{
		Context:                  context.Background(),
		Model:                    "gpt-test",
		Prompt:                   "hello",
		CompatibleControlHandler: controlHandler,
	}, client)
	if err != nil {
		t.Fatalf("StreamV2 returned error: %v", err)
	}

	client.sendText(`{"@action":"message","data":"one"}`)

	first := sink.recv(t, 500*time.Millisecond)
	if first.Action != "message" {
		t.Fatalf("expected first action=message, got %q", first.Action)
	}
	firstData, err := io.ReadAll(first.Data)
	if err != nil {
		t.Fatalf("ReadAll first action data failed: %v", err)
	}
	if string(firstData) != `"one"` {
		t.Fatalf("unexpected first action data: %q", string(firstData))
	}

	client.sendText(`{"@action":"answer","data":"two"}`)
	client.finish()

	second := sink.recv(t, 500*time.Millisecond)
	if second.Action != "answer" {
		t.Fatalf("expected second action=answer, got %q", second.Action)
	}
	secondData, err := io.ReadAll(second.Data)
	if err != nil {
		t.Fatalf("ReadAll second action data failed: %v", err)
	}
	if string(secondData) != `"two"` {
		t.Fatalf("unexpected second action data: %q", string(secondData))
	}

	if err := output.Wait(); err != nil {
		t.Fatalf("Wait returned error: %v", err)
	}
}

func TestStreamV2_WaitAllowsIncompleteJSONAfterActionStarted(t *testing.T) {
	client := newScriptedStreamChatClient()
	sink, controlHandler := newSessionControlSink()

	output, err := StreamV2(StreamInputV2{
		Context:                  context.Background(),
		Model:                    "gpt-test",
		Prompt:                   "hello",
		CompatibleControlHandler: controlHandler,
	}, client)
	if err != nil {
		t.Fatalf("StreamV2 returned error: %v", err)
	}

	client.sendText(`{"@action":"answer","data":"hello"`)
	client.close()

	if action := sink.recv(t, time.Second); action == nil {
		t.Fatal("expected action before close on incomplete JSON")
	}
	if err := output.Wait(); err != nil {
		t.Fatalf("expected Wait to succeed once action stream has started, got %v", err)
	}
}

func TestStreamV2_WithMockLLMServerStreamsActionDataStepByStep(t *testing.T) {
	proceed := make(chan struct{})
	firstExpectedDataCh := make(chan string, 1)
	secondExpectedDataCh := make(chan string, 1)

	writeDelta := func(w http.ResponseWriter, flusher http.Flusher, delta string) {
		payload, _ := json.Marshal(map[string]string{
			"delta": delta,
		})
		fmt.Fprintf(w, "event: response.output_text.delta\ndata: %s\n\n", payload)
		flusher.Flush()
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/responses" {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("response writer does not support flushing")
		}

		done := make(chan struct{})
		go func() {
			defer close(done)

			firstExpectedData := `{"text":"hello"}`
			writeDelta(w, flusher, `{"@action":"answer",`)
			writeDelta(w, flusher, `"data":{"te`)
			writeDelta(w, flusher, `xt":"hello"}}`)
			firstExpectedDataCh <- firstExpectedData
			<-proceed

			secondExpectedData := `{"name":"bash","params":{"command":"echo hi"}}`
			writeDelta(w, flusher, `{"@action":"call_tool",`)
			writeDelta(w, flusher, `"data":{"name":"ba`)
			writeDelta(w, flusher, `sh","params":{"command":"echo hi"}}}`)
			secondExpectedDataCh <- secondExpectedData
			<-proceed

			fmt.Fprint(w, "data: [DONE]\n\n")
			flusher.Flush()
		}()

		<-done
	}))
	defer server.Close()

	llmClient := provider.NewGenericClient()
	sink, controlHandler := newSessionControlSink()
	output, err := StreamV2(StreamInputV2{
		Context:                  context.Background(),
		Model:                    "gpt-test",
		Prompt:                   "hello",
		CompatibleControlHandler: controlHandler,
		ProviderOptions: &models.LLMConfig{
			Name:       "openai",
			BaseURL:    server.URL,
			APIKey:     "test-key",
			MaxRetries: 0,
		},
	}, llmClient)
	if err != nil {
		t.Fatalf("StreamV2 returned error: %v", err)
	}

	firstAction := sink.recv(t, 2*time.Second)
	if firstAction == nil {
		t.Fatal("expected first action")
	}
	if firstAction.Action != "answer" {
		t.Fatalf("expected first action=answer, got %q", firstAction.Action)
	}

	firstExpectedData := <-firstExpectedDataCh
	firstRest, err := io.ReadAll(firstAction.Data)
	if err != nil {
		t.Fatalf("ReadAll first action data failed: %v", err)
	}
	if string(firstRest) != firstExpectedData {
		t.Fatalf("expected first action data %q, got %q", firstExpectedData, string(firstRest))
	}
	if firstAction.RawJSON != fmt.Sprintf(`{"@action":"answer","data":%s}`, firstExpectedData) {
		t.Fatalf("unexpected first action raw json: %q", firstAction.RawJSON)
	}

	proceed <- struct{}{}

	var secondTool *CallToolRequest
	select {
	case secondTool = <-output.ToolCalls:
	case <-time.After(2 * time.Second):
		t.Fatal("expected second tool call after second complete JSON object")
	}
	if secondTool == nil {
		t.Fatal("expected second tool call")
	}
	if secondTool.Name != "call_tool" {
		t.Fatalf("expected second tool name=call_tool, got %q", secondTool.Name)
	}

	secondExpectedData := <-secondExpectedDataCh
	secondRest, err := io.ReadAll(secondTool.Arguments)
	if err != nil {
		t.Fatalf("ReadAll second tool args failed: %v", err)
	}
	if string(secondRest) != secondExpectedData {
		t.Fatalf("expected second tool args %q, got %q", secondExpectedData, string(secondRest))
	}

	proceed <- struct{}{}

	if err := output.Wait(); err != nil {
		t.Fatalf("Wait returned error: %v", err)
	}

	rawText, err := io.ReadAll(output.RawTextReader)
	if err != nil {
		t.Fatalf("ReadAll RawTextReader failed: %v", err)
	}
	expectedRawText := `{"@action":"answer","data":{"text":"hello"}}{"@action":"call_tool","data":{"name":"bash","params":{"command":"echo hi"}}}`
	if string(rawText) != expectedRawText {
		t.Fatalf("unexpected raw text: %q", string(rawText))
	}
}
