package anthropic

import (
	"errors"
	"fmt"
	"strings"

	agentprovider "matrixops-agent/provider"
	"github.com/anthropics/anthropic-sdk-go"
	"pkgs/db/models"

	"matrixops.local/core_agent/streamtypes"
)

func anthropicRetryableStreamError(llm *models.LLMConfig, statusCode int, headers map[string]string, reason string, rawResponse string) error {
	providerID := "anthropic"
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
		message = "anthropic native stream returned invalid response"
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

func mapAnthropicErrToProvider(llm *models.LLMConfig, err error) error {
	if err == nil {
		return nil
	}
	var aerr *anthropic.Error
	if !errors.As(err, &aerr) || aerr == nil {
		return err
	}
	retry := false
	switch aerr.Type() {
	case anthropic.ErrorTypeRateLimitError, anthropic.ErrorTypeOverloadedError, anthropic.ErrorTypeTimeoutError, anthropic.ErrorTypeAPIError:
		retry = true
	default:
		if aerr.StatusCode == 429 || (aerr.StatusCode >= 500 && aerr.StatusCode <= 599) {
			retry = true
		}
	}
	providerID := "anthropic"
	if llm != nil && strings.TrimSpace(llm.Name) != "" {
		providerID = strings.TrimSpace(llm.Name)
	}
	headers := map[string]string{}
	if aerr.Response != nil {
		headers = anthropicResponseHeaders(aerr.Response.Header)
	}
	rawBody := strings.TrimSpace(aerr.RawJSON())
	return &agentprovider.APIError{
		ProviderID:      providerID,
		Message:         strings.TrimSpace(aerr.Error()),
		StatusCode:      aerr.StatusCode,
		IsRetryable:     retry,
		ResponseBody:    rawBody,
		ResponseHeaders: headers,
	}
}
