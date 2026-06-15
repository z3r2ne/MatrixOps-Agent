package streamtypes

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	agentprovider "matrixops-agent/provider"
)

// RenderContent renders arbitrary content as a string.
func RenderContent(content interface{}) string {
	switch value := content.(type) {
	case string:
		return value
	case []byte:
		return string(value)
	default:
		data, err := json.Marshal(value)
		if err != nil {
			return fmt.Sprint(value)
		}
		return string(data)
	}
}

// TruncateStringForLog truncates a string for log output.
func TruncateStringForLog(s string, maxBytes int) string {
	if maxBytes <= 0 || len(s) <= maxBytes {
		return s
	}
	cut := s[:maxBytes]
	for len(cut) > 0 && !utf8.ValidString(cut) {
		cut = cut[:len(cut)-1]
	}
	return fmt.Sprintf("%s…(truncated, total %d bytes)", cut, len(s))
}

// RawResponseLooksLikeRetryableProxyHTML checks whether a raw response looks
// like a retryable proxy or HTML error page.
func RawResponseLooksLikeRetryableProxyHTML(raw string) bool {
	trimmed := strings.TrimSpace(strings.ToLower(raw))
	if trimmed == "" {
		return false
	}
	retryableMarkers := []string{
		"burp suite",
		"stream failed to close correctly",
		"proxy error",
		"bad gateway",
		"gateway timeout",
		"service unavailable",
		"temporarily unavailable",
		"upstream connect error",
		"disconnect/reset before headers",
		"connection terminated",
		"connection reset",
		"origin error",
		"openresty",
		"cloudflare",
		"first byte timeout",
		"tls handshake timeout",
	}
	for _, marker := range retryableMarkers {
		if strings.Contains(trimmed, marker) {
			return true
		}
	}
	if strings.HasPrefix(trimmed, "<!doctype html") || strings.HasPrefix(trimmed, "<html") {
		return true
	}
	if strings.HasPrefix(trimmed, "<?xml") && strings.Contains(trimmed, "<error") {
		return true
	}
	return strings.Contains(trimmed, "<head>") && strings.Contains(trimmed, "<body>")
}

// IsWhitespaceByte reports whether ch is a whitespace byte.
func IsWhitespaceByte(ch byte) bool {
	switch ch {
	case ' ', '\t', '\n', '\r':
		return true
	default:
		return false
	}
}

// SnippetAroundBytes returns a snippet of p around center with radius.
func SnippetAroundBytes(p []byte, center int, radius int) string {
	if len(p) == 0 {
		return ""
	}
	if center < 0 {
		center = 0
	}
	if center > len(p) {
		center = len(p)
	}
	start := center - radius
	if start < 0 {
		start = 0
	}
	end := center + radius
	if end > len(p) {
		end = len(p)
	}
	return string(p[start:end])
}

// FirstNonWhitespaceByte returns the first non-whitespace byte in data,
// or 0 if data is all whitespace.
func FirstNonWhitespaceByte(data []byte) byte {
	for _, ch := range data {
		if !IsWhitespaceByte(ch) {
			return ch
		}
	}
	return 0
}

// RenderMessageTextContent extracts a plain-text representation from a message
// content value (string, []byte, content parts, or any JSON-serialisable value).
func RenderMessageTextContent(content interface{}) string {
	switch typed := content.(type) {
	case string:
		return strings.TrimSpace(typed)
	case []byte:
		return strings.TrimSpace(string(typed))
	case []agentprovider.CommonContentPart:
		return joinCommonContentParts(typed)
	case []interface{}:
		return joinCommonContentParts(parseCommonContentParts(typed))
	default:
		if content == nil {
			return ""
		}
		encoded, err := json.Marshal(content)
		if err != nil {
			return ""
		}
		var parts []agentprovider.CommonContentPart
		if err := json.Unmarshal(encoded, &parts); err == nil && len(parts) > 0 {
			return joinCommonContentParts(parts)
		}
		return strings.TrimSpace(string(encoded))
	}
}

func joinCommonContentParts(parts []agentprovider.CommonContentPart) string {
	if len(parts) == 0 {
		return ""
	}
	lines := make([]string, 0, len(parts))
	for _, part := range parts {
		if strings.TrimSpace(part.Type) != "text" {
			continue
		}
		if text := strings.TrimSpace(part.Text); text != "" {
			lines = append(lines, text)
		}
	}
	return strings.Join(lines, "\n")
}

func parseCommonContentParts(items []interface{}) []agentprovider.CommonContentPart {
	parts := make([]agentprovider.CommonContentPart, 0, len(items))
	for _, item := range items {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		partType, _ := m["type"].(string)
		text, _ := m["text"].(string)
		parts = append(parts, agentprovider.CommonContentPart{Type: partType, Text: text})
	}
	return parts
}
