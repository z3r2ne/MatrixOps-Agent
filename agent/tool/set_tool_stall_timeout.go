package tool

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const (
	setToolStallTimeoutToolName              = "set_tool_stall_timeout"
	toolContextSetNextStallWatchdogTimeoutKey = "set_next_tool_stall_watchdog_timeout"
)

type SetToolStallTimeoutTool struct{}

var _ Tool = SetToolStallTimeoutTool{}

func (SetToolStallTimeoutTool) Name() string { return setToolStallTimeoutToolName }

func (SetToolStallTimeoutTool) VerbosName() string { return "设置工具超时" }

func (SetToolStallTimeoutTool) Description() string {
	return "为下一次工具调用设置看门狗超时时长（秒）。仅对随后的一次工具调用生效；若工具因执行停滞被看门狗取消，可先调用本工具再重试长耗时操作。"
}

func (SetToolStallTimeoutTool) Schema() map[string]interface{} {
	return ObjectParamSchema(map[string]interface{}{
		"timeout_seconds": map[string]interface{}{
			"type":        "number",
			"description": "下一次工具调用的看门狗超时（秒），范围 1–3600；仅生效一次",
		},
	}, []string{"timeout_seconds"})
}

func (SetToolStallTimeoutTool) Execute(ctx Context, input map[string]interface{}) (Result, error) {
	setter, ok := ctx.Values[toolContextSetNextStallWatchdogTimeoutKey].(func(time.Duration) error)
	if !ok || setter == nil {
		return Result{IsError: true}, fmt.Errorf("%s: 当前运行环境不支持设置看门狗超时", setToolStallTimeoutToolName)
	}

	seconds, err := parsePositiveTimeoutSeconds(input["timeout_seconds"])
	if err != nil {
		return Result{IsError: true}, err
	}
	timeout := time.Duration(seconds * float64(time.Second))
	if err := setter(timeout); err != nil {
		return Result{IsError: true}, err
	}

	payload, _ := json.Marshal(map[string]interface{}{
		"timeoutSeconds": seconds,
		"scope":          "next_tool_call_only",
		"message":        fmt.Sprintf("已设置：下一次工具调用的看门狗超时为 %.0f 秒（仅生效一次）。", seconds),
	})
	return Result{
		Content: string(payload),
		Metadata: map[string]interface{}{
			"timeoutSeconds": seconds,
			"scope":          "next_tool_call_only",
		},
	}, nil
}

func parsePositiveTimeoutSeconds(value interface{}) (float64, error) {
	switch typed := value.(type) {
	case float64:
		if typed <= 0 {
			return 0, fmt.Errorf("%s: timeout_seconds 必须为正数", setToolStallTimeoutToolName)
		}
		return typed, nil
	case float32:
		if typed <= 0 {
			return 0, fmt.Errorf("%s: timeout_seconds 必须为正数", setToolStallTimeoutToolName)
		}
		return float64(typed), nil
	case int:
		if typed <= 0 {
			return 0, fmt.Errorf("%s: timeout_seconds 必须为正数", setToolStallTimeoutToolName)
		}
		return float64(typed), nil
	case int64:
		if typed <= 0 {
			return 0, fmt.Errorf("%s: timeout_seconds 必须为正数", setToolStallTimeoutToolName)
		}
		return float64(typed), nil
	case json.Number:
		parsed, err := typed.Float64()
		if err != nil {
			return 0, fmt.Errorf("%s: timeout_seconds 无效: %w", setToolStallTimeoutToolName, err)
		}
		if parsed <= 0 {
			return 0, fmt.Errorf("%s: timeout_seconds 必须为正数", setToolStallTimeoutToolName)
		}
		return parsed, nil
	case string:
		text := strings.TrimSpace(typed)
		if text == "" {
			return 0, fmt.Errorf("%s: timeout_seconds 必填", setToolStallTimeoutToolName)
		}
		var parsed float64
		if _, err := fmt.Sscan(text, &parsed); err != nil || parsed <= 0 {
			return 0, fmt.Errorf("%s: timeout_seconds 必须为正数", setToolStallTimeoutToolName)
		}
		return parsed, nil
	default:
		return 0, fmt.Errorf("%s: timeout_seconds 必填", setToolStallTimeoutToolName)
	}
}
