package generic

import (
	"testing"

	coreagent "matrixops.local/core_agent"
)

func TestCaptureEmitterStoresDeepCopiedPartSnapshots(t *testing.T) {
	emitter := newCaptureEmitter(coreagent.NoEmitter{})

	part := &coreagent.Part{
		ID:        "part-1",
		MessageID: "message-1",
		SessionID: "session-1",
		Type:      coreagent.PartTypeTool,
		Metadata: map[string]interface{}{
			"phase": "before",
		},
		Tool: &coreagent.ToolPart{
			Name:   "bash",
			CallID: "call-1",
			State: coreagent.ToolState{
				Status: "running",
				Input: map[string]interface{}{
					"command": "echo before",
				},
				Metadata: map[string]interface{}{
					"inputPreview": "before",
				},
			},
		},
	}

	if _, err := emitter.UpdatePart(part); err != nil {
		t.Fatalf("UpdatePart: %v", err)
	}

	part.Metadata["phase"] = "after"
	part.Tool.State.Metadata["inputPreview"] = "after"
	part.Tool.State.Input.(map[string]interface{})["command"] = "echo after"

	parts := emitter.partsForMessage("message-1")
	if len(parts) != 1 {
		t.Fatalf("parts len = %d, want 1", len(parts))
	}
	snapshot := parts[0]
	if snapshot.Metadata["phase"] != "before" {
		t.Fatalf("metadata phase = %#v, want before", snapshot.Metadata["phase"])
	}
	if snapshot.Tool == nil {
		t.Fatal("expected tool snapshot")
	}
	if snapshot.Tool.State.Metadata["inputPreview"] != "before" {
		t.Fatalf("tool metadata = %#v, want before", snapshot.Tool.State.Metadata["inputPreview"])
	}
	input, _ := snapshot.Tool.State.Input.(map[string]interface{})
	if input["command"] != "echo before" {
		t.Fatalf("tool input = %#v, want echo before", input["command"])
	}
}
