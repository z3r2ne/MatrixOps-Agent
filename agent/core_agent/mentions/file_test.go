package mentions

import "testing"

func TestExtractFileMentions(t *testing.T) {
	mentions, replaced := ExtractFileMentions(`请看 [README](file://default?filePath=README.md)`)
	if len(mentions) != 1 {
		t.Fatalf("expected 1 file mention, got %d", len(mentions))
	}
	if mentions[0].Path != "./README.md" {
		t.Fatalf("expected normalized path ./README.md, got %q", mentions[0].Path)
	}
	if replaced != "请看 ./README.md" {
		t.Fatalf("unexpected replaced text: %q", replaced)
	}
}
