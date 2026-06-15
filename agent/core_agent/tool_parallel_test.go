package coreagent

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestRunIndexedParallelLimitsConcurrency(t *testing.T) {
	var inFlight atomic.Int32
	var maxInFlight atomic.Int32
	release := make(chan struct{})
	started := make(chan struct{}, MaxConcurrentToolCalls)

	errCh := make(chan error, 1)
	go func() {
		errCh <- runIndexedParallel(12, MaxConcurrentToolCalls, func(index int) error {
			current := inFlight.Add(1)
			for {
				prev := maxInFlight.Load()
				if current <= prev || maxInFlight.CompareAndSwap(prev, current) {
					break
				}
			}
			select {
			case started <- struct{}{}:
			default:
			}
			<-release
			inFlight.Add(-1)
			return nil
		})
	}()

	timeout := time.After(500 * time.Millisecond)
	for i := 0; i < MaxConcurrentToolCalls; i++ {
		select {
		case <-started:
		case <-timeout:
			t.Fatal("timed out waiting for max concurrent workers to start")
		}
	}
	close(release)

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("runIndexedParallel: %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for parallel tasks to finish")
	}
	if maxInFlight.Load() > MaxConcurrentToolCalls {
		t.Fatalf("max in-flight = %d, want <= %d", maxInFlight.Load(), MaxConcurrentToolCalls)
	}
}

func TestRunIndexedParallelWaitsForAll(t *testing.T) {
	done := make(chan struct{}, 2)
	errCh := make(chan error, 1)
	go func() {
		errCh <- runIndexedParallel(2, MaxConcurrentToolCalls, func(index int) error {
			time.Sleep(20 * time.Millisecond)
			done <- struct{}{}
			return nil
		})
	}()

	timeout := time.After(500 * time.Millisecond)
	for i := 0; i < 2; i++ {
		select {
		case <-done:
		case <-timeout:
			t.Fatal("timed out waiting for all parallel tasks")
		}
	}
	if err := <-errCh; err != nil {
		t.Fatalf("runIndexedParallel: %v", err)
	}
}
