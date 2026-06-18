package models

import (
	"strings"
	"time"
)

type TaskInputPart struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	Path     string `json:"path,omitempty"`
	URL      string `json:"url,omitempty"`
	Mime     string `json:"mime,omitempty"`
	Filename string `json:"filename,omitempty"`
	InputSource string `json:"inputSource,omitempty"`
}

// Task 任务模型
const (
	TaskMessageQueueTypeUser             = "user"
	TaskMessageQueueTypeSystem           = "system"
	TaskMessageQueueTypeAppend           = "append"
	TaskMessageQueueTypeMemoryCompaction = "memory_compaction"

	TaskMessageQueueSourceFrontend           = "frontend"
	TaskMessageQueueSourceWeChat             = "wechat"
	TaskMessageQueueSourceToolRepeatWatchdog = "tool_repeat_watchdog"
	TaskMessageQueueSourceStallWatchdog      = "stall_watchdog"
	TaskMessageQueueSourceEmptyStreamRetry  = "empty_stream_retry"
	TaskMessageQueueSourceReminder          = "reminder"
	TaskMessageQueueSourceSilentToolWatchdog = "silent_tool_watchdog"
	TaskMessageQueueSourceAsyncToolResult   = "async_tool_result"
)

// IsTaskMessageQueueSystem 判断队列项是否为系统消息。
func (item TaskMessageQueueItem) IsSystem() bool {
	return item.Type == TaskMessageQueueTypeSystem
}

// IsAppend 判断队列项是否为 append：运行中与 supplement 一样立即注入；空闲时仅写入记忆。
func (item TaskMessageQueueItem) IsAppend() bool {
	return item.Type == TaskMessageQueueTypeAppend
}

// IsSupplement 判断队列项是否应在当前对话循环的下一轮立即补充发送。
func (item TaskMessageQueueItem) IsSupplement() bool {
	if item.Supplement {
		return true
	}
	if item.Metadata == nil {
		return false
	}
	value, ok := item.Metadata["supplement"].(bool)
	return ok && value
}

// IsLoopInject 运行中的 Agent 对话循环内立即注入（仅 supplement）。
func (item TaskMessageQueueItem) IsLoopInject() bool {
	return item.IsSupplement()
}

// ResolvedSource 返回队列项来源，兼容旧 metadata.source。
func (item TaskMessageQueueItem) ResolvedSource() string {
	if source := strings.TrimSpace(item.Source); source != "" {
		return source
	}
	if item.Metadata != nil {
		if raw, ok := item.Metadata["source"].(string); ok {
			if source := strings.TrimSpace(raw); source != "" {
				return source
			}
		}
	}
	return TaskMessageQueueSourceFrontend
}

type TaskMessageQueueItem struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type,omitempty"`
	Content   string                 `json:"content"`
	Source    string                 `json:"source,omitempty"`
	Supplement bool                  `json:"supplement,omitempty"`
	Parts     []TaskInputPart        `json:"parts,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt int64                  `json:"createdAt"`
}

type Task struct {
	ID             uint      `json:"id" gorm:"primaryKey"`
	WorkspaceID    uint      `json:"workspaceId" gorm:"default:0;index"`
	ProjectID      uint      `json:"projectId" gorm:"index"`
	ParentTaskID   *uint     `json:"parentTaskId" gorm:"index"`
	Name           string    `json:"name" gorm:"size:255"` // 可选任务名称
	Content        string    `json:"content" gorm:"not null"`
	Memo           string    `json:"memo" gorm:"type:text"`
	Title          string    `json:"title" gorm:"size:255"`
	WorkerID       *uint     `json:"workerId"`
	WorkerName     string    `json:"workerName"`
	Status         string    `json:"status" gorm:"default:'queue'"` // queue/active/done/failed
	Error          string    `json:"error" gorm:"type:text"`        // 错误信息
	Branch         string    `json:"branch"`                        // 分支名
	BaseBranch     string    `json:"baseBranch" gorm:"size:255"`    // 创建任务时的基准分支（worktree 基分支等）
	BaseCommitHash string    `json:"baseCommitHash" gorm:"size:64"` // 创建时基分支解析的提交；旧任务可能为空
	WorkDir        string    `json:"workDir"`                       // 工作目录（worktree 路径或项目路径）
	SessionID      string    `json:"sessionId" gorm:"size:255"`     // Agent 会话 ID，用于持续对话
	PromptCacheKey string `json:"promptCacheKey" gorm:"size:64;index"`
	ListPosition   int       `json:"listPosition" gorm:"not null;default:0;index"` // 项目内看板/列表顺序，越小越靠前
	MemoryLibraryMode string `json:"memoryLibraryMode" gorm:"size:32;not null;default:none"`
	MemoryLibraryIDs  UintSlice `json:"memoryLibraryIds" gorm:"type:json;serializer:json"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`

	// 消息队列：任务执行中时用户发送的消息暂存于此，任务完成后自动依次执行
	MessageQueue []TaskMessageQueueItem `json:"messageQueue" gorm:"type:json;serializer:json"`
	// MessageQueueAutoSend 为 true 时，当前对话轮次结束后自动依次执行队列中的消息
	MessageQueueAutoSend bool `json:"messageQueueAutoSend" gorm:"default:true"`
}

// TaskCreate 创建任务
type TaskCreate struct {
	Name         string          `json:"name"`
	Content      string          `json:"content"`
	ProjectID    uint            `json:"projectId"`
	WorkerID     *uint           `json:"workerId"`
	WorkerName   string          `json:"workerName"`
	ParentTaskID *uint           `json:"parentTaskId"`
	Branch       string          `json:"branch"`     // 选择的分支
	NewBranch    string          `json:"newBranch"`  // 新分支名（可选，如果提供则创建 worktree）
	BaseBranch   string          `json:"baseBranch"` // 基础分支（创建新分支时使用）
	InputParts   []TaskInputPart `json:"inputParts"`
	MemoryLibraryMode string     `json:"memoryLibraryMode"`
	MemoryLibraryIDs  []uint       `json:"memoryLibraryIds"`
}

// TaskUpdate 更新任务
type TaskUpdate struct {
	Name     *string `json:"name"`
	Memo     *string `json:"memo"`
	WorkerID *uint   `json:"workerId"`
}

type TaskStatus string

const (
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusQueue     TaskStatus = "queue"
	TaskStatusActive    TaskStatus = "active"
	TaskStatusDone      TaskStatus = "done"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusCancelled TaskStatus = "cancelled"
	TaskStatusSuccess   TaskStatus = "success"
)

// TaskCancelledByUserMessage 写入 task.error，并用于子任务总结，便于主 Agent 识别用户主动停止。
const TaskCancelledByUserMessage = "用户取消了任务执行"

type TaskStatusMessage struct {
	TaskID uint
	Status TaskStatus
}

type TaskMessage struct {
	Type      string `json:"type"`      // message/status/stdout/stderr/normalized_entry
	Role      string `json:"role"`      // system/user/assistant
	Content   any    `json:"content"`   // message content
	Timestamp int64  `json:"timestamp"` // unix ms
	// NormalizedEntry 规范化条目 (仅当 Type == "normalized_entry" 时有效)
	Entry *NormalizedEntry `json:"entry,omitempty"`
}

type TaskInfo struct {
	ID            uint
	ProjectID     string
	Name          string
	Path          string
	WorkDir       string
	VCS           string
	BaseBranch    string
	CurrentBranch string
}
