package coreagent

import (
	"context"
	"errors"
	"fmt"
	"testing"

	agenttool "matrixops-agent/tool"
)

type exitStatusTool struct {
	name string
}

func (t *exitStatusTool) Name() string { return t.name }
func (t *exitStatusTool) Description() string {
	return "tool that returns exit status 1"
}
func (t *exitStatusTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type":                 "object",
		"properties":           map[string]interface{}{},
		"additionalProperties": true,
	}
}
func (t *exitStatusTool) Execute(_ ToolContext, _ map[string]interface{}) (ToolResult, error) {
	return ToolResult{
		IsError: true,
		Name:    t.name,
		Content: "pandoc not found",
		Metadata: map[string]interface{}{
			"exitCode": 1,
		},
	}, fmt.Errorf("exit status 1")
}

func TestApplyToolCallExecutionMarksUserCancelAsCancelled(t *testing.T) {
	part := &Part{
		Type: PartTypeTool,
		Tool: &ToolPart{
			Name:  "read",
			State: ToolState{},
		},
	}
	result := ToolResult{
		IsError: true,
		Name:    "read",
		Content: "[Tool Cancelled]: tool execution was cancelled by user",
		Metadata: map[string]interface{}{
			"cancelled":   true,
			"cancelledBy": "user",
		},
	}

	ApplyToolCallExecution(part, result, agenttool.ErrToolExecutionCancelledByUser)
	if part.Tool.State.Status != "cancelled" {
		t.Fatalf("expected cancelled status, got %q", part.Tool.State.Status)
	}
	if part.Tool.State.Error != "Tool execution was cancelled by user" {
		t.Fatalf("unexpected error message: %q", part.Tool.State.Error)
	}
}

func TestIsToolExecutionCancelledRecognizesContextCanceled(t *testing.T) {
	if !isToolExecutionCancelled(context.Canceled, ToolResult{}, nil) {
		t.Fatal("expected context.Canceled to be treated as cancelled")
	}
	if !isToolExecutionCancelled(errors.Join(context.Canceled, agenttool.ErrToolExecutionCancelledByUser), ToolResult{}, nil) {
		t.Fatal("expected joined cancel errors to be treated as cancelled")
	}
}

func TestApplyToolCallExecutionMarksToolFailureAsError(t *testing.T) {
	part := &Part{
		Type: PartTypeTool,
		Tool: &ToolPart{
			Name:  "bash",
			State: ToolState{},
		},
	}
	result := ToolResult{
		IsError: true,
		Name:    "bash",
		Content: "pandoc not found",
		Metadata: map[string]interface{}{
			"exitCode": 1,
		},
	}

	ApplyToolCallExecution(part, result, fmt.Errorf("exit status 1"))
	if part.Tool.State.Status != "error" {
		t.Fatalf("expected error status, got %q", part.Tool.State.Status)
	}
	if part.Tool.State.Error != "exit status 1" {
		t.Fatalf("unexpected error message: %q", part.Tool.State.Error)
	}
}
