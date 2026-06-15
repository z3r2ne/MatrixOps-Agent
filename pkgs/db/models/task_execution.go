package models

import "time"

// TaskExecution 任务执行记录
type TaskExecution struct {
	ID             uint   `json:"id" gorm:"primaryKey"`
	TaskID         uint   `json:"taskId"`         // 任务 ID
	Status         string `json:"status"`         // 状态
	AgentSessionID string `json:"agentSessionId"` // Agent 会话 ID (对应数据库的 agent_session_id 字段)

	// Git 状态信息（用于重试时恢复）
	GitCommitBefore   string `json:"gitCommitBefore"`   // 执行前的 commit hash
	GitBranchBefore   string `json:"gitBranchBefore"`   // 执行前的分支
	GitDirtyBefore    bool   `json:"gitDirtyBefore"`    // 执行前是否有未提交的更改
	GitUntrackedCount int    `json:"gitUntrackedCount"` // 执行前未跟踪的文件数量
	GitModifiedCount  int    `json:"gitModifiedCount"`  // 执行前修改的文件数量

	StartedAt  time.Time  `json:"startedAt"`
	FinishedAt *time.Time `json:"finishedAt"`
	Duration   int64      `json:"duration"` // 毫秒
	CreatedAt  time.Time  `json:"createdAt"`
}

// TaskExecutionWithLogs 带日志的执行记录（用于 API 响应）
type TaskExecutionWithLogs struct {
	TaskExecution
	Logs []ExecutionLog `json:"logs,omitempty"`
}
