package session

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestBuildLLMRetryMessageErrorIncludesReasonInVisibleMessage(t *testing.T) {
	msgErr := buildLLMRetryMessageError(errors.New("upstream 502 bad gateway"), "test-provider", 1, 5, 2*time.Second)
	if msgErr == nil {
		t.Fatal("expected message error")
	}
	if !strings.Contains(msgErr.Message, "原因：upstream 502 bad gateway") {
		t.Fatalf("expected visible retry message to include reason, got %q", msgErr.Message)
	}
	if !strings.Contains(msgErr.Message, "第 1/5 次重试") {
		t.Fatalf("expected retry counters in message, got %q", msgErr.Message)
	}
}

func TestFormatRetryReasonNormalizesWhitespaceAndTruncates(t *testing.T) {
	input := "line1\nline2\tline3 " + strings.Repeat("x", 220)
	got := formatRetryReason(input)
	if strings.Contains(got, "\n") || strings.Contains(got, "\t") {
		t.Fatalf("expected whitespace normalized, got %q", got)
	}
	if !strings.Contains(got, "line1 line2 line3") {
		t.Fatalf("expected normalized prefix preserved, got %q", got)
	}
	if len([]rune(got)) > 183 {
		t.Fatalf("expected truncated reason, got length %d", len([]rune(got)))
	}
	if !strings.HasSuffix(got, "...") {
		t.Fatalf("expected ellipsis after truncation, got %q", got)
	}
}
