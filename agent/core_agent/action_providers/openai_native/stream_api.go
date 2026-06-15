package openai_native

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"matrixops.local/core_agent/action_providers/anthropic"
	providers "matrixops.local/core_agent/providers"
	"pkgs/db/models"

	"matrixops.local/core_agent/streamtypes"
)

func StreamV2OpenAINative(input streamtypes.StreamInput) (*streamtypes.StreamOutput, error) {
	if llm := providers.NormalizeProviderOptions(input.ProviderOptions, input.Model); llm != nil && strings.EqualFold(strings.TrimSpace(llm.Type), "claude") {
		return anthropic.StreamV2Anthropic(input)
	}
	return streamtypes.StreamWithRetries(input, streamV2OpenAINativeOnce)
}

// StreamV2OpenAINativeOnce 执行单次原生 OpenAI 流式请求，不含 streamWithRetries 重试（与 StreamV2OpenAINative 相对）。
func StreamV2OpenAINativeOnce(input streamtypes.StreamInput) (*streamtypes.StreamOutput, error) {
	if llm := providers.NormalizeProviderOptions(input.ProviderOptions, input.Model); llm != nil && strings.EqualFold(strings.TrimSpace(llm.Type), "claude") {
		return anthropic.StreamV2AnthropicOnce(input)
	}
	return streamV2OpenAINativeOnce(input)
}
func streamV2OpenAINativeOnce(input streamtypes.StreamInput) (*streamtypes.StreamOutput, error) {
	if input.Abort != nil {
		select {
		case <-input.Abort.Done():
			return nil, input.Abort.Err()
		default:
		}
	}

	llm := providers.NormalizeProviderOptions(input.ProviderOptions, input.Model)
	if llm == nil {
		return nil, fmt.Errorf("openai native tools: provider options missing (need *models.LLMConfig or compatible)")
	}
	apiKey := strings.TrimSpace(llm.APIKey)
	if apiKey == "" {
		return nil, fmt.Errorf("openai native tools: API key is empty")
	}
	model := strings.TrimSpace(input.Model)
	if model == "" {
		model = strings.TrimSpace(llm.Model)
	}
	if model == "" {
		return nil, fmt.Errorf("openai native tools: model is empty")
	}

	ctx := input.Context
	if ctx == nil {
		ctx = context.Background()
	}

	transportState, opts, err := buildOpenAINativeRequestOptions(input, llm)
	if err != nil {
		return nil, err
	}

	apiType := models.NormalizeLLMAPIType(llm.APIType)
	if apiType == models.LLMAPITypeChat {
		return streamV2OpenAIChatCompletionsOnce(input, llm, model, ctx, opts, transportState)
	}
	return streamV2OpenAIResponsesOnce(input, llm, model, ctx, opts, transportState)
}
func streamOutputOpenAINativeWaitError(err error) (*streamtypes.StreamOutput, error) {
	toolCalls := make(chan *streamtypes.CallToolRequest)
	close(toolCalls)
	out := &streamtypes.StreamOutput{
		ToolCalls: toolCalls,
		RawTextReader: bytes.NewReader(nil),
		ReasonReader:  bytes.NewReader(nil),
		Wait:          func() error { return err },
	}
	return out, nil
}
