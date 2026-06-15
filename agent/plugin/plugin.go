package plugin

import (
	"matrixops-agent/llm"
	"matrixops-agent/types"
	"sync"
)

// LLMRequestKind 表示LLM请求的类型
type LLMRequestKind string

const (
	LLMRequestKindChat     LLMRequestKind = "chat"
	LLMRequestKindStream   LLMRequestKind = "stream_chat"
	LLMRequestKindGenerate LLMRequestKind = "generate"
)

// LLMRequest 表示一个LLM请求，可以是Chat或Generate类型
type LLMRequest struct {
	Kind     LLMRequestKind
	Chat     *llm.ChatRequest
	Generate *llm.GenerateRequest
}

// Messages 返回请求中的消息列表（如果是Chat类型）
func (r *LLMRequest) Messages() []*llm.ModelMessage {
	if r.Chat != nil {
		return r.Chat.Messages
	}
	return nil
}

type LLMRequestHook func(*LLMRequest) error
type SessionPartHook func(*types.Part) error
type StreamEventHook func(*llm.StreamEvent) error

type Plugin struct {
	Name          string
	OnLLMRequest  LLMRequestHook
	OnSessionPart SessionPartHook
	OnStreamEvent StreamEventHook
}

// Manager 管理插件的注册和执行
type Manager struct {
	mu      sync.RWMutex
	plugins []*Plugin
}

// NewManager 创建一个新的插件管理器
func NewManager() *Manager {
	return &Manager{plugins: []*Plugin{}}
}

// Register 注册一个插件
func (m *Manager) Register(p *Plugin) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.plugins = append(m.plugins, p)
}

// ApplyLLMRequest 对所有已注册的插件应用LLM请求钩子
func (m *Manager) ApplyLLMRequest(req *LLMRequest) error {
	if req == nil {
		return nil
	}
	m.mu.RLock()
	plugins := make([]*Plugin, len(m.plugins))
	copy(plugins, m.plugins)
	m.mu.RUnlock()

	for _, p := range plugins {
		if p.OnLLMRequest == nil {
			continue
		}
		if err := p.OnLLMRequest(req); err != nil {
			return err
		}
	}
	return nil
}

// ApplySessionPart 对所有已注册的插件应用会话Part钩子
func (m *Manager) ApplySessionPart(part *types.Part) error {
	if part == nil {
		return nil
	}
	m.mu.RLock()
	plugins := make([]*Plugin, len(m.plugins))
	copy(plugins, m.plugins)
	m.mu.RUnlock()

	for _, p := range plugins {
		if p.OnSessionPart == nil {
			continue
		}
		if err := p.OnSessionPart(part); err != nil {
			return err
		}
	}
	return nil
}

// ApplyStreamEvent 对所有已注册的插件应用流事件钩子
func (m *Manager) ApplyStreamEvent(event *llm.StreamEvent) error {
	if event == nil {
		return nil
	}
	m.mu.RLock()
	plugins := make([]*Plugin, len(m.plugins))
	copy(plugins, m.plugins)
	m.mu.RUnlock()

	for _, p := range plugins {
		if p.OnStreamEvent == nil {
			continue
		}
		if err := p.OnStreamEvent(event); err != nil {
			return err
		}
	}
	return nil
}
