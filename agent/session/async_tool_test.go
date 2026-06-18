package session

import (
	"strings"
	"testing"
	"time"

	"matrixops-agent/tool"
	coreagent "matrixops.local/core_agent"
)

func TestParseAsyncFlagStripsArgument(t *testing.T) {
	async, stripped := tool.ParseAsyncFlag(map[string]interface{}{
		"async":   true,
		"command": "echo hi",
	})
	if !async {
		t.Fatal("expected async=true")
	}
	if _, ok := stripped["async"]; ok {
		t.Fatal("expected async stripped from params")
	}
	if stripped["command"] != "echo hi" {
		t.Fatalf("unexpected stripped params: %#v", stripped)
	}
}

func TestIsAsyncEligibleTool(t *testing.T) {
	if !isAsyncEligibleTool("run_worker_task") {
		t.Fatal("run_worker_task should be async eligible")
	}
	if !isAsyncEligibleTool("read") {
		t.Fatal("builtin read should be async eligible")
	}
	if isAsyncEligibleTool("mcp_server__search") {
		t.Fatal("mcp tools should not be async eligible")
	}
}

func TestFormatAsyncToolResultMessage(t *testing.T) {
	message := formatAsyncToolResultMessage(
		"bash",
		map[string]interface{}{"command": "echo ok"},
		1500*time.Millisecond,
		"completed",
		coreagent.ToolResult{Name: "bash", Content: "ok"},
		nil,
	)
	if !strings.Contains(message, "<async_tool_result>") {
		t.Fatalf("expected async_tool_result wrapper, got %q", message)
	}
	if !strings.Contains(message, "bash") || !strings.Contains(message, "completed") {
		t.Fatalf("expected tool metadata in message, got %q", message)
	}
}

func TestExecuteSessionToolCallWithAsync_RejectsMCP(t *testing.T) {
	runner := &AgentRunner{}
	result, err := runner.executeSessionToolCallWithAsync(
		nil,
		nil,
		stubCoreTool{name: "mcp_demo__tool"},
		coreagent.ToolCall{
			Name: "mcp_demo__tool",
			Arguments: map[string]interface{}{
				"async": true,
				"q":     "x",
			},
		},
		coreagent.ToolContext{},
		func(call coreagent.ToolCall, ctx coreagent.ToolContext) (coreagent.ToolResult, error) {
			t.Fatal("sync execute should not run for rejected async mcp tool")
			return coreagent.ToolResult{}, nil
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatalf("expected error result, got %#v", result)
	}
	if !strings.Contains(result.Content, "does not support async") {
		t.Fatalf("unexpected content: %q", result.Content)
	}
}

type stubCoreTool struct {
	name string
}

func (s stubCoreTool) Name() string        { return s.name }
func (s stubCoreTool) Description() string { return s.name }
func (s stubCoreTool) Schema() map[string]interface{} {
	return map[string]interface{}{"type": "object"}
}
func (s stubCoreTool) Execute(ctx coreagent.ToolContext, input map[string]interface{}) (coreagent.ToolResult, error) {
	return coreagent.ToolResult{Name: s.name}, nil
}
