package session

import (
	"testing"

	coreagent "matrixops.local/core_agent"
)

func TestResolveSilentToolWatchdogRunnerOpts_DisabledByDefault(t *testing.T) {
	runner := &AgentRunner{}
	threshold, handler := runner.resolveSilentToolWatchdogRunnerOpts(&RuntimeConfig{})
	if threshold != 0 {
		t.Fatalf("threshold=%d, want 0 when disabled", threshold)
	}
	if handler != nil {
		t.Fatal("expected nil handler when disabled")
	}
}

func TestResolveSilentToolWatchdogRunnerOpts_EnabledWiresHandler(t *testing.T) {
	runner, runtimeConfig := newSilentToolWatchdogTestRunner(t)
	runtimeConfig.SilentToolWatchdogEnabled = true

	threshold, handler := runner.resolveSilentToolWatchdogRunnerOpts(runtimeConfig)
	if threshold != coreagent.DefaultSilentToolCallThreshold {
		t.Fatalf("threshold = %d, want %d", threshold, coreagent.DefaultSilentToolCallThreshold)
	}
	if handler == nil {
		t.Fatal("expected handler when enabled")
	}
}
