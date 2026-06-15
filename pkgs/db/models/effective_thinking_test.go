package models

import "testing"

func boolRef(b bool) *bool { return &b }

func TestEffectiveThinkingType(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		if got := EffectiveThinkingType(nil); got != "" {
			t.Fatalf("got %q", got)
		}
	})
	t.Run("ThinkingType_enabled", func(t *testing.T) {
		ms := &ModelSettings{ThinkingType: "enabled", EnableThinking: boolRef(false)}
		if got := EffectiveThinkingType(ms); got != LLMThinkingTypeEnabled {
			t.Fatalf("got %q", got)
		}
	})
	t.Run("ThinkingType_disabled", func(t *testing.T) {
		ms := &ModelSettings{ThinkingType: "disabled", EnableThinking: boolRef(true)}
		if got := EffectiveThinkingType(ms); got != LLMThinkingTypeDisabled {
			t.Fatalf("got %q", got)
		}
	})
	t.Run("EnableThinking_only_does_not_affect", func(t *testing.T) {
		ms := &ModelSettings{ThinkingType: "", EnableThinking: boolRef(true)}
		if got := EffectiveThinkingType(ms); got != "" {
			t.Fatalf("EffectiveThinkingType must ignore EnableThinking, got %q", got)
		}
	})
	t.Run("both_unset", func(t *testing.T) {
		ms := &ModelSettings{}
		if got := EffectiveThinkingType(ms); got != "" {
			t.Fatalf("got %q", got)
		}
	})
}
