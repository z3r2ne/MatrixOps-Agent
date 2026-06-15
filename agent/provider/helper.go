package provider

import "net/http"

type UsageParser interface {
	Parse(chunk string)
	Retrieve() interface{}
}

type ProviderHelper interface {
	GetFormat() string
	ModifyURL(providerAPI string, isStream bool) string
	ModifyHeaders(headers http.Header, body map[string]interface{}, apiKey string)
	ModifyBody(body map[string]interface{}) map[string]interface{}
	CreateBinaryStreamDecoder() func(chunk []byte) []byte // Return nil if not needed
	GetStreamSeparator() string
	CreateUsageParser() UsageParser
	NormalizeUsage(usage interface{}) UsageInfo
}
