package session

import (
	"testing"

	"matrixops-agent/tool"
)

type stubTool struct {
	name        string
	description string
}

func (s stubTool) Name() string        { return s.name }
func (s stubTool) VerbosName() string  { return s.name }
func (s stubTool) Description() string { return s.description }
func (s stubTool) Execute(ctx tool.Context, input map[string]interface{}) (tool.Result, error) {
	return tool.Result{}, nil
}
func (s stubTool) Schema() map[string]interface{} { return map[string]interface{}{"type": "object"} }

func TestResolveToolsUsesToolDescriptionsAndStablePriority(t *testing.T) {
	registry := tool.NewRegistry()
	registry.Register(stubTool{name: "rg", description: "search file content"})
	registry.Register(stubTool{name: "run_worker_task", description: "delegate broad repo exploration"})
	registry.Register(stubTool{name: "read", description: "read exact file"})

	defs := resolveTools(nil, registry)
	if len(defs) != 3 {
		t.Fatalf("expected 3 tool defs, got %d", len(defs))
	}
	if defs[0].Name != "run_worker_task" {
		t.Fatalf("expected run_worker_task first, got %s", defs[0].Name)
	}
	if defs[0].Description != "delegate broad repo exploration" {
		t.Fatalf("expected rich description, got %q", defs[0].Description)
	}
	if defs[1].Name != "read" || defs[2].Name != "rg" {
		t.Fatalf("unexpected prioritized order: %+v", defs)
	}
}
