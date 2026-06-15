package provider

import "testing"

func TestSimplifyTextOnlyContentFromParts(t *testing.T) {
	got := SimplifyTextOnlyContent([]CommonContentPart{
		{Type: "text", Text: "line one"},
		{Type: "text", Text: "line two"},
	})
	if s, ok := got.(string); !ok || s != "line one\nline two" {
		t.Fatalf("expected joined string, got %#v", got)
	}
}

func TestSimplifyTextOnlyContentKeepsMultimodal(t *testing.T) {
	input := []CommonContentPart{
		{Type: "text", Text: "describe"},
		{Type: "image_url", ImageURL: &CommonImageURL{URL: "https://example.com/a.png"}},
	}
	got := SimplifyTextOnlyContent(input)
	parts, ok := got.([]CommonContentPart)
	if !ok || len(parts) != 2 {
		t.Fatalf("expected original parts, got %#v", got)
	}
}
