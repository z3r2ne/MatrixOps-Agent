package coreagent

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"
)

func TestRunner_ToolExecutionErrorDoesNotAbortToolRequest(t *testing.T) {
	emitter := &testEmitter{}
	runner, err := NewRunner(RunnerConfig{
		Emitter:              emitter,
		PromptBuilder:        func(state *RunState) (string, error) { return "prompt", nil },
		MaxSteps:             3,
		StallWatchdogTimeout: 0,
		Now:                  time.Now,
	})
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}
	runner.RegisterTool(&exitStatusTool{name: "bash"})

	state := &RunState{
		Context:   context.Background(),
		SessionID: "session-1",
		Assistant: &Message{ID: "msg-1", SessionID: "session-1", Role: RoleAssistant, Time: MessageTime{Created: 1}},
		UserInput: "hello",
	}

	parts, err := runner.executeToolCallRequest(state, &CallToolRequest{
		Name:      "bash",
		Arguments: bytes.NewReader([]byte(`{"command":"which pandoc"}`)),
		RawJSON:   `{"@action":"bash","data":{"command":"which pandoc"}}`,
	})
	if err != nil {
		t.Fatalf("executeToolCallRequest should not return run-fatal error: %v", err)
	}
	if len(parts) != 1 || parts[0].Tool == nil {
		t.Fatalf("expected one tool part, got %#v", parts)
	}
	if parts[0].Tool.State.Status != "error" {
		t.Fatalf("expected tool status error, got %q", parts[0].Tool.State.Status)
	}
	if parts[0].Tool.State.Error != "exit status 1" {
		t.Fatalf("expected exit status 1 on tool part, got %q", parts[0].Tool.State.Error)
	}
	if !strings.Contains(parts[0].Tool.State.SystemMessage, "exit status 1") {
		t.Fatalf("expected tool system message to mention failure reason, got %q", parts[0].Tool.State.SystemMessage)
	}
}
