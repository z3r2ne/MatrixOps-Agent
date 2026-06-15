package providers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	agentllm "matrixops-agent/llm"
	agentprovider "matrixops-agent/provider"
	"pkgs/db/models"
)

type ProviderConfig struct {
	Name                  string
	Type                  string
	APIKey                string
	Model                 string
	BaseURL               string
	APIType               string
	SystemPromptPlacement string
	Proxy                 string
	MaxRetries            int
}

type ToolDefinition struct {
	Name        string
	Description string
	Schema      map[string]interface{}
}

type ToolCall struct {
	ID        string
	Name      string
	Arguments map[string]interface{}
}

type Message struct {
	Role       string
	Content    interface{}
	Name       string
	ToolCallID string
	ToolCalls  []ToolCall
}

type Request struct {
	Messages        []Message
	Context         context.Context
	Tools           []ToolDefinition
	Temperature     float64
	TopP            float64
	MaxOutputTokens int
	ProviderOptions any
	Model           string
	ExtraOptions    map[string]interface{}
}

type Usage struct {
	InputTokens       int
	OutputTokens      int
	ReasoningTokens   int
	CachedInputTokens int
}

type Response struct {
	Message   Message
	ToolCalls []ToolCall
	Finish    string
	Usage     *Usage
}

type StreamEvent struct {
	Type          string
	Text          string
	ToolIndex     int
	ToolCallID    string
	ToolName      string
	ToolArguments string
	Finish        string
	Usage         *Usage
	Error         error
}

type StreamOptions struct {
	OnRawRequest  func(raw string)
	OnRawResponse func(raw string)
	HTTPClient    *http.Client
	OnRetryError  func(err error, retryAttempt int, maxRetries int, nextDelay time.Duration, attemptDuration time.Duration, rawResponse string)
}

type GenericClient struct {
	client *agentprovider.GenericClient
}

func init() {
	Register(DefaultProviderName, Definition{
		New: func() Client {
			return NewGenericClient()
		},
		ValidateConfig: ValidateProviderConfig,
	})
}

func NewGenericClient() *GenericClient {
	return &GenericClient{client: agentprovider.NewGenericClient()}
}

func (c *GenericClient) Chat(request Request) (Response, error) {
	response, err := c.client.Chat(c.toAgentChatRequest(request))
	if err != nil {
		return Response{}, err
	}
	return responseFromAgent(response), nil
}

func (c *GenericClient) StreamChatWithOptions(request Request, options StreamOptions) (<-chan StreamEvent, error) {
	stream, err := c.client.StreamChatWithOptions(
		c.toAgentChatRequest(request),
		agentllm.WithOnRawRequest(options.OnRawRequest),
		agentllm.WithOnRawResponse(options.OnRawResponse),
		agentllm.WithHTTPClient(options.HTTPClient),
		agentllm.WithOnRetryError(func(err error, retryAttempt int, maxRetries int, nextDelay time.Duration, attemptDuration time.Duration, rawResponse string) {
			if options.OnRetryError != nil {
				options.OnRetryError(err, retryAttempt, maxRetries, nextDelay, attemptDuration, rawResponse)
			}
		}),
	)
	if err != nil {
		return nil, err
	}
	out := make(chan StreamEvent)
	go func() {
		defer close(out)
		for event := range stream {
			out <- streamEventFromAgent(event)
		}
	}()
	return out, nil
}

func (c *GenericClient) toAgentChatRequest(request Request) agentllm.ChatRequest {
	messages := make([]*agentllm.ModelMessage, 0, len(request.Messages))
	for _, message := range request.Messages {
		messages = append(messages, &agentllm.ModelMessage{
			Role:       message.Role,
			Content:    message.Content,
			Name:       message.Name,
			ToolCallID: message.ToolCallID,
			ToolCalls:  toolCallsToAgent(message.ToolCalls),
		})
	}
	return agentllm.ChatRequest{
		Messages:        messages,
		Context:         request.Context,
		Tools:           toolDefinitionsToAgent(request.Tools),
		Temperature:     request.Temperature,
		TopP:            request.TopP,
		MaxOutputTokens: request.MaxOutputTokens,
		ProviderOptions: NormalizeProviderOptions(request.ProviderOptions, request.Model),
		Model:           request.Model,
		ExtraOptions:    request.ExtraOptions,
	}
}

func NormalizeProviderOptions(value any, model string) *models.LLMConfig {
	switch typed := value.(type) {
	case *models.LLMConfig:
		if typed == nil {
			return &models.LLMConfig{Name: "openai", Type: "openai", Model: model, SystemPromptPlacement: models.NormalizeLLMSystemPromptPlacement(""), MaxRetries: 5}
		}
		copy := *typed
		if copy.Model == "" {
			copy.Model = model
		}
		if copy.Name == "" {
			copy.Name = copy.Type
		}
		if copy.Type == "" {
			copy.Type = copy.Name
		}
		copy.APIType = models.NormalizeLLMAPIType(copy.APIType)
		copy.SystemPromptPlacement = models.NormalizeLLMSystemPromptPlacement(copy.SystemPromptPlacement)
		if copy.MaxRetries <= 0 {
			copy.MaxRetries = 5
		}
		return &copy
	case models.LLMConfig:
		copy := typed
		return NormalizeProviderOptions(&copy, model)
	case *ProviderConfig:
		if typed == nil {
			return &models.LLMConfig{Name: "openai", Type: "openai", Model: model, SystemPromptPlacement: models.NormalizeLLMSystemPromptPlacement(""), MaxRetries: 5}
		}
		providerName := typed.Name
		if providerName == "" {
			providerName = typed.Type
		}
		providerType := typed.Type
		if providerType == "" {
			providerType = providerName
		}
		providerModel := typed.Model
		if providerModel == "" {
			providerModel = model
		}
		apiType := models.NormalizeLLMAPIType(typed.APIType)
		systemPromptPlacement := models.NormalizeLLMSystemPromptPlacement(typed.SystemPromptPlacement)
		maxRetries := typed.MaxRetries
		if maxRetries <= 0 {
			maxRetries = 5
		}
		return &models.LLMConfig{Name: providerName, Type: providerType, APIKey: typed.APIKey, Model: providerModel, BaseURL: typed.BaseURL, APIType: apiType, SystemPromptPlacement: systemPromptPlacement, Proxy: typed.Proxy, MaxRetries: maxRetries}
	case ProviderConfig:
		copy := typed
		return NormalizeProviderOptions(&copy, model)
	default:
		return &models.LLMConfig{Name: "openai", Type: "openai", Model: model, SystemPromptPlacement: models.NormalizeLLMSystemPromptPlacement(""), MaxRetries: 5}
	}
}

func ValidateProviderConfig(value any, model string) error {
	config := NormalizeProviderOptions(value, model)
	if config.Model == "" {
		return fmt.Errorf("provider model is required")
	}
	if config.APIKey == "" && config.BaseURL == "" {
		return fmt.Errorf("provider api key or base url is required")
	}
	return nil
}

func toolDefinitionsToAgent(defs []ToolDefinition) []agentllm.ToolDefinition {
	out := make([]agentllm.ToolDefinition, 0, len(defs))
	for _, def := range defs {
		out = append(out, agentllm.ToolDefinition{Name: def.Name, Description: def.Description, Schema: def.Schema})
	}
	return out
}

func toolCallsToAgent(calls []ToolCall) []agentllm.ToolCall {
	out := make([]agentllm.ToolCall, 0, len(calls))
	for _, call := range calls {
		out = append(out, agentllm.ToolCall{ID: call.ID, Name: call.Name, Arguments: call.Arguments})
	}
	return out
}

func toolCallsFromAgent(calls []agentllm.ToolCall) []ToolCall {
	out := make([]ToolCall, 0, len(calls))
	for _, call := range calls {
		out = append(out, ToolCall{ID: call.ID, Name: call.Name, Arguments: call.Arguments})
	}
	return out
}

func usageFromAgent(usage *agentllm.Usage) *Usage {
	if usage == nil {
		return nil
	}
	return &Usage{
		InputTokens:       usage.InputTokens,
		OutputTokens:      usage.OutputTokens,
		ReasoningTokens:   usage.ReasoningTokens,
		CachedInputTokens: usage.CachedInputTokens,
	}
}

func responseFromAgent(response agentllm.ChatResponse) Response {
	return Response{
		Message: Message{
			Role:       response.Message.Role,
			Content:    response.Message.Content,
			Name:       response.Message.Name,
			ToolCallID: response.Message.ToolCallID,
			ToolCalls:  toolCallsFromAgent(response.Message.ToolCalls),
		},
		ToolCalls: toolCallsFromAgent(response.ToolCalls),
		Finish:    response.Finish,
		Usage:     usageFromAgent(response.Usage),
	}
}

func streamEventFromAgent(event agentllm.StreamEvent) StreamEvent {
	return StreamEvent{
		Type:          event.Type,
		Text:          event.Text,
		ToolIndex:     event.ToolIndex,
		ToolCallID:    event.ToolCallID,
		ToolName:      event.ToolName,
		ToolArguments: event.ToolArguments,
		Finish:        event.Finish,
		Usage:         usageFromAgent(event.Usage),
		Error:         event.Error,
	}
}
