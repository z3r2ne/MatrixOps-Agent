package mentions

import "testing"

func TestExtractWorkerMentionsFromText(t *testing.T) {
	mentions, replaced := ExtractWorkerMentionsFromText(`交给 [chat](worker://default?name=chat) 处理`)
	if len(mentions) != 1 {
		t.Fatalf("expected 1 worker mention, got %d", len(mentions))
	}
	if mentions[0].Name != "chat" {
		t.Fatalf("expected worker name chat, got %q", mentions[0].Name)
	}
	if replaced != "交给 @chat 处理" {
		t.Fatalf("unexpected replaced text: %q", replaced)
	}
}

func TestCollectWorkerMentionNames(t *testing.T) {
	workers := CollectWorkerMentionNames([]string{
		`[chat](worker://default?name=chat)`,
		`[chat](worker://default?name=chat)`,
		`[review](worker://default?name=reviewer)`,
	})
	if len(workers) != 2 {
		t.Fatalf("expected 2 worker names, got %d", len(workers))
	}
}
