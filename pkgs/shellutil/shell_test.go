package shellutil

import (
	"os/exec"
	"runtime"
	"testing"
)

func TestNormalizeForInitUsesCurrentShellWhenUnset(t *testing.T) {
	selected, custom := NormalizeForInit("", "")
	if selected == "" {
		t.Fatal("expected selected shell to be non-empty")
	}
	if selected == ShellCustom && custom == "" {
		t.Fatal("expected custom shell command when selected shell is custom")
	}
}

func TestWrapCommandUsesPlatformSpecificFlags(t *testing.T) {
	if runtime.GOOS == "windows" {
		cmd, args, err := WrapCommand(Info{ID: "cmd", Command: "cmd"}, "echo hello")
		if err != nil {
			t.Fatalf("WrapCommand: %v", err)
		}
		if cmd != "cmd" {
			t.Fatalf("cmd = %q, want %q", cmd, "cmd")
		}
		if len(args) != 2 || args[0] != "/C" {
			t.Fatalf("unexpected args: %#v", args)
		}
		return
	}

	bashPath, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash not available")
	}
	cmd, args, err := WrapCommand(Info{ID: "bash", Command: "bash", Path: bashPath}, "echo hello")
	if err != nil {
		t.Fatalf("WrapCommand: %v", err)
	}
	if cmd != bashPath {
		t.Fatalf("cmd = %q, want %q", cmd, bashPath)
	}
	if len(args) != 2 || args[0] != "-lc" {
		t.Fatalf("unexpected args: %#v", args)
	}
}
