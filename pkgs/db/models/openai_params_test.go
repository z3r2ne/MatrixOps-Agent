package models

import "testing"

func TestOpenAIReasoningEffortValue_IndependentOfEnableThinking(t *testing.T) {
	eff := "high"
	en := true
	ms := &ModelSettings{
		ReasoningEffort: &eff,
		EnableThinking:  &en,
		ThinkingType:    "",
	}
	if got := OpenAIReasoningEffortValue(ms); got != "high" {
		t.Fatalf("got %q", got)
	}
}

func TestOpenAIReasoningEffortValue_Invalid(t *testing.T) {
	bad := "bogus"
	if got := OpenAIReasoningEffortValue(&ModelSettings{ReasoningEffort: &bad}); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestParallelToolCallsEnabledNil(t *testing.T) {
	if ParallelToolCallsEnabled(&ModelSettings{}) {
		t.Fatal("expected false")
	}
}

func TestSilentToolWatchdogEnabledNil(t *testing.T) {
	if SilentToolWatchdogEnabled(&ModelSettings{}) {
		t.Fatal("expected false")
	}
}

func TestSilentToolWatchdogEnabledTrue(t *testing.T) {
	enabled := true
	if !SilentToolWatchdogEnabled(&ModelSettings{EnableSilentToolWatchdog: &enabled}) {
		t.Fatal("expected true")
	}
}
