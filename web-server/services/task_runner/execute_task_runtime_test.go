package task_runner

import (
	"context"
	"testing"

	agenttool "matrixops-agent/tool"
	"matrixops/types"
	"pkgs/db/models"

	"gorm.io/gorm"
)

func TestNewTaskRuntimeInitialContextRegistersExecutionController(t *testing.T) {
	runtime, err := NewTaskRuntime(
		WithDB(&gorm.DB{}),
		WithWSHub(noopWSHub{}),
		WithTaskID(1),
		WithSessionID("session-1"),
		WithWorkDir("."),
	)
	if err != nil {
		t.Fatalf("NewTaskRuntime returned error: %v", err)
	}

	controller := agenttool.ExecutionControllerFromContext(runtime.ctx)
	if controller != runtime {
		t.Fatal("expected initial runtime context to carry execution controller")
	}
}

func TestTaskRuntimeEnsureRunContextReusesActiveContext(t *testing.T) {
	runtime := &TaskRuntime{}

	first := runtime.ensureRunContext(context.Background())
	second := runtime.ensureRunContext(context.Background())

	if first == nil {
		t.Fatal("expected first context")
	}
	if second != first {
		t.Fatal("expected active context to be reused")
	}
}

func TestTaskRuntimeEnsureRunContextRecreatesCanceledContext(t *testing.T) {
	type ctxKey string

	runtime := &TaskRuntime{}
	first := runtime.ensureRunContext(context.Background())
	runtime.cancelCurrentRun()

	if err := first.Err(); err == nil {
		t.Fatal("expected first context to be canceled")
	}
	if cause := context.Cause(first); cause != agenttool.ErrTaskExecutionCancelledByUser {
		t.Fatalf("expected first context cancel cause to be user cancel, got %v", cause)
	}

	parent := context.WithValue(context.Background(), ctxKey("trace"), "follow-up")
	second := runtime.ensureRunContext(parent)

	if second == nil {
		t.Fatal("expected second context")
	}
	if second == first {
		t.Fatal("expected canceled context to be recreated")
	}
	if err := second.Err(); err != nil {
		t.Fatalf("expected second context to be active, got %v", err)
	}
	if got := second.Value(ctxKey("trace")); got != "follow-up" {
		t.Fatalf("expected recreated context to inherit new parent value, got %#v", got)
	}
}

func TestTaskRuntimeCancelCurrentRunCancelsActiveToolCalls(t *testing.T) {
	runtime := &TaskRuntime{activeToolCalls: map[string]activeToolCall{}}
	cancelled := make(chan error, 1)

	runtime.RegisterCancelableToolCall("call-1", "read", func(cause error) {
		cancelled <- cause
	})

	runtime.cancelCurrentRun()

	select {
	case cause := <-cancelled:
		if cause != agenttool.ErrTaskExecutionCancelledByUser {
			t.Fatalf("unexpected cancel cause: %v", cause)
		}
	default:
		t.Fatal("expected active tool cancel func to be invoked")
	}
}

func TestTaskRuntimeCancelToolCallCancelsOnlyRegisteredTool(t *testing.T) {
	runtime := &TaskRuntime{activeToolCalls: map[string]activeToolCall{}}
	cancelled := make(chan error, 1)

	runtime.RegisterCancelableToolCall("call-1", "bash", func(cause error) {
		cancelled <- cause
	})

	if err := runtime.cancelToolCall("call-1"); err != nil {
		t.Fatalf("cancelToolCall returned error: %v", err)
	}

	select {
	case cause := <-cancelled:
		if cause != agenttool.ErrToolExecutionCancelledByUser {
			t.Fatalf("unexpected cancel cause: %v", cause)
		}
	default:
		t.Fatal("expected registered tool cancel func to be invoked")
	}
}

type noopWSHub struct{}

func (noopWSHub) BroadcastToTask(taskID uint, msg types.WSOutgoingMessage) {}
func (noopWSHub) BroadcastTaskMessage(taskID uint, message *models.TaskMessage) {}
func (noopWSHub) BroadcastNormalizedEntry(taskID uint, entry *models.NormalizedEntry) {}
func (noopWSHub) BroadcastTaskStatus(taskID uint, status models.TaskStatus, sessionID string, msg string) {}
func (noopWSHub) BroadcastIsWorking(taskID uint) {}
func (noopWSHub) BroadcastIsNotWorking(taskID uint) {}
func (noopWSHub) BroadcastError(taskID uint, err string) {}
func (noopWSHub) BroadcastSessionTitle(taskID uint, title string) {}
func (noopWSHub) BroadcastRetry(taskID uint) {}
func (noopWSHub) BroadcastWaitUserInput(taskID uint, id string, ack func(result map[string]interface{}), question map[string]interface{}) {
}

func (noopWSHub) BroadcastTaskQueue(taskID uint, queue []models.TaskMessageQueueItem) {}

func (noopWSHub) BroadcastTaskPlan(taskID uint, plan any) {}
