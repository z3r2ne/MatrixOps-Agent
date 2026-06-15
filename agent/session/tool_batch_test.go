package session

import (
	"testing"
	"time"

	"matrixops-agent/llm"
	"matrixops-agent/tool"
)

func TestRunToolCallPlansInParallelRunsCallsConcurrentlyAndKeepsOrder(t *testing.T) {
	started := make(chan string, 2)
	release := make(chan struct{})

	buildPlan := func(id string) toolCallPlan {
		return toolCallPlan{
			Call: llm.ToolCall{ID: id, Name: id},
			Run: func() (tool.Result, error) {
				started <- id
				<-release
				return tool.Result{Content: "result-" + id, Name: id}, nil
			},
		}
	}

	done := make(chan []toolCallExecution, 1)
	go func() {
		done <- runToolCallPlansInParallel([]toolCallPlan{
			buildPlan("call-1"),
			buildPlan("call-2"),
		}, nil)
	}()

	seen := map[string]bool{}
	timeout := time.After(500 * time.Millisecond)
	for len(seen) < 2 {
		select {
		case id := <-started:
			seen[id] = true
		case <-timeout:
			t.Fatal("expected both tool calls to start before release")
		}
	}

	close(release)

	select {
	case results := <-done:
		if len(results) != 2 {
			t.Fatalf("expected 2 results, got %d", len(results))
		}
		if results[0].Call.ID != "call-1" || results[0].Result.Content != "result-call-1" {
			t.Fatalf("unexpected first result: %#v", results[0])
		}
		if results[1].Call.ID != "call-2" || results[1].Result.Content != "result-call-2" {
			t.Fatalf("unexpected second result: %#v", results[1])
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for parallel tool executions to finish")
	}
}
