package memory

import (
	"context"
	"sort"
	"sync"
)

type InMemoryStore struct {
	mu    sync.RWMutex
	items map[string]*Memory
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{items: map[string]*Memory{}}
}

func (s *InMemoryStore) Load(ctx context.Context, key string) (*Memory, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if value, ok := s.items[key]; ok {
		return Clone(value), nil
	}
	return &Memory{}, nil
}

func (s *InMemoryStore) Save(ctx context.Context, key string, value *Memory) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[key] = Clone(value)
	return nil
}

func (s *InMemoryStore) Delete(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.items, key)
	return nil
}

func (s *InMemoryStore) ListKeys(ctx context.Context) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	keys := make([]string, 0, len(s.items))
	for key := range s.items {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys, nil
}
