package shellutil

import (
	"runtime"
	"strings"
	"testing"
)

func TestParseNullSeparatedEnv(t *testing.T) {
	env := parseNullSeparatedEnv([]byte("PATH=/bin\x00HOME=/tmp\x00\x00"))
	if env["PATH"] != "/bin" {
		t.Fatalf("PATH = %q, want /bin", env["PATH"])
	}
	if env["HOME"] != "/tmp" {
		t.Fatalf("HOME = %q, want /tmp", env["HOME"])
	}
}

func TestAugmentEnvPrefersLoginPATH(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("login shell env augmentation is unix-only")
	}

	base := []string{"PATH=/usr/bin:/bin", "HOME=/tmp"}
	merged := AugmentEnv(base)
	path := envValue(merged, "PATH")
	if path == "" {
		t.Fatal("expected PATH to be set")
	}
	if path == "/usr/bin:/bin" {
		t.Fatalf("PATH was not augmented: %q", path)
	}
	if !strings.Contains(path, "/usr/bin") {
		t.Fatalf("unexpected PATH: %q", path)
	}
}

func TestInteractiveShellArgsUsesLoginInteractiveOnUnix(t *testing.T) {
	args := InteractiveShellArgs(Info{ID: "zsh", Command: "zsh"})
	if runtime.GOOS == "windows" {
		if len(args) != 0 {
			t.Fatalf("unexpected windows args: %#v", args)
		}
		return
	}
	if len(args) != 1 || args[0] != "-il" {
		t.Fatalf("unexpected args: %#v", args)
	}
}

func TestShouldPreferLoginShellEnv(t *testing.T) {
	if !shouldPreferLoginShellEnv("PATH", "/usr/bin", "/opt/homebrew/bin:/usr/bin") {
		t.Fatal("expected PATH to prefer login shell value")
	}
	if shouldPreferLoginShellEnv("SHLVL", "1", "2") {
		t.Fatal("expected SHLVL to be ignored")
	}
	if !shouldPreferLoginShellEnv("CUSTOM_TOOL_HOME", "", "/opt/custom") {
		t.Fatal("expected missing vars to be filled from login shell")
	}
	if shouldPreferLoginShellEnv("CUSTOM_TOOL_HOME", "/keep", "/replace") {
		t.Fatal("expected existing custom vars to be preserved")
	}
}

func envValue(entries []string, key string) string {
	prefix := key + "="
	for _, entry := range entries {
		if strings.HasPrefix(entry, prefix) {
			return strings.TrimPrefix(entry, prefix)
		}
	}
	return ""
}
