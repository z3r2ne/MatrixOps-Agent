package memorysearch

import "testing"

func TestReciprocalRankFusion(t *testing.T) {
	semantic := []rankedHit{
		{docID: "a", score: 0.9},
		{docID: "b", score: 0.8},
	}
	keyword := []rankedHit{
		{docID: "b", score: 1.2},
		{docID: "c", score: 0.7},
	}
	got := reciprocalRankFusion(semantic, keyword, 2)
	if len(got) != 2 {
		t.Fatalf("expected 2 hits, got %d", len(got))
	}
	if got[0].docID != "b" {
		t.Fatalf("expected doc b first, got %s", got[0].docID)
	}
}

func TestMatchesScope(t *testing.T) {
	meta := map[string]string{
		"sessionId": "s1",
	}
	if !matchesScope("s1", nil, meta) {
		t.Fatal("expected session match")
	}
	libMeta := map[string]string{
		"memoryLibraryId": "42",
	}
	if !matchesScope("", []uint{42}, libMeta) {
		t.Fatal("expected library match")
	}
}
