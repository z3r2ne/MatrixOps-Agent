package llm

import (
	"net/http"
	"time"
)

type StreamChatOptions struct {
	OnRequest     func(request *ChatRequest) error
	OnRawRequest  func(raw string)
	OnRawResponse func(raw string)
	HTTPClient    *http.Client
	OnRetryError  func(err error, retryAttempt int, maxRetries int, nextDelay time.Duration, attemptDuration time.Duration, rawResponse string)
}

type StreamChatOption func(options *StreamChatOptions)

func NewStreamChatOptions(opts ...StreamChatOption) *StreamChatOptions {
	options := &StreamChatOptions{}
	for _, opt := range opts {
		opt(options)
	}
	return options
}

func WithOnRequest(onRequest func(request *ChatRequest) error) StreamChatOption {
	return func(options *StreamChatOptions) {
		options.OnRequest = onRequest
	}
}

func WithOnRawRequest(onRawRequest func(raw string)) StreamChatOption {
	return func(options *StreamChatOptions) {
		options.OnRawRequest = onRawRequest
	}
}

func WithOnRawResponse(onRawResponse func(raw string)) StreamChatOption {
	return func(options *StreamChatOptions) {
		options.OnRawResponse = onRawResponse
	}
}

func WithHTTPClient(httpClient *http.Client) StreamChatOption {
	return func(options *StreamChatOptions) {
		options.HTTPClient = httpClient
	}
}

func WithOnRetryError(onRetryError func(err error, retryAttempt int, maxRetries int, nextDelay time.Duration, attemptDuration time.Duration, rawResponse string)) StreamChatOption {
	return func(options *StreamChatOptions) {
		options.OnRetryError = onRetryError
	}
}
