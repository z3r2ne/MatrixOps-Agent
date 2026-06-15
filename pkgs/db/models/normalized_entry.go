package models

import "encoding/json"

// NormalizedEntryType 消息类型
type NormalizedEntryType string

const (
	EntryTypeUserMessage      NormalizedEntryType = "user_message"
	EntryTypeAssistantMessage NormalizedEntryType = "assistant_message"
	EntryTypeToolUse          NormalizedEntryType = "tool_use"
	EntryTypeSystemMessage    NormalizedEntryType = "system_message"
	EntryTypeJsonMessage      NormalizedEntryType = "json_message"
	EntryTypeErrorMessage     NormalizedEntryType = "error_message"
	EntryTypeThinking         NormalizedEntryType = "thinking"
	EntryTypeShimmerLoading   NormalizedEntryType = "shimmer-loading"
	EntryTypeCompleted        NormalizedEntryType = "completed"
)

// ToolStatus 工具状态
type ToolStatus string

const (
	ToolStatusCreated ToolStatus = "created"
	ToolStatusSuccess ToolStatus = "success"
	ToolStatusFailed  ToolStatus = "failed"
)

// ActionType 动作类型
type ActionType string

const (
	ActionFileRead   ActionType = "file_read"
	ActionFileEdit   ActionType = "file_edit"
	ActionCommandRun ActionType = "command_run"
	ActionSearch     ActionType = "search"
	ActionTool       ActionType = "tool"
	ActionOther      ActionType = "other"
)

// FileChange 文件变更
type FileChange struct {
	Action      string `json:"action"`                 // write, delete, edit
	Content     string `json:"content,omitempty"`      // for write
	UnifiedDiff string `json:"unified_diff,omitempty"` // for edit
}

// CommandRunResult 命令运行结果
type CommandRunResult struct {
	ExitCode *int   `json:"exit_code,omitempty"`
	Output   string `json:"output,omitempty"`
}

// ToolAction 工具动作详情
type ToolAction struct {
	Action    ActionType        `json:"action"`
	Path      string            `json:"path,omitempty"`
	Changes   []FileChange      `json:"changes,omitempty"`
	Command   string            `json:"command,omitempty"`
	Result    *CommandRunResult `json:"result,omitempty"`
	Query     string            `json:"query,omitempty"`
	ToolName  string            `json:"tool_name,omitempty"`
	Arguments json.RawMessage   `json:"arguments,omitempty"`
}

// NormalizedEntry 规范化的消息条目 (对应 vibe-kanban 的 NormalizedEntry)
type NormalizedEntry struct {
	ID        string              `json:"id"`
	Timestamp string              `json:"timestamp,omitempty"`
	EntryType NormalizedEntryType `json:"entry_type"`
	Content   string              `json:"content"`
	// AdditionalContent 额外信息（例如插件原始输入）
	AdditionalContent string `json:"additional_content,omitempty"`
	// ToolUse 专用字段
	ToolName   string      `json:"tool_name,omitempty"`
	ActionType *ToolAction `json:"action_type,omitempty"`
	ToolStatus ToolStatus  `json:"tool_status,omitempty"`
	// 错误消息专用字段
	ErrorType string `json:"error_type,omitempty"`
}

// JsonMessage JSON 消息
type JsonMessage struct {
	MessageType string `json:"message_type"`
	Data        any    `json:"data"`
}

type ModelInfo struct {
	ModelName string `json:"model"`
}
