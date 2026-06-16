package coreagent

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestWatchdog_NotTriggeredDuringPermissionWait(t *testing.T) {
	tool := &slowTool{name: "slow", delay: 30 * time.Millisecond}
	provider := &mockActionProviderForWatchdog{decision: "cancel", reason: "should not be called"}

	runner, emitter, err := newRunnerWithWatchdog(80*time.Millisecond, tool, provider)
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}

	authStarted := make(chan struct{}, 1)
	authAllow := make(chan struct{})
	runner.cfg.AuthorizeToolCall = func(_ *ActionContext, _ ToolCall) (*ToolResult, error) {
		select {
		case authStarted <- struct{}{}:
		default:
		}
		select {
		case <-authAllow:
			return nil, nil
		case <-time.After(3 * time.Second):
			return nil, fmt.Errorf("authorize timeout in test")
		}
	}

	state := &RunState{
		Context:   context.Background(),
		SessionID: "session-1",
		Assistant: &Message{ID: "msg-1", SessionID: "session-1", Role: RoleAssistant, Time: MessageTime{Created: 1}},
		UserInput: "hello",
	}

	done := make(chan error, 1)
	var parts []*Part
	go func() {
		var execErr error
		parts, execErr = runner.executeToolCallRequest(state, &CallToolRequest{
			Name:      "slow",
			Arguments: bytes.NewReader([]byte(`{}`)),
			RawJSON:   `{"@action":"call_tool","data":{"name":"slow","params":{}}`,
		})
		done <- execErr
	}()

	select {
	case <-authStarted:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for authorize to start")
	}

	time.Sleep(150 * time.Millisecond)

	provider.mu.Lock()
	calledDuringWait := provider.called
	provider.mu.Unlock()
	if calledDuringWait != 0 {
		t.Fatalf("stall watchdog should not run during permission wait, provider called %d times", calledDuringWait)
	}
	for _, f := range emitter.footers {
		if strings.Contains(f.Text, "停滞") {
			t.Fatalf("unexpected footer during permission wait: %q", f.Text)
		}
	}

	close(authAllow)

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("executeToolCallRequest: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for tool execution to finish")
	}

	if len(parts) != 1 || parts[0].Tool == nil {
		t.Fatalf("expected one tool part, got %#v", parts)
	}
	if parts[0].Tool.State.Status != "completed" {
		t.Fatalf("expected status completed, got %q", parts[0].Tool.State.Status)
	}

	provider.mu.Lock()
	called := provider.called
	provider.mu.Unlock()
	if called != 0 {
		t.Fatalf("stall watchdog should not have been triggered, provider called %d times", called)
	}
}
