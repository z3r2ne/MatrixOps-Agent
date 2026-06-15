package tool

import (
	"errors"
	"sort"
	"sync"
)

type Tool interface {
	Name() string
	VerbosName() string
	Description() string
	Execute(ctx Context, input map[string]interface{}) (Result, error)
	Schema() map[string]interface{}
}

type Registry struct {
	mu    sync.RWMutex
	tools map[string]Tool
	vars  map[string]interface{}
}

func NewRegistry() *Registry {
	return &Registry{
		tools: map[string]Tool{},
		vars:  map[string]interface{}{},
	}
}

func (r *Registry) SetVar(key string, value interface{}) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.vars[key] = value
}

func (r *Registry) GetVar(key string) interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.vars[key]
}

func (r *Registry) GetStringVar(key string) string {
	if !r.HasVar(key) {
		return ""
	}
	res, ok := r.GetVar(key).(string)
	if !ok {
		return ""
	}
	return res
}

func (r *Registry) GetBooleanVar(key string) bool {
	if !r.HasVar(key) {
		return false
	}
	res, ok := r.GetVar(key).(bool)
	if !ok {
		return false
	}
	return res
}

func (r *Registry) HasVar(key string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.vars[key]
	return ok
}

func (r *Registry) DeleteVar(key string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.vars, key)
}

func (r *Registry) Register(tool Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.tools == nil {
		r.tools = map[string]Tool{}
	}
	r.tools[tool.Name()] = tool
}

func (r *Registry) Get(name string) (Tool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tool, ok := r.tools[name]
	if !ok {
		return nil, errors.New("tool not found")
	}
	return tool, nil
}

func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
