package coreagent

import (
	"matrixops.local/core_agent/action_providers/anthropic"
	"matrixops.local/core_agent/action_providers/compatible"
	"matrixops.local/core_agent/action_providers/openai_native"
)

// StreamV2 provides backward-compatible access to the compatible action provider.
func StreamV2(input StreamInput, client ChatClient) (*StreamOutput, error) {
	return compatible.StreamV2(input, client)
}

// StreamV2OpenAINative provides backward-compatible access to the OpenAI native action provider.
func StreamV2OpenAINative(input StreamInput) (*StreamOutput, error) {
	return openai_native.StreamV2OpenAINative(input)
}

// StreamV2OpenAINativeOnce provides backward-compatible access to the OpenAI native single-shot provider.
func StreamV2OpenAINativeOnce(input StreamInput) (*StreamOutput, error) {
	return openai_native.StreamV2OpenAINativeOnce(input)
}

// StreamV2Anthropic provides backward-compatible access to the Anthropic Messages API streaming path.
func StreamV2Anthropic(input StreamInput) (*StreamOutput, error) {
	return anthropic.StreamV2Anthropic(input)
}
