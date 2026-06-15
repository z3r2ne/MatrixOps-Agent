package worker

import (
	"strings"
	"testing"

	"pkgs/db/models"
)

func TestBuildCommandUsesSelectedShell(t *testing.T) {
	worker := models.Worker{Provider: "codex"}
	cmd, args, err := BuildCommand(worker, map[string]interface{}{
		"shell": "sh",
	})
	if err != nil {
		t.Fatalf("BuildCommand: %v", err)
	}

	if cmd == "" {
		t.Fatal("expected command name")
	}
	if len(args) != 2 {
		t.Fatalf("expected shell args, got %#v", args)
	}
	if args[0] != "-lc" && args[0] != "/C" {
		t.Fatalf("unexpected shell flag: %#v", args)
	}
	if !strings.Contains(args[1], "codex") && !strings.Contains(args[1], "@openai/codex") {
		t.Fatalf("unexpected wrapped command: %q", args[1])
	}
}

func TestBuildCommandAppendsCursorFlagsAndAdditionalParams(t *testing.T) {
	worker := models.Worker{Provider: "cursor"}
	cmd, args, err := BuildCommand(worker, map[string]interface{}{
		"base_command_override": "cursor-agent",
		"shell":                 "sh",
		"force":                 true,
		"model":                 "gpt-5.4",
		"additional_params":     []interface{}{"--verbose"},
	})
	if err != nil {
		t.Fatalf("BuildCommand: %v", err)
	}
	if cmd == "" || len(args) != 2 {
		t.Fatalf("unexpected shell command: %q %#v", cmd, args)
	}
	if !strings.Contains(args[1], "cursor-agent") {
		t.Fatalf("missing base command: %q", args[1])
	}
	if !strings.Contains(args[1], "--force") || !strings.Contains(args[1], "--model") || !strings.Contains(args[1], "--verbose") {
		t.Fatalf("missing expected params: %q", args[1])
	}
}
