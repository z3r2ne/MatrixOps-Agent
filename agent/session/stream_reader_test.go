package session

import (
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestStreamReaderCallsCallbackOnIntervalAndFinish(t *testing.T) {
	reader, writer := io.Pipe()

	var (
		mu      sync.Mutex
		updates []string
	)

	streamReader := NewStreamReader(reader, 20*time.Millisecond, func(content string) error {
		mu.Lock()
		defer mu.Unlock()
		updates = append(updates, content)
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

	if len(updates) < 2 {
		t.Fatalf("expected at least 2 callbacks, got %d", len(updates))
	}

	if updates[len(updates)-1] != "hello world" {
		t.Fatalf("unexpected final callback content: %q", updates[len(updates)-1])
	}

	foundPartial := false
	for _, update := range updates[:len(updates)-1] {
		if update == "hello" {
			foundPartial = true
			break
		}
	}
	if !foundPartial {
		t.Fatalf("expected a partial callback, got %v", updates)
	}
}

func TestStreamReaderReturnsCallbackError(t *testing.T) {
	callbackErr := errors.New("callback failed")
	streamReader := NewStreamReader(strings.NewReader("hello"), 10*time.Millisecond, func(content string) error {
		if content == "hello" {
			return callbackErr
		}
		return nil
	})

	_, err := io.ReadAll(streamReader)
	if !errors.Is(err, callbackErr) {
		t.Fatalf("expected callback error, got %v", err)
	}
}
