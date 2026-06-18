package session

import (
	"strings"
	"testing"
)

func TestParseMemoryAnalysisResponse(t *testing.T) {
	analysis, err := parseMemoryAnalysisResponse(`{"keywords":["Go","记忆压缩","API"],"summary":"会话围绕记忆压缩与 API 设计展开。"}`)
	if err != nil {
		t.Fatalf("parseMemoryAnalysisResponse: %v", err)
	}
	if len(analysis.Keywords) != 3 {
		t.Fatalf("keywords = %#v", analysis.Keywords)
	}
	if analysis.Summary == "" {
		t.Fatal("expected summary")
	}
}

func TestParseMemoryAnalysisResponseTrimsMarkdownFence(t *testing.T) {
	raw := "```json\n{\"keywords\":[\"测试\"],\"summary\":\"这是一段总结。\"}\n```"
	analysis, err := parseMemoryAnalysisResponse(raw)
	if err != nil {
		t.Fatalf("parseMemoryAnalysisResponse: %v", err)
	}
	if analysis.Keywords[0] != "测试" {
		t.Fatalf("keywords = %#v", analysis.Keywords)
	}
}

func TestParseMemoryAnalysisResponseTruncatesLongSummary(t *testing.T) {
	longSummary := strings.Repeat("测", memoryAnalysisSummaryMaxRunes+20)
	analysis, err := parseMemoryAnalysisResponse(`{"keywords":["长文"],"summary":"` + longSummary + `"}`)
	if err != nil {
		t.Fatalf("parseMemoryAnalysisResponse: %v", err)
	}
	if len([]rune(analysis.Summary)) != memoryAnalysisSummaryMaxRunes {
		t.Fatalf("summary rune count = %d, want %d", len([]rune(analysis.Summary)), memoryAnalysisSummaryMaxRunes)
	}
}
