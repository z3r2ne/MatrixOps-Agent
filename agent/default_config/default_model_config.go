package defaultconfig

import (
	_ "embed"
	"fmt"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

//go:embed default_model_config.yaml
var embeddedDefaultModelConfig []byte

//go:embed gpt_5_model_config.yaml
var embeddedGPT5ModelConfig []byte

//go:embed deepseek_v4_model_config.yaml
var deepseekV4ModelConfig []byte

type DefaultModelConfigApplyMode string

const (
	DefaultModelConfigApplyModeInitIfMissing  DefaultModelConfigApplyMode = "init_if_missing"
	DefaultModelConfigApplyModeForceOverwrite DefaultModelConfigApplyMode = "force_overwrite"
)

type ModelBehavior struct {
	DefaultModelConfigMode DefaultModelConfigApplyMode
}

type DefaultModelSettings struct {
	Name                  string `yaml:"name"`
	ContextLimit          int    `yaml:"contextLimit"`
	OutputLimit           int    `yaml:"outputLimit"`
	Prompt                string `yaml:"prompt"`
	SystemPromptPlacement string `yaml:"systemPromptPlacement"`
	NativeOpenAIToolCalls bool   `yaml:"nativeOpenAIToolCalls"`
	ReasoningEffort       string `yaml:"reasoningEffort"`
	TextVerbosity         string `yaml:"textVerbosity"`
	EnableEncryptedReason *bool  `yaml:"enableEncryptedReasoning"`
	ParallelToolCalls     *bool  `yaml:"parallelToolCalls"`
	EnablePromptCacheKey  *bool  `yaml:"enablePromptCacheKey"`
}

var (
	modelBehaviorMu sync.RWMutex
	modelBehavior   = ModelBehavior{
		DefaultModelConfigMode: DefaultModelConfigApplyModeForceOverwrite,
	}
)

func normalizeDefaultModelConfigApplyMode(mode DefaultModelConfigApplyMode) DefaultModelConfigApplyMode {
	switch strings.TrimSpace(string(mode)) {
	case string(DefaultModelConfigApplyModeForceOverwrite):
		return DefaultModelConfigApplyModeForceOverwrite
	default:
		return DefaultModelConfigApplyModeInitIfMissing
	}
}

func SetModelBehavior(behavior ModelBehavior) {
	modelBehaviorMu.Lock()
	defer modelBehaviorMu.Unlock()
	modelBehavior.DefaultModelConfigMode = normalizeDefaultModelConfigApplyMode(behavior.DefaultModelConfigMode)
}

func SetDefaultModelConfigApplyMode(mode DefaultModelConfigApplyMode) {
	modelBehaviorMu.Lock()
	defer modelBehaviorMu.Unlock()
	modelBehavior.DefaultModelConfigMode = normalizeDefaultModelConfigApplyMode(mode)
}

func GetModelBehavior() ModelBehavior {
	modelBehaviorMu.RLock()
	defer modelBehaviorMu.RUnlock()
	return modelBehavior
}

func GetDefaultModelConfigApplyMode() DefaultModelConfigApplyMode {
	modelBehaviorMu.RLock()
	defer modelBehaviorMu.RUnlock()
	return modelBehavior.DefaultModelConfigMode
}

func LoadDefaultModelSettings() (*DefaultModelSettings, error) {
	return loadModelSettings(embeddedDefaultModelConfig, "default_model_config")
}

func LoadBuiltinModelSettings() ([]*DefaultModelSettings, error) {
	defaultSettings, err := LoadDefaultModelSettings()
	if err != nil {
		return nil, err
	}
	gpt5Settings, err := loadModelSettings(embeddedGPT5ModelConfig, "gpt-5")
	if err != nil {
		return nil, err
	}
	deepseekV4Settings, err := loadModelSettings(deepseekV4ModelConfig, "deepseek-v4")
	if err != nil {
		return nil, err
	}
	return []*DefaultModelSettings{defaultSettings, gpt5Settings, deepseekV4Settings}, nil
}

func loadModelSettings(content []byte, fallbackName string) (*DefaultModelSettings, error) {
	var settings DefaultModelSettings
	if err := yaml.Unmarshal(content, &settings); err != nil {
		return nil, fmt.Errorf("unmarshal model settings %q: %w", fallbackName, err)
	}
	if strings.TrimSpace(settings.Name) == "" {
		settings.Name = fallbackName
	}
	if strings.TrimSpace(settings.SystemPromptPlacement) == "" {
		settings.SystemPromptPlacement = "system"
	}
	return &settings, nil
}
