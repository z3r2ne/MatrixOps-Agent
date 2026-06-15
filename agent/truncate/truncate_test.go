package truncate

import (
	"strings"
	"testing"

	"matrixops-agent/global"
)

func TestOutputSingleOversizedLineKeepsHeadPreview(t *testing.T) {
	t.Setenv(global.EnvTestHome, t.TempDir())

	line := strings.Repeat("a", 200*1024)
	result, err := Output(line, Options{})
	if err != nil {
		t.Fatalf("Output: %v", err)
	}
	if !result.Truncated {
		t.Fatal("expected truncated result")
	}
	if result.OutputPath == "" {
		t.Fatal("expected spill path")
	}
	preview := strings.Split(result.Content, "\n\n...")[0]
	if len(preview) == 0 {
		t.Fatal("expected non-empty head preview")
	}
	if len([]byte(preview)) > MaxBytes {
		t.Fatalf("preview exceeds max bytes: got %d want <= %d", len([]byte(preview)), MaxBytes)
	}
	if !strings.HasPrefix(preview, "aaa") {
		t.Fatalf("unexpected preview prefix: %q", preview[:min(20, len(preview))])
	}
}

func TestOutputTailSingleOversizedLineKeepsTailPreview(t *testing.T) {
	t.Setenv(global.EnvTestHome, t.TempDir())

	line := strings.Repeat("a", 150*1024) + "TAILMARKER"
	result, err := Output(line, Options{Direction: "tail"})
	if err != nil {
		t.Fatalf("Output: %v", err)
	}
	if !result.Truncated {
		t.Fatal("expected truncated result")
	}
	if !strings.Contains(result.Content, "TAILMARKER") {
		t.Fatalf("expected tail preview to include marker, got %q", result.Content[len(result.Content)-80:])
	}
}

func TestCollectHeadPreviewPartialLine(t *testing.T) {
	lines := []string{strings.Repeat("x", 2048)}
	out, kept, hitBytes := collectHeadPreview(lines, MaxLines, 1024)
	if !hitBytes {
		t.Fatal("expected byte limit hit")
	}
	if kept != 1024 {
		t.Fatalf("kept bytes = %d, want 1024", kept)
	}
	if len(out) != 1 || len([]byte(out[0])) != 1024 {
		t.Fatalf("unexpected preview chunks: %+v", out)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
