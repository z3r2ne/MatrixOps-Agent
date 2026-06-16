package testrunner

import (
	"path/filepath"
	"runtime"
	"testing"
)

func testScenariosDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "tests", "semantic_regression", "scenarios")
}

func TestLoadScenariosFromDirIncludesSemanticYAML(t *testing.T) {
	dir := testScenariosDir(t)
	all, err := LoadScenariosFromDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	scenario, ok := all["instruction_following_semantic"]
	if !ok {
		t.Fatal("expected instruction_following_semantic scenario")
	}
	if scenario.TaskInput == "" {
		t.Fatal("expected reused task input")
	}
	if scenario.BuildVerifyInput == nil {
		t.Fatal("expected verify input builder")
	}
}
