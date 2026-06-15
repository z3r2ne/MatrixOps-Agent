package session

import (
	"strings"

	"pkgs/db/storage"
)

func isPlanMutatingTool(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "writeplan", "updateplan":
		return true
	default:
		return false
	}
}

func (r *AgentRunner) emitPlanUpdated() {
	if r == nil || r.emitter == nil || r.db == nil {
		return
	}
	sessionID := strings.TrimSpace(r.GetSessionID())
	if sessionID == "" {
		return
	}
	plan, err := storage.GetPlan(r.db, sessionID)
	if err != nil {
		return
	}
	r.emitter.Emit(EventPlanUpdated, plan.Content.Data)
}
