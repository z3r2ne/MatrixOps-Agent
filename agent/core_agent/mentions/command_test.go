package mentions

import "testing"

func TestExtractCommandMentionsFromText(t *testing.T) {
	mentions, replaced := ExtractCommandMentionsFromText(`先执行 [compress](command://default?name=compress) 再继续`)
	if len(mentions) != 1 {
		t.Fatalf("expected 1 command mention, got %d", len(mentions))
	}
	if mentions[0].Name != "compress" {
		t.Fatalf("expected command name compress, got %q", mentions[0].Name)
	}
	if replaced != "先执行  再继续" {
		t.Fatalf("unexpected replaced text: %q", replaced)
	}
}
