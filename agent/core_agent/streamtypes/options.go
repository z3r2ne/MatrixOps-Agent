package streamtypes

import (
	"net/http"
	"time"
)

// StreamChatOptions carries per-call options for streaming chat.
type StreamChatOptions struct {
	OnRequest     func(request *ChatRequest) error
	OnRawRequest  func(raw string)
	OnRawResponse func(raw string)
	HTTPClient    *http.Client
	OnRetryError  func(err error, retryAttempt int, maxRetries int, nextDelay time.Duration, attemptDuration time.Duration, rawResponse string)
}

// StreamChatOption is a functional option for StreamChatWithOptions.
type StreamChatOption func(options *StreamChatOptions)

// NewStreamChatOptions builds StreamChatOptions from the provided options.
func NewStreamChatOptions(opts ...StreamChatOption) *StreamChatOptions {
	options := &StreamChatOptions{}
	for _, opt := range opts {
		opt(options)
	}
	return options
}

// WithOnRequest sets the OnRequest callback.
func WithOnRequest(onRequest func(request *ChatRequest) error) StreamChatOption {
	return func(options *StreamChatOptions) {
		options.OnRequest = onRequest
	}
}

// WithOnRawRequest sets the OnRawRequest callback.
func WithOnRawRequest(onRawRequest func(raw string)) StreamChatOption {
	return func(options *StreamChatOptions) {
		options.OnRawRequest = onRawRequest
	}
}

// WithOnRawResponse sets the OnRawResponse callback.
func WithOnRawResponse(onRawResponse func(raw string)) StreamChatOption {
	return func(options *StreamChatOptions) {
		options.OnRawResponse = onRawResponse
	}
}

// WithHTTPClient overrides the HTTP client for this call.
func WithHTTPClient(httpClient *http.Client) StreamChatOption {
	return func(options *StreamChatOptions) {
		options.HTTPClient = httpClient
	}
}

// WithOnRetryError sets the OnRetryError callback.
func WithOnRetryError(onRetryError func(err error, retryAttempt int, maxRetries int, nextDelay time.Duration, attemptDuration time.Duration, rawResponse string)) StreamChatOption {
	return func(options *StreamChatOptions) {
		options.OnRetryError = onRetryError
	}
}
