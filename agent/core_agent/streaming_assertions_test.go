package coreagent

import (
	"bytes"
	"io"
	"testing"
	"time"
)

const streamingReadTimeout = 100 * time.Millisecond

func assertReadWithin(t *testing.T, r io.Reader, want string) {
	t.Helper()
	dataCh := make(chan []byte, 1)
	errCh := make(chan error, 1)
	go func() {
		buf := make([]byte, len(want)+16)
		n, err := r.Read(buf)
		dataCh <- append([]byte(nil), buf[:n]...)
		errCh <- err
	}()

	select {
	case got := <-dataCh:
		if string(got) != want {
			t.Fatalf("streamed chunk = %q, want %q", string(got), want)
		}
	case <-time.After(streamingReadTimeout):
		t.Fatalf("timed out waiting %s for streamed chunk %q", streamingReadTimeout, want)
	}

	if err := <-errCh; err != nil {
		t.Fatalf("unexpected streamed read error: %v", err)
	}
}

func assertReadAllEquals(t *testing.T, r io.Reader, prefix, suffix, want string) {
	t.Helper()
	rest, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}
	got := prefix + string(rest)
	if suffix != "" {
		got += suffix
	}
	if got != want {
		t.Fatalf("stream content = %q, want %q", got, want)
	}
}

func assertActionStream(t *testing.T, action *ActionOutput, firstChunk string, final string) {
	t.Helper()
	if action == nil {
		t.Fatal("expected non-nil action")
	}
	assertReadWithin(t, action.Data, firstChunk)
	assertReadAllEquals(t, action.Data, firstChunk, "", final)
}

func assertToolCallStream(t *testing.T, req *CallToolRequest, firstChunk string, final string) {
	t.Helper()
	if req == nil {
		t.Fatal("expected non-nil tool call")
	}
	assertReadWithin(t, req.Arguments, firstChunk)
	assertReadAllEquals(t, req.Arguments, firstChunk, "", final)
}

type compatibleControlSink struct {
	ch chan *ActionOutput
}

func newCompatibleControlSink() (*compatibleControlSink, CompatibleControlHandler) {
	s := &compatibleControlSink{ch: make(chan *ActionOutput, 16)}
	return s, s.handle
}

func (s *compatibleControlSink) handle(action *ActionOutput) error {
	s.ch <- action
	return nil
}

func (s *compatibleControlSink) recv(t *testing.T, timeout time.Duration) *ActionOutput {
	t.Helper()
	select {
	case action := <-s.ch:
		return action
	case <-time.After(timeout):
		t.Fatal("timeout waiting for compatible control action")
		return nil
	}
}

func concatBytes(parts ...string) []byte {
	var buf bytes.Buffer
	for _, part := range parts {
		buf.WriteString(part)
	}
	return buf.Bytes()
}
