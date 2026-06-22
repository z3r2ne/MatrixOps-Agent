package semreg_runner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func resolveSemanticRegressionDirs() (scenariosDir, baselinesDir string, err error) {
	candidates := make([]string, 0, 4)
	if root := strings.TrimSpace(os.Getenv("MATRIXOPS_REPO_ROOT")); root != "" {
		candidates = append(candidates, root)
	}
	if cwd, cwdErr := os.Getwd(); cwdErr == nil {
		candidates = append(candidates, cwd)
	}
	if exe, exeErr := os.Executable(); exeErr == nil {
		candidates = append(candidates, filepath.Dir(exe))
	}

	seen := map[string]struct{}{}
	for _, start := range candidates {
		start = strings.TrimSpace(start)
		if start == "" {
			continue
		}
		if root := findRepoRootFrom(start); root != "" {
			if _, ok := seen[root]; ok {
				continue
			}
			seen[root] = struct{}{}
			sd := filepath.Join(root, "tests", "semantic_regression", "scenarios")
			bd := filepath.Join(root, "tests", "semantic_regression", "baselines")
			if stat, statErr := os.Stat(sd); statErr == nil && stat.IsDir() {
				return sd, bd, nil
			}
		}
	}
	return "", "", fmt.Errorf("未找到语义测试场景目录 tests/semantic_regression/scenarios")
}

func findRepoRootFrom(start string) string {
	dir, err := filepath.Abs(start)
	if err != nil {
		return ""
	}
	for i := 0; i < 12; i++ {
		if _, err := os.Stat(filepath.Join(dir, "tests", "semantic_regression", "scenarios")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}
