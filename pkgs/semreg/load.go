package semreg

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// LoadScenario 从单个 YAML 文件加载场景。
func LoadScenario(path string) (*Scenario, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var scenario Scenario
	if err := yaml.Unmarshal(data, &scenario); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if err := validateScenario(&scenario, path); err != nil {
		return nil, err
	}
	return &scenario, nil
}

// LoadScenariosDir 加载目录下全部 .yaml / .yml 场景。
func LoadScenariosDir(dir string) ([]Scenario, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var scenarios []Scenario
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}
		scenario, loadErr := LoadScenario(filepath.Join(dir, name))
		if loadErr != nil {
			return nil, loadErr
		}
		scenarios = append(scenarios, *scenario)
	}
	return scenarios, nil
}

// FilterScenarios 按 tier 过滤场景。
func FilterScenarios(scenarios []Scenario, tier Tier) []Scenario {
	if tier == "" {
		return scenarios
	}
	out := make([]Scenario, 0, len(scenarios))
	for _, scenario := range scenarios {
		if scenario.Tier == tier {
			out = append(out, scenario)
		}
	}
	return out
}

func validateScenario(scenario *Scenario, path string) error {
	if scenario == nil {
		return fmt.Errorf("%s: scenario is nil", path)
	}
	if strings.TrimSpace(scenario.ID) == "" {
		return fmt.Errorf("%s: id is required", path)
	}
	if scenario.Tier == "" {
		scenario.Tier = TierL0
	}
	if scenario.Kind == "" {
		return fmt.Errorf("%s: kind is required", path)
	}
	switch scenario.Kind {
	case KindPromptRender:
		if scenario.PromptRender == nil {
			return fmt.Errorf("%s: prompt_render is required for kind prompt_render", path)
		}
	case KindTaskRunner:
		if scenario.TaskRunner == nil {
			return fmt.Errorf("%s: task_runner is required for kind task_runner", path)
		}
	default:
		return fmt.Errorf("%s: unsupported kind %q", path, scenario.Kind)
	}
	return nil
}
