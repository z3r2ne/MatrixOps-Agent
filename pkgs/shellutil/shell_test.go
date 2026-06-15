package shellutil

import (
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

	cmd, args, err := WrapCommand(Info{ID: "zsh", Command: "zsh"}, "echo hello")
	if err != nil {
		t.Fatalf("WrapCommand: %v", err)
	}
	if cmd != "zsh" {
		t.Fatalf("cmd = %q, want %q", cmd, "zsh")
	}
	if len(args) != 2 || args[0] != "-lc" {
		t.Fatalf("unexpected args: %#v", args)
	}
}
