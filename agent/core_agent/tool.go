package coreagent

import (
	"errors"
	"sort"
	"sync"
)

type ToolResult struct {
	Content        string
	Message        string
	FullContent    string
	IsError        bool
	Truncated      bool
	PreserveFullOutput bool
	OutputPath     string
	Title          string
	Metadata       map[string]interface{}
	MemoryMetadata map[string]interface{}
	Vars           map[string]interface{}
	Name           string
}

type Tool interface {
	Name() string
	Description() string
	Schema() map[string]interface{}
	Execute(ctx ToolContext, input map[string]interface{}) (ToolResult, error)
}

type ToolRegistry struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{tools: map[string]Tool{}}
}

func (r *ToolRegistry) Register(tool Tool) {
	if tool == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.tools == nil {
		r.tools = map[string]Tool{}
	}
	r.tools[tool.Name()] = tool
}

func (r *ToolRegistry) Get(name string) (Tool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tool, ok := r.tools[name]
	if !ok {
		return nil, errors.New("tool not found")
	}
	return tool, nil
}

func (r *ToolRegistry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (r *ToolRegistry) Definitions() []ToolDefinition {
	names := r.Names()
	defs := make([]ToolDefinition, 0, len(names))
	for _, name := range names {
		tool, err := r.Get(name)
		if err != nil {
			continue
		}
		defs = append(defs, ToolDefinition{
			Name:        tool.Name(),
			Description: tool.Description(),
			Schema:      tool.Schema(),
		})
	}
	return defs
}
