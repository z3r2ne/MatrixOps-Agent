package session

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	agenttool "matrixops-agent/tool"
	"matrixops-agent/types"
	"pkgs/db/models"
)

type blockingTestTool struct {
	started chan string
	allow   chan struct{}
}

func (t *blockingTestTool) Name() string        { return "dummy" }
func (t *blockingTestTool) VerbosName() string  { return "Dummy" }
func (t *blockingTestTool) Description() string { return "dummy tool" }
func (t *blockingTestTool) Schema() map[string]interface{} {
	return agenttool.ObjectParamSchema(map[string]interface{}{
		"value": map[string]interface{}{
			"type":        "string",
			"description": "test input",
		},
	}, nil)
}
func (t *blockingTestTool) Execute(ctx agenttool.Context, input map[string]interface{}) (agenttool.Result, error) {
	value, _ := input["value"].(string)
	t.started <- value
	<-t.allow
	return agenttool.Result{
		Name:    "dummy",
		Content: "done:" + value,
	}, nil
}

// handleDirectRegistryTool 会一次性 ReadAll 参数 JSON，不再支持旧版 meta call_tool 流式拆 batch。
func TestHandleRegistryToolV2ExecutesPerToolCall(t *testing.T) {
	sessionID := "session-direct-tool-test"
	emitter := NewEmitter(nil, sessionID)
	registry := agenttool.NewRegistry()
	toolInstance := &blockingTestTool{
		started: make(chan string, 4),
		allow:   make(chan struct{}, 4),
	}
	registry.Register(toolInstance)

	runner := &AgentRunner{
		session: &types.Info{ID: sessionID, Directory: "."},
		emitter: emitter,
	}

	runtimeConfig := &RuntimeConfig{
		Ctx: context.Background(),
		Assistant: &MessageInfo{
			ID:        "assistant-tool-message-id",
			SessionID: sessionID,
		},
		ToolRegistry: registry,
		Worker: &models.Worker{
			Name: "tester",
		},
	}

	runOneAsync := func(arg string) (chan []*Part, chan error) {
		t.Helper()
		payload, err := json.Marshal(map[string]string{"value": arg})
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		action := &ActionOutput{
			Action: "dummy",
			Data:   strings.NewReader(string(payload)),
		}
		partsCh := make(chan []*Part, 1)
		errCh := make(chan error, 1)
		go func() {
			parts, runErr := runner.handleCallToolV2(runtimeConfig, action)
			if runErr != nil {
				errCh <- runErr
				return
			}
			partsCh <- parts
		}()
		return partsCh, errCh
	}

	parts1Ch, err1Ch := runOneAsync("one")
	if started := <-toolInstance.started; started != "one" {
		t.Fatalf("expected first value %q, got %q", "one", started)
	}
	toolInstance.allow <- struct{}{}
	var parts1 []*Part
	select {
	case err := <-err1Ch:
		t.Fatalf("handleCallToolV2: %v", err)
	case parts1 = <-parts1Ch:
	}
	if len(parts1) != 1 || parts1[0].Tool == nil {
		t.Fatalf("expected 1 tool part, got %#v", parts1)
	}
	if parts1[0].Tool.State.Output != "done:one" {
		t.Fatalf("unexpected first output: %#v", parts1[0].Tool)
	}

	parts2Ch, err2Ch := runOneAsync("two")
	if started := <-toolInstance.started; started != "two" {
		t.Fatalf("expected second value %q, got %q", "two", started)
	}
	toolInstance.allow <- struct{}{}
	var parts2 []*Part
	select {
	case err := <-err2Ch:
		t.Fatalf("handleCallToolV2: %v", err)
	case parts2 = <-parts2Ch:
	}
	if len(parts2) != 1 || parts2[0].Tool == nil {
		t.Fatalf("expected 1 tool part, got %#v", parts2)
	}
	if parts2[0].Tool.State.Output != "done:two" {
		t.Fatalf("unexpected second output: %#v", parts2[0].Tool)
	}
}
