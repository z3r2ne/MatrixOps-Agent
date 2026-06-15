package ansi

import (
	"strings"
	"testing"
)

func TestStripTerminal(t *testing.T) {
	in := "\x1b[31mM\x1b[m file.go"
	out := StripTerminal(in)
	if strings.Contains(out, "\x1b") {
		t.Fatalf("expected escapes stripped, got %q", out)
	}
	if out != "M file.go" {
		t.Fatalf("unexpected: %q", out)
	}
}
