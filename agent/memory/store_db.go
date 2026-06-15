package memory

import (
	"context"
	"encoding/json"
	"sort"
	"time"

	"gorm.io/gorm"
)

type MemorySnapshot struct {
	Key       string `gorm:"primaryKey;size:255"`
	Payload   string `gorm:"type:text;not null"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (MemorySnapshot) TableName() string {
	return "memory_snapshots"
}

type DBStore struct {
	db *gorm.DB
}

func NewDBStore(db *gorm.DB) *DBStore {
	return &DBStore{db: db}
}

func (s *DBStore) Load(ctx context.Context, key string) (*Memory, error) {
	if err := s.ensureSchema(); err != nil {
		return nil, err
	}
	var snapshot MemorySnapshot
	if err := s.db.WithContext(ctx).Where("key = ?", key).First(&snapshot).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return &Memory{}, nil
		}
		return nil, err
	}
	var value Memory
	if err := json.Unmarshal([]byte(snapshot.Payload), &value); err != nil {
		return nil, err
	}
	return &value, nil
}

func (s *DBStore) Save(ctx context.Context, key string, value *Memory) error {
	if err := s.ensureSchema(); err != nil {
		return err
	}
	payload, err := json.Marshal(Clone(value))
	if err != nil {
		return err
	}
	return s.db.WithContext(ctx).Save(&MemorySnapshot{Key: key, Payload: string(payload)}).Error
}

func (s *DBStore) Delete(ctx context.Context, key string) error {
	if err := s.ensureSchema(); err != nil {
		return err
	}
	return s.db.WithContext(ctx).Where("key = ?", key).Delete(&MemorySnapshot{}).Error
}

func (s *DBStore) ListKeys(ctx context.Context) ([]string, error) {
	if err := s.ensureSchema(); err != nil {
		return nil, err
	}
	var keys []string
	if err := s.db.WithContext(ctx).Model(&MemorySnapshot{}).Order("key asc").Pluck("key", &keys).Error; err != nil {
		return nil, err
	}
	sort.Strings(keys)
	return keys, nil
}

func (s *DBStore) ensureSchema() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.AutoMigrate(&MemorySnapshot{})
}
