package coreagent

import (
	"matrixops.local/core_agent/action_providers/compatible"
	"matrixops.local/core_agent/action_providers/openai_native"
)

// ActionProvider abstracts the LLM streaming layer that turns a prompt into a
// stream of ActionOutput values. The core_agent package provides built-in
// defaults (compatible and OpenAI-native), and external packages (such as
// action_providers) may register alternative implementations via
// RegisterActionProviderFactory.
type ActionProvider interface {
	Stream(input StreamInput) (*StreamOutput, error)
}

// ActionProviderFactory creates an ActionProvider based on the tool-calling
// mode and the generic ChatClient.
type ActionProviderFactory func(nativeToolCalls bool, client ChatClient) ActionProvider

var defaultActionProviderFactory ActionProviderFactory

// RegisterActionProviderFactory registers a factory used by
// newDefaultActionProvider when cfg.ActionProvider is nil. The last
// registered factory wins. This is typically called from an init() function
// in the action_providers package.
func RegisterActionProviderFactory(f ActionProviderFactory) {
	defaultActionProviderFactory = f
}

// newDefaultActionProvider returns the built-in provider when no factory has
// been registered, otherwise it delegates to the registered factory.
func newDefaultActionProvider(nativeToolCalls bool, client ChatClient) ActionProvider {
	if defaultActionProviderFactory != nil {
		return defaultActionProviderFactory(nativeToolCalls, client)
	}
	if nativeToolCalls {
		return &builtinOpenAINativeProvider{}
	}
	return &builtinCompatibleProvider{client: client}
}

// builtinCompatibleProvider is the default fallback for the compatible
// (non-native) tool-calling path. It wraps the client with ToolPromptAdapter
// to inject tool definitions into the prompt, then delegates to compatible.StreamV2.
type builtinCompatibleProvider struct {
	client ChatClient
}

func (p *builtinCompatibleProvider) Stream(input StreamInput) (*StreamOutput, error) {
	client := ChatClient(&compatible.ToolPromptAdapter{Inner: p.client})
	return compatible.StreamV2(input, client)
}

// builtinOpenAINativeProvider is the default fallback for the native OpenAI
// tool-calling path. It delegates to the openai_native subpackage.
type builtinOpenAINativeProvider struct{}

func (p *builtinOpenAINativeProvider) Stream(input StreamInput) (*StreamOutput, error) {
	return openai_native.StreamV2OpenAINative(input)
}
