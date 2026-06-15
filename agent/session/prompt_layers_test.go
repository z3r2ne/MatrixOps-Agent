package session

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"matrixops-agent/taskctx"
)

func TestBuildToolPriorityPromptHighlightsExploreDelegation(t *testing.T) {
	enabledTools := map[string]struct{}{
		"rg":              {},
		"read":            {},
		"run_worker_task": {},
	}

	prompt := buildToolPriorityPrompt(true, enabledTools, []string{"explore", "plan", "verification"})
	if prompt == "" {
		t.Fatal("expected non-empty prompt")
	}
	if !containsAll(prompt,
		"`run_worker_task` → `explore`",
		"`run_worker_task` → `plan`",
		"`run_worker_task` → `verification`",
	) {
		t.Fatalf("prompt missing strengthened delegation guidance:\n%s", prompt)
	}
}

func TestBuildStandardEnvironmentPromptUsesTaskWorkDir(t *testing.T) {
	worktreePath := "/tmp/.MatrixOps/worktrees/demo-feature"
	projectPath := "/tmp/demo-project"
	prompt := buildStandardEnvironmentPrompt(taskctx.Context{
		WorkDir:   worktreePath,
		Worktree:  worktreePath,
		ProjectID: "1",
		VCS:       "git",
	}, filepath.Join(worktreePath, "ai_workspace"), time.Now())

	if !containsAll(prompt,
		"- Primary working directory: "+worktreePath,
		"- ai_workspace directory: "+filepath.Join(worktreePath, "ai_workspace"),
	) {
		t.Fatalf("prompt missing task worktree paths:\n%s", prompt)
	}
	if strings.Contains(prompt, projectPath) {
		t.Fatalf("prompt should not reference original project path %q:\n%s", projectPath, prompt)
	}
}

func TestBuildSessionGuidancePromptHighlightsExploreFirst(t *testing.T) {
	enabledTools := map[string]struct{}{
		"run_worker_task": {},
	}

	prompt := buildSessionGuidancePrompt(true, enabledTools, false, nil, []string{"explore", "frontend_engineer", "plan", "verification"}, "/tmp/ai_workspace")
	if prompt == "" {
		t.Fatal("expected non-empty prompt")
	}
	if !containsAll(prompt,
		"子 worker 用于降低不确定性",
		"`explore`：只读摸排",
	) {
		t.Fatalf("prompt missing session guidance for explore-first flow:\n%s", prompt)
	}
}

func TestAvailableWorkerNamesForPromptIncludesFrontendEngineer(t *testing.T) {
	got := availableWorkerNamesForPrompt([]string{"chat", "frontend_engineer", "explore"})
	if !containsAll(strings.Join(got, ","), "explore", "frontend_engineer") {
		t.Fatalf("expected prompt-visible workers to include frontend_engineer, got %+v", got)
	}
}

func containsAll(input string, parts ...string) bool {
	for _, part := range parts {
		if !strings.Contains(input, part) {
			return false
		}
	}
	return true
}
