package provider

func ProviderOptions(model Model, options map[string]interface{}) map[string]interface{} {
	key := ProviderOptionsKey(model)
	if key == "" {
		key = model.ProviderID
	}
	return map[string]interface{}{key: options}
}

func ProviderOptionsKey(model Model) string {
	key := sdkKey(model.API.NPM)
	if key == "" {
		key = model.ProviderID
	}
	return key
}

func sdkKey(npm string) string {
	switch npm {
	case "@ai-sdk/github-copilot", "@ai-sdk/openai", "@ai-sdk/azure":
		return "openai"
	case "@ai-sdk/amazon-bedrock":
		return "bedrock"
	case "@ai-sdk/anthropic":
		return "anthropic"
	case "@ai-sdk/google-vertex", "@ai-sdk/google":
		return "google"
	case "@ai-sdk/gateway":
		return "gateway"
	case "@openrouter/ai-sdk-provider":
		return "openrouter"
	}
	return ""
}
