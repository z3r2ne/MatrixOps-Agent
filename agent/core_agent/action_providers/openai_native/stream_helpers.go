package openai_native

import (
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"strings"

	agentprovider "matrixops-agent/provider"
	"pkgs/db/models"

	"matrixops.local/core_agent/streamtypes"
)
func parseOpenAIResponsesOutputMessage(raw string) (string, string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", ""
	}
	var env openAIResponsesOutputMessageEnvelope
	if err := json.Unmarshal([]byte(raw), &env); err != nil {
		return "", ""
	}
	phase := strings.TrimSpace(env.Phase)
	parts := make([]string, 0, len(env.Content))
	for _, part := range env.Content {
		switch strings.TrimSpace(part.Type) {
		case "", "output_text", "refusal":
			if strings.TrimSpace(part.Text) != "" {
				parts = append(parts, part.Text)
			}
		}
	}
	return phase, strings.TrimSpace(strings.Join(parts, "\n"))
}

func openAIJSONFieldReasoningContent(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	var probe struct {
		ReasoningContent string `json:"reasoning_content"`
	}
	if err := json.Unmarshal([]byte(raw), &probe); err != nil {
		return ""
	}
	return probe.ReasoningContent
}

func mergeReasoningContentIntoResponsesOutputMessageRaw(raw string, reasoningContent string) string {
	rc := strings.TrimSpace(reasoningContent)
	r := strings.TrimSpace(raw)
	if rc == "" || r == "" {
		return r
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal([]byte(r), &m); err != nil {
		return r
	}
	if existing, ok := m["reasoning_content"]; ok && len(existing) > 0 {
		var es string
		if json.Unmarshal(existing, &es) == nil && strings.TrimSpace(es) != "" {
			return r
		}
	}
	enc, err := json.Marshal(rc)
	if err != nil {
		return r
	}
	m["reasoning_content"] = enc
	out, err := json.Marshal(m)
	if err != nil {
		return r
	}
	return string(out)
}

func isOpenAIResponsesCommentaryPhase(phase string) bool {
	switch strings.ToLower(strings.TrimSpace(phase)) {
	case "commentary", "analysis":
		return true
	default:
		return false
	}
}

func firstNonWhitespaceByte(data []byte) byte {
	for _, ch := range data {
		if !streamtypes.IsWhitespaceByte(ch) {
			return ch
		}
	}
	return 0
}

func openAINativeEventStreamContentType(contentType string) bool {
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

func openAINativeResponseHeaders(header http.Header) map[string]string {
	if len(header) == 0 {
		return nil
	}
	out := make(map[string]string, len(header))
	for key, values := range header {
		if len(values) == 0 {
			continue
		}
		out[strings.ToLower(key)] = values[0]
	}
	return out
}

func openAINativeRetryableStreamError(llm *models.LLMConfig, statusCode int, headers map[string]string, reason string, rawResponse string) error {
	providerID := "openai"
	if llm != nil && strings.TrimSpace(llm.Name) != "" {
		providerID = strings.TrimSpace(llm.Name)
	}

	message := strings.TrimSpace(reason)
	if snippet := strings.TrimSpace(streamtypes.TruncateStringForLog(strings.TrimSpace(rawResponse), 1024)); snippet != "" {
		if message != "" {
			message += "; "
		}
		message += fmt.Sprintf("raw response: %s", snippet)
	}
	if message == "" {
		message = "openai native stream returned invalid response"
	}

	return &agentprovider.APIError{
		ProviderID:      providerID,
		Message:         message,
		StatusCode:      statusCode,
		IsRetryable:     true,
		ResponseBody:    rawResponse,
		ResponseHeaders: headers,
	}
}

func openAINativeNoEventStreamError(llm *models.LLMConfig, statusCode int, headers map[string]string, rawResponse string) error {
	rawResponse = strings.TrimSpace(rawResponse)
	if rawResponse == "" && statusCode == 0 {
		return nil
	}

	reason := "openai native stream ended without any events"
	if contentType := strings.TrimSpace(headers["content-type"]); contentType != "" && !openAINativeEventStreamContentType(contentType) {
		reason = fmt.Sprintf("unexpected streaming content-type %q", contentType)
	}
	if streamtypes.RawResponseLooksLikeRetryableProxyHTML(rawResponse) {
		reason = "openai native stream returned proxy/html error page before any tool output"
	}

	return openAINativeRetryableStreamError(llm, statusCode, headers, reason, rawResponse)
}
