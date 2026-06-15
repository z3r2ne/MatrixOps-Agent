package actionproviders

import (
	coreagent "matrixops.local/core_agent"
	"matrixops.local/core_agent/action_providers/compatible"
)

// CompatibleActionProvider implements coreagent.ActionProvider using the
// generic ChatClient. The model outputs JSON tool envelopes in its text
// stream, which are parsed into ActionOutput values.
//
// Tool definitions are injected into the system prompt via ToolPromptAdapter,
// enabling tool calling for any LLM API regardless of native tools support.
type CompatibleActionProvider struct {
	Client coreagent.ChatClient
}

// Stream delegates to coreagent.StreamV2 with the client wrapped by
// ToolPromptAdapter to inject tool definitions into the prompt.
func (p *CompatibleActionProvider) Stream(input coreagent.StreamInput) (*coreagent.StreamOutput, error) {
	client := coreagent.ChatClient(&compatible.ToolPromptAdapter{Inner: p.Client})
	return coreagent.StreamV2(input, client)
}
