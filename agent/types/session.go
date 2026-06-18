package types

import (
	"matrixops-agent/permission"
)

type Info struct {
	ID            string             `json:"id"`
	Slug          string             `json:"slug"`
	ProjectID     string             `json:"projectID"`
	Directory     string             `json:"directory"`
	WorkspaceRoot string             `json:"workspaceRoot,omitempty"`
	WorkspacePath string             `json:"workspacePath,omitempty"`
	EnabledSkills []string           `json:"enabledSkills,omitempty"`
	ParentID      string             `json:"parentID,omitempty"`
	Summary       *Summary           `json:"summary,omitempty"`
	MemoryAnalysis *MemoryAnalysis   `json:"memoryAnalysis,omitempty"`
	CriticalInfo   *CriticalInfo      `json:"criticalInfo,omitempty"`
	Share         *ShareInfo         `json:"share,omitempty"`
	Title         string             `json:"title"`
	Version       string             `json:"version"`
	Time          TimeInfo           `json:"time"`
	Permission    permission.Ruleset `json:"permission,omitempty"`
	Revert        *RevertInfo        `json:"revert,omitempty"`
	StartSnapshot string             `json:"startSnapshot,omitempty"`
	Tokens        *MessageTokens     `json:"tokens,omitempty"`
}

type Summary struct {
	Additions int        `json:"additions"`
	Deletions int        `json:"deletions"`
	Files     int        `json:"files"`
	Diffs     []FileDiff `json:"diffs,omitempty"`
}

// MemoryAnalysis 会话记忆分析结果：关键词与简短总结，由前端触发并持久化到 sessions 表。
type MemoryAnalysis struct {
	Keywords  []string `json:"keywords"`
	Summary   string   `json:"summary"`
	UpdatedAt int64    `json:"updatedAt"`
}

// CriticalInfo 会话关键信息列表：用于在记忆压缩后重新注入上下文。
type CriticalInfo struct {
	Items []CriticalInfoItem `json:"items"`
}

// CriticalInfoMatchSource 关键信息的匹配来源：用于在记忆中检查其是否仍存在。
type CriticalInfoMatchSource struct {
	Kind string `json:"kind"`

	// Kind=marker/message_substring/tool_call 时使用
	Text string `json:"text,omitempty"`

	// Kind=tool_call 时使用
	ToolName  string `json:"toolName,omitempty"`
	ParamsJSON string `json:"paramsJson,omitempty"`
	Output    string `json:"output,omitempty"`
}

// CriticalInfoItem 单条关键信息。
type CriticalInfoItem struct {
	ID        string            `json:"id"`
	Type      string            `json:"type"`
	Marker    string            `json:"marker"`
	Message   string            `json:"message"`
	MatchSources []CriticalInfoMatchSource `json:"matchSources,omitempty"`
	CreatedAt int64             `json:"createdAt"`
	AsyncTask *AsyncToolTaskMeta `json:"asyncTask,omitempty"`
}

// AsyncToolTaskMeta 异步工具任务元数据。
type AsyncToolTaskMeta struct {
	CallID    string                 `json:"callId"`
	ToolName  string                 `json:"toolName"`
	Params    map[string]interface{} `json:"params,omitempty"`
	Status    string                 `json:"status"`
	StartedAt int64                  `json:"startedAt"`
	TaskID    uint                   `json:"taskId,omitempty"`
}

type ShareInfo struct {
	URL string `json:"url"`
}

type TimeInfo struct {
	Created    int64 `json:"created"`
	Updated    int64 `json:"updated"`
	Compacting int64 `json:"compacting,omitempty"`
	Archived   int64 `json:"archived,omitempty"`
}

type RevertInfo struct {
	MessageID string `json:"messageID"`
	PartID    string `json:"partID,omitempty"`
	Snapshot  string `json:"snapshot,omitempty"`
	Diff      string `json:"diff,omitempty"`
}

type SessionEvent struct {
	Info *Info `json:"info"`
}

type SessionDiffEvent struct {
	SessionID string     `json:"sessionID"`
	Diff      []FileDiff `json:"diff"`
}

type FileDiff struct {
	File      string `json:"file"`
	Before    string `json:"before"`
	After     string `json:"after"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
}

type SessionErrorEvent struct {
	SessionID string        `json:"sessionID,omitempty"`
	Error     *MessageError `json:"error"`
}

type PluginVarSetEvent struct {
	Key   string `json:"key"`
	Value any    `json:"value"`
}

type WaitUserInputEvent struct {
	Questions map[string]interface{} `json:"questions"`
}

type MessageEvent struct {
	Info *MessageInfo `json:"info"`
}

type MessageRemovedEvent struct {
	SessionID string `json:"sessionID"`
	MessageID string `json:"messageID"`
}

type PartEvent struct {
	Part  *Part  `json:"part"`
	Delta string `json:"delta,omitempty"`
}

type PartRemovedEvent struct {
	SessionID string `json:"sessionID"`
	MessageID string `json:"messageID"`
	PartID    string `json:"partID"`
}
