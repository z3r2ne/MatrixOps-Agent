package semantic_regression

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

func packageDir() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "."
	}
	return filepath.Dir(file)
}

func scenariosDir() string {
	return filepath.Join(packageDir(), "scenarios")
}

func baselinesDir() string {
	return filepath.Join(packageDir(), "baselines")
}

func semregEnabled() bool {
	return strings.TrimSpace(os.Getenv("SEMREG_ENABLE")) == "1"
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func envUint(key string) (uint, bool) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return 0, false
	}
	parsed, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || parsed == 0 {
		return 0, false
	}
	return uint(parsed), true
}

func resolveScenarioPath(baseDir, relative string) string {
	relative = strings.TrimSpace(relative)
	if relative == "" {
		return ""
	}
	if filepath.IsAbs(relative) {
		return relative
	}
	return filepath.Clean(filepath.Join(baseDir, relative))
}
