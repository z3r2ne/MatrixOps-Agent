package provider

import (
	"fmt"
	"mime"
	"net/http"
	"strings"
)

type APIError struct {
	ProviderID      string
	Message         string
	StatusCode      int
	IsRetryable     bool
	ResponseBody    string
	ResponseHeaders map[string]string
}

func (e *APIError) Error() string {
	if e == nil || e.Message == "" {
		return "api error"
	}
	return e.Message
}

func isEventStreamContentType(contentType string) bool {
	contentType = strings.TrimSpace(contentType)
	if contentType == "" {
		return false
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err == nil {
		return strings.EqualFold(mediaType, "text/event-stream")
	}
	return strings.Contains(strings.ToLower(contentType), "text/event-stream")
}

func newRetryableStreamResponseError(resp *http.Response, providerID string, reason string, rawResponse string) error {
	statusCode := 0
	if resp != nil {
		statusCode = resp.StatusCode
	}

	message := strings.TrimSpace(reason)
	if rawSnippet := truncateStreamResponseSnippet(rawResponse); rawSnippet != "" {
		if message != "" {
			message += "; "
		}
		message += fmt.Sprintf("raw response: %s", rawSnippet)
	}
	if message == "" {
		message = "invalid streaming response"
	}

	return &APIError{
		ProviderID:      providerID,
		Message:         message,
		StatusCode:      statusCode,
		IsRetryable:     true,
		ResponseBody:    rawResponse,
		ResponseHeaders: responseHeadersFromResponse(resp),
	}
}

func truncateStreamResponseSnippet(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	const maxSnippetBytes = 1024
	if len(raw) <= maxSnippetBytes {
		return raw
	}
	return raw[:maxSnippetBytes] + "...<truncated>"
}
