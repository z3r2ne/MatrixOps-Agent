package session

import (
	"testing"

	"matrixops-agent/types"
	"pkgs/db/models"
)

func TestCreateUserMessagePushesAssistantCreatedAfterUser(t *testing.T) {
	sessionID := "session-order-test"
	runner := &AgentRunner{
		session: &types.Info{ID: sessionID},
		emitter: NewEmitter(nil, sessionID),
	}

	runtimeConfig := &RuntimeConfig{
		Worker: &models.Worker{
			Name: "chat",
		},
		ModelSettings: &models.ModelSettings{
			Name: "default_model_config",
		},
		LLMConfig: &models.LLMConfig{
			Name: "test-llm",
		},
		Assistant: &MessageInfo{
			ID:        "assistant-msg",
			SessionID: sessionID,
			Role:      RoleAssistant,
			Time: MessageTime{
				Created: 1,
			},
		},
		Parts: []*Part{
			{Type: types.PartTypeText, Text: "hello"},
		},
	}

	_, _, err := runner.createUserMessage(runtimeConfig)
	if err != nil {
		t.Fatalf("createUserMessage returned error: %v", err)
	}
	if runtimeConfig.Assistant.Time.Created <= 1 {
		t.Fatalf("expected assistant created time to be updated, got %d", runtimeConfig.Assistant.Time.Created)
	}
}

func TestCreateUserMessageRecognizesSummaryCommand(t *testing.T) {
	sessionID := "session-summary-command-test"
	runner := &AgentRunner{
		session: &types.Info{ID: sessionID},
		emitter: NewEmitter(nil, sessionID),
	}

	runtimeConfig := &RuntimeConfig{
		Worker:        &models.Worker{Name: "chat"},
		ModelSettings: &models.ModelSettings{Name: "default_model_config"},
		LLMConfig:     &models.LLMConfig{Name: "test-llm"},
		Assistant: &MessageInfo{
			ID:        "assistant-msg",
			SessionID: sessionID,
			Role:      RoleAssistant,
			Time:      MessageTime{Created: 1},
		},
		Parts: []*Part{
			{Type: types.PartTypeText, Text: "[summary](command://default?name=summary) 保存时保留环境信息"},
		},
	}

	userText, _, err := runner.createUserMessage(runtimeConfig)
	if err != nil {
		t.Fatalf("createUserMessage returned error: %v", err)
	}
	if !runtimeConfig.ManualSessionSummaryRequested {
		t.Fatal("expected summary command to be recognized")
	}
	if runtimeConfig.ManualSessionSummaryPrompt != "保存时保留环境信息" {
		t.Fatalf("unexpected summary prompt: %q", runtimeConfig.ManualSessionSummaryPrompt)
	}
	if userText != "保存时保留环境信息" {
		t.Fatalf("unexpected user text: %q", userText)
	}
	if runtimeConfig.CommandRequestMessageID == "" {
		t.Fatal("expected command request message id to be recorded")
	}
}
