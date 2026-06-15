package task_runner

import "github.com/google/uuid"

func NewPromptCacheKey() string {
	key, err := uuid.NewV7()
	if err != nil {
		return uuid.New().String()
	}
	return key.String()
}
