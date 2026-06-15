package actionproviders

import (
	coreagent "matrixops.local/core_agent"
)

func init() {
	coreagent.RegisterActionProviderFactory(func(nativeToolCalls bool, client coreagent.ChatClient) coreagent.ActionProvider {
		if nativeToolCalls {
			return &OpenAINativeActionProvider{}
		}
		return &CompatibleActionProvider{Client: client}
	})
}

// New creates the appropriate ActionProvider based on the tool-calling mode.
//
// When nativeToolCalls is true, it returns OpenAINativeActionProvider,
// which uses the official openai-go SDK for OpenAI models and the
// anthropic-sdk-go Messages API when ProviderOptions type is claude;
// ProviderOptions must carry an API key.
//
// When nativeToolCalls is false (the default), it returns
// CompatibleActionProvider, which uses the generic ChatClient and parses
// JSON tool envelopes from the model text stream.
func New(nativeToolCalls bool, client coreagent.ChatClient) coreagent.ActionProvider {
	if nativeToolCalls {
		return &OpenAINativeActionProvider{}
	}
	return &CompatibleActionProvider{Client: client}
}
