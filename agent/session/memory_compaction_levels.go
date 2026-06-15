package session

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"matrixops-agent/types"

	sessionmemory "matrixops-agent/session/memory"
)

const memoryEntryKindCompactionUser = "compaction_user"

type MemoryCompactionLevelLog struct {
	Level        int    `json:"level"`
	Action       string `json:"action"`
	Reason       string `json:"reason"`
	EntryIDs     []uint `json:"entryIds,omitempty"`
	SequenceFrom int64  `json:"sequenceFrom,omitempty"`
	SequenceTo   int64  `json:"sequenceTo,omitempty"`
	BeforeTokens int    `json:"beforeTokens"`
	AfterTokens  int    `json:"afterTokens"`
}

type GraduatedMemoryCompactionConfig struct {
	TriggerThresholdPercent int
	TargetPercent           int
	L2ScopePercent          int
	Force                   bool
}

type GraduatedMemoryCompactionInput struct {
	SessionID string
	Entries   []*types.MemoryEntry
	LimitTokens int
	CurrentTokens int
	Config    GraduatedMemoryCompactionConfig
	UserPrompt string
	Summarize  func(scopeMemory *types.Memory, userPrompt string, onDelta func(string)) (summary string, promptInfo compactionPromptInfo, err error)
	OnSummaryDelta func(string)
}

type GraduatedMemoryCompactionResult struct {
	Entries         []*types.MemoryEntry
	LevelsExecuted  []int
	Logs            []MemoryCompactionLevelLog
	Summary         string
	RequestPrompt   string
	BeforeTokens    int
	AfterTokens     int
	Skipped         bool
	SkipReason      string
}

func memoryUsagePercent(entries []*types.MemoryEntry, limitTokens int) int {
	if limitTokens <= 0 {
		return 0
	}
	current := totalMemoryTokens(entries)
	return current * 100 / limitTokens
}

func meetsMemoryCompactionTarget(entries []*types.MemoryEntry, limitTokens, targetPercent int) bool {
	if targetPercent <= 0 {
		return true
	}
	return memoryUsagePercent(entries, limitTokens) <= targetPercent
}

func shouldTriggerMemoryCompaction(currentTokens, limitTokens, triggerPercent int, force bool) bool {
	if force {
		return true
	}
	if limitTokens <= 0 || triggerPercent <= 0 {
		return false
	}
	return currentTokens*100/limitTokens >= triggerPercent
}

func findLastRealUserEntry(entries []*types.MemoryEntry) *types.MemoryEntry {
	for index := len(entries) - 1; index >= 0; index-- {
		entry := entries[index]
		if entry == nil || entry.Synthetic {
			continue
		}
		if strings.TrimSpace(entry.Role) != "user" {
			continue
		}
		switch strings.TrimSpace(entry.EntryKind) {
		case "summary_user", memoryEntryKindCompactionUser:
			continue
		}
		return entry
	}
	return nil
}

func isL2CompactionCandidate(entry *types.MemoryEntry) bool {
	if entry == nil || isProtectedCompactionUserEntry(entry) {
		return false
	}
	switch entry.CompressionLevel {
	case 0, 1, 2:
		return true
	default:
		return false
	}
}

func selectL2BatchEntries(entries []*types.MemoryEntry, l2ScopePercent int) []*types.MemoryEntry {
	if l2ScopePercent <= 0 {
		l2ScopePercent = 80
	}
	candidates := make([]*types.MemoryEntry, 0, len(entries))
	for _, entry := range entries {
		if isL2CompactionCandidate(entry) {
			candidates = append(candidates, entry)
		}
	}
	if len(candidates) == 0 {
		return nil
	}
	totalTokens := 0
	for _, entry := range candidates {
		totalTokens += memoryEntryTokenCount(entry)
	}
	if totalTokens <= 0 {
		return append([]*types.MemoryEntry(nil), candidates[0])
	}
	target := totalTokens * l2ScopePercent / 100
	if target <= 0 {
		target = 1
	}
	selected := make([]*types.MemoryEntry, 0, len(candidates))
	accumulated := 0
	for _, entry := range candidates {
		selected = append(selected, entry)
		accumulated += memoryEntryTokenCount(entry)
		if accumulated >= target {
			break
		}
	}
	return selected
}

func memoryEntryTokenCount(entry *types.MemoryEntry) int {
	if entry == nil {
		return 0
	}
	if entry.TokenCount > 0 {
		return entry.TokenCount
	}
	return estimateMemoryEntryTokenCount(entry)
}

func batchContainsEntryID(batch []*types.MemoryEntry, entryID uint) bool {
	if entryID == 0 {
		return false
	}
	for _, entry := range batch {
		if entry != nil && entry.ID == entryID {
			return true
		}
	}
	return false
}

func batchEntryIDs(batch []*types.MemoryEntry) []uint {
	ids := make([]uint, 0, len(batch))
	for _, entry := range batch {
		if entry != nil && entry.ID > 0 {
			ids = append(ids, entry.ID)
		}
	}
	return ids
}

func batchSequenceRange(batch []*types.MemoryEntry) (int64, int64) {
	if len(batch) == 0 {
		return 0, 0
	}
	from := batch[0].Sequence
	to := batch[len(batch)-1].Sequence
	return from, to
}

func buildPreservedUserEntry(sessionID string, source *types.MemoryEntry, level int, startSequence, startCreated int64) *types.MemoryEntry {
	if source == nil {
		return nil
	}
	now := time.Now().UnixMilli()
	entry := &types.MemoryEntry{
		SessionID:        sessionID,
		SourceMessageID:  source.SourceMessageID,
		SourcePartID:     source.SourcePartID,
		EntryKind:        memoryEntryKindCompactionUser,
		Role:             "user",
		Content:          strings.TrimSpace(source.Content),
		Synthetic:        false,
		CompressionLevel: level,
		Sequence:         startSequence,
		Created:          startCreated,
		Updated:          now,
	}
	entry.TokenCount = estimateMemoryEntryTokenCount(entry)
	return entry
}

func buildLevel2ReplacementEntries(sessionID string, batch []*types.MemoryEntry, summary string, preservedUser *types.MemoryEntry, preservedLevel int) []*types.MemoryEntry {
	if len(batch) == 0 {
		return nil
	}
	startSequence := batch[0].Sequence
	startCreated := batch[0].Created
	now := time.Now().UnixMilli()

	summaryUser := &types.MemoryEntry{
		SessionID:        sessionID,
		EntryKind:        "summary_user",
		Role:             "user",
		Content:          "总结一下之前都做了什么。",
		Synthetic:        true,
		CompressionLevel: 2,
		Sequence:         startSequence,
		Created:          startCreated,
		Updated:          now,
	}
	summaryUser.TokenCount = estimateMemoryEntryTokenCount(summaryUser)

	summaryAssistant := &types.MemoryEntry{
		SessionID:        sessionID,
		EntryKind:        "summary_assistant",
		Role:             "assistant",
		Content:          strings.TrimSpace(summary),
		Synthetic:        true,
		CompressionLevel: 2,
		Sequence:         startSequence + 1,
		Created:          startCreated + 1,
		Updated:          now,
	}
	summaryAssistant.TokenCount = estimateMemoryEntryTokenCount(summaryAssistant)

	replacement := []*types.MemoryEntry{summaryUser, summaryAssistant}
	nextSequence := startSequence + 2
	nextCreated := startCreated + 2
	if preservedUser != nil {
		preserved := buildPreservedUserEntry(sessionID, preservedUser, preservedLevel, nextSequence, nextCreated)
		if preserved != nil {
			replacement = append(replacement, preserved)
		}
	}
	return replacement
}

func replaceEntryBatch(entries []*types.MemoryEntry, batch []*types.MemoryEntry, replacement []*types.MemoryEntry) []*types.MemoryEntry {
	if len(batch) == 0 {
		return entries
	}
	batchSet := map[*types.MemoryEntry]struct{}{}
	for _, entry := range batch {
		batchSet[entry] = struct{}{}
	}
	firstIndex := -1
	for index, entry := range entries {
		if _, ok := batchSet[entry]; ok {
			firstIndex = index
			break
		}
	}
	if firstIndex < 0 {
		return entries
	}
	lastIndex := firstIndex
	for index := firstIndex; index < len(entries); index++ {
		if _, ok := batchSet[entries[index]]; ok {
			lastIndex = index
		}
	}
	out := make([]*types.MemoryEntry, 0, len(entries)-len(batch)+len(replacement))
	out = append(out, entries[:firstIndex]...)
	out = append(out, replacement...)
	out = append(out, entries[lastIndex+1:]...)
	return out
}

func memoryFromEntries(entries []*types.MemoryEntry) *types.Memory {
	return &types.Memory{Entries: sessionmemory.CloneMemoryEntries(entries)}
}

func isProtectedCompactionUserEntry(entry *types.MemoryEntry) bool {
	if entry == nil {
		return false
	}
	return strings.TrimSpace(entry.EntryKind) == memoryEntryKindCompactionUser &&
		entry.CompressionLevel == 2
}

func shouldRunGraduatedMemoryCompaction(input GraduatedMemoryCompactionInput) (bool, string) {
	if len(input.Entries) == 0 {
		return false, "empty_entries"
	}
	if input.LimitTokens <= 0 {
		return false, "missing_limit"
	}
	currentTokens := input.CurrentTokens
	if currentTokens <= 0 {
		currentTokens = totalMemoryTokens(input.Entries)
	}
	if !shouldTriggerMemoryCompaction(currentTokens, input.LimitTokens, input.Config.TriggerThresholdPercent, input.Config.Force) {
		return false, "below_trigger_threshold"
	}
	return true, ""
}

func RunGraduatedMemoryCompaction(input GraduatedMemoryCompactionInput) (GraduatedMemoryCompactionResult, error) {
	result := GraduatedMemoryCompactionResult{}
	if len(input.Entries) == 0 {
		result.Skipped = true
		result.SkipReason = "empty_entries"
		return result, nil
	}
	if input.LimitTokens <= 0 {
		result.Skipped = true
		result.SkipReason = "missing_limit"
		return result, nil
	}

	entries := sessionmemory.CloneMemoryEntries(input.Entries)
	currentTokens := input.CurrentTokens
	if currentTokens <= 0 {
		currentTokens = totalMemoryTokens(entries)
	}
	result.BeforeTokens = currentTokens

	if !shouldTriggerMemoryCompaction(currentTokens, input.LimitTokens, input.Config.TriggerThresholdPercent, input.Config.Force) {
		result.Skipped = true
		result.SkipReason = "below_trigger_threshold"
		result.Entries = entries
		result.AfterTokens = currentTokens
		return result, nil
	}

	allLogs := make([]MemoryCompactionLevelLog, 0)
	levelsExecuted := make([]int, 0)
	l2Executed := false

	for !meetsMemoryCompactionTarget(entries, input.LimitTokens, input.Config.TargetPercent) {
		lastUser := findLastRealUserEntry(entries)
		batch := selectL2BatchEntries(entries, input.Config.L2ScopePercent)
		if len(batch) == 0 || input.Summarize == nil {
			break
		}
		before := totalMemoryTokens(entries)
		preserveUser := batchContainsEntryID(batch, lastUserID(lastUser))
		scopeMemory := SanitizeMemoryForCompactionPrompt(memoryFromEntries(batch))
		summary, promptInfo, err := input.Summarize(scopeMemory, input.UserPrompt, input.OnSummaryDelta)
		if err != nil {
			return result, err
		}
		if strings.TrimSpace(summary) == "" {
			return result, fmt.Errorf("memory compaction summary is empty")
		}
		var preserved *types.MemoryEntry
		if preserveUser {
			preserved = lastUser
		}
		replacement := buildLevel2ReplacementEntries(input.SessionID, batch, summary, preserved, 2)
		entries = replaceEntryBatch(entries, batch, replacement)
		after := totalMemoryTokens(entries)
		seqFrom, seqTo := batchSequenceRange(batch)
		allLogs = append(allLogs, MemoryCompactionLevelLog{
			Level:        2,
			Action:       "summarize_batch",
			Reason:       "level2_llm_summary",
			EntryIDs:     batchEntryIDs(batch),
			SequenceFrom: seqFrom,
			SequenceTo:   seqTo,
			BeforeTokens: before,
			AfterTokens:  after,
		})
		l2Executed = true
		result.Summary = summary
		result.RequestPrompt = promptInfo.Combined()
		if after >= before {
			break
		}
	}
	if l2Executed {
		levelsExecuted = appendLevel(levelsExecuted, 2)
	}

	result.Entries = entries
	result.Logs = allLogs
	result.LevelsExecuted = levelsExecuted
	result.AfterTokens = totalMemoryTokens(entries)
	return result, nil
}

func lastUserID(user *types.MemoryEntry) uint {
	if user == nil {
		return 0
	}
	return user.ID
}

func appendLevel(levels []int, level int) []int {
	for _, existing := range levels {
		if existing == level {
			return levels
		}
	}
	return append(levels, level)
}

func formatCompactionLevelLogs(logs []MemoryCompactionLevelLog) string {
	if len(logs) == 0 {
		return ""
	}
	payload, err := json.Marshal(logs)
	if err != nil {
		return ""
	}
	return string(payload)
}
