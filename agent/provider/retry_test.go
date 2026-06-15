package provider

import (
	"errors"
	"testing"
	"time"
)

func TestRetryDelayForErrorUsesFixedDelay(t *testing.T) {
	if got := retryDelayForError(4, errors.New("boom")); got != 5*time.Second {
		t.Fatalf("delay = %s, want %s", got, 5*time.Second)
	}
}

func TestShouldRetryErrorRejectsContextCanceled(t *testing.T) {
	if shouldRetryError(errors.New("context canceled")) {
		t.Fatal("expected context canceled not to be retryable")
	}
}

func TestShouldRetryErrorAcceptsClientTimeout(t *testing.T) {
	err := errors.New("net/http: request canceled (Client.Timeout or context cancellation while reading body)")
	if !shouldRetryError(err) {
		t.Fatalf("expected Client.Timeout error to be retryable, got: %v", err)
	}
}
