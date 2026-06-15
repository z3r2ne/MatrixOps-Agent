package coreagent

import (
	"testing"
	"time"
)

func TestIsWatchdogExemptTool(t *testing.T) {
	if !IsWatchdogExemptTool(RunWorkerTaskToolName) {
		t.Fatalf("expected %q exempt", RunWorkerTaskToolName)
	}
	if !IsWatchdogExemptTool(QuestionToolName) {
		t.Fatalf("expected %q exempt", QuestionToolName)
	}
	if IsWatchdogExemptTool("bash") {
		t.Fatal("bash should not be exempt")
	}
}

func TestRunStateResolveStallWatchdogTimeout_ExemptToolsDisabled(t *testing.T) {
	state := &RunState{NextToolStallWatchdogTimeout: 45 * time.Second}

	for _, name := range []string{RunWorkerTaskToolName, QuestionToolName} {
		got := state.ResolveStallWatchdogTimeout(name, 10*time.Second)
		if got != 0 {
			t.Fatalf("%s timeout = %s, want 0 (disabled)", name, got)
		}
	}
	if state.NextToolStallWatchdogTimeout != 45*time.Second {
		t.Fatalf("exempt tool should not consume override, got %s", state.NextToolStallWatchdogTimeout)
	}
}

func TestRunStateResolveStallWatchdogTimeout_DisabledWhenGlobalZero(t *testing.T) {
	state := &RunState{}
	got := state.ResolveStallWatchdogTimeout("bash", 0)
	if got != 0 {
		t.Fatalf("global disabled timeout = %s, want 0", got)
	}
}

func TestObserveRepeatedToolCall_SkipsExemptTools(t *testing.T) {
	runner := &Runner{
		cfg: RunnerConfig{
			RepeatedToolCallThreshold: 2,
			OnRepeatedToolCall: func(_ *RunState, toolName string, _ map[string]interface{}, count int) error {
				t.Fatalf("unexpected repeated tool callback for %q count=%d", toolName, count)
				return nil
			},
		},
	}
	state := &RunState{}
	args := map[string]interface{}{"worker": "explore"}
	for i := 0; i < 3; i++ {
		runner.observeRepeatedToolCall(state, ToolCall{Name: RunWorkerTaskToolName, Arguments: args})
	}
}
