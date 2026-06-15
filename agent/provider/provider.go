package provider

import (
	"matrixops-agent/llm"
	"matrixops-agent/types"
	"strings"
)

type ModelRef = types.ModelRef

type APIInfo struct {
	ID  string
	NPM string
}

type ModelLimit struct {
	Context int
	Input   int
	Output  int
}

type ModelCacheCost struct {
	Read  float64
	Write float64
}

type ModelCost struct {
	Input                float64
	Output               float64
	Cache                ModelCacheCost
	ExperimentalOver200K *ModelCost
}

type Model struct {
	ID         string
	ProviderID string
	API        APIInfo
	Limit      ModelLimit
	Cost       *ModelCost
	Options    map[string]interface{}
	Headers    map[string]string
	Variants   map[string]map[string]interface{}
}

type LanguageModel interface{}

type Provider interface {
	DefaultModel() (types.ModelRef, error)
	GetModel(providerID string, modelID string) (Model, error)
	GetLanguage(model Model) (LanguageModel, error)
	Chat(request llm.ChatRequest) (llm.ChatResponse, error)
	StreamChat(request llm.ChatRequest) (<-chan llm.StreamEvent, error)
	StreamChatWithOptions(request llm.ChatRequest, opts ...llm.StreamChatOption) (<-chan llm.StreamEvent, error)
}

func ParseModel(model string) types.ModelRef {
	parts := strings.Split(model, "/")
	if len(parts) == 1 {
		return types.ModelRef{ProviderID: "openai", ModelID: parts[0]}
	}
	return types.ModelRef{
		ProviderID: parts[0],
		ModelID:    strings.Join(parts[1:], "/"),
	}
}
