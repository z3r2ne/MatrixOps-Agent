package semreg_runner

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	database "pkgs/db"
	"pkgs/db/models"
	"pkgs/semreg"
	"pkgs/semreg/testproject"

	"gorm.io/gorm"
)

const BuiltinTestProjectName = "内置测试项目"

type BootstrapResult struct {
	ProjectID string `json:"projectId"`
	WorkDir   string `json:"workDir"`
	Ready     bool   `json:"ready"`
}

func persistentFixturePath(workspaceID uint) (string, error) {
	base, err := database.DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "test-workspaces", strconv.FormatUint(uint64(workspaceID), 10), "fixture"), nil
}

// BootstrapTestWorkspace ensures the built-in test project exists for a test workspace.
func BootstrapTestWorkspace(db *gorm.DB, workspaceID uint) (*BootstrapResult, error) {
	if db == nil || workspaceID == 0 {
		return nil, fmt.Errorf("invalid workspace id")
	}
	workspace, err := database.GetWorkspaceByID(db, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("get workspace: %w", err)
	}
	if workspace.Type != models.WorkspaceTypeTest {
		return nil, fmt.Errorf("workspace %d is not a test workspace", workspaceID)
	}

	fixturePath, err := persistentFixturePath(workspaceID)
	if err != nil {
		return nil, err
	}
	if err := testproject.MaterializeTo(fixturePath); err != nil {
		return nil, fmt.Errorf("materialize fixture: %w", err)
	}

	projects, err := database.GetProjectsByWorkspaceID(db, workspaceID)
	if err != nil {
		return nil, err
	}
	for _, project := range projects {
		if project.Name == BuiltinTestProjectName {
			if project.Path != fixturePath || project.WorktreePath != fixturePath {
				project.Path = fixturePath
				project.WorktreePath = fixturePath
				if err := database.UpdateProject(db, &project); err != nil {
					return nil, err
				}
			}
			return &BootstrapResult{
				ProjectID: project.GetID(),
				WorkDir:   fixturePath,
				Ready:     true,
			}, nil
		}
	}

	project := &models.Project{
		Name:         BuiltinTestProjectName,
		Path:         fixturePath,
		WorktreePath: fixturePath,
		Status:       "active",
	}
	if err := database.CreateProject(db, project); err != nil {
		return nil, fmt.Errorf("create project: %w", err)
	}
	if err := database.AddProjectToWorkspace(db, workspaceID, project.ID); err != nil {
		return nil, fmt.Errorf("link project to workspace: %w", err)
	}
	return &BootstrapResult{
		ProjectID: project.GetID(),
		WorkDir:   fixturePath,
		Ready:     true,
	}, nil
}

func runNeedsL1L2(tiers []string, scenarios []semreg.Scenario, selected map[string]struct{}, tierFilter map[string]struct{}) bool {
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
		if scenario.Tier == semreg.TierL1 || scenario.Tier == semreg.TierL2 {
			return true
		}
	}
	for _, tier := range tiers {
		switch strings.TrimSpace(tier) {
		case "L1", "L2":
			return true
		}
	}
	return false
}

// prepareRunConfig bootstraps the builtin project and materializes a fresh temp workdir for L1/L2 runs.
func prepareRunConfig(db *gorm.DB, cfg *RunConfig, scenarios []semreg.Scenario) (cleanup func(), err error) {
	if cfg == nil || cfg.WorkspaceID == 0 {
		return nil, nil
	}

	selected := map[string]struct{}{}
	for _, id := range cfg.ScenarioIDs {
		id = strings.TrimSpace(id)
		if id != "" {
			selected[id] = struct{}{}
		}
	}
	tierFilter := map[string]struct{}{}
	for _, tier := range cfg.Tiers {
		tier = strings.TrimSpace(tier)
		if tier != "" {
			tierFilter[tier] = struct{}{}
		}
	}
	if !runNeedsL1L2(cfg.Tiers, scenarios, selected, tierFilter) {
		return nil, nil
	}

	boot, err := BootstrapTestWorkspace(db, cfg.WorkspaceID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(cfg.ProjectID) == "" {
		cfg.ProjectID = boot.ProjectID
	}

	tempDir, tempCleanup, err := testproject.MaterializeTemp()
	if err != nil {
		return nil, err
	}
	cfg.WorkDir = tempDir
	return tempCleanup, nil
}
