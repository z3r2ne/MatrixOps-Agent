package semantic_regression

import (
	"path/filepath"
	"runtime"
	"testing"

	"pkgs/semreg"
)

func baselinesDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "baselines")
}

func TestExploreChatHistoryTraceBaselineLoads(t *testing.T) {
	path := filepath.Join(baselinesDir(t), "explore_chat_history.trace.v1.json")
	doc, err := semreg.LoadTraceDocument(path)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Version != semreg.TraceSchemaVersion {
		t.Fatalf("version = %d", doc.Version)
	}
	if doc.Summary.TotalToolCalls != 18 {
		t.Fatalf("total_tool_calls = %d", doc.Summary.TotalToolCalls)
	}

	// 模拟一次轻微退化（工具调用 +25%）应失败
	actual := doc.Summary
	actual.TotalToolCalls = 23
	result := semreg.CompareTraceSummary(actual, doc.Summary, doc.Tolerances)
	if result.Passed {
		t.Fatal("expected trace regression when tool calls exceed tolerance")
	}
}
