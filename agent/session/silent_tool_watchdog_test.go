package session

import (
	"strings"
	"testing"

	coreagent "matrixops.local/core_agent"
	"pkgs/db/models"
	"pkgs/taskqueue"
	"pkgs/testutil"
)

func TestHandleSilentToolStreak_PrependsQueueHeadAndDoesNotConsumeImmediately(t *testing.T) {
	runner, runtimeConfig := newSilentToolWatchdogTestRunner(t)
	runner.messageQueue = taskqueue.New(runner.db, runner.task.ID, nil)

	if err := runner.handleSilentToolStreak(runtimeConfig, 10); err != nil {
		t.Fatalf("handleSilentToolStreak: %v", err)
	}

	queue, err := runner.messageQueue.Load()
	if err != nil {
		t.Fatalf("load queue: %v", err)
	}
	if len(queue) != 1 {
		t.Fatalf("queue len = %d, want 1", len(queue))
	}
	if !queue[0].Supplement {
		t.Fatalf("expected supplement=true")
	}
	if queue[0].ResolvedSource() != models.TaskMessageQueueSourceSilentToolWatchdog {
		t.Fatalf("source = %q", queue[0].ResolvedSource())
	}
	expected := coreagent.FormatSilentToolWatchdogPrompt(10)
	if strings.TrimSpace(queue[0].Content) != strings.TrimSpace(expected) {
		t.Fatalf("content = %q, want %q", queue[0].Content, expected)
	}
}

func TestHandleSilentToolStreak_SkipsDuplicateHeadSupplement(t *testing.T) {
	runner, runtimeConfig := newSilentToolWatchdogTestRunner(t)
	runner.messageQueue = taskqueue.New(runner.db, runner.task.ID, nil)

	if err := runner.handleSilentToolStreak(runtimeConfig, 10); err != nil {
		t.Fatalf("first handleSilentToolStreak: %v", err)
	}
	if err := runner.handleSilentToolStreak(runtimeConfig, 10); err != nil {
		t.Fatalf("second handleSilentToolStreak: %v", err)
	}

	queue, err := runner.messageQueue.Load()
	if err != nil {
		t.Fatalf("load queue: %v", err)
	}
	if len(queue) != 1 {
		t.Fatalf("queue len = %d, want 1", len(queue))
	}
}

func newSilentToolWatchdogTestRunner(t *testing.T) (*AgentRunner, *RuntimeConfig) {
	t.Helper()
	db := testutil.OpenTaskTestDB(t)
	task := testutil.CreateTask(t, db)
	runner := &AgentRunner{
		db:   db,
		task: task,
	}
	runtimeConfig := &RuntimeConfig{
		MemoryState: NewProcessV2MemoryState(nil),
	}
	return runner, runtimeConfig
}
