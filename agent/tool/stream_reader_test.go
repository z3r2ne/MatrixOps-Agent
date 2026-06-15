package tool

import (
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestDeltaStreamReaderCallsCallbackOnIntervalAndFinish(t *testing.T) {
	reader, writer := io.Pipe()

	var (
		mu      sync.Mutex
		deltas  []string
		accum   strings.Builder
	)

	streamReader := NewDeltaStreamReader(reader, 20*time.Millisecond, func(delta string) error {
		mu.Lock()
		defer mu.Unlock()
		deltas = append(deltas, delta)
		accum.WriteString(delta)
		return nil
	})

	resultCh := make(chan string, 1)
	errCh := make(chan error, 1)
	go func() {
		content, err := io.ReadAll(streamReader)
		if err != nil {
			errCh <- err
			return
		}
		resultCh <- string(content)
	}()

	if _, err := writer.Write([]byte("hello")); err != nil {
		t.Fatalf("write first chunk: %v", err)
	}
	time.Sleep(40 * time.Millisecond)

	if _, err := writer.Write([]byte(" world")); err != nil {
		t.Fatalf("write second chunk: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	select {
	case err := <-errCh:
		t.Fatalf("read stream: %v", err)
	case result := <-resultCh:
		if result != "hello world" {
			t.Fatalf("unexpected result: %q", result)
		}
	case <-time.After(time.Second):
		t.Fatal("read stream timeout")
	}

	mu.Lock()
	defer mu.Unlock()

	if accum.String() != "hello world" {
		t.Fatalf("unexpected accumulated deltas: %q", accum.String())
	}
	if len(deltas) < 2 {
		t.Fatalf("expected at least 2 delta callbacks, got %d (%v)", len(deltas), deltas)
	}
}

func TestDeltaStreamReaderReturnsCallbackError(t *testing.T) {
	callbackErr := errors.New("callback failed")
	streamReader := NewDeltaStreamReader(strings.NewReader("hello"), 10*time.Millisecond, func(delta string) error {
		if delta == "hello" {
			return callbackErr
		}
		return nil
	})

	_, err := io.ReadAll(streamReader)
	if !errors.Is(err, callbackErr) {
		t.Fatalf("expected callback error, got %v", err)
	}
}
