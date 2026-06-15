package session

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

const (
	RetryFixedDelay = 5000
	RetryMaxDelay   = 2147483647
)

func RetrySleep(ms int, ctx context.Context) error {
	if ms <= 0 {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	timer := time.NewTimer(time.Duration(minInt(ms, RetryMaxDelay)) * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func RetryDelay(attempt int, apiError *MessageError) int {
	return RetryFixedDelay
}

func Retryable(err *MessageError) string {
	if err == nil {
		return ""
	}
	if err.Name == "MessageAPIError" {
		if !err.IsRetryable {
			return ""
		}
		if err.Message != "" {
			if contains(err.Message, "Overloaded") {
				return "Provider is overloaded"
			}
			return err.Message
		}
		return "Provider error"
	}
	if err.Message == "" {
		return ""
	}
	if retryMessage := parseRetryableMessage(err.Message); retryMessage != "" {
		return retryMessage
	}
	return ""
}

func parseRetryableMessage(raw string) string {
	type errorEnvelope struct {
		Type  string `json:"type"`
		Code  string `json:"code"`
		Error struct {
			Type    string `json:"type"`
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	var parsed errorEnvelope
	if err := jsonUnmarshal(raw, &parsed); err != nil {
		return ""
	}
	if parsed.Type == "error" && parsed.Error.Type == "too_many_requests" {
		return "Too Many Requests"
	}
	if contains(parsed.Code, "exhausted") || contains(parsed.Code, "unavailable") {
		return "Provider is overloaded"
	}
	if parsed.Type == "error" && contains(parsed.Error.Code, "rate_limit") {
		return "Rate Limited"
	}
	if contains(parsed.Error.Message, "no_kv_space") || (parsed.Type == "error" && parsed.Error.Type == "server_error") || parsed.Error.Type != "" {
		return "Provider Server Error"
	}
	return ""
}

func contains(haystack string, needle string) bool {
	return strings.Contains(haystack, needle)
}

func minInt(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func jsonUnmarshal(input string, out interface{}) error {
	if input == "" {
		return errors.New("empty")
	}
	return json.Unmarshal([]byte(input), out)
}
