package semreg_runner

import (
	"context"
	"strings"
	"sync"
	"time"

	"pkgs/semreg"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Manager struct {
	mu     sync.RWMutex
	runner *Runner
	runs   map[string]*runState
}

type runState struct {
	Report RunReport
	cancel context.CancelFunc
}

func NewManager(runner *Runner) *Manager {
	return &Manager{
		runner: runner,
		runs:   map[string]*runState{},
	}
}

func (m *Manager) GetRun(id string) (RunReport, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	state, ok := m.runs[id]
	if !ok {
		return RunReport{}, false
	}
	return state.Report, true
}

func (m *Manager) patchRun(id string, patch func(*RunReport)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	state, ok := m.runs[id]
	if !ok {
		return
	}
	patch(&state.Report)
}

func (m *Manager) StartRun(db *gorm.DB, cfg RunConfig) (RunReport, error) {
	if m == nil || m.runner == nil {
		return RunReport{}, context.Canceled
	}
	id := uuid.NewString()
	started := time.Now()
	report := RunReport{
		ID:        id,
		Status:    "running",
		StartedAt: started.UTC().Format(time.RFC3339),
		Config:    cfg,
		Results:   []ScenarioResult{},
	}

	m.mu.Lock()
	ctx, cancel := context.WithCancel(context.Background())
	m.runs[id] = &runState{Report: report, cancel: cancel}
	m.mu.Unlock()

	go m.executeRun(ctx, id, db, cfg, started)
	return report, nil
}

func (m *Manager) updateScenarioTaskProgress(runID string, evt RunProgressEvent) {
	m.patchRun(runID, func(report *RunReport) {
		for i := len(report.Results) - 1; i >= 0; i-- {
			item := &report.Results[i]
			if item.ScenarioID != evt.ScenarioID || item.Status != "running" {
				continue
			}
			item.ActiveTaskID = evt.TaskID
			item.TaskIDs = appendUniqueUint(item.TaskIDs, evt.TaskID)
			if evt.SessionID != "" {
				item.SessionID = evt.SessionID
			}
			return
		}
	})
}

func (m *Manager) executeRun(ctx context.Context, id string, db *gorm.DB, cfg RunConfig, started time.Time) {
	runCfg := cfg

	scenarios, err := semreg.LoadScenariosDir(m.runner.scenariosDir)
	if err != nil {
		m.patchRun(id, func(report *RunReport) {
			report.Status = "failed"
			report.CompletedAt = time.Now().UTC().Format(time.RFC3339)
			report.Results = []ScenarioResult{{
				ScenarioID: "load",
				Name:       "加载场景",
				Status:     "error",
				Errors:     []string{err.Error()},
			}}
			report.Summary = SummarizeResults(report.Results, started)
		})
		return
	}

	cleanup, prepErr := prepareRunConfig(db, &runCfg, scenarios)
	if cleanup != nil {
		defer cleanup()
	}
	if prepErr != nil {
		m.patchRun(id, func(report *RunReport) {
			report.Status = "failed"
			report.CompletedAt = time.Now().UTC().Format(time.RFC3339)
			report.Results = []ScenarioResult{{
				ScenarioID: "bootstrap",
				Name:       "准备测试环境",
				Status:     "error",
				Errors:     []string{prepErr.Error()},
			}}
			report.Summary = SummarizeResults(report.Results, started)
		})
		return
	}

	runCfg.OnProgress = func(evt RunProgressEvent) {
		m.updateScenarioTaskProgress(id, evt)
	}

	selected := map[string]struct{}{}
	for _, scenarioID := range runCfg.ScenarioIDs {
		scenarioID = strings.TrimSpace(scenarioID)
		if scenarioID != "" {
			selected[scenarioID] = struct{}{}
		}
	}
	tierFilter := map[string]struct{}{}
	for _, tier := range runCfg.Tiers {
		tier = strings.TrimSpace(tier)
		if tier != "" {
			tierFilter[tier] = struct{}{}
		}
	}
	env := m.runner.EnvironmentStatus(runCfg)

	for _, scenario := range scenarios {
		if len(selected) > 0 {
			if _, ok := selected[scenario.ID]; !ok {
				continue
			}
		}
		if len(tierFilter) > 0 {
			if _, ok := tierFilter[string(scenario.Tier)]; !ok {
				continue
			}
		}

		if ctx.Err() != nil {
			m.appendResult(id, ScenarioResult{
				ScenarioID: scenario.ID,
				Name:       scenario.Name,
				Tier:       string(scenario.Tier),
				Kind:       string(scenario.Kind),
				Status:     "skipped",
				Errors:     []string{"运行已取消"},
			}, started)
			continue
		}

		if (scenario.Tier == semreg.TierL1 || scenario.Tier == semreg.TierL2) && !env.L1L2Ready {
			m.appendResult(id, ScenarioResult{
				ScenarioID: scenario.ID,
				Name:       scenario.Name,
				Tier:       string(scenario.Tier),
				Kind:       string(scenario.Kind),
				Status:     "skipped",
				Errors:     []string{env.Message},
			}, started)
			continue
		}

		pending := ScenarioResult{
			ScenarioID: scenario.ID,
			Name:       scenario.Name,
			Tier:       string(scenario.Tier),
			Kind:       string(scenario.Kind),
			Status:     "running",
		}
		m.appendResult(id, pending, started)

		result := m.runner.runScenario(ctx, db, scenario, runCfg)
		m.replaceLastResult(id, result, started)
	}

	completed := time.Now()
	m.patchRun(id, func(report *RunReport) {
		report.CompletedAt = completed.UTC().Format(time.RFC3339)
		report.Summary = SummarizeResults(report.Results, started)
		if report.Summary.Failed > 0 || report.Summary.Error > 0 {
			report.Status = "failed"
		} else {
			report.Status = "completed"
		}
	})
}

func (m *Manager) appendResult(id string, result ScenarioResult, started time.Time) {
	m.patchRun(id, func(report *RunReport) {
		report.Results = append(report.Results, result)
		report.Summary = SummarizeResults(report.Results, started)
	})
}

func (m *Manager) replaceLastResult(id string, result ScenarioResult, started time.Time) {
	m.patchRun(id, func(report *RunReport) {
		if len(report.Results) == 0 {
			report.Results = []ScenarioResult{result}
		} else {
			report.Results[len(report.Results)-1] = result
		}
		report.Summary = SummarizeResults(report.Results, started)
	})
}

func (m *Manager) CancelRun(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	state, ok := m.runs[id]
	if !ok || state.cancel == nil {
		return false
	}
	state.cancel()
	return true
}
