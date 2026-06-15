package coreagent

import (
	"strings"
	"testing"
	"time"
)

func TestRunStateResolveStallWatchdogTimeoutConsumesOverrideOnce(t *testing.T) {
	state := &RunState{NextToolStallWatchdogTimeout: 45 * time.Second}

	got := state.ResolveStallWatchdogTimeout("bash", 10*time.Second)
	if got != 45*time.Second {
		t.Fatalf("first timeout = %s, want 45s", got)
	}
	if state.NextToolStallWatchdogTimeout != 0 {
		t.Fatalf("expected override consumed, got %s", state.NextToolStallWatchdogTimeout)
	}

	got = state.ResolveStallWatchdogTimeout("bash", 10*time.Second)
	if got != 10*time.Second {
		t.Fatalf("second timeout = %s, want default 10s", got)
	}
}

func TestRunStateResolveStallWatchdogTimeoutSkipsSetterTool(t *testing.T) {
	state := &RunState{NextToolStallWatchdogTimeout: 45 * time.Second}

	got := state.ResolveStallWatchdogTimeout(SetToolStallWatchdogTimeoutToolName, 10*time.Second)
	if got != 10*time.Second {
		t.Fatalf("setter tool timeout = %s, want default 10s", got)
	}
	if state.NextToolStallWatchdogTimeout != 45*time.Second {
		t.Fatalf("expected override preserved for next tool, got %s", state.NextToolStallWatchdogTimeout)
	}
}

func TestRunStateSetNextToolStallWatchdogTimeoutValidatesRange(t *testing.T) {
	state := &RunState{}
	if err := state.SetNextToolStallWatchdogTimeout(500 * time.Millisecond); err == nil {
		t.Fatal("expected error for too-short timeout")
	}
	if err := state.SetNextToolStallWatchdogTimeout(2 * time.Hour); err == nil {
		t.Fatal("expected error for too-long timeout")
	}
	if err := state.SetNextToolStallWatchdogTimeout(30 * time.Second); err != nil {
		t.Fatalf("SetNextToolStallWatchdogTimeout: %v", err)
	}
	if state.NextToolStallWatchdogTimeout != 30*time.Second {
		t.Fatalf("timeout = %s, want 30s", state.NextToolStallWatchdogTimeout)
	}
}

func TestFormatStallWatchdogToolCancelledWarningMentionsSetterTool(t *testing.T) {
	msg := FormatStallWatchdogToolCancelledWarning("bash", "taking too long", 12*time.Second)
	if !strings.Contains(msg, SetToolStallWatchdogTimeoutToolName) || !strings.Contains(msg, "仅生效一次") {
		t.Fatalf("message = %q", msg)
	}
}
