package coreagent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

const (
	// DefaultRepeatedToolCallThreshold 为连续相同工具调用触发看门狗的默认次数。
	DefaultRepeatedToolCallThreshold = 5
	// SystemMessageQueuePrefix 为写入消息队列时的系统消息前缀。
	SystemMessageQueuePrefix = "[系统消息]"
)

// ToolRepeatTracker 跟踪连续相同工具调用次数。
type ToolRepeatTracker struct {
	threshold       int
	lastKey         string
	count           int
	lastWarnedCount int
}

func NewToolRepeatTracker(threshold int) *ToolRepeatTracker {
	if threshold <= 0 {
		threshold = DefaultRepeatedToolCallThreshold
	}
	return &ToolRepeatTracker{threshold: threshold}
}

func ensureToolRepeatTracker(state *RunState, threshold int) *ToolRepeatTracker {
	if state == nil {
		return NewToolRepeatTracker(threshold)
	}
	if state.ToolRepeatTracker == nil {
		state.ToolRepeatTracker = NewToolRepeatTracker(threshold)
	}
	return state.ToolRepeatTracker
}

// Observe 记录一次工具调用；首次达到 threshold 时告警，之后每再重复 DefaultRepeatedToolCallReminderInterval 次继续告警。
func (t *ToolRepeatTracker) Observe(toolName string, args map[string]interface{}, threshold int) (count int, triggered bool) {
	if t == nil {
		return 0, false
	}
	if threshold <= 0 {
		threshold = t.threshold
	}
	if threshold <= 0 {
		threshold = DefaultRepeatedToolCallThreshold
	}
	t.threshold = threshold

	key := ToolCallFingerprint(toolName, args)
	if key == "" {
		t.lastKey = ""
		t.count = 0
		t.lastWarnedCount = 0
		return 0, false
	}
	if key == t.lastKey {
		t.count++
	} else {
		t.lastKey = key
		t.count = 1
		t.lastWarnedCount = 0
	}
	if t.count < threshold {
		return t.count, false
	}
	if t.lastWarnedCount == 0 {
		t.lastWarnedCount = t.count
		return t.count, true
	}
	if t.count-t.lastWarnedCount >= DefaultRepeatedToolCallReminderInterval {
		t.lastWarnedCount = t.count
		return t.count, true
	}
	return t.count, false
}

// FormatRepeatedToolCallWarning 生成写入消息队列/记忆的警告文案（含系统消息前缀）。
func FormatRepeatedToolCallWarning(toolName string, count int) string {
	name := strings.TrimSpace(toolName)
	if name == "" {
		name = "未知工具"
	}
	if count <= 0 {
		count = DefaultRepeatedToolCallThreshold
	}
	body := fmt.Sprintf(
		"警告：你已经连续 %d 次重复调用工具 %q，且参数完全相同。如果不是用户明确要求或确有特殊需求，请立刻停止这种重复调用，换一种思路、调整策略，或直接给出结论结束当前状态。",
		count,
		name,
	)
	return FormatSystemSupplementMessage(body)
}

type normalizedToolCall struct {
	name string
	args map[string]interface{}
}

func normalizeToolCallsForRepeatTracking(toolName string, args map[string]interface{}) []normalizedToolCall {
	name := strings.TrimSpace(toolName)
	if name != "call_tool" {
		return []normalizedToolCall{{name: name, args: cloneAnyMap(args)}}
	}
	if args == nil {
		return nil
	}
	if rawCalls, ok := args["tool_calls"].([]interface{}); ok && len(rawCalls) > 0 {
		out := make([]normalizedToolCall, 0, len(rawCalls))
		for _, item := range rawCalls {
			entry, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			innerName, _ := entry["name"].(string)
			if strings.TrimSpace(innerName) == "" {
				innerName, _ = entry["tool_name"].(string)
			}
			innerName = strings.TrimSpace(innerName)
			if innerName == "" {
				continue
			}
			innerArgs := cloneAnyMap(entry)
			if params, ok := entry["params"].(map[string]interface{}); ok {
				innerArgs = cloneAnyMap(params)
			} else if toolInput, ok := entry["tool_input"].(map[string]interface{}); ok {
				innerArgs = cloneAnyMap(toolInput)
			}
			out = append(out, normalizedToolCall{name: innerName, args: innerArgs})
		}
		if len(out) > 0 {
			return out
		}
	}
	innerName, _ := args["name"].(string)
	if strings.TrimSpace(innerName) == "" {
		innerName, _ = args["tool_name"].(string)
	}
	innerName = strings.TrimSpace(innerName)
	if innerName == "" {
		return []normalizedToolCall{{name: name, args: cloneAnyMap(args)}}
	}
	innerArgs := cloneAnyMap(args)
	if params, ok := args["params"].(map[string]interface{}); ok {
		innerArgs = cloneAnyMap(params)
	} else if toolInput, ok := args["tool_input"].(map[string]interface{}); ok {
		innerArgs = cloneAnyMap(toolInput)
	} else {
		delete(innerArgs, "name")
		delete(innerArgs, "tool_name")
		delete(innerArgs, "reason")
	}
	return []normalizedToolCall{{name: innerName, args: innerArgs}}
}

// ToolCallFingerprint 生成工具名+参数的稳定指纹，用于重复调用检测。
func ToolCallFingerprint(toolName string, args map[string]interface{}) string {
	name := strings.TrimSpace(toolName)
	if name == "" {
		return ""
	}
	canonical, err := canonicalJSON(args)
	if err != nil {
		return name + "\x00" + fmt.Sprintf("%v", args)
	}
	return name + "\x00" + string(canonical)
}

func canonicalJSON(value interface{}) ([]byte, error) {
	normalized := normalizeJSONValue(value)
	return json.Marshal(normalized)
}

func normalizeJSONValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case nil:
		return nil
	case map[string]interface{}:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		out := make(map[string]interface{}, len(keys))
		for _, key := range keys {
			out[key] = normalizeJSONValue(typed[key])
		}
		return out
	case []interface{}:
		out := make([]interface{}, len(typed))
		for i, item := range typed {
			out[i] = normalizeJSONValue(item)
		}
		return out
	case json.Number:
		return typed.String()
	case string, bool, float64, float32, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return typed
	default:
		raw, err := json.Marshal(typed)
		if err != nil {
			return fmt.Sprint(typed)
		}
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.UseNumber()
		var decoded interface{}
		if err := dec.Decode(&decoded); err != nil {
			return fmt.Sprint(typed)
		}
		return normalizeJSONValue(decoded)
	}
}

func cloneAnyMap(src map[string]interface{}) map[string]interface{} {
	if len(src) == 0 {
		return map[string]interface{}{}
	}
	out := make(map[string]interface{}, len(src))
	for key, value := range src {
		out[key] = value
	}
	return out
}

func (r *Runner) observeRepeatedToolCall(state *RunState, call ToolCall) {
	if r == nil || r.cfg.OnRepeatedToolCall == nil || state == nil {
		return
	}
	threshold := r.cfg.RepeatedToolCallThreshold
	if threshold <= 0 {
		threshold = DefaultRepeatedToolCallThreshold
	}
	tracker := ensureToolRepeatTracker(state, threshold)
	calls := normalizeToolCallsForRepeatTracking(call.Name, call.Arguments)
	for _, normalized := range calls {
		if IsWatchdogExemptTool(normalized.name) {
			continue
		}
		count, triggered := tracker.Observe(normalized.name, normalized.args, threshold)
		if !triggered {
			continue
		}
		if err := r.cfg.OnRepeatedToolCall(state, normalized.name, normalized.args, count); err != nil {
			// 看门狗不应阻断工具执行。
			continue
		}
	}
}
