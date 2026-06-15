package anthropic

import (
	"testing"

	sdk "github.com/anthropics/anthropic-sdk-go"
	"pkgs/db/models"

	"matrixops.local/core_agent/streamtypes"
)

func TestBuildAnthropicMessageParams_ThinkingBudgetOptional(t *testing.T) {
	t.Run("omits thinking when budgetTokens is not configured", func(t *testing.T) {
		input := streamtypes.StreamInput{
			Prompt:            "hello",
			ThinkingType:      models.LLMThinkingTypeEnabled,
			ParallelToolCalls: true,
		}

		params, err := buildAnthropicMessageParams(input, sdk.Model("claude-test"), 8192)
		if err != nil {
			t.Fatalf("buildAnthropicMessageParams returned error: %v", err)
		}
		if params.Thinking.OfEnabled != nil {
			t.Fatalf("expected thinking to be omitted when budgetTokens is unset")
		}
	})

	t.Run("uses configured budgetTokens", func(t *testing.T) {
		budgetTokens := 4096
		input := streamtypes.StreamInput{
			Prompt:            "hello",
			EnableThinking:    boolPtr(true),
			BudgetTokens:      &budgetTokens,
			ParallelToolCalls: true,
		}

		params, err := buildAnthropicMessageParams(input, sdk.Model("claude-test"), 8192)
		if err != nil {
			t.Fatalf("buildAnthropicMessageParams returned error: %v", err)
		}
		if params.Thinking.OfEnabled == nil {
			t.Fatalf("expected thinking to be present when budgetTokens is configured")
		}
		if got := params.Thinking.GetBudgetTokens(); got == nil || *got != int64(budgetTokens) {
			if got == nil {
				t.Fatalf("thinking budget_tokens is nil, want %d", budgetTokens)
			}
			t.Fatalf("thinking budget_tokens = %d, want %d", *got, budgetTokens)
		}
	})
}

func boolPtr(v bool) *bool {
	return &v
}
