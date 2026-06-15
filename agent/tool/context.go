package tool

import (
	"context"
	"errors"
)

var ErrToolExecutionCancelledByUser = errors.New("tool execution cancelled by user")
var ErrTaskExecutionCancelledByUser = errors.New("task execution cancelled by user")

type ExecutionController interface {
	RegisterCancelableToolCall(callID string, toolName string, cancel context.CancelCauseFunc)
	FinishCancelableToolCall(callID string)
}

type executionControllerContextKey struct{}

type StreamEvent struct {
	Status   string
	Stream   string
	Content  string
	Title    string
	Metadata map[string]interface{}
}

type Context struct {
	Context   context.Context
	SessionID string
	Directory string
	Worktree  string
	Values    map[string]interface{}
	OnEvent   func(StreamEvent)
}

func (c Context) EmitEvent(event StreamEvent) {
	if c.OnEvent == nil {
		return
	}
	c.OnEvent(event)
}

// CheckContext returns ctx.Context.Err() when the task or tool call was cancelled.
func CheckContext(ctx Context) error {
	if ctx.Context == nil {
		return nil
	}
	return ctx.Context.Err()
}

func WithExecutionController(ctx context.Context, controller ExecutionController) context.Context {
	if controller == nil {
		return ctx
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, executionControllerContextKey{}, controller)
}

func ExecutionControllerFromContext(ctx context.Context) ExecutionController {
	if ctx == nil {
		return nil
	}
	controller, _ := ctx.Value(executionControllerContextKey{}).(ExecutionController)
	return controller
}

func DeriveToolCallContext(parent context.Context, callID string, toolName string) (context.Context, context.CancelCauseFunc, func()) {
	if parent == nil {
		parent = context.Background()
	}
	child, cancel := context.WithCancelCause(parent)
	controller := ExecutionControllerFromContext(parent)
	if controller == nil || callID == "" {
		return child, cancel, func() {}
	}
	controller.RegisterCancelableToolCall(callID, toolName, cancel)
	return child, cancel, func() {
		controller.FinishCancelableToolCall(callID)
	}
}
