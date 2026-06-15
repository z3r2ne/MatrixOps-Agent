package coreagent

import "strings"

const (
	// RunWorkerTaskToolName 为子 worker 委派工具名。
	RunWorkerTaskToolName = "run_worker_task"
	// QuestionToolName 为向用户提问工具名。
	QuestionToolName = "question"
)

var watchdogExemptTools = map[string]struct{}{
	RunWorkerTaskToolName: {},
	QuestionToolName:      {},
}

// IsWatchdogExemptTool 表示该工具不受 stall / 重复调用看门狗审查。
func IsWatchdogExemptTool(toolName string) bool {
	_, ok := watchdogExemptTools[strings.TrimSpace(toolName)]
	return ok
}
