package semreg_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"pkgs/semreg"
)

func testdataDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "testdata")
}

func TestLoadScenario(t *testing.T) {
	scenario, err := semreg.LoadScenario(filepath.Join(testdataDir(t), "sample_prompt.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if scenario.ID != "sample_prompt" {
		t.Fatalf("id = %q", scenario.ID)
	}
	if scenario.Kind != semreg.KindPromptRender {
		t.Fatalf("kind = %q", scenario.Kind)
	}
}

func TestEvaluateStructAssertions(t *testing.T) {
	result := semreg.EvaluateStructAssertions(
		"<system_prompt> hello <global_prompt>",
		"do it",
		semreg.AssertSpec{
			SystemPromptContains: []string{"<system_prompt>", "<global_prompt>"},
			UserInputEquals:      "do it",
		},
	)
	if !result.Passed {
		t.Fatalf("expected pass, errors=%v", result.Errors)
	}
}

func TestBuildTraceSummaryCountsDuplicateReads(t *testing.T) {
	calls := []semreg.TraceToolCall{
		{Tool: "read", Input: map[string]any{"path": "a.go", "offset": 0, "limit": 100}},
		{Tool: "read", Input: map[string]any{"path": "a.go", "offset": 0, "limit": 100}},
		{Tool: "rg", Input: map[string]any{"pattern": "fmt"}},
	}
	summary := semreg.BuildTraceSummary(calls)
	if summary.TotalToolCalls != 3 {
		t.Fatalf("total=%d", summary.TotalToolCalls)
	}
	if len(summary.ReadDuplicateRanges) != 1 {
		t.Fatalf("dup ranges=%d", len(summary.ReadDuplicateRanges))
	}
}

func TestCompareTraceSummaryRejectsDuplicateReads(t *testing.T) {
	baseline := semreg.TraceSummary{
		TotalToolCalls: 10,
		ReadTotal:      5,
		ReadDuplicateRanges: []semreg.TraceReadDuplicateRange{
			{Path: "a.go", Count: 1},
		},
	}
	actual := baseline
	actual.ReadDuplicateRanges = []semreg.TraceReadDuplicateRange{
		{Path: "a.go", Count: 2},
		{Path: "b.go", Count: 2},
	}
	result := semreg.CompareTraceSummary(actual, baseline, &semreg.TraceTolerances{
		TotalToolCalls:      ptrFloat(0.2),
		ReadDuplicateRanges: ptrInt(0),
	})
	if result.Passed {
		t.Fatal("expected duplicate read regression to fail")
	}
}

func ptrFloat(v float64) *float64 { return &v }
func ptrInt(v int) *int           { return &v }
