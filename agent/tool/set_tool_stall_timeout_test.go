package tool

import (
	"testing"
	"time"
)

func TestSetToolStallTimeoutTool_SetsNextTimeout(t *testing.T) {
	var got time.Duration
	ctx := Context{
		Values: map[string]interface{}{
			toolContextSetNextStallWatchdogTimeoutKey: func(timeout time.Duration) error {
				got = timeout
				return nil
			},
		},
	}
	result, err := (SetToolStallTimeoutTool{}).Execute(ctx, map[string]interface{}{"timeout_seconds": 90})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result: %#v", result)
	}
	if got != 90*time.Second {
		t.Fatalf("timeout = %s, want 90s", got)
	}
}

func TestSetToolStallTimeoutTool_RequiresSetter(t *testing.T) {
	_, err := (SetToolStallTimeoutTool{}).Execute(Context{}, map[string]interface{}{"timeout_seconds": 30})
	if err == nil {
		t.Fatal("expected error without setter")
	}
}

func TestSetToolStallTimeoutTool_ValidatesTimeout(t *testing.T) {
	ctx := Context{
		Values: map[string]interface{}{
			toolContextSetNextStallWatchdogTimeoutKey: func(time.Duration) error { return nil },
		},
	}

	_, err := (SetToolStallTimeoutTool{}).Execute(ctx, map[string]interface{}{"timeout_seconds": 0})
	if err == nil {
		t.Fatal("expected validation error")
	}
}
