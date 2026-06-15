package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"matrixops-agent/config"
	"matrixops-agent/global"
	"matrixops-agent/llm"
	"matrixops-agent/types"
	"matrixops-agent/util"

	"pkgs/llmheaders"
)

type OpenAIClient struct {
	DefaultModelID string
	cfg            *config.Config
	timeout        time.Duration
	generic        *GenericClient
}

type chatCompletionResponse struct {
	ID      string                 `json:"id"`
	Model   string                 `json:"model"`
	Choices []chatCompletionChoice `json:"choices"`
	Usage   *completionUsage       `json:"usage"`
}

type chatCompletionChoice struct {
	Index        int                   `json:"index"`
	Message      chatCompletionMessage `json:"message"`
	FinishReason string                `json:"finish_reason"`
}

type chatCompletionMessage struct {
	Role      string                   `json:"role"`
	Content   interface{}              `json:"content"`
	ToolCalls []chatCompletionToolCall `json:"tool_calls"`
}

type chatCompletionToolCall struct {
	ID       string                         `json:"id"`
	Type     string                         `json:"type"`
	Function chatCompletionToolCallFunction `json:"function"`
}

type chatCompletionToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type completionUsage struct {
	PromptTokens        int `json:"prompt_tokens"`
	CompletionTokens    int `json:"completion_tokens"`
	TotalTokens         int `json:"total_tokens"`
	CachedTokens        int `json:"cached_tokens"`
	PromptTokensDetails struct {
		CachedTokens int `json:"cached_tokens"`
	} `json:"prompt_tokens_details"`
	CompletionTokensDetails struct {
		ReasoningTokens int `json:"reasoning_tokens"`
	} `json:"completion_tokens_details"`
}

type apiErrorResponse struct {
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

type streamChunk struct {
	ID      string           `json:"id"`
	Model   string           `json:"model"`
	Choices []streamChoice   `json:"choices"`
	Usage   *completionUsage `json:"usage"`
}

type streamChoice struct {
	Index        int         `json:"index"`
	Delta        streamDelta `json:"delta"`
	FinishReason string      `json:"finish_reason"`
}

type streamDelta struct {
	Role      string                     `json:"role,omitempty"`
	Content   string                     `json:"content,omitempty"`
	ToolCalls []streamToolCall           `json:"tool_calls,omitempty"`
	Extra     map[string]json.RawMessage `json:"-"`
}

type streamToolCall struct {
	Index    int    `json:"index"`
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

func (d *streamDelta) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if v, ok := raw["role"]; ok {
		_ = json.Unmarshal(v, &d.Role)
		delete(raw, "role")
	}
	if v, ok := raw["content"]; ok {
		_ = json.Unmarshal(v, &d.Content)
		delete(raw, "content")
	}
	if v, ok := raw["tool_calls"]; ok {
		_ = json.Unmarshal(v, &d.ToolCalls)
		delete(raw, "tool_calls")
	}
	d.Extra = raw
	return nil
}

func NewOpenAIClient(cfg *config.Config) *OpenAIClient {
	if cfg == nil {
		cfg = &config.Config{}
	}
	defaultModel := cfg.Model
	if defaultModel == "" {
		defaultModel = os.Getenv(global.EnvModel)
	}
	if defaultModel == "" {
		defaultModel = "openai/gpt-4o-mini"
	}
	return &OpenAIClient{
		DefaultModelID: defaultModel,
		cfg:            cfg,
		timeout:        300 * time.Second,
		generic:        NewGenericClient(),
	}
}

func (c *OpenAIClient) DefaultModel() (types.ModelRef, error) {
	return ParseModel(c.DefaultModelID), nil
}

func (c *OpenAIClient) GetModel(providerID string, modelID string) (Model, error) {
	key, baseURL, proxy := c.resolveProviderOptions(providerID)
	if providerID == "" {
		providerID = "openai"
	}
	if modelID == "" {
		return Model{}, errors.New("model id required")
	}
	model := Model{
		ID:         modelID,
		ProviderID: providerID,
		API: APIInfo{
			ID:  modelID,
			NPM: "@ai-sdk/openai",
		},
		Limit: defaultModelLimit(providerID, modelID),
		Options: map[string]interface{}{
			"apiKey":  key,
			"baseURL": baseURL,
			"proxy":   proxy,
		},
		Headers:  map[string]string{},
		Variants: map[string]map[string]interface{}{},
	}
	if devProvider, devModel := LookupModelsDevModel(providerID, modelID); devProvider != nil {
		if devProvider.NPM != "" {
			model.API.NPM = devProvider.NPM
		}
		if devModel != nil {
			if devModel.Provider != nil && devModel.Provider.NPM != "" {
				model.API.NPM = devModel.Provider.NPM
			}
			if devModel.Limit.Context != 0 {
				model.Limit.Context = devModel.Limit.Context
			}
			if devModel.Limit.Output != 0 {
				model.Limit.Output = devModel.Limit.Output
			}
			if devModel.Limit.Input != 0 {
				model.Limit.Input = devModel.Limit.Input
			}
			if devModel.Options != nil {
				model.Options = devModel.Options
			}
			if devModel.Headers != nil {
				model.Headers = devModel.Headers
			}
			if devModel.Variants != nil {
				model.Variants = devModel.Variants
			}
			model.Cost = modelCostFromModelsDev(devModel.Cost)
		}
	}
	return model, nil
}

func (c *OpenAIClient) GetLanguage(model Model) (LanguageModel, error) {
	return model, nil
}

func defaultModelLimit(providerID string, modelID string) ModelLimit {
	id := strings.ToLower(modelID)
	switch {
	case strings.Contains(id, "gpt-4o") || strings.Contains(id, "gpt-4.1") || strings.Contains(id, "gpt-4-turbo"):
		return ModelLimit{Context: 128000, Output: 16384}
	case strings.Contains(id, "gpt-4"):
		return ModelLimit{Context: 128000, Output: 8192}
	case strings.Contains(id, "gpt-3.5"):
		return ModelLimit{Context: 16000, Output: 4096}
	default:
		return ModelLimit{}
	}
}

func (c *OpenAIClient) Chat(request llm.ChatRequest) (llm.ChatResponse, error) {
	modelID, ok := request.Model.(Model)
	if !ok {
		return llm.ChatResponse{}, errors.New("invalid model")
	}
	messages := normalizeMessages(request.Messages, modelID)
	body := buildChatBody(modelID, messages, request, false)

	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	resp, err := c.doChatRequest(ctx, modelID, body)
	if err != nil {
		return llm.ChatResponse{}, err
	}
	if len(resp.Choices) == 0 {
		return llm.ChatResponse{}, errors.New("no choices")
	}
	choice := resp.Choices[0]
	toolCalls := []llm.ToolCall{}
	for _, call := range choice.Message.ToolCalls {
		args := map[string]interface{}{}
		if call.Function.Arguments != "" {
			_ = json.Unmarshal([]byte(call.Function.Arguments), &args)
		}
		toolCalls = append(toolCalls, llm.ToolCall{
			ID:        call.ID,
			Name:      call.Function.Name,
			Arguments: args,
		})
	}
	return llm.ChatResponse{
		Message: llm.ModelMessage{
			Role:    "assistant",
			Content: messageContentText(choice.Message.Content),
		},
		ToolCalls: toolCalls,
		Finish:    choice.FinishReason,
		Usage:     usageFromCompletionUsage(resp.Usage),
	}, nil
}

func (c *OpenAIClient) GenerateObject(request llm.GenerateRequest) (llm.GenerateResult, error) {
	modelID, ok := request.Model.(Model)
	if !ok {
		return llm.GenerateResult{}, errors.New("invalid model")
	}
	messages := normalizeMessages(request.Messages, modelID)
	body := buildChatBody(modelID, messages, llm.ChatRequest{
		Messages:        messages,
		Temperature:     request.Temperature,
		ProviderOptions: request.ProviderOptions,
	}, false)
	body["response_format"] = map[string]string{"type": "json_object"}

	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	resp, err := c.doChatRequest(ctx, modelID, body)
	if err != nil {
		return llm.GenerateResult{}, err
	}
	if len(resp.Choices) == 0 {
		return llm.GenerateResult{}, errors.New("no choices")
	}
	content := messageContentText(resp.Choices[0].Message.Content)
	var result llm.GenerateResult
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return llm.GenerateResult{}, err
	}
	return result, nil
}

func (c *OpenAIClient) StreamObject(request llm.GenerateRequest) (llm.GenerateResult, error) {
	return c.GenerateObject(request)
}

func (c *OpenAIClient) StreamChat(request llm.ChatRequest) (<-chan llm.StreamEvent, error) {
	return c.StreamChatWithOptions(request)
}

func (c *OpenAIClient) StreamChatWithOptions(request llm.ChatRequest, opts ...llm.StreamChatOption) (<-chan llm.StreamEvent, error) {
	cfg := llm.NewStreamChatOptions(opts...)
	if cfg.OnRequest != nil {
		if err := cfg.OnRequest(&request); err != nil {
			return nil, err
		}
	}
	modelID, ok := request.Model.(Model)
	if !ok {
		return nil, errors.New("invalid model")
	}
	messages := normalizeMessages(request.Messages, modelID)
	body := buildChatBody(modelID, messages, request, true)
	if cfg.OnRawRequest != nil {
		if raw, err := json.Marshal(body); err == nil {
			cfg.OnRawRequest(string(raw))
		}
	}

	parentCtx := request.Context
	if parentCtx == nil {
		parentCtx = context.Background()
	}
	ctx, cancel := context.WithTimeout(parentCtx, c.timeout)
	req, client, err := c.newRequest(ctx, modelID, body, true)
	if err != nil {
		cancel()
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		cancel()
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		cancel()
		return nil, apiErrorFromResponse(resp, data, modelID.ProviderID)
	}

	out := make(chan llm.StreamEvent)
	go func() {
		defer close(out)
		defer cancel()
		defer resp.Body.Close()
		var rawResponseBuilder strings.Builder

		reader := bufio.NewReader(resp.Body)
		var usage *llm.Usage
		finishReason := ""
		for {
			block, err := readSSEEvent(reader)
			if err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				out <- llm.StreamEvent{Type: types.PartTypeError, Error: err}
				return
			}
			if block == "" {
				continue
			}
			rawResponseBuilder.WriteString(block)
			rawResponseBuilder.WriteString("\n\n")
			lines := strings.Split(block, "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line == "" || strings.HasPrefix(line, ":") {
					continue
				}
				if !strings.HasPrefix(line, "data:") {
					continue
				}
				payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
				if payload == "" {
					continue
				}
				if payload == "[DONE]" {
					if cfg.OnRawResponse != nil {
						cfg.OnRawResponse(rawResponseBuilder.String())
					}
					out <- llm.StreamEvent{Type: types.PartTypeFinish, Finish: finishReason, Usage: usage}
					return
				}

				var chunk streamChunk
				if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
					continue
				}
				if chunk.Usage != nil {
					usage = usageFromCompletionUsage(chunk.Usage)
				}
				for _, choice := range chunk.Choices {
					if choice.Delta.Content != "" {
						out <- llm.StreamEvent{Type: types.PartTypeTextDelta, Text: choice.Delta.Content}
					}
					if reasoning := deltaReasoningText(choice.Delta); reasoning != "" {
						out <- llm.StreamEvent{Type: types.PartTypeReasoningDelta, Text: reasoning}
					}
					for _, call := range choice.Delta.ToolCalls {
						out <- llm.StreamEvent{
							Type:          types.PartTypeToolDelta,
							ToolIndex:     call.Index,
							ToolCallID:    call.ID,
							ToolName:      call.Function.Name,
							ToolArguments: call.Function.Arguments,
						}
					}
					if choice.FinishReason != "" {
						finishReason = choice.FinishReason
					}
				}
			}
		}
		if cfg.OnRawResponse != nil {
			cfg.OnRawResponse(rawResponseBuilder.String())
		}
		out <- llm.StreamEvent{Type: types.PartTypeFinish, Finish: finishReason, Usage: usage}
	}()
	return out, nil
}

func (c *OpenAIClient) doChatRequest(ctx context.Context, model Model, body map[string]interface{}) (*chatCompletionResponse, error) {
	req, client, err := c.newRequest(ctx, model, body, false)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, apiErrorFromResponse(resp, data, model.ProviderID)
	}

	var parsed chatCompletionResponse
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, err
	}
	return &parsed, nil
}

func (c *OpenAIClient) newRequest(ctx context.Context, model Model, body map[string]interface{}, stream bool) (*http.Request, *http.Client, error) {
	apiKey, baseURL, proxy := c.resolveProviderOptions(model.ProviderID)
	if apiKey == "" {
		return nil, nil, errors.New("missing API key")
	}
	client, err := c.httpClient(proxy)
	if err != nil {
		return nil, nil, err
	}
	urlStr := buildChatURL(baseURL)
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, nil, err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, urlStr, bytes.NewReader(payload))
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("authorization", "Bearer "+apiKey)
	if stream {
		req.Header.Set("accept", "text/event-stream")
	}
	for key, value := range model.Headers {
		if value == "" {
			continue
		}
		req.Header.Set(key, value)
	}
	llmheaders.Apply(req.Header)
	return req, client, nil
}

func buildChatURL(baseURL string) string {
	trimmed := strings.TrimRight(baseURL, "/")
	if strings.HasSuffix(trimmed, "/chat/completions") {
		return trimmed
	}
	return trimmed + "/chat/completions"
}

func buildChatBody(model Model, messages []llm.ModelMessage, request llm.ChatRequest, stream bool) map[string]interface{} {
	body := map[string]interface{}{
		"model":    model.ID,
		"messages": toOpenAICompatibleMessages(messages),
	}
	if request.Temperature != 0 {
		body["temperature"] = request.Temperature
	}
	if request.TopP != 0 {
		body["top_p"] = request.TopP
	}
	if request.MaxOutputTokens != 0 {
		body["max_tokens"] = request.MaxOutputTokens
	}
	if len(request.Tools) > 0 {
		body["tools"] = toOpenAITools(request.Tools)
	}
	if stream {
		body["stream"] = true
		body["stream_options"] = map[string]interface{}{"include_usage": true}
	}
	if expanded := expandProviderOptions(model, request.ProviderOptions); len(expanded) > 0 {
		body = util.MergeMaps(body, expanded)
	}
	return body
}

func usageFromCompletionUsage(usage *completionUsage) *llm.Usage {
	if usage == nil {
		return nil
	}
	cached := usage.CachedTokens
	if cached == 0 {
		cached = usage.PromptTokensDetails.CachedTokens
	}
	if usage.PromptTokens == 0 && usage.CompletionTokens == 0 && usage.TotalTokens == 0 &&
		usage.CompletionTokensDetails.ReasoningTokens == 0 && cached == 0 {
		return nil
	}
	return &llm.Usage{
		InputTokens:       usage.PromptTokens,
		OutputTokens:      usage.CompletionTokens,
		ReasoningTokens:   usage.CompletionTokensDetails.ReasoningTokens,
		CachedInputTokens: cached,
	}
}

func apiErrorFromResponse(resp *http.Response, data []byte, providerID string) error {
	statusCode := 0
	if resp != nil {
		statusCode = resp.StatusCode
	}
	message := ""
	if len(data) > 0 {
		var parsed apiErrorResponse
		if err := json.Unmarshal(data, &parsed); err == nil {
			message = parsed.Error.Message
		}
		if message == "" {
			message = string(data)
		}
	}
	if message == "" && statusCode != 0 {
		message = http.StatusText(statusCode)
	}
	return &APIError{
		ProviderID:      providerID,
		Message:         message,
		StatusCode:      statusCode,
		ResponseBody:    string(data),
		ResponseHeaders: responseHeadersFromResponse(resp),
		IsRetryable:     retryableStatus(statusCode),
	}
}

func (c *OpenAIClient) resolveProviderOptions(providerID string) (string, string, string) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	baseURL := os.Getenv("OPENAI_BASE_URL")
	proxy := proxyFromEnv()

	if c.cfg != nil {
		if c.cfg.Proxy != "" {
			proxy = c.cfg.Proxy
		}
		if providerID == "" {
			providerID = "openai"
		}
		if provider, ok := c.cfg.Provider[providerID]; ok {
			apiKey = firstString(apiKey, provider.Options, "apiKey", "key")
			baseURL = firstString(baseURL, provider.Options, "endpoint", "baseURL", "baseurl")
			proxy = firstString(proxy, provider.Options, "proxy", "httpProxy", "httpsProxy")
		} else if providerID != "openai" {
			if provider, ok := c.cfg.Provider["openai"]; ok {
				apiKey = firstString(apiKey, provider.Options, "apiKey", "key")
				baseURL = firstString(baseURL, provider.Options, "endpoint", "baseURL", "baseurl")
				proxy = firstString(proxy, provider.Options, "proxy", "httpProxy", "httpsProxy")
			}
		}
	}
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	return apiKey, baseURL, proxy
}

func (c *OpenAIClient) httpClient(proxy string) (*http.Client, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if proxy != "" {
		parsed, err := url.Parse(proxy)
		if err != nil {
			return nil, err
		}
		transport.Proxy = http.ProxyURL(parsed)
	}
	return &http.Client{Timeout: c.timeout, Transport: transport}, nil
}

func proxyFromEnv() string {
	if value := os.Getenv("HTTPS_PROXY"); value != "" {
		return value
	}
	if value := os.Getenv("HTTP_PROXY"); value != "" {
		return value
	}
	return ""
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

func deltaReasoningText(delta streamDelta) string {
	if delta.Extra == nil {
		return ""
	}
	for _, key := range []string{"reasoning_content", "reasoning", "thinking"} {
		raw, ok := delta.Extra[key]
		if !ok {
			continue
		}
		if text := rawJSONString(raw); text != "" {
			return text
		}
	}
	return ""
}

func rawJSONString(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text
	}
	return ""
}

func normalizeMessages(messages []llm.ModelMessage, model Model) []llm.ModelMessage {
	lowerProvider := strings.ToLower(model.ProviderID)
	lowerAPI := strings.ToLower(model.API.ID)
	lowerModel := strings.ToLower(model.ID)
	lowerNPM := strings.ToLower(model.API.NPM)

	isAnthropic := strings.Contains(lowerProvider, "anthropic") ||
		strings.Contains(lowerAPI, "claude") ||
		strings.Contains(lowerModel, "claude") ||
		strings.Contains(lowerNPM, "anthropic")
	isClaude := isAnthropic
	isMistral := strings.Contains(lowerProvider, "mistral") ||
		strings.Contains(lowerAPI, "mistral") ||
		strings.Contains(lowerModel, "mistral")

	out := make([]llm.ModelMessage, 0, len(messages))
	for i, msg := range messages {
		if isAnthropic {
			if text, ok := msg.Content.(string); ok && text == "" {
				continue
			}
		}
		if len(msg.ToolCalls) > 0 {
			for i := range msg.ToolCalls {
				switch {
				case isMistral:
					msg.ToolCalls[i].ID = normalizeMistralToolCallID(msg.ToolCalls[i].ID)
				case isClaude:
					msg.ToolCalls[i].ID = normalizeClaudeToolCallID(msg.ToolCalls[i].ID)
				}
			}
		}
		if msg.Role == "tool" && msg.ToolCallID != "" {
			switch {
			case isMistral:
				msg.ToolCallID = normalizeMistralToolCallID(msg.ToolCallID)
			case isClaude:
				msg.ToolCallID = normalizeClaudeToolCallID(msg.ToolCallID)
			}
		}
		out = append(out, msg)
		if isMistral && msg.Role == "tool" && i+1 < len(messages) && messages[i+1].Role == "user" {
			out = append(out, llm.ModelMessage{Role: "assistant", Content: "Done."})
		}
	}
	return out
}

func normalizeClaudeToolCallID(value string) string {
	out := make([]rune, 0, len(value))
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			out = append(out, r)
			continue
		}
		out = append(out, '_')
	}
	return string(out)
}

func normalizeMistralToolCallID(value string) string {
	out := make([]rune, 0, 9)
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			out = append(out, r)
		}
		if len(out) >= 9 {
			break
		}
	}
	for len(out) < 9 {
		out = append(out, '0')
	}
	return string(out)
}

func expandProviderOptions(model Model, options map[string]interface{}) map[string]interface{} {
	if options == nil {
		return nil
	}
	expanded := map[string]interface{}{}
	key := ProviderOptionsKey(model)
	for k, v := range options {
		if k == key {
			if nested, ok := v.(map[string]interface{}); ok {
				for nk, nv := range nested {
					expanded[nk] = nv
				}
				continue
			}
		}
		expanded[k] = v
	}
	return expanded
}

func responseHeadersFromResponse(resp *http.Response) map[string]string {
	headers := map[string]string{}
	if resp == nil {
		return headers
	}
	for key, values := range resp.Header {
		if len(values) == 0 {
			continue
		}
		headers[strings.ToLower(key)] = values[0]
	}
	return headers
}

func retryableStatus(code int) bool {
	if code == http.StatusTooManyRequests {
		return true
	}
	return code >= 500 && code <= 599
}

func firstString(fallback string, options map[string]interface{}, keys ...string) string {
	value := fallback
	for _, key := range keys {
		if option, ok := options[key]; ok {
			if text, ok := option.(string); ok && text != "" {
				value = text
			}
		}
	}
	return value
}

func toOpenAICompatibleMessages(messages []llm.ModelMessage) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(messages))
	for _, msg := range messages {
		role := msg.Role
		if role == "" {
			role = "user"
		}
		item := map[string]interface{}{
			"role": role,
		}
		switch role {
		case "assistant":
			if msg.Content != nil {
				item["content"] = normalizeContent(msg.Content)
			}
			if msg.Name != "" {
				item["name"] = msg.Name
			}
			if len(msg.ToolCalls) > 0 {
				item["tool_calls"] = toOpenAIToolCalls(msg.ToolCalls)
			}
		case "tool":
			item["tool_call_id"] = msg.ToolCallID
			if msg.Content != nil {
				item["content"] = contentToString(msg.Content)
			}
		case "function":
			if msg.Name != "" {
				item["name"] = msg.Name
			}
			if msg.Content != nil {
				item["content"] = contentToString(msg.Content)
			}
		default:
			if msg.Name != "" {
				item["name"] = msg.Name
			}
			if msg.Content != nil {
				item["content"] = normalizeContent(msg.Content)
			}
		}
		out = append(out, item)
	}
	return out
}

func normalizeContent(content interface{}) interface{} {
	switch typed := content.(type) {
	case string:
		return typed
	case []map[string]interface{}:
		return typed
	case []interface{}:
		return typed
	default:
		if content == nil {
			return ""
		}
		return contentToString(content)
	}
}

func contentToString(content interface{}) string {
	switch typed := content.(type) {
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	case nil:
		return ""
	default:
		return fmt.Sprint(content)
	}
}

func messageContentText(content interface{}) string {
	switch typed := content.(type) {
	case nil:
		return ""
	case string:
		return typed
	case []interface{}:
		var builder strings.Builder
		for _, part := range typed {
			item, ok := part.(map[string]interface{})
			if !ok {
				continue
			}
			if item["type"] == "text" {
				if text, ok := item["text"].(string); ok {
					builder.WriteString(text)
				}
			}
		}
		return builder.String()
	case []map[string]interface{}:
		var builder strings.Builder
		for _, item := range typed {
			if item["type"] == "text" {
				if text, ok := item["text"].(string); ok {
					builder.WriteString(text)
				}
			}
		}
		return builder.String()
	default:
		return contentToString(content)
	}
}

func toOpenAIToolCalls(calls []llm.ToolCall) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(calls))
	for _, call := range calls {
		args, _ := json.Marshal(call.Arguments)
		out = append(out, map[string]interface{}{
			"id":   call.ID,
			"type": "function",
			"function": map[string]interface{}{
				"name":      call.Name,
				"arguments": string(args),
			},
		})
	}
	return out
}

func toOpenAITools(defs []llm.ToolDefinition) []map[string]interface{} {
	tools := make([]map[string]interface{}, 0, len(defs))
	for _, def := range defs {
		props := map[string]interface{}{}
		for key, value := range def.Schema {
			props[key] = toOpenAIToolSchema(value)
		}
		function := map[string]interface{}{
			"name":       def.Name,
			"parameters": map[string]interface{}{"type": "object", "properties": props},
		}
		if def.Description != "" {
			function["description"] = def.Description
		}
		tools = append(tools, map[string]interface{}{
			"type":     "function",
			"function": function,
		})
	}
	return tools
}

func toOpenAIToolSchema(value interface{}) map[string]interface{} {
	switch typed := value.(type) {
	case map[string]interface{}:
		return typed
	case string:
		return map[string]interface{}{"type": typed}
	default:
		return map[string]interface{}{"type": "string"}
	}
}
