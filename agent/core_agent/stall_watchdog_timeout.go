package coreagent

import (
	"fmt"
	"strings"
	"time"
)

const (
	// SetToolStallWatchdogTimeoutToolName 为设置下一次工具看门狗超时的工具名。
	SetToolStallWatchdogTimeoutToolName = "set_tool_stall_timeout"
	// ToolContextSetNextStallWatchdogTimeoutKey 注入 ToolContext.Values，供 set_tool_stall_timeout 写入 RunState。
	ToolContextSetNextStallWatchdogTimeoutKey = "set_next_tool_stall_watchdog_timeout"

	minNextToolStallWatchdogTimeout = time.Second
	maxNextToolStallWatchdogTimeout = time.Hour
)

// SetNextToolStallWatchdogTimeout 设置下一次工具调用的看门狗超时（仅生效一次）。
func (s *RunState) SetNextToolStallWatchdogTimeout(timeout time.Duration) error {
	if s == nil {
		return fmt.Errorf("run state unavailable")
	}
	if timeout < minNextToolStallWatchdogTimeout {
		return fmt.Errorf("timeout must be at least %s", minNextToolStallWatchdogTimeout)
	}
	if timeout > maxNextToolStallWatchdogTimeout {
		return fmt.Errorf("timeout must be at most %s", maxNextToolStallWatchdogTimeout)
	}
	s.NextToolStallWatchdogTimeout = timeout
	return nil
}

// ResolveStallWatchdogTimeout 返回本次工具调用的看门狗超时；若存在一次性 override 则消费之。
// 返回 ≤0 表示本次不启用 stall 看门狗（全局禁用或豁免工具）。
func (s *RunState) ResolveStallWatchdogTimeout(toolName string, defaultTimeout time.Duration) time.Duration {
	if IsWatchdogExemptTool(toolName) {
		return 0
	}
	if defaultTimeout <= 0 {
		return 0
	}
	if strings.TrimSpace(toolName) == SetToolStallWatchdogTimeoutToolName {
		return defaultTimeout
	}
	if s == nil || s.NextToolStallWatchdogTimeout <= 0 {
		return defaultTimeout
	}
	override := s.NextToolStallWatchdogTimeout
	s.NextToolStallWatchdogTimeout = 0
	return override
}

// EnrichToolContextValues 注入与 RunState 相关的工具上下文回调。
func EnrichToolContextValues(state *RunState, values map[string]interface{}) map[string]interface{} {
	if values == nil {
		values = map[string]interface{}{}
	}
	if state == nil {
		return values
	}
	values[ToolContextSetNextStallWatchdogTimeoutKey] = func(timeout time.Duration) error {
		return state.SetNextToolStallWatchdogTimeout(timeout)
	}
	return values
}
