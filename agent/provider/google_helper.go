package provider

import (
	"encoding/json"
	"net/http"
	"strings"
)

type GoogleHelper struct {
	ProviderModel string
}

type GoogleUsage struct {
	PromptTokenCount        int `json:"promptTokenCount"`
	CandidatesTokenCount    int `json:"candidatesTokenCount"`
	CachedContentTokenCount int `json:"cachedContentTokenCount"`
	ThoughtsTokenCount      int `json:"thoughtsTokenCount"`
}

type GoogleChunk struct {
	UsageMetadata *GoogleUsage `json:"usageMetadata"`
}

func (h *GoogleHelper) GetFormat() string {
	return "google"
}

func (h *GoogleHelper) ModifyURL(providerAPI string, isStream bool) string {
	action := "generateContent"
	if isStream {
		action = "streamGenerateContent?alt=sse"
	}
	return providerAPI + "/models/" + h.ProviderModel + ":" + action
}

func (h *GoogleHelper) ModifyHeaders(headers http.Header, body map[string]interface{}, apiKey string) {
	headers.Set("x-goog-api-key", apiKey)
}

func (h *GoogleHelper) ModifyBody(body map[string]interface{}) map[string]interface{} {
	return body
}

func (h *GoogleHelper) CreateBinaryStreamDecoder() func(chunk []byte) []byte {
	return nil
}

func (h *GoogleHelper) GetStreamSeparator() string {
	return "\r\n\r\n"
}

type GoogleUsageParser struct {
	usage *GoogleUsage
}

func (p *GoogleUsageParser) Parse(chunk string) {
	if !strings.HasPrefix(chunk, "data: ") {
		return
	}
	var jsonBody GoogleChunk
	if err := json.Unmarshal([]byte(chunk[6:]), &jsonBody); err != nil {
		return
	}
	if jsonBody.UsageMetadata != nil {
		p.usage = jsonBody.UsageMetadata
	}
}

func (p *GoogleUsageParser) Retrieve() interface{} {
	return p.usage
}

func (h *GoogleHelper) CreateUsageParser() UsageParser {
	return &GoogleUsageParser{}
}

func (h *GoogleHelper) NormalizeUsage(usage interface{}) UsageInfo {
	u, ok := usage.(*GoogleUsage)
	if !ok || u == nil {
		return UsageInfo{}
	}
	return UsageInfo{
		InputTokens:     u.PromptTokenCount - u.CachedContentTokenCount,
		OutputTokens:    u.CandidatesTokenCount,
		ReasoningTokens: u.ThoughtsTokenCount,
		CacheReadTokens: u.CachedContentTokenCount,
	}
}
