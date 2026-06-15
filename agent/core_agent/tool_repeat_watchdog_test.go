package coreagent

import (
	"testing"
)

func TestToolRepeatTracker_TriggersOnFifthIdenticalCall(t *testing.T) {
	tracker := NewToolRepeatTracker(5)
	args := map[string]interface{}{"path": "/tmp/a"}

	for i := 1; i <= 4; i++ {
		count, triggered := tracker.Observe("read_file", args, 5)
		if triggered {
			t.Fatalf("call %d should not trigger, got count=%d", i, count)
		}
	}

	count, triggered := tracker.Observe("read_file", args, 5)
	if !triggered || count != 5 {
		t.Fatalf("5th call should trigger, got triggered=%v count=%d", triggered, count)
	}

	count, triggered = tracker.Observe("read_file", args, 5)
	if triggered {
		t.Fatalf("6th call in same streak should not retrigger, count=%d", count)
	}

	for i := 7; i <= 14; i++ {
		_, triggered = tracker.Observe("read_file", args, 5)
		if triggered {
			t.Fatalf("call %d should not trigger reminder yet", i)
		}
	}

	count, triggered = tracker.Observe("read_file", args, 5)
	if !triggered || count != 15 {
		t.Fatalf("15th call should retrigger reminder, got triggered=%v count=%d", triggered, count)
	}
}

func TestToolRepeatTracker_ResetsWhenArgsChange(t *testing.T) {
	tracker := NewToolRepeatTracker(5)
	argsA := map[string]interface{}{"path": "/tmp/a"}
	argsB := map[string]interface{}{"path": "/tmp/b"}

	for i := 0; i < 4; i++ {
		tracker.Observe("read_file", argsA, 5)
	}
	count, triggered := tracker.Observe("read_file", argsB, 5)
	if triggered || count != 1 {
		t.Fatalf("changed args should reset counter, got triggered=%v count=%d", triggered, count)
	}
}

func TestNormalizeToolCallsForRepeatTracking_CallToolSingle(t *testing.T) {
	calls := normalizeToolCallsForRepeatTracking("call_tool", map[string]interface{}{
		"tool_name":  "grep",
		"tool_input": map[string]interface{}{"pattern": "foo"},
	})
	if len(calls) != 1 || calls[0].name != "grep" {
		t.Fatalf("unexpected normalized calls: %#v", calls)
	}
	if calls[0].args["pattern"] != "foo" {
		t.Fatalf("unexpected args: %#v", calls[0].args)
	}
}

func TestFormatRepeatedToolCallWarning_HasSystemTag(t *testing.T) {
	msg := FormatRepeatedToolCallWarning("grep", 5)
	if msg == "" {
		t.Fatal("expected non-empty warning")
	}
	if msg != FormatToolSystemTag("警告：你已经连续 5 次重复调用工具 \"grep\"，且参数完全相同。如果不是用户明确要求或确有特殊需求，请立刻停止这种重复调用，换一种思路、调整策略，或直接给出结论结束当前状态。") {
		t.Fatalf("expected <system> wrapped warning, got %q", msg)
	}
}

func TestToolCallFingerprint_StableForKeyOrder(t *testing.T) {
	a := ToolCallFingerprint("read_file", map[string]interface{}{
		"b": 2,
		"a": 1,
	})
	b := ToolCallFingerprint("read_file", map[string]interface{}{
		"a": 1,
		"b": 2,
	})
	if a != b {
		t.Fatalf("fingerprints differ: %q vs %q", a, b)
	}
}
