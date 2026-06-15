package session

import (
	"errors"
	"fmt"
	"strings"

	coreagent "matrixops.local/core_agent"
	database "pkgs/db"
	"pkgs/db/models"

	"gorm.io/gorm"
)

// MemoryCompactionRuntime holds LLM settings dedicated to memory compaction.
type MemoryCompactionRuntime struct {
	Worker        *models.Worker
	LLMConfig     *models.LLMConfig
	ModelSettings *models.ModelSettings
}

// ResolveMemoryCompactionRuntime loads the dedicated compaction worker and its LLM settings.
func ResolveMemoryCompactionRuntime(db *gorm.DB) (*MemoryCompactionRuntime, error) {
	worker, err := resolveMemoryCompactionWorkerRecord(db)
	if err != nil {
		return nil, err
	}

	llmConfig, err := resolveWorkerLLMConfig(db, worker)
	if err != nil {
		return nil, err
	}

	modelSettings, err := database.GetModelSettingsForWorker(db, worker)
	if err != nil {
		modelSettings = &models.ModelSettings{Name: database.DefaultModelSettingsName}
	}

	return &MemoryCompactionRuntime{
		Worker:        worker,
		LLMConfig:     llmConfig,
		ModelSettings: modelSettings,
	}, nil
}

func (runtime *MemoryCompactionRuntime) WorkerExtraPrompt() string {
	if runtime == nil || runtime.Worker == nil {
		return ""
	}
	return strings.TrimSpace(runtime.Worker.SystemPrompt)
}

func (runtime *MemoryCompactionRuntime) ModelName() string {
	if runtime == nil || runtime.Worker == nil {
		return ""
	}
	return strings.TrimSpace(runtime.Worker.Model)
}

// SystemPromptPlacement 返回 compaction worker 关联的 system/instruction/user_input 放置策略。
// 优先 model_settings，其次 llm_config。
func (runtime *MemoryCompactionRuntime) SystemPromptPlacement() string {
	if runtime == nil {
		return coreagent.SystemPromptPlacementUserInput
	}
	if runtime.ModelSettings != nil {
		if placement := strings.TrimSpace(runtime.ModelSettings.SystemPromptPlacement); placement != "" {
			return coreagent.NormalizeSystemPromptPlacement(placement)
		}
	}
	if runtime.LLMConfig != nil {
		if placement := strings.TrimSpace(runtime.LLMConfig.SystemPromptPlacement); placement != "" {
			return coreagent.NormalizeSystemPromptPlacement(placement)
		}
	}
	return coreagent.SystemPromptPlacementUserInput
}

func resolveMemoryCompactionWorkerRecord(db *gorm.DB) (*models.Worker, error) {
	if db == nil {
		return nil, fmt.Errorf("database is not configured")
	}

	worker, err := database.GetWorkerByName(db, models.WorkerCompaction)
	if err == nil && worker != nil {
		return worker, nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	return nil, fmt.Errorf("compaction worker %q not found; restore default workers or create it in settings", models.WorkerCompaction)
}

func resolveWorkerLLMConfig(db *gorm.DB, worker *models.Worker) (*models.LLMConfig, error) {
	if worker != nil && worker.LLMConfigID != nil {
		if llmConfig, err := database.GetLLMConfigByID(db, *worker.LLMConfigID); err == nil {
			return llmConfig, nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}

	if llmConfig, err := database.GetDefaultLLMConfig(db); err == nil {
		return llmConfig, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	return nil, fmt.Errorf("no llm config available for compaction worker")
}
