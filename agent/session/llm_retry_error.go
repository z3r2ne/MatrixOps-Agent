package session

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

func buildLLMRetryMessageError(err error, providerID string, retryAttempt int, maxRetries int, nextDelay time.Duration) *MessageError {
	messageError := FromError(err, providerID)
	if messageError == nil {
		messageError = &MessageError{
			Name:       "MessageRetryError",
			Message:    err.Error(),
			ProviderID: providerID,
		}
	}

	reason := strings.TrimSpace(messageError.Message)
	if reason == "" && err != nil {
		reason = strings.TrimSpace(err.Error())
	}

	if messageError.Metadata == nil {
		messageError.Metadata = map[string]string{}
	}
	if reason != "" {
		messageError.Metadata["reason"] = reason
	}
	if retryAttempt > 0 {
		messageError.Metadata["retryAttempt"] = strconv.Itoa(retryAttempt)
	}
	if maxRetries > 0 {
		messageError.Metadata["maxRetries"] = strconv.Itoa(maxRetries)
	}
	if nextDelay > 0 {
		messageError.Metadata["nextDelayMs"] = strconv.FormatInt(nextDelay.Milliseconds(), 10)
	}

	reasonText := formatRetryReason(reason)
	delayText := ""
	if nextDelay > 0 {
		delayText = fmt.Sprintf("，将在 %s 后重试", humanizeRetryDelay(nextDelay))
	}
	if reasonText != "" {
		messageError.Message = fmt.Sprintf("大模型调用失败（原因：%s），准备进行第 %d/%d 次重试%s", reasonText, retryAttempt, maxRetries, delayText)
		return messageError
	}
	messageError.Message = fmt.Sprintf("大模型调用失败，准备进行第 %d/%d 次重试%s", retryAttempt, maxRetries, delayText)
	return messageError
}

func humanizeRetryDelay(delay time.Duration) string {
	if delay < time.Second {
		return fmt.Sprintf("%dms", delay.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", delay.Seconds())
}

func formatRetryReason(reason string) string {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return ""
	}
	reason = strings.Join(strings.Fields(reason), " ")
	const maxReasonRunes = 180
	runes := []rune(reason)
	if len(runes) <= maxReasonRunes {
		return reason
	}
	return strings.TrimSpace(string(runes[:maxReasonRunes])) + "..."
}
