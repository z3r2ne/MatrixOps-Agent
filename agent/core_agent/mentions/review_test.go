package mentions

import (
	"strings"
	"testing"
)

func TestExtractReviewMentions(t *testing.T) {
	mentions, replaced := ExtractReviewMentions(`做一下 [review](review://default?fromType=branch&from=main&toType=branch&to=feat)`)
	if len(mentions) != 1 {
		t.Fatalf("expected 1 review mention, got %d", len(mentions))
	}
	if !strings.Contains(replaced, "review from branch:main to branch:feat") {
		t.Fatalf("unexpected replaced text: %q", replaced)
	}
}
