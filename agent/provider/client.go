package provider

import (
	"matrixops-agent/llm"
	"matrixops-agent/types"
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"pkgs/db/models"
	"pkgs/httpclient"
	"pkgs/llmheaders"
)

type GenericClient struct {
	Timeout    time.Duration
	HTTPClient *http.Client
}

func NewGenericClient() *GenericClient {
	return &GenericClient{
		Timeout: 300 * time.Second,
	}
}

func (c *GenericClient) GetModel(providerID string, modelID string) (Model, error) {
	return Model{
		ID:         modelID,
		ProviderID: providerID,
	}, nil
}

func (c *GenericClient) GetLanguage(model Model) (LanguageModel, error) {
	return model, nil
}

func (c *GenericClient) DefaultModel() (types.ModelRef, error) {
	return ParseModel("openai/gpt-4o-mini"), nil
}

func (c *GenericClient) Chat(request llm.ChatRequest) (llm.ChatResponse, error) {
	providerOptions := request.ProviderOptions

	helper := getHelper(providerOptions, request.Model)
	commonReq := toCommonRequest(request)
	commonReq.Stream = false

	var body map[string]interface{}
	switch helper.GetFormat() {
	case "openai":
		body = ToOpenAIRequest(commonReq)
	case "anthropic":
		body = ToAnthropicRequest(commonReq)
	case "oa-compat":
		body = ToOaCompatibleRequest(commonReq)
	case "google":
		// Google logic is missing in provided TS reference for request conversion.
		// Assuming OpenAI compatible or manual construction for now, or skipping.
		return llm.ChatResponse{}, errors.New("google provider request conversion not implemented")
	default:
		body = ToOaCompatibleRequest(commonReq)
	}

	// Apply provider options
	if helper.GetFormat() == "anthropic" {
		// Example: ensure max_tokens is set
		if _, ok := body["max_tokens"]; !ok {
			body["max_tokens"] = models.DefaultLLMMaxOutputTokens
		}
	}

	// Prepare Request
	providerID := providerOptions.Name
	if providerID == "" {
		providerID = "openai"
	}

	apiKeyStr, baseURLStr, _ := resolveOptions(providerOptions)

	body = helper.ModifyBody(body)
	urlStr := helper.ModifyURL(baseURLStr, false)

	reqBytes, _ := json.Marshal(body)
	ctx := request.Context
	if ctx == nil {
		ctx = context.Background()
	}
	req, err := http.NewRequestWithContext(ctx, "POST", urlStr, bytes.NewReader(reqBytes))
	if err != nil {
		return llm.ChatResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	helper.ModifyHeaders(req.Header, body, apiKeyStr)
	llmheaders.Apply(req.Header)

	client := c.resolveHTTPClientForProvider(nil, providerOptions)
	resp, err := client.Do(req)
	if err != nil {
		return llm.ChatResponse{}, formatTimeoutError(err, client.Timeout)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return llm.ChatResponse{}, apiErrorFromHTTPResponse(resp, bodyBytes, providerOptions.Name)
	}

	respBodyBytes, _ := io.ReadAll(resp.Body)
	var respJson map[string]interface{}
	if err := json.Unmarshal(respBodyBytes, &respJson); err != nil {
		return llm.ChatResponse{}, err
	}

	// Response Conversion
	var commonResp CommonResponse
	switch helper.GetFormat() {
	case "openai":
		commonResp = FromOpenAIResponse(respJson)
	case "anthropic":
		commonResp = FromAnthropicResponse(respJson)
	case "oa-compat":
		commonResp = FromOaCompatibleResponse(respJson)
	}

	return fromCommonResponse(commonResp), nil
}

func (c *GenericClient) StreamChat(request llm.ChatRequest) (<-chan llm.StreamEvent, error) {
	return c.StreamChatWithOptions(request)
}

func (c *GenericClient) StreamChatWithOptions(request llm.ChatRequest, opts ...llm.StreamChatOption) (<-chan llm.StreamEvent, error) {
	options := llm.NewStreamChatOptions(opts...)
	if options.OnRequest != nil {
		if err := options.OnRequest(&request); err != nil {
			return nil, err
		}
	}

	providerOptions := request.ProviderOptions

	helper := getHelper(providerOptions, request.Model)
	commonReq := toCommonRequest(request)
	commonReq.Stream = true

	var body map[string]interface{}
	switch helper.GetFormat() {
	case "openai":
		body = ToOpenAIRequest(commonReq)
	case "anthropic":
		body = ToAnthropicRequest(commonReq)
	case "oa-compat":
		body = ToOaCompatibleRequest(commonReq)
	default:
		body = ToOaCompatibleRequest(commonReq)
	}

	if helper.GetFormat() == "anthropic" {
		if _, ok := body["max_tokens"]; !ok {
			body["max_tokens"] = models.DefaultLLMMaxOutputTokens
		}
	}

	apiKeyStr, baseURLStr, _ := resolveOptions(providerOptions)
	body = helper.ModifyBody(body)
	urlStr := helper.ModifyURL(baseURLStr, true)

	reqBytes, _ := json.Marshal(body)
	if options.OnRawRequest != nil {
		options.OnRawRequest(string(reqBytes))
	}

	ctx := request.Context
	if ctx == nil {
		ctx = context.Background()
	}
	client := c.resolveHTTPClientForProvider(options.HTTPClient, providerOptions)
	providerID := providerOptions.Name
	if providerID == "" {
		providerID = "openai"
	}

	req, err := http.NewRequestWithContext(ctx, "POST", urlStr, bytes.NewReader(reqBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	helper.ModifyHeaders(req.Header, body, apiKeyStr)
	llmheaders.Apply(req.Header)

	resp, err := client.Do(req)
	if err != nil {
		return nil, formatTimeoutError(err, client.Timeout)
	}

	if resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		attemptRawResponse := string(bodyBytes)
		if options.OnRawResponse != nil {
			options.OnRawResponse(attemptRawResponse)
		}
		return nil, apiErrorFromHTTPResponse(resp, bodyBytes, providerID)
	}
	if contentType := strings.TrimSpace(resp.Header.Get("Content-Type")); contentType != "" && !isEventStreamContentType(contentType) {
		bodyBytes, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		attemptRawResponse := string(bodyBytes)
		if options.OnRawResponse != nil {
			options.OnRawResponse(attemptRawResponse)
		}
		return nil, newRetryableStreamResponseError(resp, providerID, fmt.Sprintf("unexpected streaming content-type %q", contentType), attemptRawResponse)
	}

	out := make(chan llm.StreamEvent)

	go func() {
		defer close(out)
		defer resp.Body.Close()
		var rawResponseBuilder strings.Builder
		var (
			rawBlockCount    int
			parsedChunkCount int
			textDeltaCount   int
			toolDeltaCount   int
		)

		done := make(chan struct{})
		defer close(done)
		if ctx != nil {
			go func() {
				select {
				case <-ctx.Done():
					_ = resp.Body.Close()
				case <-done:
				}
			}()
		}

		reader := bufio.NewReader(resp.Body)
		usageParser := helper.CreateUsageParser()
		finishSent := false
		lastFinishReason := ""
		sawChunk := false

		for {
			select {
			case <-ctx.Done():
				_ = sendStreamEvent(ctx, out, llm.StreamEvent{Type: "error", Error: ctx.Err()})
				return
			default:
			}

			block, err := readSSEEvent(reader)
			if err != nil {
				if err != io.EOF {
					_ = sendStreamEvent(ctx, out, llm.StreamEvent{Type: "error", Error: formatTimeoutError(err, client.Timeout)})
				}
				break
			}
			if block == "" {
				continue
			}
			rawBlockCount++
			rawResponseBuilder.WriteString(block)
			rawResponseBuilder.WriteString("\n\n")

			event, err := parseSSEBlock(block)
			if err != nil {
				continue
			}

			if isDoneEvent(event) {
				if !finishSent {
					finishReason := lastFinishReason
					if finishReason == "" {
						finishReason = "stop"
					}
					if !emitFinishEvent(ctx, out, helper, usageParser, finishReason) {
						return
					}
				}
				return
			}

			usageParser.Parse(block)

			if helper.GetFormat() == "openai" {
				if streamErr := openAIStreamErrorFromEvent(event); streamErr != nil {
					if options.OnRawResponse != nil {
						options.OnRawResponse(rawResponseBuilder.String())
					}
					_ = sendStreamEvent(ctx, out, llm.StreamEvent{Type: "error", Error: streamErr})
					return
				}
			}

			if helper.GetFormat() == "openai" && event.Type == "response.completed" {
				finishReason := openAIResponseCompletedFinishReasonFromEvent(event)
				if finishReason == "" {
					finishReason = "stop"
				}
				lastFinishReason = finishReason

				if !emitFinishEvent(ctx, out, helper, usageParser, finishReason) {
					return
				}
				if options.OnRawResponse != nil {
					options.OnRawResponse(rawResponseBuilder.String())
				}
				finishSent = true
				continue
			}

			// Handle chunk conversion
			var commonChunk CommonChunk
			var convErr error

			switch helper.GetFormat() {
			case "openai":
				commonChunk, convErr = FromOpenAIChunk(block)
			case "anthropic":
				commonChunk, convErr = FromAnthropicChunk(block)
			case "oa-compat":
				commonChunk, convErr = FromOaCompatibleChunk(block)
			}

			if convErr == nil {
				if len(commonChunk.Choices) > 0 {
					sawChunk = true
					parsedChunkCount++
				}
				for _, choice := range commonChunk.Choices {
					if choice.Delta.Content != "" {
						textDeltaCount++
						if !sendStreamEvent(ctx, out, llm.StreamEvent{Type: string(llm.GeneratorMessageTypeTextDelta), Text: choice.Delta.Content}) {
							return
						}
					}
					for _, tc := range choice.Delta.ToolCalls {
						toolDeltaCount++
						args := ""
						name := ""
						if tc.Function != nil {
							args = tc.Function.Arguments
							name = tc.Function.Name
						}
						if !sendStreamEvent(ctx, out, llm.StreamEvent{
							Type:          string(llm.GeneratorMessageTypeToolDelta),
							ToolIndex:     tc.Index,
							ToolCallID:    tc.ID,
							ToolName:      name,
							ToolArguments: args,
						}) {
							return
						}
					}
					if choice.Delta.ReasoningContent != "" {
						if !sendStreamEvent(ctx, out, llm.StreamEvent{Type: string(llm.GeneratorMessageTypeReasoningDelta), Text: choice.Delta.ReasoningContent}) {
							return
						}
					}
					if choice.FinishReason != "" {
						lastFinishReason = choice.FinishReason
						if !emitFinishEvent(ctx, out, helper, usageParser, choice.FinishReason) {
							return
						}
						if options.OnRawResponse != nil {
							options.OnRawResponse(rawResponseBuilder.String())
						}
						finishSent = true
					}
				}
			}
		}

		if !finishSent && sawChunk {
			u := usageParser.Retrieve()
			uInfo := helper.NormalizeUsage(u)
			finishReason := lastFinishReason
			if finishReason == "" {
				finishReason = "stop"
			}
			_ = sendStreamEvent(ctx, out, llm.StreamEvent{
				Type:   string(llm.GeneratorMessageTypeFinish),
				Finish: finishReason,
				Usage: &llm.Usage{
					InputTokens:       uInfo.InputTokens,
					OutputTokens:      uInfo.OutputTokens,
					ReasoningTokens:   uInfo.ReasoningTokens,
					CachedInputTokens: uInfo.CacheReadTokens,
				},
			})
		}
		rawResponse := rawResponseBuilder.String()
		if options.OnRawResponse != nil {
			options.OnRawResponse(rawResponse)
		}
		if rawBlockCount == 0 {
			err := newRetryableStreamResponseError(resp, providerID, "stream ended without SSE blocks", rawResponse)
			log.Printf("[provider.stream] stream ended without SSE blocks provider=%s model=%s finishSent=%t sawChunk=%t", providerID, request.Model, finishSent, sawChunk)
			if options.OnRawResponse != nil {
				options.OnRawResponse(rawResponse)
			}
			_ = sendStreamEvent(ctx, out, llm.StreamEvent{Type: "error", Error: err})
			return
		}
		if !sawChunk && !finishSent {
			err := newRetryableStreamResponseError(resp, providerID, "stream ended without parsed chunks", rawResponse)
			log.Printf("[provider.stream] stream ended without parsed chunks provider=%s model=%s rawBlocks=%d rawBytes=%d", providerID, request.Model, rawBlockCount, rawResponseBuilder.Len())
			if options.OnRawResponse != nil {
				options.OnRawResponse(rawResponse)
			}
			_ = sendStreamEvent(ctx, out, llm.StreamEvent{Type: "error", Error: err})
			return
		}
		log.Printf("[provider.stream] stream summary provider=%s model=%s rawBlocks=%d parsedChunks=%d textDeltas=%d toolDeltas=%d finishSent=%t rawBytes=%d", providerID, request.Model, rawBlockCount, parsedChunkCount, textDeltaCount, toolDeltaCount, finishSent, rawResponseBuilder.Len())
	}()

	return out, nil
}

type sseEvent struct {
	Type string
	Data []byte
}

func parseSSEBlock(block string) (sseEvent, error) {
	var event sseEvent
	var data bytes.Buffer

	for _, line := range strings.Split(block, "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}
		name, value, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		if strings.HasPrefix(value, " ") {
			value = value[1:]
		}

		switch name {
		case "event":
			event.Type = value
		case "data":
			if data.Len() > 0 {
				data.WriteByte('\n')
			}
			data.WriteString(value)
		}
	}

	event.Data = data.Bytes()
	if event.Type == "" && len(event.Data) == 0 {
		return sseEvent{}, fmt.Errorf("empty sse block")
	}
	return event, nil
}

func readSSEEvent(reader *bufio.Reader) (string, error) {
	var builder strings.Builder
	for {
		line, err := reader.ReadString('\n')
		trimmed := strings.TrimRight(line, "\r\n")
		if trimmed != "" {
			builder.WriteString(trimmed)
			builder.WriteString("\n")
		}

		if err != nil {
			if errors.Is(err, io.EOF) {
				if builder.Len() > 0 {
					return strings.TrimSuffix(builder.String(), "\n"), nil
				}
				return "", err
			}
			return "", err
		}
		if trimmed == "" {
			break
		}
	}
	return strings.TrimSuffix(builder.String(), "\n"), nil
}

func isDoneEvent(event sseEvent) bool {
	return bytes.Equal(bytes.TrimSpace(event.Data), []byte("[DONE]"))
}

func openAIResponseCompletedFinishReasonFromEvent(event sseEvent) string {
	var jsonBody map[string]interface{}
	if err := json.Unmarshal(event.Data, &jsonBody); err != nil {
		return ""
	}
	if len(jsonBody) == 0 {
		return ""
	}

	respObj, _ := jsonBody["response"].(map[string]interface{})
	stopReason := ""
	if respObj != nil {
		stopReason = getString(respObj, "stop_reason")
	}
	if stopReason == "" {
		stopReason = getString(jsonBody, "stop_reason")
	}

	switch stopReason {
	case "", "stop", "end_turn":
		return "stop"
	case "tool_call", "tool_calls":
		return "tool_calls"
	case "length", "max_output_tokens":
		return "length"
	case "content_filter":
		return "content_filter"
	default:
		return stopReason
	}
}

func openAIStreamErrorFromEvent(event sseEvent) error {
	if event.Type != "error" && event.Type != "response.failed" {
		return nil
	}

	var jsonBody map[string]interface{}
	if err := json.Unmarshal(event.Data, &jsonBody); err != nil {
		return fmt.Errorf("openai stream error event (%s): %s", event.Type, string(event.Data))
	}

	var errObj map[string]interface{}
	if event.Type == "error" {
		errObj, _ = jsonBody["error"].(map[string]interface{})
	} else {
		respObj, _ := jsonBody["response"].(map[string]interface{})
		if respObj != nil {
			errObj, _ = respObj["error"].(map[string]interface{})
		}
	}

	if errObj == nil {
		return fmt.Errorf("openai stream error event (%s): %s", event.Type, string(event.Data))
	}

	message := getString(errObj, "message")
	code := getString(errObj, "code")
	errType := getString(errObj, "type")

	parts := make([]string, 0, 3)
	if message != "" {
		parts = append(parts, message)
	}
	if code != "" {
		parts = append(parts, "code="+code)
	}
	if errType != "" {
		parts = append(parts, "type="+errType)
	}
	if len(parts) == 0 {
		parts = append(parts, string(event.Data))
	}

	return fmt.Errorf("openai stream error event (%s): %s", event.Type, strings.Join(parts, ", "))
}

func emitFinishEvent(ctx context.Context, out chan<- llm.StreamEvent, helper ProviderHelper, usageParser UsageParser, finishReason string) bool {
	u := usageParser.Retrieve()
	uInfo := helper.NormalizeUsage(u)
	return sendStreamEvent(ctx, out, llm.StreamEvent{
		Type:   string(llm.GeneratorMessageTypeFinish),
		Finish: finishReason,
		Usage: &llm.Usage{
			InputTokens:       uInfo.InputTokens,
			OutputTokens:      uInfo.OutputTokens,
			ReasoningTokens:   uInfo.ReasoningTokens,
			CachedInputTokens: uInfo.CacheReadTokens,
		},
	})
}

func sendStreamEvent(ctx context.Context, out chan<- llm.StreamEvent, event llm.StreamEvent) bool {
	if ctx == nil {
		out <- event
		return true
	}

	select {
	case <-ctx.Done():
		return false
	case out <- event:
		return true
	}
}

func getHelper(provider *models.LLMConfig, modelName string) ProviderHelper {
	pid := ""
	ptype := ""
	apiType := models.LLMAPITypeResponse
	if provider != nil {
		pid = strings.ToLower(strings.TrimSpace(provider.Name))
		ptype = strings.ToLower(strings.TrimSpace(provider.Type))
		apiType = models.NormalizeLLMAPIType(provider.APIType)
	}
	if strings.Contains(pid, "anthropic") || strings.Contains(ptype, "anthropic") || ptype == "claude" {
		return &AnthropicHelper{ReqModel: modelName, ProviderModel: modelName}
	}
	if strings.Contains(pid, "google") || strings.Contains(ptype, "google") {
		return &GoogleHelper{ProviderModel: modelName}
	}
	if apiType == models.LLMAPITypeChat {
		return &OACompatHelper{}
	}
	if strings.Contains(pid, "openai-compatible") {
		return &OACompatHelper{}
	}
	if pid == "openai" || ptype == "openai" || ptype == "custom" {
		return &OpenAIHelper{}
	}
	return &OpenAIHelper{}
}

func resolveOptions(provider *models.LLMConfig) (string, string, string) {
	if provider == nil {
		return "", "", ""
	}
	apiKey := provider.APIKey
	baseURL := provider.BaseURL
	if baseURL == "" {
		if strings.Contains(provider.Name, "anthropic") {
			baseURL = "https://api.anthropic.com/v1"
		} else {
			baseURL = "https://api.openai.com/v1"
		}
	}
	return apiKey, baseURL, strings.TrimSpace(provider.Proxy)
}

// formatTimeoutError wraps an error with the actual timeout duration when it
// looks like an HTTP client timeout (either from client.Do or from reading body).
func formatTimeoutError(err error, timeout time.Duration) error {
	if err == nil || timeout <= 0 {
		return err
	}
	errStr := err.Error()
	if strings.Contains(errStr, "Client.Timeout") ||
		strings.Contains(errStr, "request canceled") ||
		strings.Contains(errStr, "context deadline exceeded") {
		return fmt.Errorf("%s (timeout: %s)", errStr, timeout)
	}
	return err
}

func (c *GenericClient) resolveHTTPClient(override *http.Client) *http.Client {
	var client *http.Client
	switch {
	case override != nil:
		cloned := *override
		client = &cloned
	case c.HTTPClient != nil:
		cloned := *c.HTTPClient
		client = &cloned
	default:
		client = &http.Client{}
	}

	if client.Timeout == 0 && c.Timeout > 0 {
		client.Timeout = c.Timeout
	}

	return client
}

// resolveHTTPClientForProvider applies LLMConfig.Proxy when override is nil; when override is non-nil, the caller
// (e.g. session tracing client) is responsible for building proxy into the base transport.
func (c *GenericClient) resolveHTTPClientForProvider(override *http.Client, provider *models.LLMConfig) *http.Client {
	if override != nil {
		return c.resolveHTTPClient(override)
	}
	_, _, proxy := resolveOptions(provider)
	if proxy == "" {
		return c.resolveHTTPClient(nil)
	}
	pc := httpclient.ClientWithOptionalProxy(proxy)
	if pc == nil {
		return c.resolveHTTPClient(nil)
	}
	return c.resolveHTTPClient(pc)
}

func apiErrorFromHTTPResponse(resp *http.Response, body []byte, providerID string) error {
	statusCode := 0
	if resp != nil {
		statusCode = resp.StatusCode
	}

	message := strings.TrimSpace(string(body))
	if message == "" && statusCode != 0 {
		message = http.StatusText(statusCode)
	}

	return &APIError{
		ProviderID:      providerID,
		Message:         message,
		StatusCode:      statusCode,
		IsRetryable:     retryableStatus(statusCode),
		ResponseBody:    string(body),
		ResponseHeaders: responseHeadersFromResponse(resp),
	}
}

func toCommonRequest(req llm.ChatRequest) CommonRequest {
	cr := CommonRequest{
		Temperature: req.Temperature,
		TopP:        req.TopP,
		MaxTokens:   req.MaxOutputTokens,
		Stream:      false,
		Model:       req.Model,
	}
	if req.ExtraOptions != nil {
		if instruction, ok := req.ExtraOptions["instructions"].(string); ok {
			cr.Instructions = strings.TrimSpace(instruction)
		}
	}

	// Convert Messages
	for _, m := range req.Messages {
		// if m.Role == "tool" {
		// 	cm := CommonMessage{
		// 		Type:   "function_call_output",
		// 		CallID: m.ToolCallID,
		// 		Output: m.Content.(string),
		// 	}
		// 	if s, ok := m.Content.(string); ok {
		// 		cm.Content = s
		// 	}
		// 	cr.Messages = append(cr.Messages, cm)
		// 	continue
		// }
		cm := CommonMessage{
			Role:    m.Role,
			CallID:  m.ToolCallID,
			Content: m.Content,
		}
		// ... handle tool calls ...
		if len(m.ToolCalls) > 0 {
			for _, tc := range m.ToolCalls {
				argsBytes, _ := json.Marshal(tc.Arguments)
				cm.ToolCalls = append(cm.ToolCalls, CommonToolCall{
					ID:   tc.ID,
					Type: "function",
					Function: CommonToolCallFunction{
						Name:      tc.Name,
						Arguments: string(argsBytes),
					},
				})
			}
		}
		cr.Messages = append(cr.Messages, cm)
	}

	// Convert Tools
	// for _, t := range req.Tools {
	// 	cr.Tools = append(cr.Tools, CommonTool{
	// 		Type: "function",
	// 		Function: CommonToolFunction{
	// 			Name:        t.Name,
	// 			Description: t.Description,
	// 			Parameters: map[string]interface{}{
	// 				"type":       "object",
	// 				"properties": t.Schema,
	// 			},
	// 		},
	// 	})
	// }

	return cr
}

func fromCommonResponse(cr CommonResponse) llm.ChatResponse {
	lcr := llm.ChatResponse{}
	if len(cr.Choices) > 0 {
		choice := cr.Choices[0]
		lcr.Finish = choice.FinishReason
		lcr.Message.Role = choice.Message.Role

		if s, ok := choice.Message.Content.(string); ok {
			lcr.Message.Content = s
		}

		for _, tc := range choice.Message.ToolCalls {
			var args map[string]interface{}
			_ = json.Unmarshal([]byte(tc.Function.Arguments), &args)
			lcr.ToolCalls = append(lcr.ToolCalls, llm.ToolCall{
				ID:        tc.ID,
				Name:      tc.Function.Name,
				Arguments: args,
			})
		}
	}
	if cr.Usage != nil {
		lcr.Usage = &llm.Usage{
			InputTokens:  cr.Usage.PromptTokens,
			OutputTokens: cr.Usage.CompletionTokens,
			// ...
		}
	}
	return lcr
}

// Missing Response Converters stubs
func FromOaCompatibleResponse(resp map[string]interface{}) CommonResponse {
	cr := CommonResponse{}
	cr.Object = "chat.completion"

	if id, ok := resp["id"].(string); ok {
		cr.ID = id
	}
	if model, ok := resp["model"].(string); ok {
		cr.Model = model
	}
	if created, ok := resp["created"].(float64); ok {
		cr.Created = int64(created)
	}

	if choices, ok := resp["choices"].([]interface{}); ok {
		for _, c := range choices {
			cm, ok := c.(map[string]interface{})
			if !ok {
				continue
			}

			crc := CommonResponseChoice{
				Index:        int(getFloat(cm, "index")),
				FinishReason: getString(cm, "finish_reason"),
			}

			if msg, ok := cm["message"].(map[string]interface{}); ok {
				crc.Message.Role = getString(msg, "role")
				if content, ok := msg["content"].(string); ok {
					crc.Message.Content = content
				}
				if tcs, ok := msg["tool_calls"].([]interface{}); ok {
					for _, tc := range tcs {
						tcm, ok := tc.(map[string]interface{})
						if !ok {
							continue
						}
						fn, _ := tcm["function"].(map[string]interface{})
						crc.Message.ToolCalls = append(crc.Message.ToolCalls, CommonToolCall{
							ID:   getString(tcm, "id"),
							Type: getString(tcm, "type"),
							Function: CommonToolCallFunction{
								Name:      getString(fn, "name"),
								Arguments: getString(fn, "arguments"),
							},
						})
					}
				}
			}
			cr.Choices = append(cr.Choices, crc)
		}
	}

	if u, ok := resp["usage"].(map[string]interface{}); ok {
		cr.Usage = &CommonUsage{
			PromptTokens:     int(getFloat(u, "prompt_tokens")),
			CompletionTokens: int(getFloat(u, "completion_tokens")),
			TotalTokens:      int(getFloat(u, "total_tokens")),
		}
	}

	return cr
}
func FromOpenAIResponse(resp map[string]interface{}) CommonResponse {
	cr := CommonResponse{}
	cr.Object = "chat.completion"

	if id, ok := resp["id"].(string); ok {
		cr.ID = id
	}
	if model, ok := resp["model"].(string); ok {
		cr.Model = model
	}
	if created, ok := resp["created_at"].(float64); ok {
		cr.Created = int64(created)
	} else if created, ok := resp["created"].(float64); ok {
		cr.Created = int64(created)
	}

	// 处理新版 OpenAI Response API 格式 (object: "response")
	if objType := getString(resp, "object"); objType == "response" {
		// 从 output 数组中提取消息
		if outputs, ok := resp["output"].([]interface{}); ok {
			choiceIndex := 0
			for _, output := range outputs {
				outputMap, ok := output.(map[string]interface{})
				if !ok {
					continue
				}

				outputType := getString(outputMap, "type")

				// 处理 message 类型的输出
				if outputType == "message" {
					crc := CommonResponseChoice{
						Index: choiceIndex,
						Message: CommonMessage{
							Role: getString(outputMap, "role"),
						},
					}

					// 提取 content 文本
					if contentParts, ok := outputMap["content"].([]interface{}); ok {
						var textContent string
						var toolCalls []CommonToolCall

						for _, part := range contentParts {
							partMap, ok := part.(map[string]interface{})
							if !ok {
								continue
							}
							partType := getString(partMap, "type")

							// 处理文本内容
							if partType == "output_text" || partType == "text" {
								if txt, ok := partMap["text"].(string); ok {
									textContent += txt
								}
							}

							// 处理工具调用
							if partType == "function_call" {
								toolCall := CommonToolCall{
									ID:   getString(partMap, "id"),
									Type: "function",
									Function: CommonToolCallFunction{
										Name:      getString(partMap, "name"),
										Arguments: getString(partMap, "arguments"),
									},
								}
								toolCalls = append(toolCalls, toolCall)
							}
						}

						if textContent != "" {
							crc.Message.Content = textContent
						}
						if len(toolCalls) > 0 {
							crc.Message.ToolCalls = toolCalls
						}
					}

					// 映射 status 到 finish_reason
					status := getString(outputMap, "status")
					if status == "completed" {
						crc.FinishReason = "stop"
					} else if status == "incomplete" {
						crc.FinishReason = "length"
					} else if status == "failed" {
						crc.FinishReason = "error"
					}

					// 如果有 stop_reason，优先使用
					if stopReason := getString(resp, "stop_reason"); stopReason != "" {
						crc.FinishReason = stopReason
					}

					cr.Choices = append(cr.Choices, crc)
					choiceIndex++
				}
			}
		}

		// 解析 usage 信息（新格式）
		if u, ok := resp["usage"].(map[string]interface{}); ok {
			cr.Usage = &CommonUsage{
				PromptTokens:     int(getFloat(u, "input_tokens")),
				CompletionTokens: int(getFloat(u, "output_tokens")),
				TotalTokens:      int(getFloat(u, "total_tokens")),
			}

			// 处理缓存的 tokens
			if details, ok := u["input_tokens_details"].(map[string]interface{}); ok {
				if cachedTokens := int(getFloat(details, "cached_tokens")); cachedTokens > 0 {
					if cr.Usage.PromptTokensDetails == nil {
						cr.Usage.PromptTokensDetails = &struct {
							CachedTokens int `json:"cached_tokens,omitempty"`
						}{}
					}
					cr.Usage.PromptTokensDetails.CachedTokens = cachedTokens
				}
			}
		}

		return cr
	}

	// 处理标准 OpenAI Chat Completion 格式 (object: "chat.completion")
	if choices, ok := resp["choices"].([]interface{}); ok {
		for _, c := range choices {
			cm, ok := c.(map[string]interface{})
			if !ok {
				continue
			}

			crc := CommonResponseChoice{
				Index:        int(getFloat(cm, "index")),
				FinishReason: getString(cm, "finish_reason"),
			}

			if msg, ok := cm["message"].(map[string]interface{}); ok {
				crc.Message.Role = getString(msg, "role")

				// 处理 content
				if content, ok := msg["content"].(string); ok {
					crc.Message.Content = content
				}

				// 解析 tool calls
				if tcs, ok := msg["tool_calls"].([]interface{}); ok {
					for _, tc := range tcs {
						tcm, ok := tc.(map[string]interface{})
						if !ok {
							continue
						}
						fn, _ := tcm["function"].(map[string]interface{})
						crc.Message.ToolCalls = append(crc.Message.ToolCalls, CommonToolCall{
							ID:   getString(tcm, "id"),
							Type: getString(tcm, "type"),
							Function: CommonToolCallFunction{
								Name:      getString(fn, "name"),
								Arguments: getString(fn, "arguments"),
							},
						})
					}
				}
			}
			cr.Choices = append(cr.Choices, crc)
		}
	}

	// 解析 usage 信息（标准格式）
	if u, ok := resp["usage"].(map[string]interface{}); ok {
		cr.Usage = &CommonUsage{
			PromptTokens:     int(getFloat(u, "prompt_tokens")),
			CompletionTokens: int(getFloat(u, "completion_tokens")),
			TotalTokens:      int(getFloat(u, "total_tokens")),
		}

		if details, ok := u["prompt_tokens_details"].(map[string]interface{}); ok {
			if cachedTokens := int(getFloat(details, "cached_tokens")); cachedTokens > 0 {
				if cr.Usage.PromptTokensDetails == nil {
					cr.Usage.PromptTokensDetails = &struct {
						CachedTokens int `json:"cached_tokens,omitempty"`
					}{}
				}
				cr.Usage.PromptTokensDetails.CachedTokens = cachedTokens
			}
		}
	}

	return cr
}

func FromAnthropicResponse(resp map[string]interface{}) CommonResponse {
	// Simplified converter for Anthropic response (message type)
	cr := CommonResponse{
		Object: "chat.completion",
	}

	if id, ok := resp["id"].(string); ok {
		cr.ID = id
	}
	if model, ok := resp["model"].(string); ok {
		cr.Model = model
	}

	msgType := getString(resp, "type")
	if msgType == "message" {
		crc := CommonResponseChoice{
			Index:        0,
			Message:      CommonMessage{Role: "assistant"},
			FinishReason: getString(resp, "stop_reason"),
		}

		// Map Anthropic stop reasons to standard
		if crc.FinishReason == "end_turn" {
			crc.FinishReason = "stop"
		} else if crc.FinishReason == "tool_use" {
			crc.FinishReason = "tool_calls"
		}

		contentList, _ := resp["content"].([]interface{})
		for _, c := range contentList {
			cm, ok := c.(map[string]interface{})
			if !ok {
				continue
			}
			ctype := getString(cm, "type")
			if ctype == "text" {
				if s, ok := crc.Message.Content.(string); ok {
					crc.Message.Content = s + getString(cm, "text")
				} else {
					crc.Message.Content = getString(cm, "text")
				}
			} else if ctype == "tool_use" {
				input := cm["input"]
				inputStr := ""
				if s, ok := input.(string); ok {
					inputStr = s
				} else {
					b, _ := json.Marshal(input)
					inputStr = string(b)
				}

				crc.Message.ToolCalls = append(crc.Message.ToolCalls, CommonToolCall{
					ID:   getString(cm, "id"),
					Type: "function",
					Function: CommonToolCallFunction{
						Name:      getString(cm, "name"),
						Arguments: inputStr,
					},
				})
			}
		}
		cr.Choices = append(cr.Choices, crc)
	}

	if u, ok := resp["usage"].(map[string]interface{}); ok {
		cr.Usage = &CommonUsage{
			PromptTokens:     int(getFloat(u, "input_tokens")),
			CompletionTokens: int(getFloat(u, "output_tokens")),
			TotalTokens:      int(getFloat(u, "input_tokens")) + int(getFloat(u, "output_tokens")),
		}
	}

	return cr
}

func getFloat(m map[string]interface{}, k string) float64 {
	if v, ok := m[k].(float64); ok {
		return v
	}
	return 0
}
