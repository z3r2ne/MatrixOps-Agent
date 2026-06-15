package session

import (
	"matrixops-agent/llm"
	"matrixops-agent/provider"
)

func GetMessageTokens(usage *llm.Usage) MessageTokens {
	if usage == nil {
		return MessageTokens{}
	}

	cacheRead := safeInt(usage.CachedInputTokens)
	input := safeInt(usage.InputTokens) - cacheRead
	if input < 0 {
		input = 0
	}

	return MessageTokens{
		Input:     input,
		Output:    safeInt(usage.OutputTokens),
		Reasoning: safeInt(usage.ReasoningTokens),
		Cache: TokenCache{
			Read:  cacheRead,
			Write: 0,
		},
	}
}

func GetUsage(model provider.Model, usage *llm.Usage) (MessageTokens, float64) {
	if usage == nil {
		return MessageTokens{}, 0
	}
	tokens := GetMessageTokens(usage)
	return tokens, calculateCost(tokens, usage, model)
}

func safeInt(value int) int {
	if value < 0 {
		return 0
	}
	return value
}

func calculateCost(tokens MessageTokens, usage *llm.Usage, model provider.Model) float64 {
	if usage == nil || model.Cost == nil {
		return 0
	}
	cost := model.Cost
	if model.Cost.ExperimentalOver200K != nil && tokens.Input+tokens.Cache.Read > 200_000 {
		cost = model.Cost.ExperimentalOver200K
	}
	total := 0.0
	total += float64(tokens.Input) * cost.Input
	total += float64(tokens.Output) * cost.Output
	total += float64(tokens.Cache.Read) * cost.Cache.Read
	total += float64(tokens.Cache.Write) * cost.Cache.Write
	total += float64(tokens.Reasoning) * cost.Output
	result := total / 1_000_000
	if result < 0 || result != result {
		return 0
	}
	return result
}
