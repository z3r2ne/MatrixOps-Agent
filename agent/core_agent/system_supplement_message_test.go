package coreagent

import "testing"

func TestFormatSystemSupplementMessage_WrapsPlainBody(t *testing.T) {
	got := FormatSystemSupplementMessage("继续")
	want := "<system>继续</system>"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFormatSystemSupplementMessage_StripsLegacyPrefix(t *testing.T) {
	got := FormatSystemSupplementMessage(SystemMessageQueuePrefix + " 提醒内容")
	if got != "<system>提醒内容</system>" {
		t.Fatalf("got %q", got)
	}
}

func TestFormatSystemSupplementMessage_Idempotent(t *testing.T) {
	wrapped := "<system>already wrapped</system>"
	if FormatSystemSupplementMessage(wrapped) != wrapped {
		t.Fatalf("expected idempotent wrap")
	}
}
