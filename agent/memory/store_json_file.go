package memory

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

type JSONFileStore struct {
	mu   sync.Mutex
	path string
}

func NewJSONFileStore(path string) *JSONFileStore {
	return &JSONFileStore{path: path}
}

func (s *JSONFileStore) Load(ctx context.Context, key string) (*Memory, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items, err := s.readAll()
	if err != nil {
		return nil, err
	}
	if value, ok := items[key]; ok {
		return Clone(value), nil
	}
	return &Memory{}, nil
}

func (s *JSONFileStore) Save(ctx context.Context, key string, value *Memory) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	items, err := s.readAll()
	if err != nil {
		return err
	}
	items[key] = Clone(value)
	return s.writeAll(items)
}

func (s *JSONFileStore) Delete(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	items, err := s.readAll()
	if err != nil {
		return err
	}
	delete(items, key)
	return s.writeAll(items)
}

func (s *JSONFileStore) ListKeys(ctx context.Context) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items, err := s.readAll()
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(items))
	for key := range items {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys, nil
}

func (s *JSONFileStore) readAll() (map[string]*Memory, error) {
	if s.path == "" {
		return map[string]*Memory{}, nil
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]*Memory{}, nil
		}
		return nil, err
	}
	if len(data) == 0 {
		return map[string]*Memory{}, nil
	}
	items := map[string]*Memory{}
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *JSONFileStore) writeAll(items map[string]*Memory) error {
	if s.path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o644)
}
