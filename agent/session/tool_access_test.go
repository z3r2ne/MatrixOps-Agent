package session

import (
	"testing"

	"matrixops-agent/llm"
	agenttool "matrixops-agent/tool"
	"pkgs/db/models"
)

func TestProjectToolAction_exemptMetaToolsDefaultAllow(t *testing.T) {
	project := &models.Project{Name: "demo", YoloMode: false}
	perms := map[string]string{}

	if got := projectToolAction(project, perms, "set_tool_stall_timeout"); got != models.ProjectToolPermissionAllow {
		t.Fatalf("set_tool_stall_timeout = %q, want allow", got)
	}
	if got := projectToolAction(project, perms, "question"); got != models.ProjectToolPermissionAllow {
		t.Fatalf("question = %q, want allow", got)
	}
	if got := projectToolAction(project, perms, "bash"); got != models.ProjectToolPermissionAsk {
		t.Fatalf("bash = %q, want ask", got)
	}
}

func TestProjectToolAction_exemptToolRespectsExplicitDeny(t *testing.T) {
	project := &models.Project{Name: "demo", YoloMode: false}
	perms := map[string]string{
		"set_tool_stall_timeout": models.ProjectToolPermissionDeny,
	}

	if got := projectToolAction(project, perms, "set_tool_stall_timeout"); got != models.ProjectToolPermissionDeny {
		t.Fatalf("explicit deny = %q, want deny", got)
	}
}

func TestAuthorizeToolCall_exemptToolSkipsPrompt(t *testing.T) {
	runner := &AgentRunner{}
	runtimeConfig := &RuntimeConfig{
		Project:                &models.Project{Name: "demo", YoloMode: false},
		ProjectToolPermissions: map[string]string{},
		Worker:                 &models.Worker{Name: "leader"},
	}
	toolInstance := agenttool.SetToolStallTimeoutTool{}
	call := llm.ToolCall{Name: "set_tool_stall_timeout", Arguments: map[string]interface{}{"timeout_seconds": 60}}

	if err := runner.authorizeToolCall(runtimeConfig, runtimeConfig.Worker, call, toolInstance); err != nil {
		t.Fatalf("authorize exempt tool: %v", err)
	}
}
