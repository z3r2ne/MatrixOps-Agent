package coreagent

import (
	"fmt"
	"strings"
)

const (
	// DefaultSilentToolCallThreshold 为连续无文字输出的工具调用触发看门狗的默认次数。
	DefaultSilentToolCallThreshold = 10
)

// SilentToolTracker 跟踪连续无思考/文本输出的工具调用次数。
type SilentToolTracker struct {
	threshold int
	count     int
	warned    bool
}

func NewSilentToolTracker(threshold int) *SilentToolTracker {
	if threshold <= 0 {
		threshold = DefaultSilentToolCallThreshold
	}
	return &SilentToolTracker{threshold: threshold}
}

func ensureSilentToolTracker(state *RunState, threshold int) *SilentToolTracker {
	if state == nil {
		return NewSilentToolTracker(threshold)
	}
	if state.SilentToolTracker == nil {
		state.SilentToolTracker = NewSilentToolTracker(threshold)
	}
	return state.SilentToolTracker
}

// Reset 在模型输出思考或文本后清零连续工具调用计数。
func (t *SilentToolTracker) Reset() {
	if t == nil {
		return
	}
	t.count = 0
	t.warned = false
}

// Observe 记录一次工具调用；连续达到 threshold 且尚未告警时返回 triggered=true。
func (t *SilentToolTracker) Observe(threshold int) (count int, triggered bool) {
	if t == nil {
		return 0, false
	}
	if threshold <= 0 {
		threshold = t.threshold
	}
	if threshold <= 0 {
		threshold = DefaultSilentToolCallThreshold
	}
	t.threshold = threshold

	t.count++
	if t.warned {
		return t.count, false
	}
	if t.count >= threshold {
		t.warned = true
		return t.count, true
	}
	return t.count, false
}

// FormatSilentToolWatchdogPrompt 生成写入消息队列/记忆的补充提示（含系统消息前缀）。
func FormatSilentToolWatchdogPrompt(count int) string {
	if count <= 0 {
		count = DefaultSilentToolCallThreshold
	}
	body := fmt.Sprintf(
		"提醒：你已经连续 %d 次调用工具，期间没有输出任何思考或文本。请先用一句话简要总结你目前已经完成了什么、下一步打算做什么，然后再继续执行任务。",
		count,
	)
	return FormatSystemSupplementMessage(body)
}

func partHasVisibleTextOutput(part *Part) bool {
	if part == nil {
		return false
	}
	switch part.Type {
	case PartTypeText:
		return strings.TrimSpace(part.Text) != ""
	case PartTypeReasoning:
		return strings.TrimSpace(part.Reasoning) != ""
	default:
		return false
	}
}

func (r *Runner) markSilentToolTextOutput(state *RunState) {
	if r == nil || state == nil {
		return
	}
	threshold := r.cfg.SilentToolCallThreshold
	if threshold <= 0 {
		threshold = DefaultSilentToolCallThreshold
	}
	tracker := ensureSilentToolTracker(state, threshold)
	tracker.Reset()
}

func (r *Runner) markSilentToolTextOutputFromParts(state *RunState, parts ...*Part) {
	if r == nil || state == nil {
		return
	}
	for _, part := range parts {
		if partHasVisibleTextOutput(part) {
			r.markSilentToolTextOutput(state)
			return
		}
	}
}

func (r *Runner) observeSilentToolCall(state *RunState, call ToolCall) {
	if r == nil || r.cfg.OnSilentToolStreak == nil || state == nil {
		return
	}
	threshold := r.cfg.SilentToolCallThreshold
	if threshold <= 0 {
		threshold = DefaultSilentToolCallThreshold
	}
	tracker := ensureSilentToolTracker(state, threshold)
	calls := normalizeToolCallsForRepeatTracking(call.Name, call.Arguments)
	if len(calls) == 0 {
		return
	}
	for _, normalized := range calls {
		if IsWatchdogExemptTool(normalized.name) {
			continue
		}
		count, triggered := tracker.Observe(threshold)
		if !triggered {
			continue
		}
		if err := r.cfg.OnSilentToolStreak(state, count); err != nil {
			continue
		}
	}
}
