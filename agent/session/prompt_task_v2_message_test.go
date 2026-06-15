package session

import (
	"io"
	"strings"
	"testing"
	"time"

	"matrixops-agent/types"
)

func TestHandleMessageV2StreamsMessageField(t *testing.T) {
	sessionID := "session-message-test"
	emitter := NewEmitter(nil, sessionID)
	runner := &AgentRunner{
		session: &types.Info{ID: sessionID},
		emitter: emitter,
	}

	runtimeConfig := &RuntimeConfig{
		Assistant: &MessageInfo{
			ID:        "assistant-message-id",
			SessionID: sessionID,
		},
	}

	updateCh := make(chan string, 8)
	emitter.On(EventPartUpdated, func(args ...interface{}) {
		event := args[0].(PartEvent)
		if event.Part == nil || event.Part.Type != types.PartTypeText {
			return
		}
		select {
		case updateCh <- event.Part.Text:
		default:
		}
	})

	reader, writer := io.Pipe()
	action := &ActionOutput{
		Action: "message",
		Data:   reader,
	}

	partCh := make(chan *Part, 1)
	errCh := make(chan error, 1)
	go func() {
		part, err := runner.handleMessageV2(runtimeConfig, action)
		if err != nil {
			errCh <- err
			return
		}
		partCh <- part
	}()

	if _, err := writer.Write([]byte(`{"message":"hel`)); err != nil {
		t.Fatalf("write partial message payload: %v", err)
	}

	var partial string
	select {
	case partial = <-updateCh:
		if partial == "" {
			t.Fatal("expected non-empty partial message update")
		}
		if !strings.HasPrefix("hello", partial) {
			t.Fatalf("expected partial update to be prefix of %q, got %q", "hello", partial)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected streaming message update before payload completed")
	}

	if _, err := writer.Write([]byte(`lo","next_step":"continue"}`)); err != nil {
		t.Fatalf("write remaining message payload: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close message writer: %v", err)
	}

	select {
	case err := <-errCh:
		t.Fatalf("handleMessageV2 returned error: %v", err)
	case part := <-partCh:
		if part == nil {
			t.Fatal("expected non-nil part")
		}
		if part.Text != "hello" {
			t.Fatalf("expected final message text %q, got %q", "hello", part.Text)
		}
		if got, _ := part.Metadata["next_step"].(string); got != "continue" {
			t.Fatalf("expected next_step %q, got %#v", "continue", part.Metadata["next_step"])
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for handleMessageV2 result")
	}
}
