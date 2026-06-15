package memory

import "context"

// Store persists memories by key. A key is typically a session ID.
type Store interface {
	Load(ctx context.Context, key string) (*Memory, error)
	Save(ctx context.Context, key string, value *Memory) error
	Delete(ctx context.Context, key string) error
	ListKeys(ctx context.Context) ([]string, error)
}
