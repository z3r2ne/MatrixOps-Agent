package session

import (
	"fmt"
	"strings"
)

const (
	minRepeatedPhraseRunes = 12
	minRepeatCount         = 3
	maxRepeatedPhraseRunes = 200
)

func sanitizeMemoryCompactionSummary(summary string) (string, error) {
	summary = strings.TrimSpace(summary)
	if summary == "" {
		return "", fmt.Errorf("memory compaction summary is empty")
	}
	if truncated, changed := truncateRepetitiveCompactionSummary(summary); changed {
		summary = strings.TrimSpace(truncated)
	}
	if truncated, changed := truncateAgentPlanningLoop(summary); changed {
		summary = strings.TrimSpace(truncated)
	}
	if summary == "" {
		return "", fmt.Errorf("memory compaction summary is repetitive")
	}
	if looksLikeAgentPlanningLoop(summary) {
		return "", fmt.Errorf("memory compaction summary looks like agent planning instead of a summary")
	}
	return summary, nil
}

func truncateRepetitiveCompactionSummary(summary string) (string, bool) {
	if truncated, ok := truncateConsecutiveRepeat(summary); ok {
		return truncated, true
	}
	if truncated, ok := truncateDominantRepeatedPhrase(summary); ok {
		return truncated, true
	}
	return summary, false
}

func truncateConsecutiveRepeat(summary string) (string, bool) {
	runes := []rune(summary)
	n := len(runes)
	if n < minRepeatedPhraseRunes*minRepeatCount {
		return summary, false
	}

	maxPeriod := n / minRepeatCount
	if maxPeriod > maxRepeatedPhraseRunes {
		maxPeriod = maxRepeatedPhraseRunes
	}
	for period := minRepeatedPhraseRunes; period <= maxPeriod; period++ {
		chunk := string(runes[:period])
		count := 1
		pos := period
		for pos+period <= n && string(runes[pos:pos+period]) == chunk {
			count++
			pos += period
		}
		if count >= minRepeatCount {
			return chunk, true
		}
	}
	return summary, false
}

func truncateDominantRepeatedPhrase(summary string) (string, bool) {
	runes := []rune(summary)
	n := len(runes)
	if n < minRepeatedPhraseRunes*minRepeatCount {
		return summary, false
	}

	maxPhraseLen := maxRepeatedPhraseRunes
	if maxPhraseLen > n/2 {
		maxPhraseLen = n / 2
	}

	type candidate struct {
		phrase string
		count  int
		start  int
	}

	best := candidate{}
	for length := minRepeatedPhraseRunes; length <= maxPhraseLen; length++ {
		counts := make(map[string]int)
		first := make(map[string]int)
		for i := 0; i+length <= n; i++ {
			phrase := string(runes[i : i+length])
			counts[phrase]++
			if _, ok := first[phrase]; !ok {
				first[phrase] = i
			}
		}
		for phrase, count := range counts {
			if count < minRepeatCount {
				continue
			}
			covered := count * length
			if covered*100/n < 55 {
				continue
			}
			if count > best.count || (count == best.count && len([]rune(phrase)) > len([]rune(best.phrase))) {
				best = candidate{
					phrase: phrase,
					count:  count,
					start:  first[phrase],
				}
			}
		}
	}
	if best.count < minRepeatCount {
		return summary, false
	}
	end := best.start + len([]rune(best.phrase))
	return string(runes[:end]), true
}

var agentPlanningLoopMarkers = []string{
	"用户要求",
	"让我开始",
	"我需要",
	"开始实施",
	"逐步重构",
	"逐步实施",
	"让我开始逐步",
}

func truncateAgentPlanningLoop(summary string) (string, bool) {
	trimmed := strings.TrimSpace(summary)
	if trimmed == "" {
		return summary, false
	}

	for _, marker := range agentPlanningLoopMarkers {
		if count := strings.Count(trimmed, marker); count >= 2 {
			if truncated, ok := truncateAtSecondOccurrence(trimmed, marker); ok {
				return truncated, true
			}
		}
	}

	return summary, false
}

func truncateAtSecondOccurrence(text, marker string) (string, bool) {
	first := strings.Index(text, marker)
	if first < 0 {
		return text, false
	}
	rest := text[first+len(marker):]
	secondRel := strings.Index(rest, marker)
	if secondRel < 0 {
		return text, false
	}
	cutAt := first + len(marker) + secondRel
	candidate := strings.TrimSpace(text[:cutAt])
	if len([]rune(candidate)) < 40 {
		return text, false
	}
	return candidate, true
}

func looksLikeAgentPlanningLoop(summary string) bool {
	trimmed := strings.TrimSpace(summary)
	if trimmed == "" {
		return false
	}

	if strings.Contains(trimmed, "让我开始") {
		return true
	}

	markerHits := 0
	for _, marker := range agentPlanningLoopMarkers {
		if strings.Contains(trimmed, marker) {
			markerHits++
		}
	}
	if strings.Count(trimmed, "用户要求") >= 2 {
		return true
	}
	if strings.Count(trimmed, "让我开始") >= 2 {
		return true
	}
	// One long paragraph full of execution voice but no factual summary structure.
	if markerHits >= 3 && len([]rune(trimmed)) > 180 && !strings.Contains(trimmed, "已完成") && !strings.Contains(trimmed, "当前") {
		return true
	}
	return false
}
