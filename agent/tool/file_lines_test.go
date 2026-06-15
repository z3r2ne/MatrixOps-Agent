package tool

import (
	"strings"
	"testing"
)

func TestFormatFileTextWithLineNumbers_fullFile(t *testing.T) {
	got, err := formatFileTextWithLineNumbers("alpha\nbeta\n", 0, 0)
	if err != nil {
		t.Fatalf("format: %v", err)
	}
	want := "1\talpha\n2\tbeta\n3\t"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFormatFileTextWithLineNumbers_offsetLimit(t *testing.T) {
	content := strings.Join([]string{"a", "b", "c", "d", "e"}, "\n")
	got, err := formatFileTextWithLineNumbers(content, 2, 2)
	if err != nil {
		t.Fatalf("format: %v", err)
	}
	want := "3\tc\n4\td"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestReadFileContent_offsetBeyondEnd(t *testing.T) {
	info := readFileContent("one\ntwo\nthree\n", 5, 0)
	if !info.BeyondEnd {
		t.Fatal("expected beyond_end")
	}
	if info.TotalLines != 4 {
		t.Fatalf("total lines: got %d want 4", info.TotalLines)
	}
	systemMessage := BuildReadToolSystemMessage(info)
	if !strings.Contains(systemMessage, "beyond end") {
		t.Fatalf("expected guidance in system message, got %q", systemMessage)
	}
	if info.Content != "" {
		t.Fatalf("beyond-end read should not include numbered lines in body, got %q", info.Content)
	}
}

func TestReadFileContent_defaultReadsWholeFileUpToMaxLines(t *testing.T) {
	lines := make([]string, 1200)
	for i := range lines {
		lines[i] = "x"
	}
	content := strings.Join(lines, "\n")
	info := readFileContent(content, 0, 0)
	if info.ReturnedLines != maxReadLines {
		t.Fatalf("returned lines: got %d want %d", info.ReturnedLines, maxReadLines)
	}
	if !info.HasMore {
		t.Fatal("expected has_more")
	}
	if !info.MaxLinesReached {
		t.Fatal("expected max_lines_reached")
	}
	systemMessage := BuildReadToolSystemMessage(info)
	if !strings.Contains(systemMessage, "Max 1000 lines reached") {
		t.Fatalf("expected max-lines hint in system message, got %q", systemMessage)
	}
	if strings.Contains(info.Content, "---\n[read]") {
		t.Fatalf("pagination footer should not be appended to body, got tail %q", info.Content[len(info.Content)-160:])
	}
}

func TestReadFileContent_smallFileReadsWholeWithoutPaginationHint(t *testing.T) {
	content := strings.Join([]string{"a", "b", "c"}, "\n")
	info := readFileContent(content, 0, 0)
	if info.ReturnedLines != 3 {
		t.Fatalf("returned lines: got %d want 3", info.ReturnedLines)
	}
	if info.HasMore {
		t.Fatal("expected no has_more for small file")
	}
	if strings.Contains(info.Content, "---\n[read]") {
		t.Fatalf("small whole-file read should not append footer, got %q", info.Content)
	}
}

func TestReadFileContent_byteCapTruncatesBeforeLineCap(t *testing.T) {
	longLine := strings.Repeat("x", maxReadBytes)
	content := strings.Join([]string{longLine, "tail"}, "\n")
	info := readFileContent(content, 0, 0)
	if info.ReturnedLines != 1 {
		t.Fatalf("returned lines: got %d want 1", info.ReturnedLines)
	}
	if !info.MaxBytesReached {
		t.Fatal("expected max_bytes_reached")
	}
	if !info.HasMore {
		t.Fatal("expected has_more after byte truncation")
	}
}

func TestReadFileContent_explicitLimitPaginates(t *testing.T) {
	content := strings.Join([]string{"a", "b", "c", "d", "e"}, "\n")
	info := readFileContent(content, 0, 2)
	if !info.HasMore {
		t.Fatal("expected has_more when limit is smaller than file")
	}
	if info.ReturnedLines != 2 {
		t.Fatalf("returned lines: got %d want 2", info.ReturnedLines)
	}
	if strings.Contains(info.Content, "---\n[read]") {
		t.Fatalf("explicit limit should not append footer hint, got %q", info.Content)
	}
	if info.OffsetUsed+info.ReturnedLines != 2 {
		t.Fatalf("next_offset should be 2, metadata path uses offset+returned")
	}
}

func TestBuildReadToolSystemMessageFromMetadata(t *testing.T) {
	msg := BuildReadToolSystemMessageFromMetadata(map[string]interface{}{
		"total_lines":    1200,
		"lines_returned": 1000,
		"start_line":     1,
		"has_more":       true,
		"max_lines_reached": true,
	})
	if !strings.Contains(msg, "1000 lines read") {
		t.Fatalf("expected lines read summary, got %q", msg)
	}
	if !strings.Contains(msg, "Max 1000 lines reached") {
		t.Fatalf("expected max lines hint, got %q", msg)
	}
}

func TestAppendReadPaginationMetadata(t *testing.T) {
	meta := map[string]interface{}{"fileRead": map[string]interface{}{}}
	appendReadPaginationMetadata(meta, FileReadInfo{
		TotalLines:      100,
		ReturnedLines:   50,
		OffsetUsed:      10,
		LimitUsed:       50,
		HasMore:         true,
		StartLine:       11,
		MaxBytesReached: true,
	})
	if meta["next_offset"] != 60 {
		t.Fatalf("next_offset: got %v want 60", meta["next_offset"])
	}
	if meta["total_lines"] != 100 {
		t.Fatalf("total_lines: got %v", meta["total_lines"])
	}
	if meta["max_bytes_reached"] != true {
		t.Fatalf("max_bytes_reached: got %v", meta["max_bytes_reached"])
	}
}
