package memory

import "testing"

func TestRenderToolResultContentEmptySuccessDoesNotUseCompleted(t *testing.T) {
	got := RenderToolResultContent("", "", "", "completed", "")
	if got != EmptyToolResultMessage {
		t.Fatalf("expected empty success placeholder, got %q", got)
	}
}

func TestRenderToolResultContentPrefersOutput(t *testing.T) {
	got := RenderToolResultContent("git log output", "", "", "completed", "")
	if got != "git log output" {
		t.Fatalf("expected tool output, got %q", got)
	}
}

func TestRenderToolResultContentNonSuccessStatus(t *testing.T) {
	got := RenderToolResultContent("", "", "", "cancelled", "")
	if got != "cancelled" {
		t.Fatalf("expected cancelled status, got %q", got)
	}
}

func TestRenderToolResultContentFromEntry(t *testing.T) {
	got := RenderToolResultContentFromEntry(&MemoryEntry{
		ToolOutput: "",
		ToolStatus: "completed",
	})
	if got != EmptyToolResultMessage {
		t.Fatalf("expected empty success placeholder, got %q", got)
	}
}
