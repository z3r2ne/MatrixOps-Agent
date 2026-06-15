package streamtypes

import (
	"errors"
	"strings"
	"testing"
	"time"

	agentprovider "matrixops-agent/provider"
)

func TestStreamRetryDelayForErrorUsesFixedDelay(t *testing.T) {
	got := StreamRetryDelayForError(3, errors.New("boom"))
	if got != 5*time.Second {
		t.Fatalf("delay = %s, want %s", got, 5*time.Second)
	}
}

func TestStreamRetryDelayForErrorUsesRetryAfterMsHeader(t *testing.T) {
	err := &agentprovider.APIError{
		ResponseHeaders: map[string]string{"retry-after-ms": "1"},
	}
	got := StreamRetryDelayForError(1, err)
	if got != time.Millisecond {
		t.Fatalf("delay = %s, want 1ms", got)
	}
}

func TestStreamFinishReasonIsRetryable(t *testing.T) {
	if !StreamFinishReasonIsRetryable("content_filter") {
		t.Fatal("expected content_filter to be retryable")
	}
	if StreamFinishReasonIsRetryable("stop") {
		t.Fatal("expected stop not to be retryable")
	}
}

func TestRawStreamResponseHasFinishReason(t *testing.T) {
	raw := `data: {"choices":[{"delta":{"content":""},"finish_reason":"content_filter"}]}`
	if !RawStreamResponseHasFinishReason(raw, "content_filter") {
		t.Fatal("expected content_filter in raw response")
	}
}

func TestStreamShouldRetryErrorAcceptsEmptyStreamContentFilter(t *testing.T) {
	err := NewRetryableEmptyStreamOutputError("content_filter", `finish_reason":"content_filter"`)
	if !StreamShouldRetryError(err) {
		t.Fatalf("expected retryable error, got %#v", err)
	}
}

func TestInferEmptyStreamOutputReasonReadsAnthropicStopReason(t *testing.T) {
	raw := `event:message_delta` + "\n" + `data:{"type":"message_delta","delta":{"stop_reason":"end_turn"}}`
	if got := InferEmptyStreamOutputReason("", raw); got != "end_turn" {
		t.Fatalf("InferEmptyStreamOutputReason = %q, want end_turn", got)
	}
}

func TestRetryErrorForEmptyStreamOutputReturnsRetryableForEndTurn(t *testing.T) {
	raw := `data:{"type":"message_delta","delta":{"stop_reason":"end_turn"}}`
	err := RetryErrorForEmptyStreamOutput("", raw, false)
	if err == nil {
		t.Fatal("expected retryable error")
	}
	if !StreamShouldRetryError(err) {
		t.Fatalf("expected retryable error, got %#v", err)
	}
	var apiErr *agentprovider.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T", err)
	}
	if !strings.Contains(apiErr.Message, "end_turn") {
		t.Fatalf("message = %q, want end_turn hint", apiErr.Message)
	}
}

func TestRetryErrorForEmptyStreamOutputSkipsWhenOutputPresent(t *testing.T) {
	if err := RetryErrorForEmptyStreamOutput("", "data:{}", true); err != nil {
		t.Fatalf("expected nil when output present, got %v", err)
	}
}

func TestIsEmptyStreamOutputError(t *testing.T) {
	err := NewRetryableEmptyStreamOutputError("end_turn", "")
	if !IsEmptyStreamOutputError(err) {
		t.Fatal("expected empty stream error")
	}
	if IsEmptyStreamOutputError(errors.New("upstream 502")) {
		t.Fatal("expected non-empty-stream error to be false")
	}
}

func TestStreamShouldRetryErrorAcceptsClientTimeout(t *testing.T) {
	err := errors.New("net/http: request canceled (Client.Timeout or context cancellation while reading body)")
	if !StreamShouldRetryError(err) {
		t.Fatalf("expected Client.Timeout error to be retryable, got: %v", err)
	}
}
