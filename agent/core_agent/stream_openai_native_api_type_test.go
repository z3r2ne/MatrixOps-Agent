package coreagent

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"pkgs/db/models"
)

type nativeRouteRecordingRoundTripper struct {
	requestURL  string
	requestBody string
	payload     string
}

func (rt *nativeRouteRecordingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	rt.requestURL = req.URL.String()
	body, _ := io.ReadAll(req.Body)
	rt.requestBody = string(body)

	payload := rt.payload
	if payload == "" {
		payload = "event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"hello\",\"output_index\":0,\"item_id\":\"msg_1\"}\n\nevent: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{\"stop_reason\":\"stop\",\"usage\":{\"input_tokens\":1,\"output_tokens\":1,\"total_tokens\":2,\"input_tokens_details\":{\"cached_tokens\":0},\"output_tokens_details\":{\"reasoning_tokens\":0}}}}\n\n"
	}
	if strings.HasSuffix(req.URL.Path, "/chat/completions") {
		payload = "data: {\"id\":\"chatcmpl-1\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"gpt-test\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"hello\"},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":1,\"completion_tokens\":1,\"total_tokens\":2,\"prompt_tokens_details\":{\"cached_tokens\":0},\"completion_tokens_details\":{\"reasoning_tokens\":0}}}\n\ndata: [DONE]\n\n"
	}

	return &http.Response{
		StatusCode: 200,
		Header: http.Header{
			"Content-Type": []string{"text/event-stream"},
		},
		Body: io.NopCloser(strings.NewReader(payload)),
	}, nil
}

func TestStreamV2OpenAINativeUsesChatCompletionsWhenAPITypeIsChat(t *testing.T) {
	rt := &nativeRouteRecordingRoundTripper{}
	httpClient := &http.Client{Transport: rt}

	output, err := StreamV2OpenAINative(StreamInput{
		Context:         context.Background(),
		Model:           "gpt-test",
		Prompt:          "hello",
		MaxOutputTokens: 1234,
		HTTPClient:      httpClient,
		ProviderOptions: &models.LLMConfig{
			Name:       "doubao",
			Type:       "custom",
			BaseURL:    "https://example.com/v1",
			APIKey:     "test-key",
			APIType:    models.LLMAPITypeChat,
			MaxRetries: 1,
		},
	})
	if err != nil {
		t.Fatalf("StreamV2OpenAINative returned error: %v", err)
	}

	if err := output.Wait(); err != nil {
		t.Fatalf("Wait returned error: %v", err)
	}
	if output.NativeAssistantTextFinishesTurn != true {
		t.Fatal("expected native assistant text to finish turn")
	}
	contentBytes, err := io.ReadAll(output.ContentReader)
	if err != nil {
		t.Fatalf("ReadAll ContentReader: %v", err)
	}
	if strings.TrimSpace(string(contentBytes)) != "hello" {
		t.Fatalf("unexpected content reader payload: %q", string(contentBytes))
	}
	if rt.requestURL != "https://example.com/v1/chat/completions" {
		t.Fatalf("unexpected request URL: %s", rt.requestURL)
	}
	if !strings.Contains(rt.requestBody, "\"messages\"") {
		t.Fatalf("expected chat completions body, got %s", rt.requestBody)
	}
}

func TestStreamV2OpenAINativeUsesResponsesWhenAPITypeIsResponse(t *testing.T) {
	rt := &nativeRouteRecordingRoundTripper{}
	httpClient := &http.Client{Transport: rt}

	output, err := StreamV2OpenAINative(StreamInput{
		Context:         context.Background(),
		Model:           "gpt-test",
		Prompt:          "hello",
		MaxOutputTokens: 1234,
		HTTPClient:      httpClient,
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

	if err := output.Wait(); err != nil {
		t.Fatalf("Wait returned error: %v", err)
	}
	if output.NativeAssistantTextFinishesTurn != true {
		t.Fatal("expected native assistant text to finish turn")
	}
	contentBytes, err := io.ReadAll(output.ContentReader)
	if err != nil {
		t.Fatalf("ReadAll ContentReader: %v", err)
	}
	if strings.TrimSpace(string(contentBytes)) != "hello" {
		t.Fatalf("unexpected content reader payload: %q", string(contentBytes))
	}
	if rt.requestURL != "https://example.com/v1/responses" {
		t.Fatalf("unexpected request URL: %s", rt.requestURL)
	}
	if !strings.Contains(rt.requestBody, "\"input\"") {
		t.Fatalf("expected responses body, got %s", rt.requestBody)
	}
	if strings.Contains(rt.requestBody, "\"max_output_tokens\"") {
		t.Fatalf("expected responses body to omit max_output_tokens, got %s", rt.requestBody)
	}
	var body map[string]interface{}
	if err := json.Unmarshal([]byte(rt.requestBody), &body); err != nil {
		t.Fatalf("unmarshal request body: %v", err)
	}
	if _, ok := body["input"].([]interface{}); !ok {
		t.Fatalf("expected input to be an array, got %#v", body["input"])
	}
}

func TestStreamV2OpenAINativeEmitsCommentaryMessageBeforeToolCalls(t *testing.T) {
	rt := &nativeRouteRecordingRoundTripper{
		payload: strings.Join([]string{
			`event: response.output_text.delta`,
			`data: {"type":"response.output_text.delta","delta":"我先快速梳理了仓库结构。","output_index":0,"item_id":"msg_1"}`,
			``,
			`event: response.output_text.done`,
			`data: {"type":"response.output_text.done","text":"我先快速梳理了仓库结构。","output_index":0,"item_id":"msg_1"}`,
			``,
			`event: response.output_item.done`,
			`data: {"type":"response.output_item.done","item":{"id":"msg_1","type":"message","status":"completed","content":[{"type":"output_text","text":"我先快速梳理了仓库结构。"}],"phase":"commentary","role":"assistant"},"output_index":0}`,
			``,
			`event: response.output_item.added`,
			`data: {"type":"response.output_item.added","item":{"id":"fc_1","type":"function_call","status":"in_progress","arguments":"","call_id":"call_1","name":"read"},"output_index":1}`,
			``,
			`event: response.function_call_arguments.delta`,
			`data: {"type":"response.function_call_arguments.delta","delta":"{\"path\":\"README.md\"}","item_id":"fc_1","output_index":1}`,
			``,
			`event: response.function_call_arguments.done`,
			`data: {"type":"response.function_call_arguments.done","arguments":"{\"path\":\"README.md\"}","item_id":"fc_1","output_index":1}`,
			``,
			`event: response.completed`,
			`data: {"type":"response.completed","response":{"stop_reason":"stop","usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2,"input_tokens_details":{"cached_tokens":0},"output_tokens_details":{"reasoning_tokens":0}}}}`,
			``,
		}, "\n"),
	}
	httpClient := &http.Client{Transport: rt}

	output, err := StreamV2OpenAINative(StreamInput{
		Context:    context.Background(),
		Model:      "gpt-test",
		Prompt:     "hello",
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

	var actions []*CallToolRequest
	for req := range output.ToolCalls {
		if req != nil {
			actions = append(actions, req)
		}
	}
	if err := output.Wait(); err != nil {
		t.Fatalf("Wait returned error: %v", err)
	}
	if len(actions) != 1 {
		t.Fatalf("expected 1 tool action, got %d", len(actions))
	}
	if actions[0].Name != "read" {
		t.Fatalf("expected tool action to be read, got %q", actions[0].Name)
	}
	if strings.TrimSpace(output.Phase) != "commentary" {
		t.Fatalf("expected commentary phase on output, got %q", output.Phase)
	}
	contentBytes, err := io.ReadAll(output.ContentReader)
	if err != nil {
		t.Fatalf("read ContentReader: %v", err)
	}
	if strings.TrimSpace(string(contentBytes)) != "我先快速梳理了仓库结构。" {
		t.Fatalf("unexpected content reader payload: %q", string(contentBytes))
	}
}

func TestStreamV2OpenAINativeTreatsCommentaryWithoutToolsAsMessage(t *testing.T) {
	rt := &nativeRouteRecordingRoundTripper{
		payload: strings.Join([]string{
			`event: response.output_text.delta`,
			`data: {"type":"response.output_text.delta","delta":"我先说明一下思路。","output_index":0,"item_id":"msg_1"}`,
			``,
			`event: response.output_item.done`,
			`data: {"type":"response.output_item.done","item":{"id":"msg_1","type":"message","status":"completed","content":[{"type":"output_text","text":"我先说明一下思路。"}],"phase":"commentary","role":"assistant"},"output_index":0}`,
			``,
			`event: response.completed`,
			`data: {"type":"response.completed","response":{"stop_reason":"stop","usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2,"input_tokens_details":{"cached_tokens":0},"output_tokens_details":{"reasoning_tokens":0}}}}`,
			``,
		}, "\n"),
	}
	httpClient := &http.Client{Transport: rt}

	output, err := StreamV2OpenAINative(StreamInput{
		Context:    context.Background(),
		Model:      "gpt-test",
		Prompt:     "hello",
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

	var actions []*CallToolRequest
	for req := range output.ToolCalls {
		if req != nil {
			actions = append(actions, req)
		}
	}
	if err := output.Wait(); err != nil {
		t.Fatalf("Wait returned error: %v", err)
	}
	if len(actions) != 0 {
		t.Fatalf("expected 0 actions for commentary-only native text, got %d", len(actions))
	}
	if strings.TrimSpace(output.Phase) != "commentary" {
		t.Fatalf("expected commentary phase on output, got %q", output.Phase)
	}
}

func TestStreamV2OpenAINativeReplaysReasoningAndPhaseHistory(t *testing.T) {
	rt := &nativeRouteRecordingRoundTripper{}
	httpClient := &http.Client{Transport: rt}

	history := []*ModelMessage{
		{
			Role:                      "assistant",
			Content:                   "图片里写的是：你好！想让我在这个仓库里做什么？",
			ResponsesOutputMessageRaw: `{"type":"message","id":"msg_hist_1","role":"assistant","status":"completed","phase":"final_answer","content":[{"type":"output_text","text":"图片里写的是：你好！想让我在这个仓库里做什么？"}]}`,
			ResponsesReasoningItemRaws: []string{
				`{"type":"reasoning","id":"rs_hist_1","status":"completed","summary":[{"type":"summary_text","text":"先理解图片内容"}],"encrypted_content":"enc_hist_1"}`,
			},
		},
	}

	output, err := StreamV2OpenAINative(StreamInput{
		Context:                  context.Background(),
		Model:                    "gpt-test",
		Prompt:                   "继续",
		HTTPClient:               httpClient,
		HistoryMessages:          history,
		EnableEncryptedReasoning: true,
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

	for range output.ToolCalls {
	}
	if err := output.Wait(); err != nil {
		t.Fatalf("Wait returned error: %v", err)
	}
	if !strings.Contains(rt.requestBody, `"phase":"final_answer"`) {
		t.Fatalf("expected request body to replay assistant phase, got %s", rt.requestBody)
	}
	if !strings.Contains(rt.requestBody, `"encrypted_content":"enc_hist_1"`) {
		t.Fatalf("expected request body to replay encrypted reasoning, got %s", rt.requestBody)
	}
	if !strings.Contains(rt.requestBody, `"include":["reasoning.encrypted_content"]`) {
		t.Fatalf("expected request body to request encrypted reasoning, got %s", rt.requestBody)
	}
}

func TestStreamV2OpenAINativeAppliesReasoningOptions(t *testing.T) {
	rt := &nativeRouteRecordingRoundTripper{}
	httpClient := &http.Client{Transport: rt}

	output, err := StreamV2OpenAINative(StreamInput{
		Context:                  context.Background(),
		Model:                    "gpt-test",
		Prompt:                   "hello",
		HTTPClient:               httpClient,
		ReasoningEffort:          "xhigh",
		TextVerbosity:            "low",
		EnableEncryptedReasoning: true,
		ParallelToolCalls:        true,
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
	for range output.ToolCalls {
	}
	if err := output.Wait(); err != nil {
		t.Fatalf("Wait returned error: %v", err)
	}
	if !strings.Contains(rt.requestBody, `"reasoning":{"effort":"xhigh"`) {
		t.Fatalf("expected xhigh reasoning effort, got %s", rt.requestBody)
	}
	if !strings.Contains(rt.requestBody, `"text":{"verbosity":"low"}`) {
		t.Fatalf("expected text verbosity low, got %s", rt.requestBody)
	}
	if !strings.Contains(rt.requestBody, `"parallel_tool_calls":true`) {
		t.Fatalf("expected parallel_tool_calls=true, got %s", rt.requestBody)
	}
	if !strings.Contains(rt.requestBody, `"store":false`) {
		t.Fatalf("expected store=false, got %s", rt.requestBody)
	}
	if !strings.Contains(rt.requestBody, `"include":["reasoning.encrypted_content"]`) {
		t.Fatalf("expected include encrypted reasoning, got %s", rt.requestBody)
	}
}

func TestStreamV2OpenAINativeOmitsEncryptedReasoningIncludeWhenDisabled(t *testing.T) {
	rt := &nativeRouteRecordingRoundTripper{}
	httpClient := &http.Client{Transport: rt}

	output, err := StreamV2OpenAINative(StreamInput{
		Context:                  context.Background(),
		Model:                    "gpt-test",
		Prompt:                   "hello",
		HTTPClient:               httpClient,
		EnableEncryptedReasoning: false,
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
	for range output.ToolCalls {
	}
	if err := output.Wait(); err != nil {
		t.Fatalf("Wait returned error: %v", err)
	}
	if strings.Contains(rt.requestBody, "reasoning.encrypted_content") {
		t.Fatalf("did not expect encrypted reasoning include when disabled, got %s", rt.requestBody)
	}
	if strings.Contains(rt.requestBody, `"store":false`) {
		t.Fatalf("did not expect store=false when encrypted reasoning disabled, got %s", rt.requestBody)
	}
}

func TestStreamV2OpenAINativeChatRequestIncludesReasoningContentFromHistory(t *testing.T) {
	rt := &nativeRouteRecordingRoundTripper{}
	httpClient := &http.Client{Transport: rt}

	history := []*ModelMessage{{
		Role:             "assistant",
		Content:          "visible answer",
		ReasoningContent: "internal chain-of-thought",
	}}

	output, err := StreamV2OpenAINative(StreamInput{
		Context:         context.Background(),
		Model:           "gpt-test",
		Prompt:          "follow-up",
		HTTPClient:      httpClient,
		HistoryMessages: history,
		ProviderOptions: &models.LLMConfig{
			Name:       "doubao",
			Type:       "custom",
			BaseURL:    "https://example.com/v1",
			APIKey:     "test-key",
			APIType:    models.LLMAPITypeChat,
			MaxRetries: 1,
		},
	})
	if err != nil {
		t.Fatalf("StreamV2OpenAINative returned error: %v", err)
	}
	for range output.ToolCalls {
	}
	if err := output.Wait(); err != nil {
		t.Fatalf("Wait returned error: %v", err)
	}
	if !strings.Contains(rt.requestBody, `"reasoning_content"`) {
		t.Fatalf("expected chat request messages to include reasoning_content, got %s", rt.requestBody)
	}
	if !strings.Contains(rt.requestBody, "internal chain-of-thought") {
		t.Fatalf("expected reasoning_content value in request, got %s", rt.requestBody)
	}
}

func TestStreamV2OpenAINativeChatSingleAssistantCombinesContentReasoningAndToolCalls(t *testing.T) {
	rt := &nativeRouteRecordingRoundTripper{}
	httpClient := &http.Client{Transport: rt}

	history := []*ModelMessage{{
		Role:             "assistant",
		Content:          "visible assistant text",
		ReasoningContent: "model reasoning text",
		ToolCalls: []ToolCall{
			{ID: "call-1", Name: "read", Arguments: map[string]interface{}{"path": "a.md"}},
			{ID: "call-2", Name: "list", Arguments: map[string]interface{}{"path": "."}},
		},
	}}

	output, err := StreamV2OpenAINative(StreamInput{
		Context:         context.Background(),
		Model:           "gpt-test",
		Prompt:          "next",
		HTTPClient:      httpClient,
		HistoryMessages: history,
		ProviderOptions: &models.LLMConfig{
			Name:       "doubao",
			Type:       "custom",
			BaseURL:    "https://example.com/v1",
			APIKey:     "test-key",
			APIType:    models.LLMAPITypeChat,
			MaxRetries: 1,
		},
	})
	if err != nil {
		t.Fatalf("StreamV2OpenAINative returned error: %v", err)
	}
	for range output.ToolCalls {
	}
	if err := output.Wait(); err != nil {
		t.Fatalf("Wait returned error: %v", err)
	}
	var body struct {
		Messages []json.RawMessage `json:"messages"`
	}
	if err := json.Unmarshal([]byte(rt.requestBody), &body); err != nil {
		t.Fatalf("unmarshal request: %v", err)
	}
	asstWithTools := 0
	for _, raw := range body.Messages {
		var probe struct {
			Role              string          `json:"role"`
			Content           string          `json:"content"`
			ReasoningContent  string          `json:"reasoning_content"`
			ToolCalls         json.RawMessage `json:"tool_calls"`
		}
		if err := json.Unmarshal(raw, &probe); err != nil {
			continue
		}
		if probe.Role != "assistant" || len(probe.ToolCalls) == 0 {
			continue
		}
		asstWithTools++
		if probe.Content != "visible assistant text" {
			t.Fatalf("expected combined content on assistant with tools, got %q", probe.Content)
		}
		if probe.ReasoningContent != "model reasoning text" {
			t.Fatalf("expected combined reasoning_content, got %q", probe.ReasoningContent)
		}
		var calls []interface{}
		if err := json.Unmarshal(probe.ToolCalls, &calls); err != nil || len(calls) != 2 {
			t.Fatalf("expected 2 tool_calls on same assistant message, got %v err=%v", probe.ToolCalls, err)
		}
	}
	if asstWithTools != 1 {
		t.Fatalf("expected exactly one assistant message with tool_calls, got %d (body=%s)", asstWithTools, rt.requestBody)
	}
}

func TestStreamV2OpenAINativeResponsesRequestMergesReasoningContentIntoOutputMessage(t *testing.T) {
	rt := &nativeRouteRecordingRoundTripper{}
	httpClient := &http.Client{Transport: rt}

	history := []*ModelMessage{{
		Role:                      "assistant",
		ResponsesOutputMessageRaw: `{"type":"message","id":"msg_hist_rc","role":"assistant","status":"completed","content":[{"type":"output_text","text":"hello from history"}]}`,
		ReasoningContent:          "merged reasoning text",
	}}

	output, err := StreamV2OpenAINative(StreamInput{
		Context:         context.Background(),
		Model:           "gpt-test",
		Prompt:          "next",
		HTTPClient:      httpClient,
		HistoryMessages: history,
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
	for range output.ToolCalls {
	}
	if err := output.Wait(); err != nil {
		t.Fatalf("Wait returned error: %v", err)
	}
	if !strings.Contains(rt.requestBody, `"reasoning_content"`) {
		t.Fatalf("expected responses input to include reasoning_content, got %s", rt.requestBody)
	}
	if !strings.Contains(rt.requestBody, "merged reasoning text") {
		t.Fatalf("expected merged reasoning_content in request, got %s", rt.requestBody)
	}
}

type chatReasoningStreamRoundTripper struct{}

func (chatReasoningStreamRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	payload := strings.Join([]string{
		`data: {"id":"chatcmpl-rc","object":"chat.completion.chunk","created":1,"model":"gpt-test","choices":[{"index":0,"delta":{"reasoning_content":"think"}}]}`,
		``,
		`data: {"id":"chatcmpl-rc","object":"chat.completion.chunk","created":1,"model":"gpt-test","choices":[{"index":0,"delta":{"content":"hi"}}]}`,
		``,
		`data: {"id":"chatcmpl-rc","object":"chat.completion.chunk","created":1,"model":"gpt-test","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2,"prompt_tokens_details":{"cached_tokens":0},"completion_tokens_details":{"reasoning_tokens":0}}}`,
		``,
		`data: [DONE]`,
		``,
	}, "\n")
	return &http.Response{
		StatusCode: 200,
		Header: http.Header{
			"Content-Type": []string{"text/event-stream"},
		},
		Body: io.NopCloser(strings.NewReader(payload)),
	}, nil
}

func TestStreamV2OpenAINativeChatStreamAccumulatesReasoningContent(t *testing.T) {
	httpClient := &http.Client{Transport: chatReasoningStreamRoundTripper{}}

	output, err := StreamV2OpenAINative(StreamInput{
		Context:    context.Background(),
		Model:      "gpt-test",
		Prompt:     "hello",
		HTTPClient: httpClient,
		ProviderOptions: &models.LLMConfig{
			Name:       "doubao",
			Type:       "custom",
			BaseURL:    "https://example.com/v1",
			APIKey:     "test-key",
			APIType:    models.LLMAPITypeChat,
			MaxRetries: 1,
		},
	})
	if err != nil {
		t.Fatalf("StreamV2OpenAINative returned error: %v", err)
	}

	// StreamWithRetries forwards ReasonReader through io.Pipe; a consumer must read
	// concurrently before Wait, otherwise io.Copy in the retry wrapper deadlocks.
	reasonCh := make(chan []byte, 1)
	reasonErrCh := make(chan error, 1)
	go func() {
		b, err := io.ReadAll(output.ReasonReader)
		reasonCh <- b
		reasonErrCh <- err
	}()

	for range output.ToolCalls {
	}
	if err := output.Wait(); err != nil {
		t.Fatalf("Wait returned error: %v", err)
	}
	contentBytes, err := io.ReadAll(output.ContentReader)
	if err != nil {
		t.Fatalf("read ContentReader: %v", err)
	}
	if strings.TrimSpace(string(contentBytes)) != "hi" {
		t.Fatalf("expected streamed content hi, got %q", string(contentBytes))
	}
	if err := <-reasonErrCh; err != nil {
		t.Fatalf("read ReasonReader: %v", err)
	}
	reasonBytes := <-reasonCh
	if strings.TrimSpace(string(reasonBytes)) != "think" {
		t.Fatalf("expected streamed reasoning think, got %q", string(reasonBytes))
	}
}
