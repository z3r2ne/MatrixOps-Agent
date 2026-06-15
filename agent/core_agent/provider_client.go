package coreagent

import (
	providers "matrixops.local/core_agent/providers"
)

type ProviderConfig = providers.ProviderConfig

const DefaultProviderName = providers.DefaultProviderName

type GenericProviderClient struct {
	client providers.Client
}

func NewGenericProviderClient() *GenericProviderClient {
	return &GenericProviderClient{client: providers.MustCreate(DefaultProviderName)}
}

func NewProviderClient(name string) (*GenericProviderClient, error) {
	client, err := providers.Create(name)
	if err != nil {
		return nil, err
	}
	return &GenericProviderClient{client: client}, nil
}

func MustCreateProviderClient(name string) *GenericProviderClient {
	client, err := NewProviderClient(name)
	if err != nil {
		panic(err)
	}
	return client
}

func (c *GenericProviderClient) Chat(request ChatRequest) (ChatResponse, error) {
	response, err := c.client.Chat(toProviderRequest(request))
	if err != nil {
		return ChatResponse{}, err
	}
	return fromProviderResponse(response), nil
}

func (c *GenericProviderClient) StreamChatWithOptions(request ChatRequest, opts ...StreamChatOption) (<-chan StreamEvent, error) {
	options := NewStreamChatOptions(opts...)
	stream, err := c.client.StreamChatWithOptions(toProviderRequest(request), providers.StreamOptions{
		OnRawRequest:  options.OnRawRequest,
		OnRawResponse: options.OnRawResponse,
		HTTPClient:    options.HTTPClient,
		OnRetryError:  options.OnRetryError,
	})
	if err != nil {
		return nil, err
	}
	out := make(chan StreamEvent)
	go func() {
		defer close(out)
		for event := range stream {
			out <- fromProviderStreamEvent(event)
		}
	}()
	return out, nil
}

func ValidateProviderConfig(value any, model string) error {
	return providers.Validate(DefaultProviderName, value, model)
}

func ValidateNamedProviderConfig(name string, value any, model string) error {
	return providers.Validate(name, value, model)
}

func toProviderRequest(request ChatRequest) providers.Request {
	messages := make([]providers.Message, 0, len(request.Messages))
	for _, message := range request.Messages {
		if message == nil {
			continue
		}
		messages = append(messages, providers.Message{
			Role:       message.Role,
			Content:    message.Content,
			Name:       message.Name,
			ToolCallID: message.ToolCallID,
			ToolCalls:  toProviderToolCalls(message.ToolCalls),
		})
	}
	return providers.Request{
		Messages: messages,
		Context:  request.Context,
		Tools:    toProviderToolDefinitions(request.Tools),
		// Temperature:     request.Temperature,
		TopP:            request.TopP,
		MaxOutputTokens: request.MaxOutputTokens,
		ProviderOptions: request.ProviderOptions,
		Model:           request.Model,
		ExtraOptions:    request.ExtraOptions,
	}
}

func toProviderToolDefinitions(defs []ToolDefinition) []providers.ToolDefinition {
	out := make([]providers.ToolDefinition, 0, len(defs))
	for _, def := range defs {
		out = append(out, providers.ToolDefinition{Name: def.Name, Description: def.Description, Schema: def.Schema})
	}
	return out
}

func toProviderToolCalls(calls []ToolCall) []providers.ToolCall {
	out := make([]providers.ToolCall, 0, len(calls))
	for _, call := range calls {
		out = append(out, providers.ToolCall{ID: call.ID, Name: call.Name, Arguments: call.Arguments})
	}
	return out
}

func fromProviderResponse(response providers.Response) ChatResponse {
	return ChatResponse{
		Message: ModelMessage{
			Role:       response.Message.Role,
			Content:    response.Message.Content,
			Name:       response.Message.Name,
			ToolCallID: response.Message.ToolCallID,
			ToolCalls:  fromProviderToolCalls(response.Message.ToolCalls),
		},
		ToolCalls: fromProviderToolCalls(response.ToolCalls),
		Finish:    response.Finish,
		Usage:     fromProviderUsage(response.Usage),
	}
}

func fromProviderToolCalls(calls []providers.ToolCall) []ToolCall {
	out := make([]ToolCall, 0, len(calls))
	for _, call := range calls {
		out = append(out, ToolCall{ID: call.ID, Name: call.Name, Arguments: call.Arguments})
	}
	return out
}

func fromProviderUsage(usage *providers.Usage) *Usage {
	if usage == nil {
		return nil
	}
	return &Usage{InputTokens: usage.InputTokens, OutputTokens: usage.OutputTokens, ReasoningTokens: usage.ReasoningTokens, CachedInputTokens: usage.CachedInputTokens}
}

func fromProviderStreamEvent(event providers.StreamEvent) StreamEvent {
	return StreamEvent{
		Type:          event.Type,
		Text:          event.Text,
		ToolIndex:     event.ToolIndex,
		ToolCallID:    event.ToolCallID,
		ToolName:      event.ToolName,
		ToolArguments: event.ToolArguments,
		Finish:        event.Finish,
		Usage:         fromProviderUsage(event.Usage),
		Error:         event.Error,
	}
}
