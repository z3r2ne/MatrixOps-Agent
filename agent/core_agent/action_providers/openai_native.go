package actionproviders

import (
	coreagent "matrixops.local/core_agent"
)

// OpenAINativeActionProvider implements coreagent.ActionProvider using the
// official openai-go SDK. It corresponds to the stream_openai_native.go
// path: native function/tool_calls are streamed via OpenAI Chat Completions
// or Responses API and converted to ActionOutput values.
type OpenAINativeActionProvider struct{}

// Stream delegates to coreagent.StreamV2OpenAINative, which handles the
// openai-go streaming lifecycle, retry logic, and conversion to ActionOutput.
func (p *OpenAINativeActionProvider) Stream(input coreagent.StreamInput) (*coreagent.StreamOutput, error) {
	return coreagent.StreamV2OpenAINative(input)
}

// OpenAINativeActionProviderOnce is a variant that performs a single native
// OpenAI stream request without retries. It is used for lightweight
// one-shot calls such as session title generation.
type OpenAINativeActionProviderOnce struct{}

// Stream delegates to coreagent.StreamV2OpenAINativeOnce.
func (p *OpenAINativeActionProviderOnce) Stream(input coreagent.StreamInput) (*coreagent.StreamOutput, error) {
	return coreagent.StreamV2OpenAINativeOnce(input)
}
