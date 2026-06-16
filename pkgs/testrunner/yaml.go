package testrunner

import (
	"fmt"
	"strings"

	"matrixops-agent/types"
	"pkgs/db/models"
	"pkgs/semreg"
)

// LoadScenariosFromDir 从 YAML 目录加载 L2 semantic 场景并合并内置场景。
func LoadScenariosFromDir(dir string) (map[string]TestScenario, error) {
	merged := make(map[string]TestScenario, len(Scenarios))
	for id, scenario := range Scenarios {
		merged[id] = scenario
	}

	items, err := semreg.LoadScenariosDir(dir)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if item.Kind != semreg.KindSemantic {
			continue
		}
		scenario, convErr := TestScenarioFromSemantic(item)
		if convErr != nil {
			return nil, fmt.Errorf("scenario %s: %w", item.ID, convErr)
		}
		merged[scenario.ID] = scenario
	}
	return merged, nil
}

// TestScenarioFromSemantic 将 YAML semantic 场景转为 testrunner 场景。
func TestScenarioFromSemantic(item semreg.Scenario) (TestScenario, error) {
	spec := item.Semantic
	if spec == nil {
		return TestScenario{}, fmt.Errorf("semantic spec is nil")
	}
	if reuse := strings.TrimSpace(spec.ReuseScenario); reuse != "" {
		base, ok := Scenarios[reuse]
		if !ok {
			return TestScenario{}, fmt.Errorf("unknown reuse_scenario %q", reuse)
		}
		out := base
		out.ID = item.ID
		if strings.TrimSpace(item.Name) != "" {
			out.Name = item.Name
		}
		if strings.TrimSpace(item.Description) != "" {
			out.Description = item.Description
		}
		if strings.TrimSpace(spec.TaskInput) != "" {
			out.TaskInput = spec.TaskInput
		}
		if strings.TrimSpace(spec.VerifyPrompt) != "" {
			verifyPrompt := spec.VerifyPrompt
			out.BuildVerifyInput = func(task *models.Task, memoryEntries []*types.MemoryEntry) string {
				return strings.ReplaceAll(verifyPrompt, "{{memory}}", formatMemoryForVerification(memoryEntries))
			}
		}
		return out, nil
	}

	taskInput := strings.TrimSpace(spec.TaskInput)
	if taskInput == "" {
		return TestScenario{}, fmt.Errorf("task_input is required when reuse_scenario is empty")
	}
	verifyPrompt := strings.TrimSpace(spec.VerifyPrompt)
	if verifyPrompt == "" {
		verifyPrompt = "请根据执行记录判断任务是否完成。全部满足回答 PASS，否则 FAIL。\n\n执行记录：\n{{memory}}"
	}
	return TestScenario{
		ID:          item.ID,
		Name:        item.Name,
		Description: item.Description,
		TaskInput:   taskInput,
		BuildVerifyInput: func(task *models.Task, memoryEntries []*types.MemoryEntry) string {
			return strings.ReplaceAll(verifyPrompt, "{{memory}}", formatMemoryForVerification(memoryEntries))
		},
	}, nil
}

// ResolveScenario 按 ID 查找场景，优先 YAML 目录覆盖内置场景。
func ResolveScenario(dir, id string) (TestScenario, bool, error) {
	if dir != "" {
		all, err := LoadScenariosFromDir(dir)
		if err != nil {
			return TestScenario{}, false, err
		}
		if scenario, ok := all[id]; ok {
			return scenario, true, nil
		}
	}
	scenario, ok := Scenarios[id]
	return scenario, ok, nil
}
