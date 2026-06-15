package session

import (
	"strings"

	coreagent "matrixops.local/core_agent"
	"pkgs/db/storage"
)

func (r *AgentRunner) persistPromptSnapshot(state *coreagent.RunState, info *coreagent.LLMCallInfo) error {
	if r == nil || r.db == nil || state == nil || state.Assistant == nil || info == nil {
		return nil
	}

	messageID := strings.TrimSpace(state.Assistant.ID)
	sessionID := strings.TrimSpace(state.Assistant.SessionID)
	prompt := strings.TrimSpace(info.Prompt)
	rawResponse := firstNonEmptyTrimmed(info.RawResponse, info.RawOutput, info.ParsedResponse)

	return storage.UpsertMessagePromptSnapshot(r.db, messageID, sessionID, prompt, rawResponse)
}

func firstNonEmptyTrimmed(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
