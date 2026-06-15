package session

import (
	"testing"

	"matrixops-agent/types"
	"pkgs/db/models"
)

func TestBuildPromptContextInfoPrefersAutoCompressionLimitTokens(t *testing.T) {
	runtimeConfig := &RuntimeConfig{
		AutoCompressionLimitTokens: 128000,
		ModelSettings:              nil,
	}

	info := buildPromptContextInfo(runtimeConfig, &types.Memory{}, "hello")
	if info == nil {
		t.Fatal("expected context info")
	}
	if info.LimitTokens != 128000 {
		t.Fatalf("limit tokens = %d, want %d", info.LimitTokens, 128000)
	}
}

func TestBuildPromptContextInfoUsesSessionUsageTokensForCurrentTokens(t *testing.T) {
	runtimeConfig := &RuntimeConfig{
		AutoCompressionLimitTokens: 128000,
		SessionTokens: &MessageTokens{
			Input: 12000,
			Cache: TokenCache{Read: 2000},
		},
	}

	info := buildPromptContextInfo(runtimeConfig, &types.Memory{}, "hello")
	if info == nil {
		t.Fatal("expected context info")
	}
	if info.CurrentTokens != 14000 {
		t.Fatalf("current tokens = %d, want %d", info.CurrentTokens, 14000)
	}
}

func TestBuildPromptContextInfoPrefersAssistantTokensOverSessionTokens(t *testing.T) {
	runtimeConfig := &RuntimeConfig{
		SessionTokens: &MessageTokens{
			Input: 1000,
			Cache: TokenCache{Read: 100},
		},
		Assistant: &MessageInfo{
			Tokens: &MessageTokens{
				Input: 2000,
				Cache: TokenCache{Read: 300},
			},
		},
	}

	info := buildPromptContextInfo(runtimeConfig, &types.Memory{}, "hello")
	if info == nil {
		t.Fatal("expected context info")
	}
	if info.CurrentTokens != 2300 {
		t.Fatalf("current tokens = %d, want %d", info.CurrentTokens, 2300)
	}
}

func TestBuildPromptContextInfoFallsBackToModelSettingsWhenAutoLimitMissing(t *testing.T) {
	runtimeConfig := &RuntimeConfig{
		ModelSettings: &models.ModelSettings{
			ContextLimit: 100000,
			OutputLimit:  2000,
		},
	}

	info := buildPromptContextInfo(runtimeConfig, &types.Memory{}, "hello")
	if info == nil {
		t.Fatal("expected context info")
	}
	if info.LimitTokens != 98000 {
		t.Fatalf("limit tokens = %d, want %d", info.LimitTokens, 98000)
	}
}

func TestResolveAutoCompressionLimitTokensUsesModelContextLimit(t *testing.T) {
	limit := resolveAutoCompressionLimitTokens(&models.ModelSettings{
		ContextLimit: 64000,
		OutputLimit:  8000,
	})
	if limit != 64000 {
		t.Fatalf("limit = %d, want %d", limit, 64000)
	}
}

func TestResolveAutoCompressionLimitTokensReturnsZeroWithoutContextLimit(t *testing.T) {
	limit := resolveAutoCompressionLimitTokens(&models.ModelSettings{
		ContextLimit: 0,
		OutputLimit:  8000,
	})
	if limit != 0 {
		t.Fatalf("limit = %d, want 0", limit)
	}
}
