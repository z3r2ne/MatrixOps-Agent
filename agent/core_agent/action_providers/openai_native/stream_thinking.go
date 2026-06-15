package openai_native

import "pkgs/db/models"
func thinkingRequestExtra(override string) map[string]any {
	switch models.NormalizeLLMThinkingType(override) {
	case models.LLMThinkingTypeEnabled:
		return map[string]any{"thinking": map[string]any{"type": models.LLMThinkingTypeEnabled}}
	case models.LLMThinkingTypeDisabled:
		return map[string]any{"thinking": map[string]any{"type": models.LLMThinkingTypeDisabled}}
	default:
		return nil
	}
}

// mergeNativeOpenAIThinkingExtras 合并 thinking.type 与顶层 enable_thinking（二者独立）；均无配置时返回 nil。
func mergeNativeOpenAIThinkingExtras(thinkingType string, enableThinking *bool) map[string]any {
	out := map[string]any{}
	if extra := thinkingRequestExtra(thinkingType); extra != nil {
		for k, v := range extra {
			out[k] = v
		}
	}
	if enableThinking != nil {
		out["enable_thinking"] = *enableThinking
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
