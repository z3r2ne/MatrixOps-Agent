package providers

import (
	"fmt"
	"strings"
)

const DefaultProviderName = "generic"

type Client interface {
	Chat(request Request) (Response, error)
	StreamChatWithOptions(request Request, options StreamOptions) (<-chan StreamEvent, error)
}

type Definition struct {
	New            func() Client
	ValidateConfig func(value any, model string) error
}

var registry = map[string]Definition{}

func Register(name string, definition Definition) {
	name = normalizeName(name)
	if name == "" || definition.New == nil {
		return
	}
	registry[name] = definition
}

func Create(name string) (Client, error) {
	name = normalizeName(name)
	definition, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("unknown provider client: %s", name)
	}
	client := definition.New()
	if client == nil {
		return nil, fmt.Errorf("provider client %s returned nil", name)
	}
	return client, nil
}

func MustCreate(name string) Client {
	client, err := Create(name)
	if err != nil {
		panic(err)
	}
	return client
}

func Validate(name string, value any, model string) error {
	name = normalizeName(name)
	definition, ok := registry[name]
	if !ok {
		return fmt.Errorf("unknown provider client: %s", name)
	}
	if definition.ValidateConfig == nil {
		return nil
	}
	return definition.ValidateConfig(value, model)
}

func normalizeName(name string) string {
	trimmed := strings.TrimSpace(strings.ToLower(name))
	if trimmed == "" {
		return DefaultProviderName
	}
	return trimmed
}
