//go:build explorecompare

package explore_comparison

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestAnalyzeTask833 从本地数据库提取 task 833 的 trace 并做基础断言。
// 运行: go test -tags explorecompare ./tests/explore_comparison/ -run TestAnalyzeTask833 -v
func TestAnalyzeTask833(t *testing.T) {
	db := filepath.Join(os.Getenv("HOME"), ".matrixops", "matrixops.db")
	if _, err := os.Stat(db); err != nil {
		t.Skipf("本地数据库不存在，跳过: %v", err)
	}

	repoRoot := findRepoRoot(t)
	script := filepath.Join(repoRoot, "tests", "explore_comparison", "collect_trace.py")

	outFile := filepath.Join(t.TempDir(), "task833.json")
	cmd := exec.Command("python3", script, "--task-id", "833", "--db", db, "--output", outFile)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("collect_trace: %v\n%s", err, out)
	}

	raw, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatal(err)
	}
	var trace struct {
		Summary struct {
			TotalToolCalls int            `json:"total_tool_calls"`
			ReadTotal      int            `json:"read_total"`
			ToolCounts     map[string]int `json:"tool_counts"`
		} `json:"summary"`
	}
	if err := json.Unmarshal(raw, &trace); err != nil {
		t.Fatal(err)
	}

	if trace.Summary.ReadTotal < 50 {
		t.Fatalf("task 833 预期有大量 read 调用，got %d", trace.Summary.ReadTotal)
	}
	if trace.Summary.ToolCounts["read"] != trace.Summary.ReadTotal {
		t.Fatalf("read 计数不一致: %v", trace.Summary.ToolCounts)
	}
	t.Logf("task833: total=%d read=%d", trace.Summary.TotalToolCalls, trace.Summary.ReadTotal)
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			if _, err := os.Stat(filepath.Join(dir, "agent")); err == nil {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return wd
		}
		dir = parent
	}
}
