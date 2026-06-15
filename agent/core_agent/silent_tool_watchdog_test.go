package coreagent

import (
	"strings"
	"testing"
)

func TestSilentToolTracker_TriggersOnTenthCall(t *testing.T) {
	tracker := NewSilentToolTracker(10)
	for i := 1; i < 10; i++ {
		count, triggered := tracker.Observe(10)
		if triggered {
			t.Fatalf("unexpected trigger at count=%d", count)
		}
	}
	count, triggered := tracker.Observe(10)
	if !triggered || count != 10 {
		t.Fatalf("expected trigger at 10, got count=%d triggered=%v", count, triggered)
	}
	count, triggered = tracker.Observe(10)
	if triggered {
		t.Fatalf("expected no re-trigger while warned, got count=%d", count)
	}
}

func TestSilentToolTracker_ResetClearsStreak(t *testing.T) {
	tracker := NewSilentToolTracker(10)
	for i := 0; i < 7; i++ {
		tracker.Observe(10)
	}
	tracker.Reset()
	count, triggered := tracker.Observe(10)
	if triggered || count != 1 {
		t.Fatalf("after reset expected count=1 triggered=false, got count=%d triggered=%v", count, triggered)
	}
}

func TestFormatSilentToolWatchdogPrompt_HasSystemTag(t *testing.T) {
	msg := FormatSilentToolWatchdogPrompt(10)
	if !strings.HasPrefix(msg, "<system>") || !strings.HasSuffix(msg, "</system>") {
		t.Fatalf("expected <system> wrapper, got %q", msg)
	}
	if !strings.Contains(msg, "10") {
		t.Fatalf("expected count in message, got %q", msg)
	}
}

func TestObserveSilentToolCall_SkipsExemptTools(t *testing.T) {
	runner := &Runner{
		cfg: RunnerConfig{
			SilentToolCallThreshold: 2,
			OnSilentToolStreak: func(_ *RunState, count int) error {
				t.Fatalf("unexpected silent tool callback count=%d", count)
				return nil
			},
		},
	}
	state := &RunState{}
	args := map[string]interface{}{"worker": "explore"}
	for i := 0; i < 3; i++ {
		runner.observeSilentToolCall(state, ToolCall{Name: RunWorkerTaskToolName, Arguments: args})
	}
}

func TestObserveSilentToolCall_TriggersCallback(t *testing.T) {
	var triggered int
	runner := &Runner{
		cfg: RunnerConfig{
			SilentToolCallThreshold: 3,
			OnSilentToolStreak: func(_ *RunState, count int) error {
				triggered = count
				return nil
			},
		},
	}
	state := &RunState{}
	for i := 0; i < 3; i++ {
		runner.observeSilentToolCall(state, ToolCall{Name: "read", Arguments: map[string]interface{}{"path": "/tmp/a"}})
	}
	if triggered != 3 {
		t.Fatalf("triggered count = %d, want 3", triggered)
	}
}
