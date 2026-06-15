package task_runner

import (
	"context"
	"testing"
	"time"
)

func TestMergeContextsCancelsWhenAnyParentCancels(t *testing.T) {
	ctx1, cancel1 := context.WithCancel(context.Background())
	defer cancel1()

	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()

	mergedCtx, cleanup := mergeContexts(ctx1, ctx2)
	defer cleanup()

	cancel2()

	select {
	case <-mergedCtx.Done():
	case <-time.After(200 * time.Millisecond):
		t.Fatal("merged context was not canceled when a parent context was canceled")
	}
}

func TestMergeContextsPreservesFirstContextValues(t *testing.T) {
	type ctxKey string

	ctxWithValue := context.WithValue(context.Background(), ctxKey("key"), "value")
	ctxOther, cancelOther := context.WithCancel(context.Background())
	defer cancelOther()

	mergedCtx, cleanup := mergeContexts(ctxWithValue, ctxOther)
	defer cleanup()

	if got := mergedCtx.Value(ctxKey("key")); got != "value" {
		t.Fatalf("expected merged context to preserve first context value, got %#v", got)
	}
}
