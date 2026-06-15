package coreagent

import "testing"

func TestMergeToolResultMetadataForCorePreservesEditStats(t *testing.T) {
	part := &Part{
		Tool: &ToolPart{
			State: ToolState{
				Metadata: map[string]interface{}{
					"cancelable": true,
				},
			},
		},
	}

	mergeToolResultMetadataForCore(part, ToolResult{
		Metadata: map[string]interface{}{
			"filesChanged": 2,
			"linesAdded":   42,
			"linesRemoved": 18,
		},
	})

	if part.Tool == nil {
		t.Fatal("expected tool part")
	}
	if part.Tool.State.Metadata == nil {
		t.Fatal("expected metadata to be preserved")
	}
	if got := part.Tool.State.Metadata["filesChanged"]; got != 2 {
		t.Fatalf("filesChanged = %#v, want 2", got)
	}
	if got := part.Tool.State.Metadata["linesAdded"]; got != 42 {
		t.Fatalf("linesAdded = %#v, want 42", got)
	}
	if got := part.Tool.State.Metadata["linesRemoved"]; got != 18 {
		t.Fatalf("linesRemoved = %#v, want 18", got)
	}
}

func TestMergeToolResultMetadataForCorePreservesSubtaskDisplayMetadata(t *testing.T) {
	part := &Part{
		Tool: &ToolPart{
			State: ToolState{
				Metadata: map[string]interface{}{
					"cancelable":    true,
					"subtaskTaskId": 123.0,
				},
			},
		},
	}

	mergeToolResultMetadataForCore(part, ToolResult{
		Metadata: map[string]interface{}{
			"subtaskTaskId":     123.0,
			"subtaskWorkerName": "frontend_engineer",
			"subtaskStatus":     "running",
		},
	})

	if part.Tool == nil {
		t.Fatal("expected tool part")
	}
	if part.Tool.State.Metadata == nil {
		t.Fatal("expected metadata to be preserved")
	}
	if got := part.Tool.State.Metadata["subtaskTaskId"]; got != 123.0 {
		t.Fatalf("subtaskTaskId = %#v, want 123", got)
	}
	if got := part.Tool.State.Metadata["subtaskWorkerName"]; got != "frontend_engineer" {
		t.Fatalf("subtaskWorkerName = %#v, want frontend_engineer", got)
	}
	if got := part.Tool.State.Metadata["subtaskStatus"]; got != "running" {
		t.Fatalf("subtaskStatus = %#v, want running", got)
	}
}
