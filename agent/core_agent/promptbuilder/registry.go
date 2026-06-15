package promptbuilder

import (
	"fmt"
	"sync"
)

type Factory func(params map[string]interface{}) Builder

var (
	registryMu sync.RWMutex
	registry   = map[string]Factory{}
)

func Register(name string, factory Factory) {
	if name == "" || factory == nil {
		return
	}
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[name] = factory
}

func Create(name string, params map[string]interface{}) (Builder, error) {
	registryMu.RLock()
	factory, ok := registry[name]
	registryMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown prompt builder: %s", name)
	}
	return factory(params), nil
}

func MustCreate(name string, params map[string]interface{}) Builder {
	builder, err := Create(name, params)
	if err != nil {
		panic(err)
	}
	return builder
}
