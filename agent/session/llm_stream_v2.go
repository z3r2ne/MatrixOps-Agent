package session

import (
	"context"
	"io"
	"net/http"
	"time"

	"matrixops-agent/llm"
	"matrixops-agent/plugin"
	coreagent "matrixops.local/core_agent"
	"pkgs/db/models"
)

// StreamInputV2 V2版本的流输入
type StreamInputV2 struct {
	Context         context.Context
	Model           string
	Prompt          string
	Abort           context.Context
	Temperature     float64
	TopP            float64
	MaxOutputTokens int
	ProviderOptions *models.LLMConfig
	PluginManager   *plugin.Manager
	HTTPClient      *http.Client
	OnRawRequest    func(raw string)
	OnRawResponse   func(raw string)
	OnRetryError              func(err error, retryAttempt int, maxRetries int, nextDelay time.Duration, attemptDuration time.Duration, rawResponse string)
	CompatibleControlHandler  coreagent.CompatibleControlHandler
}

// ActionOutput 兼容模式控制类 action（message/answer）。
type ActionOutput = coreagent.ActionOutput

// CallToolRequest 归一化后的工具调用请求。
type CallToolRequest = coreagent.CallToolRequest

// StreamOutputV2 V2版本的流输出
type StreamOutputV2 struct {
	ToolCalls     <-chan *CallToolRequest
	RawTextReader io.Reader
	Wait          func() error
	Usage         *llm.Usage
}

// StreamV2 V2版本的流式处理，支持连续多个 JSON action 输出
func StreamV2(input StreamInputV2, client llm.ChatClient) (*StreamOutputV2, error) {
	coreOutput, err := coreagent.StreamV2(coreagent.StreamInput{
		Context:         input.Context,
		Model:           input.Model,
		Prompt:          input.Prompt,
		Abort:           input.Abort,
		Temperature:     input.Temperature,
		TopP:            input.TopP,
		MaxOutputTokens: input.MaxOutputTokens,
		ProviderOptions: input.ProviderOptions,
		HTTPClient:      input.HTTPClient,
		OnRawRequest:    input.OnRawRequest,
		OnRawResponse:   input.OnRawResponse,
		OnRetryError:              input.OnRetryError,
		CompatibleControlHandler:  input.CompatibleControlHandler,
	}, &coreLLMClientAdapter{
		inner:         client,
		pluginManager: input.PluginManager,
	})
	if err != nil {
		return nil, err
	}

	output := &StreamOutputV2{
		ToolCalls:     coreOutput.ToolCalls,
		RawTextReader: coreOutput.RawTextReader,
		Usage:         coreUsageToLLM(coreOutput.Usage),
	}
	output.Wait = func() error {
		if coreOutput.Wait == nil {
			output.Usage = coreUsageToLLM(coreOutput.Usage)
			return nil
		}
		err := coreOutput.Wait()
		output.Usage = coreUsageToLLM(coreOutput.Usage)
		return err
	}
	return output, nil
}

func coreUsageToLLM(usage *coreagent.Usage) *llm.Usage {
	if usage == nil {
		return nil
	}
	return &llm.Usage{
		InputTokens:       usage.InputTokens,
		OutputTokens:      usage.OutputTokens,
		ReasoningTokens:   usage.ReasoningTokens,
		CachedInputTokens: usage.CachedInputTokens,
	}
}
