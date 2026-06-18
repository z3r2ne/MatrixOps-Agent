package session

import (
	"strings"
	"testing"

	"matrixops-agent/types"
	"pkgs/db/storage"
	"pkgs/testutil"
)

func TestCriticalInfoMarkerAndMessage(t *testing.T) {
	message := formatAsyncToolStartMessage("bash", map[string]interface{}{"command": "echo hi"}, "call-1", 0)
	if !strings.Contains(message, "<system>") {
		t.Fatalf("expected system wrapper, got %q", message)
	}
}

func TestAsyncToolStartBodyIncludesSubtaskTaskID(t *testing.T) {
	body := formatAsyncToolStartBody("run_worker_task", map[string]interface{}{"worker": "explore"}, "call-2", 123)
	if !strings.Contains(body, "task_id: 123") {
		t.Fatalf("expected body to include task_id, got %q", body)
	}
}

func TestCriticalInfoPresentInTranscript(t *testing.T) {
	item := newAsyncToolCriticalInfoItem("call-2", "read", map[string]interface{}{"path": "a.go"}, 0, "started")
	transcript := "earlier context\nread\n{\"path\":\"a.go\"}\nstarted\nlater"
	if !criticalInfoPresentInTranscript(transcript, item) {
		t.Fatal("expected marker/message to be detected in transcript")
	}
	if criticalInfoPresentInTranscript("unrelated transcript", item) {
		t.Fatal("expected unrelated transcript to miss critical info")
	}
}

func TestEnsureCriticalInfoInContext_ReinjectsMissingMarker(t *testing.T) {
	runner := &AgentRunner{
		session: &types.Info{
			ID: "session-critical-info",
			CriticalInfo: &types.CriticalInfo{
				Items: []types.CriticalInfoItem{
					newAsyncToolCriticalInfoItem("call-3", "bash", map[string]interface{}{"command": "sleep 1"}, 0, "started"),
				},
			},
		},
	}
	runtimeConfig := &RuntimeConfig{
		MemoryState: NewProcessV2MemoryState(&types.Memory{
			Entries: []*types.MemoryEntry{
				{
					SessionID: "session-critical-info",
					Role:      "user",
					Content:   "hello",
				},
			},
		}),
	}
	if err := runner.ensureCriticalInfoInContext(runtimeConfig); err != nil {
		t.Fatalf("ensureCriticalInfoInContext: %v", err)
	}
	transcript := runtimeConfig.MemoryState.Snapshot().PromptContent()
	// ensureCriticalInfoInContext reinjects the system message; matching is handled by tool_call sources.
	if !strings.Contains(transcript, "<system>") {
		t.Fatalf("expected reinjected system message in transcript: %q", transcript)
	}
}

func TestPersistAndRemoveCriticalInfoItem(t *testing.T) {
	db := testutil.OpenTaskTestDB(t)
	sessionInfo := &types.Info{
		ID:        "session-critical-info-persist",
		ProjectID: "project-critical-info",
		Directory: t.TempDir(),
		Time: types.TimeInfo{
			Created: 1,
			Updated: 1,
		},
	}
	if err := storage.UpdateSession(db, sessionInfo); err != nil {
		t.Fatalf("seed session: %v", err)
	}
	runner := &AgentRunner{
		db:      db,
		session: sessionInfo,
	}
	item := newAsyncToolCriticalInfoItem("call-4", "glob", map[string]interface{}{"pattern": "*.go"}, 0, "started")
	if err := runner.upsertCriticalInfoItem(item); err != nil {
		t.Fatalf("upsertCriticalInfoItem: %v", err)
	}
	if runner.session.CriticalInfo == nil || len(runner.session.CriticalInfo.Items) != 1 {
		t.Fatalf("expected one critical info item, got %#v", runner.session.CriticalInfo)
	}
	if err := runner.removeCriticalInfoItem(item.ID); err != nil {
		t.Fatalf("removeCriticalInfoItem: %v", err)
	}
	if runner.session.CriticalInfo != nil {
		t.Fatalf("expected critical info cleared, got %#v", runner.session.CriticalInfo)
	}
}
