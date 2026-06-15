package tool

import (
	"testing"

	"pkgs/db/models"
	mcppkg "pkgs/mcp"
)

func TestIsToolVisibleBuiltinWhitelistSkipsMcp(t *testing.T) {
	opts := VisibleCatalogOptions{
		HasWorkerEnabledTools: true,
		WorkerEnabledTools: map[string]struct{}{
			"read": {},
		},
	}

	if !isToolVisible("read", opts) {
		t.Fatal("expected builtin whitelisted tool to be visible")
	}
	if isToolVisible("write", opts) {
		t.Fatal("expected non-whitelisted builtin tool to be hidden")
	}
	if !isToolVisible(mcppkg.BuildToolFullName("filesystem", "read_file"), opts) {
		t.Fatal("expected MCP tool to bypass worker whitelist")
	}
}

func TestIsToolVisibleProjectDeny(t *testing.T) {
	opts := VisibleCatalogOptions{
		ProjectToolPermissions: map[string]string{
			"bash": models.ProjectToolPermissionDeny,
		},
	}
	if isToolVisible("bash", opts) {
		t.Fatal("expected denied tool to be hidden")
	}
	if !isToolVisible(mcppkg.BuildToolFullName("demo", "tool"), opts) {
		t.Fatal("expected MCP tool to remain visible when not denied")
	}
}

func TestCatalogForWorkerUIIncludesRunWorkerTask(t *testing.T) {
	names := map[string]struct{}{}
	for _, info := range CatalogForWorkerUI(nil) {
		names[info.Name] = struct{}{}
	}
	if _, ok := names["run_worker_task"]; !ok {
		t.Fatal("expected run_worker_task in CatalogForWorkerUI")
	}
	if _, ok := names["publish_task"]; ok {
		t.Fatal("expected publish_task removed from CatalogForWorkerUI")
	}
}
