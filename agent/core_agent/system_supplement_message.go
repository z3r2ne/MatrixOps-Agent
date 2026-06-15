package coreagent

import "strings"

const (
	// DefaultRepeatedToolCallReminderInterval 为首次告警后，再次触发重复工具告警的间隔次数。
	DefaultRepeatedToolCallReminderInterval = 10
)

// FormatSystemSupplementMessage 将系统补充消息正文包在 <system> 标签内。
func FormatSystemSupplementMessage(body string) string {
	body = strings.TrimSpace(body)
	if body == "" {
		return ""
	}
	if isSystemSupplementMessageWrapped(body) {
		return body
	}
	if strings.HasPrefix(body, SystemMessageQueuePrefix) {
		body = strings.TrimSpace(strings.TrimPrefix(body, SystemMessageQueuePrefix))
	}
	return FormatToolSystemTag(body)
}

func isSystemSupplementMessageWrapped(body string) bool {
	body = strings.TrimSpace(body)
	return strings.HasPrefix(body, "<system>") && strings.HasSuffix(body, "</system>")
}
