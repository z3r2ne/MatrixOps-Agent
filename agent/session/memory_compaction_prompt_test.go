package session

import (
	"strings"
	"testing"
	"unicode/utf8"

	"matrixops-agent/types"
)

func TestSanitizeMemoryForCompactionPromptStripsInvalidUTF8AndTruncatesToolOutput(t *testing.T) {
	longOutput := strings.Repeat("x", compactionPromptMaxFieldRunes+100)
	memory := SanitizeMemoryForCompactionPrompt(&types.Memory{
		Entries: []*types.MemoryEntry{
			{
				Role:       "tool",
				ToolOutput: "before\x00after\xff\xfe" + longOutput,
			},
		},
	})
	if memory == nil || len(memory.Entries) != 1 {
		t.Fatalf("unexpected memory: %#v", memory)
	}
	got := memory.Entries[0].ToolOutput
	if !utf8.ValidString(got) {
		t.Fatalf("expected valid utf8, got %q", got)
	}
	if strings.Contains(got, "\x00") {
		t.Fatal("expected null bytes removed")
	}
	if len([]rune(got)) > compactionPromptMaxFieldRunes+len([]rune(compactionPromptTruncationNote))+2 {
		t.Fatalf("expected truncated tool output, got %d runes", len([]rune(got)))
	}
}

func TestCapCompactionPromptTranscriptLimitsTotalSize(t *testing.T) {
	input := strings.Repeat("字", compactionPromptMaxTotalRunes+50)
	got := capCompactionPromptTranscript(input)
	if len([]rune(got)) > compactionPromptMaxTotalRunes+len([]rune(compactionPromptTruncationNote))+2 {
		t.Fatalf("expected capped transcript, got %d runes", len([]rune(got)))
	}
}
