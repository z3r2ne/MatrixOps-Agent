package openai_native

import (
	"testing"

	"pkgs/db/models"
)

func TestMergeNativeOpenAIThinkingExtras(t *testing.T) {
	t.Run("only_enable_thinking_true", func(t *testing.T) {
		v := true
		m := mergeNativeOpenAIThinkingExtras("", &v)
		if m == nil || m["enable_thinking"] != true {
			t.Fatalf("got %#v", m)
		}
		if _, ok := m["thinking"]; ok {
			t.Fatal("unexpected thinking block")
		}
	})
	t.Run("only_thinking_type", func(t *testing.T) {
		m := mergeNativeOpenAIThinkingExtras(models.LLMThinkingTypeEnabled, nil)
		if m == nil {
			t.Fatal("nil map")
		}
		th, ok := m["thinking"].(map[string]any)
		if !ok || th["type"] != models.LLMThinkingTypeEnabled {
			t.Fatalf("thinking = %#v", m["thinking"])
		}
		if _, ok := m["enable_thinking"]; ok {
			t.Fatal("unexpected enable_thinking")
		}
	})
	t.Run("both", func(t *testing.T) {
		v := false
		m := mergeNativeOpenAIThinkingExtras(models.LLMThinkingTypeDisabled, &v)
		if m["enable_thinking"] != false {
			t.Fatalf("enable_thinking = %#v", m["enable_thinking"])
		}
		th := m["thinking"].(map[string]any)
		if th["type"] != models.LLMThinkingTypeDisabled {
			t.Fatalf("thinking = %#v", th)
		}
	})
	t.Run("neither", func(t *testing.T) {
		if m := mergeNativeOpenAIThinkingExtras("", nil); m != nil {
			t.Fatalf("got %#v", m)
		}
	})
}
