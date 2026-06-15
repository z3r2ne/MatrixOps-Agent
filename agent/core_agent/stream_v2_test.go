package coreagent

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	agentprovider "matrixops-agent/provider"
	actioncompatible "matrixops.local/core_agent/action_providers/compatible"
	"pkgs/db/models"
	"pkgs/jsonextractor"
)

type scriptedStreamChatClient struct {
	stream chan StreamEvent
	req    ChatRequest
}

func newScriptedStreamChatClient() *scriptedStreamChatClient {
	return &scriptedStreamChatClient{stream: make(chan StreamEvent, 128)}
}

func (m *scriptedStreamChatClient) StreamChatWithOptions(req ChatRequest, opts ...StreamChatOption) (<-chan StreamEvent, error) {
	m.req = req
	options := NewStreamChatOptions(opts...)
	if options.OnRequest != nil {
		if err := options.OnRequest(&req); err != nil {
			return nil, err
		}
	}
	return m.stream, nil
}

func (m *scriptedStreamChatClient) Chat(req ChatRequest) (ChatResponse, error) {
	return ChatResponse{}, errors.New("not implemented")
}

func (m *scriptedStreamChatClient) sendText(text string) {
	m.stream <- StreamEvent{Type: "text-delta", Text: text}
}

func (m *scriptedStreamChatClient) finish() {
	m.stream <- StreamEvent{Type: "finish", Finish: "stop", Usage: &Usage{InputTokens: 10, OutputTokens: 20}}
	close(m.stream)
}

func (m *scriptedStreamChatClient) close() {
	close(m.stream)
}

type retryingStreamChatClient struct {
	attempts int
	mode     string
}

type retryingProviderRoundTripper struct {
	mu       sync.Mutex
	attempts int
	mode     string
}

type providerFailureThenSuccessClient struct {
	attempts int
	delegate *GenericProviderClient
}

type htmlRawResponseThenSuccessClient struct {
	attempts int
}

type rawResponseThenSuccessClient struct {
	attempts int
	firstRaw string
}

type messageThenIncompleteToolThenSuccessClient struct {
	attempts int
}

type truncatedEnvelopeThenSuccessClient struct {
	attempts int
}

func (rt *retryingProviderRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	rt.attempts++
	switch rt.mode {
	case "html-content-type":
		return &http.Response{
			StatusCode: http.StatusOK,
			Header: http.Header{
				"Content-Type":   []string{"text/html; charset=utf-8"},
				"Retry-After-Ms": []string{"1"},
			},
			Body: io.NopCloser(strings.NewReader(`<html><body><h1>proxy error</h1></body></html>`)),
		}, nil
	case "html-body-event-stream":
		return &http.Response{
			StatusCode: http.StatusOK,
			Header: http.Header{
				"Content-Type":   []string{"text/event-stream"},
				"Retry-After-Ms": []string{"1"},
			},
			Body: io.NopCloser(strings.NewReader(`<html><body><h1>proxy error</h1></body></html>`)),
		}, nil
	}
	return nil, errors.New("unexpected test mode")
}

func (c *providerFailureThenSuccessClient) StreamChatWithOptions(req ChatRequest, opts ...StreamChatOption) (<-chan StreamEvent, error) {
	c.attempts++
	if c.attempts == 1 {
		return c.delegate.StreamChatWithOptions(req, opts...)
	}

	stream := make(chan StreamEvent, 2)
	go func() {
		defer close(stream)
		stream <- StreamEvent{Type: "text-delta", Text: `{"@action":"answer","data":"ok"}`}
		stream <- StreamEvent{Type: "finish", Finish: "stop"}
	}()
	return stream, nil
}

func (c *providerFailureThenSuccessClient) Chat(req ChatRequest) (ChatResponse, error) {
	return ChatResponse{}, errors.New("not implemented")
}

func (c *htmlRawResponseThenSuccessClient) StreamChatWithOptions(req ChatRequest, opts ...StreamChatOption) (<-chan StreamEvent, error) {
	c.attempts++
	options := NewStreamChatOptions(opts...)
	stream := make(chan StreamEvent, 2)
	go func() {
		defer close(stream)
		if c.attempts == 1 {
			if options.OnRawResponse != nil {
				options.OnRawResponse(`<html><head><title>Burp Suite Community Edition</title></head><body><h1>Error</h1><p>Stream failed to close correctly</p></body></html>`)
			}
			stream <- StreamEvent{Type: "finish", Finish: "stop"}
			return
		}
		stream <- StreamEvent{Type: "text-delta", Text: `{"@action":"answer","data":"ok"}`}
		stream <- StreamEvent{Type: "finish", Finish: "stop"}
	}()
	return stream, nil
}

func (c *htmlRawResponseThenSuccessClient) Chat(req ChatRequest) (ChatResponse, error) {
	return ChatResponse{}, errors.New("not implemented")
}

func (c *rawResponseThenSuccessClient) StreamChatWithOptions(req ChatRequest, opts ...StreamChatOption) (<-chan StreamEvent, error) {
	c.attempts++
	options := NewStreamChatOptions(opts...)
	stream := make(chan StreamEvent, 2)
	go func() {
		defer close(stream)
		if c.attempts == 1 {
			if options.OnRawResponse != nil {
				options.OnRawResponse(c.firstRaw)
			}
			stream <- StreamEvent{Type: "finish", Finish: "stop"}
			return
		}
		stream <- StreamEvent{Type: "text-delta", Text: `{"@action":"answer","data":"ok"}`}
		stream <- StreamEvent{Type: "finish", Finish: "stop"}
	}()
	return stream, nil
}

func (c *rawResponseThenSuccessClient) Chat(req ChatRequest) (ChatResponse, error) {
	return ChatResponse{}, errors.New("not implemented")
}

func (c *messageThenIncompleteToolThenSuccessClient) StreamChatWithOptions(req ChatRequest, opts ...StreamChatOption) (<-chan StreamEvent, error) {
	c.attempts++
	stream := make(chan StreamEvent, 4)
	go func() {
		defer close(stream)
		if c.attempts == 1 {
			stream <- StreamEvent{Type: "text-delta", Text: `{"@action":"message","data":{"message":"working","next_step":"continue"}}`}
			stream <- StreamEvent{Type: "text-delta", Text: `{"@action":"call_tool","data":{"name":"read","params":{"path":"/tmp/demo"`}
			stream <- StreamEvent{Type: "finish", Finish: "stop"}
			return
		}
		stream <- StreamEvent{Type: "text-delta", Text: `{"@action":"answer","data":"ok"}`}
		stream <- StreamEvent{Type: "finish", Finish: "stop"}
	}()
	return stream, nil
}

func (c *messageThenIncompleteToolThenSuccessClient) Chat(req ChatRequest) (ChatResponse, error) {
	return ChatResponse{}, errors.New("not implemented")
}

func (c *truncatedEnvelopeThenSuccessClient) StreamChatWithOptions(req ChatRequest, opts ...StreamChatOption) (<-chan StreamEvent, error) {
	c.attempts++
	stream := make(chan StreamEvent, 2)
	go func() {
		defer close(stream)
		if c.attempts == 1 {
			stream <- StreamEvent{Type: "text-delta", Text: `{"@action":"answer","data":"par`}
			stream <- StreamEvent{Type: "finish", Finish: "stop"}
			return
		}
		stream <- StreamEvent{Type: "text-delta", Text: `{"@action":"answer","data":"ok"}`}
		stream <- StreamEvent{Type: "finish", Finish: "stop"}
	}()
	return stream, nil
}

func (c *truncatedEnvelopeThenSuccessClient) Chat(req ChatRequest) (ChatResponse, error) {
	return ChatResponse{}, errors.New("not implemented")
}

func (m *retryingStreamChatClient) StreamChatWithOptions(req ChatRequest, opts ...StreamChatOption) (<-chan StreamEvent, error) {
	m.attempts++
	stream := make(chan StreamEvent, 8)
	go func() {
		defer close(stream)
		switch m.mode {
		case "error-before-output":
			if m.attempts == 1 {
				stream <- StreamEvent{Type: "error", Error: &agentprovider.APIError{Message: "rate limited", StatusCode: 429, IsRetryable: true, ResponseHeaders: map[string]string{"retry-after-ms": "1"}}}
				return
			}
		case "error-after-output":
			if m.attempts == 1 {
				stream <- StreamEvent{Type: "text-delta", Text: `{"@action":"answer","data":"partial`}
				stream <- StreamEvent{Type: "error", Error: &agentprovider.APIError{Message: "rate limited", StatusCode: 429, IsRetryable: true}}
				return
			}
		}
		stream <- StreamEvent{Type: "text-delta", Text: `{"@action":"answer","data":"ok"}`}
		stream <- StreamEvent{Type: "finish", Finish: "stop"}
	}()
	return stream, nil
}

func (m *retryingStreamChatClient) Chat(req ChatRequest) (ChatResponse, error) {
	return ChatResponse{}, errors.New("not implemented")
}

type contentFilterThenSuccessClient struct {
	attempts int
}

func (c *contentFilterThenSuccessClient) StreamChatWithOptions(req ChatRequest, opts ...StreamChatOption) (<-chan StreamEvent, error) {
	c.attempts++
	stream := make(chan StreamEvent, 4)
	go func() {
		defer close(stream)
		if c.attempts == 1 {
			stream <- StreamEvent{Type: "finish", Finish: "content_filter"}
			return
		}
		stream <- StreamEvent{Type: "text-delta", Text: `{"@action":"answer","data":"ok"}`}
		stream <- StreamEvent{Type: "finish", Finish: "stop"}
	}()
	return stream, nil
}

func (c *contentFilterThenSuccessClient) Chat(req ChatRequest) (ChatResponse, error) {
	return ChatResponse{}, errors.New("not implemented")
}

func TestStreamV2RetriesContentFilterBeforeOutput(t *testing.T) {
	client := &contentFilterThenSuccessClient{}
	var retryAttempts []int
	sink, controlHandler := newCompatibleControlSink()
	output, err := StreamV2(withCompatibleControl(StreamInput{
		Context: context.Background(),
		Model:   "gpt-test",
		Prompt:  "hello",
		ProviderOptions: &models.LLMConfig{
			Name:       "openai",
			Model:      "gpt-test",
			MaxRetries: 1,
		},
		OnRetryError: func(err error, retryAttempt int, maxRetries int, nextDelay time.Duration, attemptDuration time.Duration, rawResponse string) {
			retryAttempts = append(retryAttempts, retryAttempt)
		},
	}, controlHandler), client)
	if err != nil {
		t.Fatalf("StreamV2 returned error: %v", err)
	}

	expectCompatibleAnswerAfterRetry(t, sink, output)
	if client.attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", client.attempts)
	}
	if len(retryAttempts) != 1 || retryAttempts[0] != 1 {
		t.Fatalf("unexpected retry callbacks: %#v", retryAttempts)
	}
}

func TestStreamV2RetriesRetryableStreamFailureBeforeOutput(t *testing.T) {
	client := &retryingStreamChatClient{mode: "error-before-output"}
	var retryAttempts []int
	sink, controlHandler := newCompatibleControlSink()
	output, err := StreamV2(withCompatibleControl(StreamInput{
		Context: context.Background(),
		Model:   "gpt-test",
		Prompt:  "hello",
		ProviderOptions: &models.LLMConfig{
			Name:       "openai",
			Model:      "gpt-test",
			MaxRetries: 1,
		},
		OnRetryError: func(err error, retryAttempt int, maxRetries int, nextDelay time.Duration, attemptDuration time.Duration, rawResponse string) {
			retryAttempts = append(retryAttempts, retryAttempt)
		},
	}, controlHandler), client)
	if err != nil {
		t.Fatalf("StreamV2 returned error: %v", err)
	}

	expectCompatibleAnswerAfterRetry(t, sink, output)
	if client.attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", client.attempts)
	}
	if len(retryAttempts) != 1 || retryAttempts[0] != 1 {
		t.Fatalf("unexpected retry callbacks: %#v", retryAttempts)
	}
}

func TestStreamV2RetriesAfterTruncatedEnvelopeBeforeAnyAction(t *testing.T) {
	client := &truncatedEnvelopeThenSuccessClient{}
	var retryAttempts []int
	sink, controlHandler := newCompatibleControlSink()
	output, err := StreamV2(withCompatibleControl(StreamInput{
		Context: context.Background(),
		Model:   "gpt-test",
		Prompt:  "hello",
		ProviderOptions: &models.LLMConfig{
			Name:       "openai",
			Model:      "gpt-test",
			MaxRetries: 1,
		},
		OnRetryError: func(err error, retryAttempt int, maxRetries int, nextDelay time.Duration, attemptDuration time.Duration, rawResponse string) {
			retryAttempts = append(retryAttempts, retryAttempt)
		},
	}, controlHandler), client)
	if err != nil {
		t.Fatalf("StreamV2 returned error: %v", err)
	}

	expectCompatibleAnswerAfterRetry(t, sink, output)
	if client.attempts != 2 {
		t.Fatalf("expected retry after truncated envelope, got %d attempts", client.attempts)
	}
	if len(retryAttempts) != 1 || retryAttempts[0] != 1 {
		t.Fatalf("unexpected retry callbacks: %#v", retryAttempts)
	}
}

func TestStreamV2RetriesRetryableProviderContentTypeMismatch(t *testing.T) {
	roundTripper := &retryingProviderRoundTripper{mode: "html-content-type"}
	httpClient := &http.Client{Transport: roundTripper}
	client := &providerFailureThenSuccessClient{delegate: NewGenericProviderClient()}

	var retryAttempts []int
	var retryRawResponses []string
	sink, controlHandler := newCompatibleControlSink()
	output, err := StreamV2(withCompatibleControl(StreamInput{
		Context:    context.Background(),
		Model:      "gpt-test",
		Prompt:     "hello",
		HTTPClient: httpClient,
		ProviderOptions: &models.LLMConfig{
			Name:       "openai",
			Model:      "gpt-test",
			BaseURL:    "https://example.com/v1",
			APIKey:     "test-key",
			MaxRetries: 1,
		},
		OnRetryError: func(err error, retryAttempt int, maxRetries int, nextDelay time.Duration, attemptDuration time.Duration, rawResponse string) {
			retryAttempts = append(retryAttempts, retryAttempt)
			retryRawResponses = append(retryRawResponses, rawResponse)
		},
	}, controlHandler), client)
	if err != nil {
		t.Fatalf("StreamV2 returned error: %v", err)
	}

	expectCompatibleAnswerAfterRetry(t, sink, output)
	if client.attempts != 2 {
		t.Fatalf("expected 2 stream attempts, got %d", client.attempts)
	}
	if roundTripper.attempts != 1 {
		t.Fatalf("expected provider to be hit once for the failing attempt, got %d", roundTripper.attempts)
	}
	if len(retryAttempts) != 1 || retryAttempts[0] != 1 {
		t.Fatalf("unexpected retry callbacks: %#v", retryAttempts)
	}
	if len(retryRawResponses) != 1 || !strings.Contains(retryRawResponses[0], "proxy error") {
		t.Fatalf("expected retry raw response to contain proxy body, got %#v", retryRawResponses)
	}
}

func TestStreamV2RetriesRetryableProviderStreamWithoutParsedChunks(t *testing.T) {
	roundTripper := &retryingProviderRoundTripper{mode: "html-body-event-stream"}
	httpClient := &http.Client{Transport: roundTripper}
	client := &providerFailureThenSuccessClient{delegate: NewGenericProviderClient()}

	var retryAttempts []int
	var retryRawResponses []string
	sink, controlHandler := newCompatibleControlSink()
	output, err := StreamV2(withCompatibleControl(StreamInput{
		Context:    context.Background(),
		Model:      "gpt-test",
		Prompt:     "hello",
		HTTPClient: httpClient,
		ProviderOptions: &models.LLMConfig{
			Name:       "openai",
			Model:      "gpt-test",
			BaseURL:    "https://example.com/v1",
			APIKey:     "test-key",
			MaxRetries: 1,
		},
		OnRetryError: func(err error, retryAttempt int, maxRetries int, nextDelay time.Duration, attemptDuration time.Duration, rawResponse string) {
			retryAttempts = append(retryAttempts, retryAttempt)
			retryRawResponses = append(retryRawResponses, rawResponse)
		},
	}, controlHandler), client)
	if err != nil {
		t.Fatalf("StreamV2 returned error: %v", err)
	}

	expectCompatibleAnswerAfterRetry(t, sink, output)
	if client.attempts != 2 {
		t.Fatalf("expected 2 stream attempts, got %d", client.attempts)
	}
	if roundTripper.attempts != 1 {
		t.Fatalf("expected provider to be hit once for the failing attempt, got %d", roundTripper.attempts)
	}
	if len(retryAttempts) != 1 || retryAttempts[0] != 1 {
		t.Fatalf("unexpected retry callbacks: %#v", retryAttempts)
	}
	if len(retryRawResponses) != 1 || !strings.Contains(retryRawResponses[0], "proxy error") {
		t.Fatalf("expected retry raw response to contain proxy body, got %#v", retryRawResponses)
	}
}

func TestStreamV2RetriesWhenOnlyRawHTMLResponseIsCaptured(t *testing.T) {
	client := &htmlRawResponseThenSuccessClient{}
	var retryAttempts []int
	var retryRawResponses []string
	sink, controlHandler := newCompatibleControlSink()

	output, err := StreamV2(withCompatibleControl(StreamInput{
		Context: context.Background(),
		Model:   "gpt-test",
		Prompt:  "hello",
		ProviderOptions: &models.LLMConfig{
			Name:       "openai",
			Model:      "gpt-test",
			MaxRetries: 1,
		},
		OnRetryError: func(err error, retryAttempt int, maxRetries int, nextDelay time.Duration, attemptDuration time.Duration, rawResponse string) {
			retryAttempts = append(retryAttempts, retryAttempt)
			retryRawResponses = append(retryRawResponses, rawResponse)
		},
	}, controlHandler), client)
	if err != nil {
		t.Fatalf("StreamV2 returned error: %v", err)
	}

	expectCompatibleAnswerAfterRetry(t, sink, output)
	if client.attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", client.attempts)
	}
	if len(retryAttempts) != 1 || retryAttempts[0] != 1 {
		t.Fatalf("unexpected retry callbacks: %#v", retryAttempts)
	}
	if len(retryRawResponses) != 1 || !strings.Contains(retryRawResponses[0], "Burp Suite Community Edition") {
		t.Fatalf("expected retry raw response to contain html body, got %#v", retryRawResponses)
	}
}

func TestStreamV2RetriesWhenOnlyRawProxyFailureTextIsCaptured(t *testing.T) {
	client := &rawResponseThenSuccessClient{firstRaw: "upstream connect error or disconnect/reset before headers. reset reason: connection termination"}
	var retryAttempts []int
	var retryRawResponses []string
	sink, controlHandler := newCompatibleControlSink()

	output, err := StreamV2(withCompatibleControl(StreamInput{
		Context: context.Background(),
		Model:   "gpt-test",
		Prompt:  "hello",
		ProviderOptions: &models.LLMConfig{
			Name:       "openai",
			Model:      "gpt-test",
			MaxRetries: 1,
		},
		OnRetryError: func(err error, retryAttempt int, maxRetries int, nextDelay time.Duration, attemptDuration time.Duration, rawResponse string) {
			retryAttempts = append(retryAttempts, retryAttempt)
			retryRawResponses = append(retryRawResponses, rawResponse)
		},
	}, controlHandler), client)
	if err != nil {
		t.Fatalf("StreamV2 returned error: %v", err)
	}

	expectCompatibleAnswerAfterRetry(t, sink, output)
	if client.attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", client.attempts)
	}
	if len(retryAttempts) != 1 || retryAttempts[0] != 1 {
		t.Fatalf("unexpected retry callbacks: %#v", retryAttempts)
	}
	if len(retryRawResponses) != 1 || !strings.Contains(retryRawResponses[0], "disconnect/reset before headers") {
		t.Fatalf("expected retry raw response to contain proxy text, got %#v", retryRawResponses)
	}
}

func TestStreamV2RetriesWhenOnlyRawProxyFailureXMLIsCaptured(t *testing.T) {
	client := &rawResponseThenSuccessClient{firstRaw: `<?xml version="1.0" encoding="UTF-8"?><Error><Code>BadGateway</Code><Message>upstream connect error</Message></Error>`}
	var retryAttempts []int
	var retryRawResponses []string
	sink, controlHandler := newCompatibleControlSink()

	output, err := StreamV2(withCompatibleControl(StreamInput{
		Context: context.Background(),
		Model:   "gpt-test",
		Prompt:  "hello",
		ProviderOptions: &models.LLMConfig{
			Name:       "openai",
			Model:      "gpt-test",
			MaxRetries: 1,
		},
		OnRetryError: func(err error, retryAttempt int, maxRetries int, nextDelay time.Duration, attemptDuration time.Duration, rawResponse string) {
			retryAttempts = append(retryAttempts, retryAttempt)
			retryRawResponses = append(retryRawResponses, rawResponse)
		},
	}, controlHandler), client)
	if err != nil {
		t.Fatalf("StreamV2 returned error: %v", err)
	}

	expectCompatibleAnswerAfterRetry(t, sink, output)
	if client.attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", client.attempts)
	}
	if len(retryAttempts) != 1 || retryAttempts[0] != 1 {
		t.Fatalf("unexpected retry callbacks: %#v", retryAttempts)
	}
	if len(retryRawResponses) != 1 || !strings.Contains(retryRawResponses[0], "<Error>") {
		t.Fatalf("expected retry raw response to contain xml body, got %#v", retryRawResponses)
	}
}

func TestStreamV2_AssemblesSplitChunksIntoOneAction(t *testing.T) {
	client := newScriptedStreamChatClient()
	sink, controlHandler := newCompatibleControlSink()
	output, err := StreamV2(withCompatibleControl(StreamInput{Context: context.Background(), Model: "gpt-test", Prompt: "hello"}, controlHandler), client)
	if err != nil {
		t.Fatalf("StreamV2 returned error: %v", err)
	}

	client.sendText(`{"@action":"answer","data":"hel`)

	action := sink.recv(t, 150*time.Millisecond)
	if action.Action != "answer" {
		t.Fatalf("expected action=answer, got %q", action.Action)
	}
	assertReadWithin(t, action.Data, `"hel`)

	client.sendText(`lo"}`)
	client.finish()

	assertReadAllEquals(t, action.Data, `"hel`, "", `"hello"`)

	if err := output.Wait(); err != nil {
		t.Fatalf("Wait returned error: %v", err)
	}
}

func TestStreamV2_StreamsActionDataIncrementallyAcrossTwoActions(t *testing.T) {
	client := newScriptedStreamChatClient()
	sink, controlHandler := newCompatibleControlSink()
	output, err := StreamV2(withCompatibleControl(StreamInput{Context: context.Background(), Model: "gpt-test", Prompt: "hello"}, controlHandler), client)
	if err != nil {
		t.Fatalf("StreamV2 returned error: %v", err)
	}

	client.sendText(`{"@action":"answer","data":{"text":"he`)

	first := sink.recv(t, streamingReadTimeout)
	if first == nil || first.Action != "answer" {
		t.Fatalf("unexpected first action: %#v", first)
	}
	firstCh := make(chan []byte, 1)
	firstErrCh := make(chan error, 1)
	go func() {
		buf := make([]byte, 32)
		n, err := first.Data.Read(buf)
		firstCh <- append([]byte(nil), buf[:n]...)
		firstErrCh <- err
	}()
	var firstPrefix string
	select {
	case got := <-firstCh:
		if len(got) == 0 || !strings.HasPrefix(`{"text":"he`, string(got)) {
			t.Fatalf("expected first action first chunk to be a prefix of %q, got %q", `{"text":"he`, string(got))
		}
		firstPrefix = string(got)
	case <-time.After(streamingReadTimeout):
		t.Fatal("expected first action first chunk within streaming timeout")
	}
	if err := <-firstErrCh; err != nil {
		t.Fatalf("unexpected first action read err: %v", err)
	}

	client.sendText(`llo"}}`)
	assertReadAllEquals(t, first.Data, firstPrefix, "", `{"text":"hello"}`)

	client.sendText(`{"@action":"call_tool","data":{"name":"bash","params":{"command":"echo `)

	var second *CallToolRequest
	select {
	case second = <-output.ToolCalls:
	case <-time.After(streamingReadTimeout):
		t.Fatal("expected second tool call within streaming timeout")
	}
	if second == nil || second.Name != "call_tool" {
		t.Fatalf("unexpected second tool call: %#v", second)
	}
	dataCh := make(chan []byte, 1)
	errCh := make(chan error, 1)
	go func() {
		buf := make([]byte, 64)
		n, err := second.Arguments.Read(buf)
		dataCh <- append([]byte(nil), buf[:n]...)
		errCh <- err
	}()
	var secondPrefix string
	select {
	case got := <-dataCh:
		if len(got) == 0 || !strings.HasPrefix(`{"name":"bash","params":{"command":"echo `, string(got)) {
			t.Fatalf("expected second tool args first chunk to be a prefix of %q, got %q", `{"name":"bash","params":{"command":"echo `, string(got))
		}
		secondPrefix = string(got)
	case <-time.After(streamingReadTimeout):
		t.Fatal("expected second tool args first chunk within streaming timeout")
	}
	if err := <-errCh; err != nil {
		t.Fatalf("unexpected second tool args read err: %v", err)
	}

	client.sendText(`hi"}}`)
	client.finish()
	assertReadAllEquals(t, second.Arguments, secondPrefix, "", `{"name":"bash","params":{"command":"echo hi"}}`)

	if err := output.Wait(); err != nil {
		t.Fatalf("Wait returned error: %v", err)
	}
}

func TestParseActionStream_singleObjectReadAllWithoutSecondWrite(t *testing.T) {
	r, w := jsonextractor.NewPipe()
	actions := make(chan *ActionOutput, 64)
	setParseErr := func(e error) {
		if e != nil {
			t.Logf("parse err: %v", e)
		}
	}
	parserDone := make(chan struct{})
	go func() {
		defer close(parserDone)
		actioncompatible.ParseActionStream(r, actions, setParseErr)
	}()

	if _, err := w.Write([]byte(`{"@action":"message","data":"one"}`)); err != nil {
		t.Fatalf("write: %v", err)
	}
	first, ok := <-actions
	if !ok || first == nil {
		t.Fatal("expected first action")
	}
	if _, err := io.ReadAll(first.Data); err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	<-parserDone
}

func TestStreamV2BuildsSystemMessageFromStreamInput(t *testing.T) {
	client := newScriptedStreamChatClient()
	close(client.stream)

	output, err := StreamV2(StreamInput{
		Context:      context.Background(),
		Model:        "gpt-test",
		Prompt:       "<developer_prompt>dev</developer_prompt>",
		SystemPrompt: "<system_prompt>sys</system_prompt>",
	}, client)
	if err != nil {
		t.Fatalf("StreamV2 returned error: %v", err)
	}
	if err := output.Wait(); err != nil {
		t.Fatalf("Wait returned error: %v", err)
	}
	if len(client.req.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(client.req.Messages))
	}
	if client.req.Messages[0].Role != "system" {
		t.Fatalf("expected first message role=system, got %q", client.req.Messages[0].Role)
	}
	if content, _ := client.req.Messages[0].Content.(string); content != "<system_prompt>sys</system_prompt>" {
		t.Fatalf("unexpected system content: %q", content)
	}
	if client.req.Messages[1].Role != "user" {
		t.Fatalf("expected second message role=user, got %q", client.req.Messages[1].Role)
	}
}

func TestStreamV2BuildsInstructionExtraOptionFromStreamInput(t *testing.T) {
	client := newScriptedStreamChatClient()
	close(client.stream)

	output, err := StreamV2(StreamInput{
		Context:     context.Background(),
		Model:       "gpt-test",
		Prompt:      "<developer_prompt>dev</developer_prompt>",
		Instruction: "<system_prompt>sys</system_prompt>",
	}, client)
	if err != nil {
		t.Fatalf("StreamV2 returned error: %v", err)
	}
	if err := output.Wait(); err != nil {
		t.Fatalf("Wait returned error: %v", err)
	}
	if len(client.req.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(client.req.Messages))
	}
	if client.req.Messages[0].Role != "user" {
		t.Fatalf("expected user role, got %q", client.req.Messages[0].Role)
	}
	if client.req.ExtraOptions == nil {
		t.Fatal("expected extra options")
	}
	if got, _ := client.req.ExtraOptions["instructions"].(string); got != "<system_prompt>sys</system_prompt>" {
		t.Fatalf("unexpected instructions value: %q", got)
	}
}

func TestStreamV2IncludesHistoryMessagesFromStreamInput(t *testing.T) {
	client := newScriptedStreamChatClient()
	close(client.stream)

	output, err := StreamV2(StreamInput{
		Context: context.Background(),
		Model:   "gpt-test",
		Prompt:  "<developer_prompt>dev</developer_prompt>",
		HistoryMessages: []*ModelMessage{
			{Role: "user", Content: "历史用户消息"},
			{Role: "assistant", Content: "历史助手消息"},
		},
	}, client)
	if err != nil {
		t.Fatalf("StreamV2 returned error: %v", err)
	}
	if err := output.Wait(); err != nil {
		t.Fatalf("Wait returned error: %v", err)
	}
	if len(client.req.Messages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(client.req.Messages))
	}
	if content, _ := client.req.Messages[0].Content.(string); content != "历史用户消息" {
		t.Fatalf("unexpected first history content: %q", content)
	}
	if content, _ := client.req.Messages[1].Content.(string); content != "历史助手消息" {
		t.Fatalf("unexpected second history content: %q", content)
	}
	if content, _ := client.req.Messages[2].Content.(string); content != "<developer_prompt>dev</developer_prompt>" {
		t.Fatalf("unexpected final prompt content: %q", content)
	}
}

func TestParseActionStream_sequentialObjectsSplitWrites(t *testing.T) {
	r, w := jsonextractor.NewPipe()
	actions := make(chan *ActionOutput, 64)
	var parseErr error
	var parseErrMu sync.Mutex
	setParseErr := func(e error) {
		if e == nil {
			return
		}
		parseErrMu.Lock()
		defer parseErrMu.Unlock()
		if parseErr == nil {
			parseErr = e
		}
	}
	parserDone := make(chan struct{})
	go func() {
		defer close(parserDone)
		actioncompatible.ParseActionStream(r, actions, setParseErr)
	}()

	go func() {
		if _, err := w.Write([]byte(`{"@action":"message","data":"one"}`)); err != nil {
			return
		}
		time.Sleep(50 * time.Millisecond)
		if _, err := w.Write([]byte(`{"@action":"answer","data":"two"}`)); err != nil {
			return
		}
		_ = w.Close()
	}()

	first, ok := <-actions
	if !ok || first == nil {
		t.Fatal("expected first action")
	}
	if _, err := io.ReadAll(first.Data); err != nil {
		t.Fatalf("ReadAll first: %v", err)
	}
	second, ok := <-actions
	if !ok || second == nil {
		t.Fatal("expected second action")
	}
	if _, err := io.ReadAll(second.Data); err != nil {
		t.Fatalf("ReadAll second: %v", err)
	}
	<-parserDone
	parseErrMu.Lock()
	err := parseErr
	parseErrMu.Unlock()
	if err != nil {
		t.Fatalf("parse err: %v", err)
	}
}

func TestStreamV2_TwoActionObjectsInSingleDelta(t *testing.T) {
	client := newScriptedStreamChatClient()
	sink, controlHandler := newCompatibleControlSink()
	output, err := StreamV2(withCompatibleControl(StreamInput{Context: context.Background(), Model: "gpt-test", Prompt: "hello"}, controlHandler), client)
	if err != nil {
		t.Fatalf("StreamV2 returned error: %v", err)
	}
	client.sendText(`{"@action":"message","data":"one"}{"@action":"answer","data":"two"}`)
	client.finish()

	first := sink.recv(t, 500*time.Millisecond)
	second := sink.recv(t, 500*time.Millisecond)
	if first.Action != "message" || second.Action != "answer" {
		t.Fatalf("unexpected actions: %q, %q", first.Action, second.Action)
	}
	if b, err := io.ReadAll(first.Data); err != nil || string(b) != `"one"` {
		t.Fatalf("first data: %q err=%v", string(b), err)
	}
	if b, err := io.ReadAll(second.Data); err != nil || string(b) != `"two"` {
		t.Fatalf("second data: %q err=%v", string(b), err)
	}
	if err := output.Wait(); err != nil {
		t.Fatalf("Wait: %v", err)
	}
}

func TestStreamV2_StreamsMultipleActionsSequentially(t *testing.T) {
	client := newScriptedStreamChatClient()
	sink, controlHandler := newCompatibleControlSink()
	output, err := StreamV2(withCompatibleControl(StreamInput{Context: context.Background(), Model: "gpt-test", Prompt: "hello"}, controlHandler), client)
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

type chatOnlyInvalidEnvelopeClient struct{}

func (chatOnlyInvalidEnvelopeClient) Chat(req ChatRequest) (ChatResponse, error) {
	return ChatResponse{
		Message: ModelMessage{
			Role:    "assistant",
			Content: `}{"@action":"answer","data":"oops"}`,
		},
		Usage: &Usage{InputTokens: 1, OutputTokens: 1},
	}, nil
}

func TestStreamV2_NonStreamParseFailureReturnsOutputWithRawText(t *testing.T) {
	client := chatOnlyInvalidEnvelopeClient{}
	output, err := StreamV2(StreamInput{Context: context.Background(), Model: "gpt-test", Prompt: "hello"}, client)
	if err != nil {
		t.Fatalf("StreamV2 returned error: %v", err)
	}
	if output == nil || output.RawTextReader == nil {
		t.Fatal("expected non-nil output with RawTextReader")
	}
	rawPreview, err := io.ReadAll(output.RawTextReader)
	if err != nil {
		t.Fatalf("ReadAll RawTextReader: %v", err)
	}
	if !strings.Contains(string(rawPreview), `"@action":"answer"`) {
		t.Fatalf("expected model text in RawTextReader, got %q", string(rawPreview))
	}
	if _, ok := <-output.ToolCalls; ok {
		t.Fatal("expected no actions on parse failure")
	}
	waitErr := output.Wait()
	if waitErr == nil {
		t.Fatal("expected Wait parse error")
	}
	if !strings.Contains(waitErr.Error(), "parse JSON stream") {
		t.Fatalf("unexpected wait error: %v", waitErr)
	}
}

func TestParseActionStream_doesNotSplitOnBraceInsideString(t *testing.T) {
	// Regression: top-level splitter must not treat `}` inside a JSON string (e.g. patch text) as envelope end.
	env1 := `{"@action":"message","data":{"message":"hi","next_step":"x"}}`
	env2 := `{"@action":"call_tool","data":{"tool_calls":[{"tool_name":"patch","tool_input":{"patch":"line\n}\nmore \"quotes\""}}]}}`
	payload := env1 + env2

	pr, pw := jsonextractor.NewPipe()
	go func() {
		defer pw.Close()
		const step = 19
		for i := 0; i < len(payload); i += step {
			j := i + step
			if j > len(payload) {
				j = len(payload)
			}
			if _, err := pw.Write([]byte(payload[i:j])); err != nil {
				return
			}
		}
	}()

	var parseErr error
	setErr := func(e error) {
		if e != nil && parseErr == nil {
			parseErr = e
		}
	}
	done := make(chan struct{})
	actions := make(chan *ActionOutput, 8)
	go func() {
		actioncompatible.ParseActionStream(pr, actions, setErr)
		close(done)
	}()

	got := make([]*ActionOutput, 0, 2)
	for len(got) < 2 {
		select {
		case a, ok := <-actions:
			if !ok {
				t.Fatalf("actions channel closed early; parseErr=%v got=%d", parseErr, len(got))
			}
			if a != nil {
				got = append(got, a)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("timeout; parseErr=%v got=%d", parseErr, len(got))
		}
	}
	<-done
	if parseErr != nil {
		t.Fatalf("compatible.ParseActionStream: %v", parseErr)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 actions, got %d", len(got))
	}
	if got[0].Action != "message" || got[1].Action != "call_tool" {
		t.Fatalf("got actions %q, %q", got[0].Action, got[1].Action)
	}
}

func TestStreamV2RetriesWhenMessageIsFollowedByIncompleteToolJSON(t *testing.T) {
	client := &messageThenIncompleteToolThenSuccessClient{}
	var retryAttempts []int
	sink, controlHandler := newCompatibleControlSink()
	output, err := StreamV2(withCompatibleControl(StreamInput{
		Context: context.Background(),
		Model:   "gpt-test",
		Prompt:  "hello",
		ProviderOptions: &models.LLMConfig{
			Name:       "openai",
			Model:      "gpt-test",
			MaxRetries: 1,
		},
		OnRetryError: func(err error, retryAttempt int, maxRetries int, nextDelay time.Duration, attemptDuration time.Duration, rawResponse string) {
			retryAttempts = append(retryAttempts, retryAttempt)
		},
	}, controlHandler), client)
	if err != nil {
		t.Fatalf("StreamV2 returned error: %v", err)
	}

	first := sink.recv(t, 10*time.Second)
	if first == nil || first.Action != "message" {
		t.Fatalf("expected first emitted action to be progress message, got %#v", first)
	}
	second := sink.recv(t, 10*time.Second)
	if second == nil || second.Action != "answer" {
		t.Fatalf("expected answer action after retry, got %#v", second)
	}
	if err := output.Wait(); err != nil {
		t.Fatalf("Wait returned error: %v attempts=%d retries=%v", err, client.attempts, retryAttempts)
	}
	if client.attempts != 2 {
		t.Fatalf("expected retry after incomplete tool JSON, got %d attempts", client.attempts)
	}
	if len(retryAttempts) != 1 || retryAttempts[0] != 1 {
		t.Fatalf("unexpected retry callbacks: %#v", retryAttempts)
	}
}
