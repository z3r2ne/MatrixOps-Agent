package coreagent

import (
	"testing"
	"time"
)

func withCompatibleControl(input StreamInput, handler CompatibleControlHandler) StreamInput {
	input.CompatibleControlHandler = handler
	return input
}

func expectCompatibleAnswerAfterRetry(t *testing.T, sink *compatibleControlSink, output *StreamOutput) *ActionOutput {
	t.Helper()
	action := sink.recv(t, 10*time.Second)
	if action == nil || action.Action != "answer" {
		t.Fatalf("unexpected answer control action: %#v", action)
	}
	if err := output.Wait(); err != nil {
		t.Fatalf("Wait returned error: %v", err)
	}
	return action
}
