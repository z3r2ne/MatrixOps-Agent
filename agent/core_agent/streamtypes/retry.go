package streamtypes

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	agentprovider "matrixops-agent/provider"
	"pkgs/db/models"
)

const (
	StreamRetryDefaultMaxRetries = 5
	StreamRetryFixedDelay        = 5 * time.Second
)

// StreamWithRetries wraps a streaming run with retry logic.
func StreamWithRetries(input StreamInput, run func(StreamInput) (*StreamOutput, error)) (*StreamOutput, error) {
	if input.Abort != nil {
		select {
		case <-input.Abort.Done():
			return nil, input.Abort.Err()
		default:
		}
	}

	toolCalls := make(chan *CallToolRequest, 64)
	rawText := NewRetryRawBuffer()
	contentReaderBuf := NewRetryRawBuffer()
	reasonReaderBuf := NewRetryRawBuffer()
	var usage *Usage
	var finalErr error
	done := make(chan struct{})
	out := &StreamOutput{
		ToolCalls:     toolCalls,
		RawTextReader: rawText,
		ContentReader: contentReaderBuf,
		ReasonReader:  reasonReaderBuf,
		Wait: func() error {
			<-done
			return finalErr
		},
	}

	go func() {
		defer close(done)
		defer close(toolCalls)
		defer func() {
			out.Usage = usage
			rawText.Close()
			contentReaderBuf.Close()
			reasonReaderBuf.Close()
		}()

		maxRetries := StreamMaxRetries(input)
		for retryAttempt := 0; ; retryAttempt++ {
			attemptStartedAt := time.Now()
			attemptRawResponse := ""
			attemptInput := input
			originalOnRawResponse := input.OnRawResponse
			attemptInput.OnRawResponse = func(raw string) {
				attemptRawResponse = raw
				if originalOnRawResponse != nil {
					originalOnRawResponse(raw)
				}
			}

			output, err := run(attemptInput)
			emittedAnyToolCall := false
			emittedNonMessageToolCall := false
			if err == nil && output != nil {
				forwardDone := make(chan struct{})
				go func() {
					defer close(forwardDone)
					for req := range output.ToolCalls {
						if req != nil {
							emittedAnyToolCall = true
							if strings.TrimSpace(req.Name) != "message" {
								emittedNonMessageToolCall = true
							}
						}
						toolCalls <- req
					}
				}()

				reasonForwardDone := make(chan struct{})
				go func() {
					defer close(reasonForwardDone)
					if output.ReasonReader == nil {
						return
					}
					_, _ = io.Copy(reasonReaderBuf, output.ReasonReader)
				}()

				contentForwardDone := make(chan struct{})
				go func() {
					defer close(contentForwardDone)
					if output.ContentReader == nil {
						return
					}
					_, _ = io.Copy(contentReaderBuf, output.ContentReader)
				}()

				if output.Wait != nil {
					err = output.Wait()
				}
				<-forwardDone
				<-reasonForwardDone
				<-contentForwardDone

				if output.Usage != nil {
					usage = output.Usage
				}
				if strings.TrimSpace(output.Phase) != "" {
					out.Phase = strings.TrimSpace(output.Phase)
				}
				if strings.TrimSpace(output.ResponsesOutputMessageRaw) != "" {
					out.ResponsesOutputMessageRaw = strings.TrimSpace(output.ResponsesOutputMessageRaw)
				}
				if len(output.ResponsesReasoningItemRaws) > 0 {
					out.ResponsesReasoningItemRaws = append([]string(nil), output.ResponsesReasoningItemRaws...)
				}
				if sig := output.AnthropicThinkingSignature(); sig != "" {
					out.SetAnthropicThinkingSignature(sig)
				}
				if output.RawTextReader != nil {
					rawBytes, readErr := io.ReadAll(output.RawTextReader)
					if readErr == nil && len(rawBytes) > 0 {
						_, _ = rawText.Write(rawBytes)
					}
				}
				if err == nil && output.NativeAssistantTextFinishesTurn {
					out.NativeAssistantTextFinishesTurn = true
				}
			}

			if err == nil {
				return
			}

			retryableParseError := StreamShouldRetryParseError(err)
			if retryAttempt >= maxRetries || !StreamShouldRetryError(err) {
				finalErr = err
				return
			}
			if emittedNonMessageToolCall && !retryableParseError {
				finalErr = err
				return
			}
			if emittedAnyToolCall && !retryableParseError {
				finalErr = err
				return
			}

			nextDelay := StreamRetryDelayForError(retryAttempt+1, err)
			if input.OnRetryError != nil {
				input.OnRetryError(err, retryAttempt+1, maxRetries, nextDelay, time.Since(attemptStartedAt), attemptRawResponse)
			}
			if sleepErr := StreamSleepWithContext(input.Context, input.Abort, nextDelay); sleepErr != nil {
				finalErr = sleepErr
				return
			}
		}
	}()

	return out, nil
}

// RetryRawBuffer is a thread-safe buffer that implements io.Reader and io.Writer.
type RetryRawBuffer struct {
	mu     sync.Mutex
	cond   *sync.Cond
	buffer bytes.Buffer
	closed bool
}

// NewRetryRawBuffer creates a new RetryRawBuffer.
func NewRetryRawBuffer() *RetryRawBuffer {
	b := &RetryRawBuffer{}
	b.cond = sync.NewCond(&b.mu)
	return b
}

// Write implements io.Writer.
func (b *RetryRawBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return 0, io.ErrClosedPipe
	}
	n, err := b.buffer.Write(p)
	b.cond.Broadcast()
	return n, err
}

// Read implements io.Reader.
func (b *RetryRawBuffer) Read(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for b.buffer.Len() == 0 && !b.closed {
		b.cond.Wait()
	}
	if b.buffer.Len() > 0 {
		return b.buffer.Read(p)
	}
	return 0, io.EOF
}

// Close marks the buffer as closed.
func (b *RetryRawBuffer) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closed = true
	b.cond.Broadcast()
}

// StreamMaxRetries returns the max retries for a stream input.
func StreamMaxRetries(input StreamInput) int {
	if po, ok := input.ProviderOptions.(*models.LLMConfig); ok && po != nil && po.MaxRetries > 0 {
		return po.MaxRetries
	}
	return StreamRetryDefaultMaxRetries
}

// IsEmptyStreamOutputError reports whether err indicates a completed stream without usable model output.
func IsEmptyStreamOutputError(err error) bool {
	if err == nil {
		return false
	}
	var apiErr *agentprovider.APIError
	if errors.As(err, &apiErr) && apiErr != nil {
		return strings.Contains(strings.ToLower(strings.TrimSpace(apiErr.Message)), "stream ended without model output")
	}
	return strings.Contains(strings.ToLower(strings.TrimSpace(err.Error())), "stream ended without model output")
}

// StreamFinishReasonIsRetryable reports whether an empty stream ending with this finish reason should be retried.
func StreamFinishReasonIsRetryable(finishReason string) bool {
	switch strings.ToLower(strings.TrimSpace(finishReason)) {
	case "content_filter":
		return true
	default:
		return false
	}
}

// RawStreamResponseHasFinishReason reports whether raw SSE payload contains the given finish_reason.
func RawStreamResponseHasFinishReason(rawResponse, finishReason string) bool {
	raw := strings.ToLower(strings.TrimSpace(rawResponse))
	reason := strings.ToLower(strings.TrimSpace(finishReason))
	if raw == "" || reason == "" {
		return false
	}
	return strings.Contains(raw, `"finish_reason":"`+reason+`"`) ||
		strings.Contains(raw, `"finish_reason": "`+reason+`"`)
}

// RetryableEmptyStreamOutputFinishReason returns a finish reason from event or raw SSE worth retrying when output is empty.
func RetryableEmptyStreamOutputFinishReason(streamFinish, rawResponse string) string {
	if fr := strings.TrimSpace(streamFinish); fr != "" && StreamFinishReasonIsRetryable(fr) {
		return fr
	}
	if RawStreamResponseHasFinishReason(rawResponse, "content_filter") {
		return "content_filter"
	}
	return ""
}

// InferEmptyStreamOutputReason extracts a stop/finish reason hint from stream metadata or raw SSE.
func InferEmptyStreamOutputReason(streamFinish, rawResponse string) string {
	if fr := strings.TrimSpace(streamFinish); fr != "" {
		return fr
	}
	if reason := extractRawStreamReason(rawResponse, "stop_reason"); reason != "" {
		return reason
	}
	if reason := extractRawStreamReason(rawResponse, "finish_reason"); reason != "" {
		return reason
	}
	return "empty"
}

func extractRawStreamReason(rawResponse, field string) string {
	raw := strings.TrimSpace(rawResponse)
	field = strings.TrimSpace(field)
	if raw == "" || field == "" {
		return ""
	}
	needle := `"` + strings.ToLower(field) + `"`
	rawLower := strings.ToLower(raw)
	idx := strings.Index(rawLower, needle)
	if idx < 0 {
		return ""
	}
	rest := raw[idx+len(needle):]
	colon := strings.Index(rest, ":")
	if colon < 0 {
		return ""
	}
	rest = strings.TrimSpace(rest[colon+1:])
	if !strings.HasPrefix(rest, `"`) {
		return ""
	}
	rest = rest[1:]
	end := strings.Index(rest, `"`)
	if end < 0 {
		return ""
	}
	return strings.TrimSpace(rest[:end])
}

// RetryErrorForEmptyStreamOutput returns a retryable error when a stream completed without usable output.
func RetryErrorForEmptyStreamOutput(streamFinish, rawResponse string, hasOutput bool) error {
	if hasOutput {
		return nil
	}
	rawResponse = strings.TrimSpace(rawResponse)
	if RawResponseLooksLikeRetryableProxyHTML(rawResponse) {
		return &agentprovider.APIError{
			Message:      "stream returned proxy/html error page before any tool output",
			IsRetryable:  true,
			ResponseBody: rawResponse,
		}
	}
	reason := InferEmptyStreamOutputReason(streamFinish, rawResponse)
	if reason == "" {
		reason = "empty"
	}
	return NewRetryableEmptyStreamOutputError(reason, rawResponse)
}

// NewRetryableEmptyStreamOutputError builds a retryable error when a stream ends without usable model output.
func NewRetryableEmptyStreamOutputError(finishReason, rawResponse string) error {
	message := "stream ended without model output"
	if fr := strings.TrimSpace(finishReason); fr != "" {
		message = fmt.Sprintf("stream ended without model output (finish_reason=%s)", fr)
	}
	return &agentprovider.APIError{
		Message:      message,
		IsRetryable:  true,
		ResponseBody: rawResponse,
	}
}

// StreamShouldRetryError reports whether err is retryable.
func StreamShouldRetryError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	var apiErr *agentprovider.APIError
	if errors.As(err, &apiErr) {
		return apiErr.IsRetryable
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}

	// http.Client.Timeout 在读取 body 阶段触发时，net/http 返回的错误不是
	// context.DeadlineExceeded，也不是 net.Error，需要按字符串识别。
	if strings.Contains(err.Error(), "Client.Timeout") {
		return true
	}

	return StreamShouldRetryParseError(err)
}

// StreamShouldRetryParseError reports whether a parse error is retryable.
func StreamShouldRetryParseError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	text := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(text, "unexpected eof") || strings.Contains(text, "unexpected end of json input")
}

// StreamRetryDelayForError returns the delay before the next retry attempt.
func StreamRetryDelayForError(retryAttempt int, err error) time.Duration {
	var apiErr *agentprovider.APIError
	if errors.As(err, &apiErr) && apiErr != nil && apiErr.ResponseHeaders != nil {
		for k, v := range apiErr.ResponseHeaders {
			if !strings.EqualFold(strings.TrimSpace(k), "retry-after-ms") {
				continue
			}
			ms, convErr := strconv.Atoi(strings.TrimSpace(v))
			if convErr != nil || ms < 0 {
				break
			}
			d := time.Duration(ms) * time.Millisecond
			const maxRetryAfter = 5 * time.Minute
			if d > maxRetryAfter {
				d = maxRetryAfter
			}
			return d
		}
	}
	return StreamRetryFixedDelay
}

// StreamSleepWithContext sleeps for delay or until ctx/abort is done.
func StreamSleepWithContext(ctx context.Context, abort context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-AbortDone(abort):
		return abort.Err()
	case <-timer.C:
		return nil
	}
}

// AbortDone returns abort.Done() or nil if abort is nil.
func AbortDone(abort context.Context) <-chan struct{} {
	if abort == nil {
		return nil
	}
	return abort.Done()
}

// RetryableHTTPStatus reports whether an HTTP status code is retryable.
func RetryableHTTPStatus(code int) bool {
	return code == http.StatusTooManyRequests || (code >= 500 && code <= 599)
}
