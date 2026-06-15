package session

import "testing"

func TestRetryDelayUsesFixedDelay(t *testing.T) {
	if got := RetryDelay(5, nil); got != 5000 {
		t.Fatalf("delay = %d, want 5000", got)
	}
}
