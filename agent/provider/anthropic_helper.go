package provider

import (
	"encoding/json"
	"net/http"
	"strings"
)

type AnthropicHelper struct {
	ReqModel      string
	ProviderModel string
}

type AnthropicUsage struct {
	InputTokens             int `json:"input_tokens"`
	OutputTokens            int `json:"output_tokens"`
	CacheReadInputTokens    int `json:"cache_read_input_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	CacheCreation           struct {
		Ephemeral5mInputTokens int `json:"ephemeral_5m_input_tokens"`
		Ephemeral1hInputTokens int `json:"ephemeral_1h_input_tokens"`
	} `json:"cache_creation"`
}

func (h *AnthropicHelper) GetFormat() string {
	return "anthropic"
}

func (h *AnthropicHelper) ModifyURL(providerAPI string, isStream bool) string {
	// Simplified logic skipping full Bedrock URL construction for now
	// assuming standard Anthropic API
	return providerAPI + "/messages"
}

func (h *AnthropicHelper) ModifyHeaders(headers http.Header, body map[string]interface{}, apiKey string) {
	headers.Set("x-api-key", apiKey)
	if headers.Get("anthropic-version") == "" {
		headers.Set("anthropic-version", "2023-06-01")
	}
	if model, ok := body["model"].(string); ok && strings.HasPrefix(model, "claude-sonnet-") {
		headers.Set("anthropic-beta", "context-1m-2025-08-07")
	}
}

func (h *AnthropicHelper) ModifyBody(body map[string]interface{}) map[string]interface{} {
	body["service_tier"] = "standard_only"
	return body
}

func (h *AnthropicHelper) CreateBinaryStreamDecoder() func(chunk []byte) []byte {
	return nil
}

func (h *AnthropicHelper) GetStreamSeparator() string {
	return "\n\n"
}

type AnthropicUsageParser struct {
	usage *AnthropicUsage
}

func (p *AnthropicUsageParser) Parse(chunk string) {
	lines := strings.Split(chunk, "\n")
	var dataLine string
	for _, l := range lines {
		if strings.HasPrefix(l, "data: ") {
			dataLine = l
			break
		}
	}
	if dataLine == "" {
		return
	}

	var jsonBody map[string]interface{}
	if err := json.Unmarshal([]byte(dataLine[6:]), &jsonBody); err != nil {
		return
	}

	// Try to extract usage
	// usage can be top level or in message
	var uMap map[string]interface{}
	if u, ok := jsonBody["usage"].(map[string]interface{}); ok {
		uMap = u
	} else if msg, ok := jsonBody["message"].(map[string]interface{}); ok {
		if u, ok := msg["usage"].(map[string]interface{}); ok {
			uMap = u
		}
	}

	if uMap != nil {
		if p.usage == nil {
			p.usage = &AnthropicUsage{}
		}
		// Update existing usage
		updateAnthropicUsage(p.usage, uMap)
	}
}

func updateAnthropicUsage(target *AnthropicUsage, source map[string]interface{}) {
	if v, ok := source["input_tokens"].(float64); ok {
		target.InputTokens = int(v)
	}
	if v, ok := source["output_tokens"].(float64); ok {
		target.OutputTokens = int(v)
	}
	if v, ok := source["cache_read_input_tokens"].(float64); ok {
		target.CacheReadInputTokens = int(v)
	}
	if v, ok := source["cache_creation_input_tokens"].(float64); ok {
		target.CacheCreationInputTokens = int(v)
	}
	// cache_creation object
	if cc, ok := source["cache_creation"].(map[string]interface{}); ok {
		if v, ok := cc["ephemeral_5m_input_tokens"].(float64); ok {
			target.CacheCreation.Ephemeral5mInputTokens = int(v)
		}
		if v, ok := cc["ephemeral_1h_input_tokens"].(float64); ok {
			target.CacheCreation.Ephemeral1hInputTokens = int(v)
		}
	}
}

func (p *AnthropicUsageParser) Retrieve() interface{} {
	return p.usage
}

func (h *AnthropicHelper) CreateUsageParser() UsageParser {
	return &AnthropicUsageParser{}
}

func (h *AnthropicHelper) NormalizeUsage(usage interface{}) UsageInfo {
	u, ok := usage.(*AnthropicUsage)
	if !ok || u == nil {
		return UsageInfo{}
	}
	return UsageInfo{
		InputTokens:        u.InputTokens,
		OutputTokens:       u.OutputTokens,
		CacheReadTokens:    u.CacheReadInputTokens,
		CacheWrite5mTokens: u.CacheCreation.Ephemeral5mInputTokens,
		CacheWrite1hTokens: u.CacheCreation.Ephemeral1hInputTokens,
	}
}
