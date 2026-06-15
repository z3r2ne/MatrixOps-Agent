package shellutil

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

const loginShellEnvTimeout = 5 * time.Second

var loginShellPreferKeys = map[string]struct{}{
	"PATH":             {},
	"GOPATH":           {},
	"GOROOT":           {},
	"GOBIN":            {},
	"NVM_DIR":          {},
	"PNPM_HOME":        {},
	"HOMEBREW_PREFIX":  {},
	"HOMEBREW_CELLAR":  {},
	"HOMEBREW_REPOSITORY": {},
}

// ApplyLoginShellEnv merges environment variables from the user's login shell
// into the current process. This helps GUI-launched servers inherit PATH and
// tool-specific variables configured in shell profiles.
func ApplyLoginShellEnv() {
	merged := AugmentEnv(os.Environ())
	for _, entry := range merged {
		key, value, ok := strings.Cut(entry, "=")
		if !ok || key == "" {
			continue
		}
		_ = os.Setenv(key, value)
	}
}

// AugmentEnv returns env with login-shell values merged in.
func AugmentEnv(base []string) []string {
	if runtime.GOOS == "windows" {
		return append([]string(nil), base...)
	}

	login := resolveLoginShellEnv()
	if len(login) == 0 {
		return append([]string(nil), base...)
	}

	merged := envSliceToMap(base)
	for key, value := range login {
		if shouldPreferLoginShellEnv(key, merged[key], value) {
			merged[key] = value
		}
	}
	return envMapToSlice(merged)
}

// InteractiveShellArgs returns shell flags for an interactive terminal session.
func InteractiveShellArgs(info Info) []string {
	switch normalizeID(info) {
	case "cmd":
		return nil
	case "pwsh", "powershell":
		return []string{"-NoLogo"}
	case "fish":
		return []string{"-l"}
	default:
		if runtime.GOOS == "windows" {
			return nil
		}
		return []string{"-il"}
	}
}

func resolveLoginShellEnv() map[string]string {
	info := Current()
	executable := strings.TrimSpace(info.Path)
	if executable == "" {
		executable = strings.TrimSpace(info.Command)
	}
	if executable == "" {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), loginShellEnvTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, executable, "-lc", "env -0")
	cmd.Env = os.Environ()
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	return parseNullSeparatedEnv(out)
}

func shouldPreferLoginShellEnv(key, current, login string) bool {
	if login == "" || isShellTransientKey(key) {
		return false
	}
	if _, ok := loginShellPreferKeys[key]; ok {
		return true
	}
	return strings.TrimSpace(current) == ""
}

func isShellTransientKey(key string) bool {
	switch key {
	case "_", "SHLVL", "PWD", "OLDPWD", "TERM", "COLORTERM":
		return true
	default:
		return false
	}
}

func parseNullSeparatedEnv(raw []byte) map[string]string {
	env := make(map[string]string)
	if len(raw) == 0 {
		return env
	}
	for _, entry := range strings.Split(string(raw), "\x00") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		key, value, ok := strings.Cut(entry, "=")
		if !ok || key == "" {
			continue
		}
		env[key] = value
	}
	return env
}

func envSliceToMap(entries []string) map[string]string {
	env := make(map[string]string, len(entries))
	for _, entry := range entries {
		key, value, ok := strings.Cut(entry, "=")
		if !ok || key == "" {
			continue
		}
		env[key] = value
	}
	return env
}

func envMapToSlice(env map[string]string) []string {
	entries := make([]string, 0, len(env))
	for key, value := range env {
		entries = append(entries, key+"="+value)
	}
	return entries
}
