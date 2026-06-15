package embedding

import (
	"context"
	"fmt"

	"pkgs/db/models"
	"pkgs/embedding/llamacpp"

	"gorm.io/gorm"
)

func NewClientFromConfig(config models.EmbeddingConfig) (Client, error) {
	switch models.NormalizeEmbeddingConfigType(config.Type) {
	case models.EmbeddingConfigTypeLlamaCpp:
		return llamacpp.NewClient(config), nil
	default:
		return nil, fmt.Errorf("unsupported embedding config type: %s", config.Type)
	}
}

func GetActiveClient(db *gorm.DB) (Client, *models.EmbeddingConfig, error) {
	if db == nil {
		return nil, nil, fmt.Errorf("database is required")
	}
	config, err := GetActiveEmbeddingConfig(db)
	if err != nil {
		return nil, nil, err
	}
	client, err := NewClientFromConfig(*config)
	if err != nil {
		return nil, nil, err
	}
	return client, config, nil
}

func TestConfig(ctx context.Context, config models.EmbeddingConfig, sample string) (*TestResult, error) {
	dimension, vector, err := llamacpp.Test(ctx, config, sample)
	if err != nil {
		return nil, err
	}
	return &TestResult{
		Dimension: dimension,
		Vector:    vector,
		Sample:    sample,
	}, nil
}
