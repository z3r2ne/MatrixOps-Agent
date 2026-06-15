package coreagent

import (
	"fmt"
	"strings"
	"time"
)

// FormatStallWatchdogToolCancelledWarning 生成写入消息队列/记忆的看门狗取消提示（含系统消息前缀）。
func FormatStallWatchdogToolCancelledWarning(toolName, reason string, elapsed time.Duration) string {
	name := strings.TrimSpace(toolName)
	if name == "" {
		name = "未知工具"
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "执行时间过长"
	}
	elapsedText := elapsed.Round(time.Second).String()
	if elapsed <= 0 {
		elapsedText = "未知时长"
	}
	body := fmt.Sprintf(
		"提示：工具 %q 因执行停滞已被系统取消（已运行 %s）。取消原因：%s。请调整策略后继续，不要重复无效调用。若默认超时过短，可先调用 %q 为下一次工具调用设置更长的超时时长（仅生效一次）。",
		name,
		elapsedText,
		reason,
		SetToolStallWatchdogTimeoutToolName,
	)
	return FormatSystemSupplementMessage(body)
}

func (r *Runner) notifyStallWatchdogToolCancelled(state *RunState, toolName, callID, reason string, elapsed time.Duration) {
	if r == nil || r.cfg.OnStallWatchdogToolCancelled == nil || state == nil {
		return
	}
	if err := r.cfg.OnStallWatchdogToolCancelled(state, toolName, callID, reason, elapsed); err != nil {
		// 看门狗不应阻断工具执行。
	}
}
