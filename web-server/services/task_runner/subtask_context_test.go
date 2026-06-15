package task_runner

import (
	"context"
	"testing"
)

// 回归：newTaskHandler 返回 waitResult 后、waitResult 执行前，子任务 context 不能被提前 cleanup。
func TestSubtaskRunContextSurvivesUntilWaitResult(t *testing.T) {
	t.Parallel()

	parent, cancelParent := context.WithCancel(context.Background())
	t.Cleanup(cancelParent)

	subtaskRunCtx, cleanupSubtaskRunCtx := buildSubtaskRunContext(parent, nil)

	if subtaskRunCtx.Err() != nil {
		t.Fatalf("subtask context should start active, got %v", subtaskRunCtx.Err())
	}

	waitResult := func() error {
		defer cleanupSubtaskRunCtx()
		if err := subtaskRunCtx.Err(); err != nil {
			return err
		}
		return nil
	}

	if err := subtaskRunCtx.Err(); err != nil {
		t.Fatalf("context canceled before waitResult: %v", err)
	}

	if err := waitResult(); err != nil {
		t.Fatalf("waitResult failed: %v", err)
	}
	if subtaskRunCtx.Err() == nil {
		t.Fatal("expected subtask context to be canceled after waitResult cleanup")
	}
}
