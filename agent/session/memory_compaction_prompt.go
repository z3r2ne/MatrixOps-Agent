package session

import (
	"strings"
	"unicode/utf8"

	"matrixops-agent/types"

	sessionmemory "matrixops-agent/session/memory"
	"pkgs/ansi"
)

const (
	compactionPromptMaxFieldRunes  = 6000
	compactionPromptMaxTotalRunes  = 120000
	compactionPromptTruncationNote = "\n…(truncated for compaction prompt)"
)

// SanitizeMemoryForCompactionPrompt clones memory entries and normalizes text fields
// before sending them to the compaction LLM (strip ANSI/null bytes, fix UTF-8, truncate).
func SanitizeMemoryForCompactionPrompt(memory *types.Memory) *types.Memory {
	if memory == nil {
		return nil
	}
	entries := sessionmemory.CloneMemoryEntries(memory.TranscriptSourceEntries())
	if len(entries) == 0 {
		return &types.Memory{}
	}
	out := make([]*types.MemoryEntry, 0, len(entries))
	for _, entry := range entries {
		if entry == nil {
			continue
		}
		out = append(out, sanitizeMemoryEntryForCompactionPrompt(entry))
	}
	return &types.Memory{Entries: out}
}

func sanitizeMemoryEntryForCompactionPrompt(entry *types.MemoryEntry) *types.MemoryEntry {
	clone := *entry
	clone.Content = sanitizeCompactionPromptText(clone.Content, compactionPromptMaxFieldRunes)
	clone.RawOutput = sanitizeCompactionPromptText(clone.RawOutput, compactionPromptMaxFieldRunes)
	clone.CallToolInfo = sanitizeCompactionPromptText(clone.CallToolInfo, compactionPromptMaxFieldRunes)
	clone.ToolRequestRawJSON = sanitizeCompactionPromptText(clone.ToolRequestRawJSON, compactionPromptMaxFieldRunes)
	clone.ToolInputJSON = sanitizeCompactionPromptText(clone.ToolInputJSON, compactionPromptMaxFieldRunes)
	clone.ToolOutput = sanitizeCompactionPromptText(clone.ToolOutput, compactionPromptMaxFieldRunes)
	clone.ToolError = sanitizeCompactionPromptText(clone.ToolError, compactionPromptMaxFieldRunes)
	clone.ToolTitle = sanitizeCompactionPromptText(clone.ToolTitle, compactionPromptMaxFieldRunes)
	clone.ToolMetadataJSON = sanitizeCompactionPromptText(clone.ToolMetadataJSON, compactionPromptMaxFieldRunes)
	return &clone
}

func sanitizeCompactionPromptText(value string, maxRunes int) string {
	value = strings.ReplaceAll(value, "\x00", "")
	value = ansi.StripTerminal(value)
	if !utf8.ValidString(value) {
		value = strings.ToValidUTF8(value, "")
	}
	value = strings.TrimSpace(value)
	if value == "" || maxRunes <= 0 {
		return value
	}
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return strings.TrimSpace(string(runes[:maxRunes])) + compactionPromptTruncationNote
}

func capCompactionPromptTranscript(transcript string) string {
	transcript = sanitizeCompactionPromptText(transcript, 0)
	if transcript == "" || compactionPromptMaxTotalRunes <= 0 {
		return transcript
	}
	runes := []rune(transcript)
	if len(runes) <= compactionPromptMaxTotalRunes {
		return transcript
	}
	return strings.TrimSpace(string(runes[:compactionPromptMaxTotalRunes])) + compactionPromptTruncationNote
}
