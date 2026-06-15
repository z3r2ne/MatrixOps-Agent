package provider

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"

	"pkgs/db/models"
)

const (
	defaultLLMMaxRetries = 5
	retryFixedDelay      = 5 * time.Second
)

func resolveMaxRetries(cfg *models.LLMConfig) int {
	if cfg != nil && cfg.MaxRetries > 0 {
		return cfg.MaxRetries
	}
	return defaultLLMMaxRetries
}

func shouldRetryError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.IsRetryable
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}

	// http.Client.Timeout 在读取 body 阶段触发时，net/http 返回的错误不是
	// context.DeadlineExceeded，也不是 net.Error，需要按字符串识别。
	if strings.Contains(err.Error(), "Client.Timeout") {
		return true
	}

	return false
}

func retryDelayForError(retryAttempt int, err error) time.Duration {
	return retryFixedDelay
}

func sleepWithContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func responseHeadersFromResponse(resp *http.Response) map[string]string {
	headers := map[string]string{}
	if resp == nil {
		return headers
	}
	for key, values := range resp.Header {
		if len(values) == 0 {
			continue
		}
		headers[strings.ToLower(key)] = values[0]
	}
	return headers
}

func retryableStatus(code int) bool {
	if code == http.StatusTooManyRequests {
		return true
	}
	return code >= 500 && code <= 599
}
