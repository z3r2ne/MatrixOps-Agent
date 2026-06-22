package semreg_runner

import "pkgs/semreg"

type ScenarioInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Tier        string `json:"tier"`
	Kind        string `json:"kind"`
	RequiresLLM bool   `json:"requiresLlm"`
	HasBaseline bool   `json:"hasBaseline"`
}

type MetricComparison struct {
	Name       string `json:"name"`
	Actual     int    `json:"actual"`
	Baseline   int    `json:"baseline"`
	MaxAllowed int    `json:"maxAllowed,omitempty"`
	Passed     bool   `json:"passed"`
	Detail     string `json:"detail,omitempty"`
}

type ScenarioResult struct {
	ScenarioID   string                 `json:"scenarioId"`
	Name         string                 `json:"name"`
	Tier         string                 `json:"tier"`
	Kind         string                 `json:"kind"`
	Status       string                 `json:"status"`
	DurationMs   int64                  `json:"durationMs"`
	ActiveTaskID uint                   `json:"activeTaskId,omitempty"`
	TaskIDs      []uint                 `json:"taskIds,omitempty"`
	SessionID    string                 `json:"sessionId,omitempty"`
	Errors       []string               `json:"errors,omitempty"`
	Details      map[string]interface{} `json:"details,omitempty"`
	Metrics      []MetricComparison     `json:"metrics,omitempty"`
	ToolCalls    []semreg.TraceToolCall `json:"toolCalls,omitempty"`
}

type RunProgressEvent struct {
	ScenarioID string
	TaskID     uint
	SessionID  string
	Phase      string
}

type RunSummary struct {
	Total      int   `json:"total"`
	Passed     int   `json:"passed"`
	Failed     int   `json:"failed"`
	Skipped    int   `json:"skipped"`
	Error      int   `json:"error"`
	DurationMs int64 `json:"durationMs"`
}

type RunConfig struct {
	Tiers       []string `json:"tiers"`
	ScenarioIDs []string `json:"scenarioIds,omitempty"`
	WorkDir     string   `json:"workDir,omitempty"`
	WorkspaceID uint     `json:"workspaceId,omitempty"`
	ProjectID   string   `json:"projectId,omitempty"`
	OnProgress  func(RunProgressEvent) `json:"-"`
}

type RunReport struct {
	ID          string           `json:"id"`
	Status      string           `json:"status"`
	StartedAt   string           `json:"startedAt"`
	CompletedAt string           `json:"completedAt,omitempty"`
	Summary     RunSummary       `json:"summary"`
	Results     []ScenarioResult `json:"results"`
	Config      RunConfig        `json:"config"`
}

type EnvironmentStatus struct {
	ScenariosDir string `json:"scenariosDir"`
	BaselinesDir string `json:"baselinesDir"`
	L1L2Ready    bool   `json:"l1l2Ready"`
	WorkDir      string `json:"workDir,omitempty"`
	WorkspaceID  uint   `json:"workspaceId,omitempty"`
	ProjectID    string `json:"projectId,omitempty"`
	Message      string `json:"message,omitempty"`
}
