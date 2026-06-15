package provider

import (
	"matrixops-agent/llm"
	"matrixops-agent/types"
)

// MockProvider 模拟 Provider，用于测试
type MockProvider struct {
	Client          *MockClient
	DefaultModelRef types.ModelRef
	Models          map[string]Model
}

// NewMockProvider 创建一个新的 Mock Provider
func NewMockProvider(client *MockClient) *MockProvider {
	if client == nil {
		client = NewMockClient()
	}

	return &MockProvider{
		Client: client,
		DefaultModelRef: types.ModelRef{
			ProviderID: "mock",
			ModelID:    "mock-model",
		},
		Models: map[string]Model{
			"mock/mock-model": {
				ID:         "mock-model",
				ProviderID: "mock",
				API: APIInfo{
					ID:  "mock-model",
					NPM: "",
				},
				Limit: ModelLimit{
					Context: 128000,
					Input:   100000,
					Output:  8000,
				},
			},
			"mock/gpt-4": {
				ID:         "gpt-4",
				ProviderID: "mock",
				API: APIInfo{
					ID:  "gpt-4",
					NPM: "",
				},
				Limit: ModelLimit{
					Context: 128000,
					Input:   100000,
					Output:  8000,
				},
			},
		},
	}
}

// DefaultModel 实现 Provider 接口
func (p *MockProvider) DefaultModel() (types.ModelRef, error) {
	return p.DefaultModelRef, nil
}

// GetModel 实现 Provider 接口
func (p *MockProvider) GetModel(providerID string, modelID string) (Model, error) {
	key := providerID + "/" + modelID
	if model, ok := p.Models[key]; ok {
		return model, nil
	}

	// 返回默认模型
	return p.Models["mock/mock-model"], nil
}

// GetLanguage 实现 Provider 接口
func (p *MockProvider) GetLanguage(model Model) (LanguageModel, error) {
	return nil, nil
}

// Chat 实现 Provider 接口
func (p *MockProvider) Chat(request llm.ChatRequest) (llm.ChatResponse, error) {
	return p.Client.Chat(request)
}

// StreamChat 实现 Provider 接口
func (p *MockProvider) StreamChat(request llm.ChatRequest) (<-chan llm.StreamEvent, error) {
	return p.Client.StreamChat(request)
}

// StreamChatWithOptions 实现 Provider 接口
func (p *MockProvider) StreamChatWithOptions(request llm.ChatRequest, opts ...llm.StreamChatOption) (<-chan llm.StreamEvent, error) {
	return p.Client.StreamChatWithOptions(request, opts...)
}

// AddModel 添加自定义模型
func (p *MockProvider) AddModel(providerID, modelID string, model Model) {
	key := providerID + "/" + modelID
	p.Models[key] = model
}
