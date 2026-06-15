package tool

import (
	"strings"
	"testing"
)

type stubTool struct {
	name   string
	result Result
	err    error
}

func (s stubTool) Name() string {
	return s.name
}

func (s stubTool) VerbosName() string {
	return s.name
}

func (s stubTool) Description() string {
	return s.name
}

func (s stubTool) Execute(Context, map[string]interface{}) (Result, error) {
	return s.result, s.err
}

func (s stubTool) Schema() map[string]interface{} {
	return ObjectParamSchema(map[string]interface{}{}, nil)
}

func TestExecuteWithOutputTruncationTruncatesToolContent(t *testing.T) {
	toolInstance := stubTool{
		name: "stub",
		result: Result{
			Content: strings.Repeat("line\n", truncateLineCountForTest()),
		},
	}

	result, err := ExecuteWithOutputTruncation(toolInstance, Context{}, nil)
	if err != nil {
		t.Fatalf("execute with truncation: %v", err)
	}
	if result.Name != "stub" {
		t.Fatalf("expected tool name to be propagated, got %q", result.Name)
	}
	if !result.Truncated {
		t.Fatal("expected result to be truncated")
	}
	if result.OutputPath == "" {
		t.Fatal("expected truncated result to include output path")
	}
	if !strings.Contains(result.Content, "The tool call succeeded but the output was truncated.") {
		t.Fatalf("expected truncation hint in content, got %q", result.Content)
	}
}

func TestExecuteWithOutputTruncationPreservesFullOutput(t *testing.T) {
	toolInstance := stubTool{
		name: "load_skill",
		result: Result{
			Content:            strings.Repeat("skill-line\n", truncateLineCountForTest()),
			PreserveFullOutput: true,
		},
	}

	result, err := ExecuteWithOutputTruncation(toolInstance, Context{}, nil)
	if err != nil {
		t.Fatalf("execute with truncation: %v", err)
	}
	if result.Truncated {
		t.Fatal("expected PreserveFullOutput to skip truncation")
	}
	if result.OutputPath != "" {
		t.Fatal("expected no spill output path when preserving full output")
	}
	if !strings.Contains(result.Content, "skill-line") {
		t.Fatalf("expected full skill content in result, got len=%d", len(result.Content))
	}
}

func TestPrepareFileOpRecordForStorageTruncatesContent(t *testing.T) {
	record, err := PrepareFileOpRecordForStorage(&FileOpRecord{
		Path:    "sample.txt",
		Action:  FileOpRecordActionRead,
		Content: strings.Repeat("line\n", truncateLineCountForTest()),
	})
	if err != nil {
		t.Fatalf("prepare file op record: %v", err)
	}
	if record == nil {
		t.Fatal("expected prepared record")
	}
	if !strings.Contains(record.Content, "The tool call succeeded but the output was truncated.") {
		t.Fatalf("expected stored content to be truncated, got %q", record.Content)
	}
}

func truncateLineCountForTest() int {
	return 360
}
