package database

import (
	"encoding/json"
	"errors"
	"sort"
	"strings"

	"pkgs/db/models"

	"gopkg.in/yaml.v3"
	"gorm.io/gorm"
)

type workerDefinition struct {
	Name         string                 `yaml:"name"`
	Provider     string                 `yaml:"provider"`
	Model        string                 `yaml:"model"`
	Description  string                 `yaml:"description"`
	Mode         string                 `yaml:"mode"`
	Native       bool                   `yaml:"native"`
	Hidden       bool                   `yaml:"hidden"`
	TopP         float64                `yaml:"topP"`
	Temperature  *float64                `yaml:"temperature"`
	Color        string                 `yaml:"color"`
	SystemPrompt string                 `yaml:"systemPrompt"`
	WorkerPrompt string                 `yaml:"workerPrompt"`
	Options      map[string]interface{} `yaml:"options"`
	Steps        int                    `yaml:"steps"`
	EnabledTools []string               `yaml:"enabledTools"`
	EnabledSkills []string              `yaml:"enabledSkills"`
	Occupation   string                 `yaml:"occupation"`
	WorkingDir   string                 `yaml:"workingDir"`
}

// InitBuiltInWorkers 将缺失的内置 Worker 写入 workers 表；已存在的同名 Worker 保持用户当前配置不变。
func InitBuiltInWorkers(db *gorm.DB, model string, provider *models.LLMConfig, files map[string][]byte) error {
	return applyBuiltInWorkers(db, model, provider, files, false)
}

// RestoreBuiltInWorkers 将 YAML 定义写入或更新到 workers 表，用于显式恢复默认 Worker 配置。
func RestoreBuiltInWorkers(db *gorm.DB, model string, provider *models.LLMConfig, files map[string][]byte) error {
	return applyBuiltInWorkers(db, model, provider, files, true)
}

func applyBuiltInWorkers(db *gorm.DB, model string, provider *models.LLMConfig, files map[string][]byte, overwriteExisting bool) error {
	if len(files) == 0 {
		return nil
	}
	filenames := make([]string, 0, len(files))
	for name := range files {
		filenames = append(filenames, name)
	}
	sort.Strings(filenames)
	for _, name := range filenames {
		data := files[name]
		if len(data) == 0 {
			continue
		}
		var def workerDefinition
		if err := yaml.Unmarshal(data, &def); err != nil {
			return err
		}
		if strings.TrimSpace(def.Name) == "" {
			continue
		}
		worker, err := toWorkerDefinition(def, model, provider)
		if err != nil {
			return err
		}
		if err := saveBuiltInWorker(db, worker, overwriteExisting); err != nil {
			return err
		}
	}

	return nil
}

func toWorkerDefinition(def workerDefinition, model string, provider *models.LLMConfig) (models.Worker, error) {
	optionsJSON := ""
	if def.Options != nil {
		if bytes, err := json.Marshal(def.Options); err == nil {
			optionsJSON = string(bytes)
		}
	}

	enabledToolsJSON := models.NormalizeEnabledToolsJSON(def.EnabledTools)
	enabledSkillsJSON := models.NormalizeEnabledSkillsJSON(def.EnabledSkills)

	llmConfigID := provider.ID
	return models.Worker{
		Name:         strings.TrimSpace(def.Name),
		Provider:     provider.Name,
		LLMConfigID:  &llmConfigID,
		Model:        model,
		Description:  def.Description,
		Temperature:  def.Temperature,
		TopP:         def.TopP,
		SystemPrompt: firstNonEmptyTrimmed(def.WorkerPrompt, def.SystemPrompt),
		Mode:         def.Mode,
		Native:       def.Native,
		Hidden:       def.Hidden,
		Color:        def.Color,
		Steps:        def.Steps,
		EnabledTools: enabledToolsJSON,
		EnabledSkills: enabledSkillsJSON,
		Options:      optionsJSON,
		Occupation:   def.Occupation,
		WorkingDir:   def.WorkingDir,
	}, nil
}

func firstNonEmptyTrimmed(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func saveBuiltInWorker(db *gorm.DB, worker models.Worker, overwriteExisting bool) error {
	var existing models.Worker
	if err := db.Where("name = ?", worker.Name).First(&existing).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return db.Create(&worker).Error
		}
		return err
	}
	if !overwriteExisting {
		return nil
	}

	worker.ID = existing.ID
	worker.CreatedAt = existing.CreatedAt
	return db.Model(&existing).Select("*").Updates(worker).Error
}
