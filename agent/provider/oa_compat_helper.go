package provider

import (
	"encoding/json"
	"net/http"
	"strings"
)

type OACompatHelper struct{}

type OACompatUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
	// Moonshot
	CachedTokens int `json:"cached_tokens"`
	// XAI & others
	PromptTokensDetails *struct {
		TextTokens   int `json:"text_tokens"`
		AudioTokens  int `json:"audio_tokens"`
		ImageTokens  int `json:"image_tokens"`
		CachedTokens int `json:"cached_tokens"`
	} `json:"prompt_tokens_details"`
	CompletionTokensDetails *struct {
		ReasoningTokens          int `json:"reasoning_tokens"`
		AudioTokens              int `json:"audio_tokens"`
		AcceptedPredictionTokens int `json:"accepted_prediction_tokens"`
		RejectedPredictionTokens int `json:"rejected_prediction_tokens"`
	} `json:"completion_tokens_details"`
}

type OACompatUsageWrapper struct {
	Usage *OACompatUsage `json:"usage"`
}

func (h *OACompatHelper) GetFormat() string {
	return "oa-compat"
}

func (h *OACompatHelper) ModifyURL(providerAPI string, isStream bool) string {
	return providerAPI + "/chat/completions"
}

func (h *OACompatHelper) ModifyHeaders(headers http.Header, body map[string]interface{}, apiKey string) {
	headers.Set("authorization", "Bearer "+apiKey)
}

func (h *OACompatHelper) ModifyBody(body map[string]interface{}) map[string]interface{} {
	if stream, ok := body["stream"].(bool); ok && stream {
		// Create a copy if needed, or modify in place
		// Go maps are references
		if _, ok := body["stream_options"]; !ok {
			body["stream_options"] = map[string]interface{}{"include_usage": true}
		}
	}
	return body
}

func (h *OACompatHelper) CreateBinaryStreamDecoder() func(chunk []byte) []byte {
	return nil
}

func (h *OACompatHelper) GetStreamSeparator() string {
	return "\n\n"
}

type OACompatUsageParser struct {
	usage *OACompatUsage
}

func (p *OACompatUsageParser) Parse(chunk string) {
	if !strings.HasPrefix(chunk, "data: ") {
		return
	}
	var jsonBody OACompatUsageWrapper
	if err := json.Unmarshal([]byte(chunk[6:]), &jsonBody); err != nil {
		return
	}
	if jsonBody.Usage != nil {
		p.usage = jsonBody.Usage
	}
}

func (p *OACompatUsageParser) Retrieve() interface{} {
	return p.usage
}

func (h *OACompatHelper) CreateUsageParser() UsageParser {
	return &OACompatUsageParser{}
}

func (h *OACompatHelper) NormalizeUsage(usage interface{}) UsageInfo {
	u, ok := usage.(*OACompatUsage)
	if !ok || u == nil {
		return UsageInfo{}
	}

	inputTokens := u.PromptTokens
	outputTokens := u.CompletionTokens
	
	reasoningTokens := 0
	if u.CompletionTokensDetails != nil {
		reasoningTokens = u.CompletionTokensDetails.ReasoningTokens
	}

	cacheReadTokens := u.CachedTokens
	if u.PromptTokensDetails != nil && u.PromptTokensDetails.CachedTokens > 0 {
		cacheReadTokens = u.PromptTokensDetails.CachedTokens
	}

	return UsageInfo{
		InputTokens:     inputTokens - cacheReadTokens,
		OutputTokens:    outputTokens,
		ReasoningTokens: reasoningTokens,
		CacheReadTokens: cacheReadTokens,
	}
}
