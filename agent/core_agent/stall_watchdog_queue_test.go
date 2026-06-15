package coreagent

import (
	"strings"
	"testing"
	"time"
)

func TestFormatStallWatchdogToolCancelledWarning_HasSystemTag(t *testing.T) {
	msg := FormatStallWatchdogToolCancelledWarning("bash", "taking too long", 12*time.Second)
	if !strings.HasPrefix(msg, "<system>") || !strings.HasSuffix(msg, "</system>") {
		t.Fatalf("message %q missing <system> wrapper", msg)
	}
	if !strings.Contains(msg, "bash") {
		t.Fatalf("message %q missing tool name", msg)
	}
	if !strings.Contains(msg, "taking too long") {
		t.Fatalf("message %q missing cancel reason", msg)
	}
}
