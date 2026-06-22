package tool

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestBashSessionManagerReadOutput(t *testing.T) {
	ctx := Context{
		Context:   context.Background(),
		SessionID: "test-session",
	}
	session, err := globalBashSessionManager.Start(bashScopeKey(ctx.SessionID), ctx, "echo hello-bash-session", t.TempDir())
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	deadline := time.After(10 * time.Second)
	for {
		output, status, readErr := globalBashSessionManager.ReadOutput(bashScopeKey(ctx.SessionID), session.ID, 8000)
		if readErr != nil {
			t.Fatalf("ReadOutput: %v", readErr)
		}
		if status != bashSessionStatusRunning {
			if !strings.Contains(output, "hello-bash-session") {
				t.Fatalf("expected output to contain greeting, got status=%s output=%q", status, output)
			}
			return
		}
		if strings.Contains(output, "hello-bash-session") {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for bash output, last status=%s output=%q", status, output)
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func TestBashSessionManagerStop(t *testing.T) {
	ctx := Context{
		Context:   context.Background(),
		SessionID: "test-session-stop",
	}
	session, err := globalBashSessionManager.Start(bashScopeKey(ctx.SessionID), ctx, "sleep 30", t.TempDir())
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	time.Sleep(300 * time.Millisecond)

	done := make(chan struct{})
	go func() {
		_, _ = session.wait()
		close(done)
	}()

	if err := globalBashSessionManager.Stop(bashScopeKey(ctx.SessionID), session.ID); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	select {
	case <-done:
	case <-time.After(8 * time.Second):
		t.Fatal("timed out waiting for bash session to stop")
	}

	_, status, readErr := globalBashSessionManager.ReadOutput(bashScopeKey(ctx.SessionID), session.ID, 1000)
	if readErr != nil {
		t.Fatalf("ReadOutput: %v", readErr)
	}
	if status != bashSessionStatusCancelled && status != bashSessionStatusFailed {
		t.Fatalf("unexpected terminal status: %s", status)
	}
}

func TestTerminalOutputBufferTail(t *testing.T) {
	buffer := &terminalOutputBuffer{}
	buffer.Append("abcdef")
	if got := buffer.Tail(3); got != "def" {
		t.Fatalf("Tail(3) = %q, want def", got)
	}
	if got := buffer.Tail(0); got != "abcdef" {
		t.Fatalf("Tail(0) = %q, want full content", got)
	}
}

func TestParseBashJobID(t *testing.T) {
	id, err := parseBashJobID(map[string]interface{}{"bash_job_id": "bash-123"})
	if err != nil || id != "bash-123" {
		t.Fatalf("parseBashJobID: id=%q err=%v", id, err)
	}
}
