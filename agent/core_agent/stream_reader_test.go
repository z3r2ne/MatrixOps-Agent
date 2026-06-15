package coreagent

import (
	"io"
	"strings"
	"testing"
	"time"
)

func TestStreamReaderFinalCallbackDoesNotBlockReadAll(t *testing.T) {
	release := make(chan struct{})
	reader := NewStreamReader(strings.NewReader("done"), 0, func(content string) error {
		if content == "done" {
			<-release
		}
		return nil
	})

	done := make(chan struct {
		data string
		err  error
	}, 1)
	go func() {
		payload, err := io.ReadAll(reader)
		done <- struct {
			data string
			err  error
		}{data: string(payload), err: err}
	}()

	select {
	case result := <-done:
		if result.err != nil {
			t.Fatalf("ReadAll returned error: %v", result.err)
		}
		if result.data != "done" {
			t.Fatalf("unexpected ReadAll content: %q", result.data)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("ReadAll blocked on final callback")
	}

	close(release)
}
