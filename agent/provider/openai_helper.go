package provider

import (
	"encoding/json"
	"net/http"
	"strings"
)

type OpenAIHelper struct{}

type OpenAIUsage struct {
	InputTokens        *int `json:"input_tokens"`
	InputTokensDetails *struct {
		CachedTokens *int `json:"cached_tokens"`
	} `json:"input_tokens_details"`
	OutputTokens        *int `json:"output_tokens"`
	OutputTokensDetails *struct {
		ReasoningTokens *int `json:"reasoning_tokens"`
	} `json:"output_tokens_details"`
	TotalTokens *int `json:"total_tokens"`
}

type OpenAIUsageWrapper struct {
	Response *struct {
		Usage *OpenAIUsage `json:"usage"`
	} `json:"response"`
}

func (h *OpenAIHelper) GetFormat() string {
	return "openai"
}

func (h *OpenAIHelper) ModifyURL(providerAPI string, isStream bool) string {
	return providerAPI + "/responses"
}

func (h *OpenAIHelper) ModifyHeaders(headers http.Header, body map[string]interface{}, apiKey string) {
	headers.Set("authorization", "Bearer "+apiKey)
}

func (h *OpenAIHelper) ModifyBody(body map[string]interface{}) map[string]interface{} {
	return body
}

func (h *OpenAIHelper) CreateBinaryStreamDecoder() func(chunk []byte) []byte {
	return nil
}

func (h *OpenAIHelper) GetStreamSeparator() string {
	return "\n\n"
}

type OpenAIUsageParser struct {
	usage *OpenAIUsage
}

func (p *OpenAIUsageParser) Parse(chunk string) {
	parts := strings.Split(chunk, "\n")
	if len(parts) < 2 {
		return
	}
	event := parts[0]
	data := parts[1]

	if event != "event: response.completed" {
		return
	}
	if !strings.HasPrefix(data, "data: ") {
		return
	}

	var jsonBody OpenAIUsageWrapper
	if err := json.Unmarshal([]byte(data[6:]), &jsonBody); err != nil {
		return
	}

	if jsonBody.Response != nil && jsonBody.Response.Usage != nil {
		p.usage = jsonBody.Response.Usage
	}
}

func (p *OpenAIUsageParser) Retrieve() interface{} {
	return p.usage
}

func (h *OpenAIHelper) CreateUsageParser() UsageParser {
	return &OpenAIUsageParser{}
}

func (h *OpenAIHelper) NormalizeUsage(usage interface{}) UsageInfo {
	u, ok := usage.(*OpenAIUsage)
	if !ok || u == nil {
		return UsageInfo{}
	}

	inputTokens := 0
	if u.InputTokens != nil {
		inputTokens = *u.InputTokens
	}

	outputTokens := 0
	if u.OutputTokens != nil {
		outputTokens = *u.OutputTokens
	}

	reasoningTokens := 0
	if u.OutputTokensDetails != nil && u.OutputTokensDetails.ReasoningTokens != nil {
		reasoningTokens = *u.OutputTokensDetails.ReasoningTokens
	}

	cacheReadTokens := 0
	if u.InputTokensDetails != nil && u.InputTokensDetails.CachedTokens != nil {
		cacheReadTokens = *u.InputTokensDetails.CachedTokens
	}

	return UsageInfo{
		InputTokens:     inputTokens - cacheReadTokens,
		OutputTokens:    outputTokens - reasoningTokens,
		ReasoningTokens: reasoningTokens,
		CacheReadTokens: cacheReadTokens,
	}
}
