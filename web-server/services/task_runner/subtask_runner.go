package task_runner

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"matrixops-agent/types"
	coregit "matrixops.local/core_agent/git"
	"gorm.io/gorm"
	database "pkgs/db"
	"pkgs/db/models"
	"pkgs/db/storage"
)

type subtaskCompletionResult struct {
	TaskID        uint
	SessionID     string
	Status        string
	Answer        string
	Summary       string
	Error         string
	DurationMs    int64
	WorkDir       string
	Branch        string
	BaseBranch    string
	ModifiedFiles []string
	CreatedFiles  []string
}

func getTaskFinalAnswer(dbConn *gorm.DB, taskID uint) (string, string, error) {
	task, err := database.GetTaskByID(dbConn, taskID)
	if err != nil {
		return "", "", err
	}
	sessionID := strings.TrimSpace(task.SessionID)
	if sessionID == "" {
		sessionID, err = database.GetTaskSessionID(dbConn, taskID)
		if err != nil {
			return "", "", err
		}
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return "", "", fmt.Errorf("subtask session id is empty")
	}

	messages, err := storage.GetMessageWithPartsBySessionID(dbConn, sessionID)
	if err != nil {
		return sessionID, "", err
	}
	for index := len(messages) - 1; index >= 0; index-- {
		msg := messages[index]
		if msg == nil || msg.Info == nil || msg.Info.Role != types.RoleAssistant {
			continue
		}
		answer := collectAssistantAnswerText(msg.Parts)
		if answer != "" {
			return sessionID, answer, nil
		}
	}
	return sessionID, "", nil
}

func collectAssistantAnswerText(parts []*types.Part) string {
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == nil || part.Type != "text" {
			continue
		}
		text := strings.TrimSpace(part.Text)
		if text == "" {
			continue
		}
		out = append(out, text)
	}
	return strings.Join(out, "\n")
}

func seedTaskSessionMemorySnapshot(dbConn *gorm.DB, sessionID string, snapshot *types.Memory) error {
	if dbConn == nil || strings.TrimSpace(sessionID) == "" || snapshot == nil {
		return nil
	}
	entries := normalizeMemorySnapshotEntries(sessionID, snapshot)
	if len(entries) == 0 {
		return nil
	}
	return storage.ReplaceSessionMemoryWithEntries(dbConn, sessionID, entries)
}

func normalizeMemorySnapshotEntries(sessionID string, snapshot *types.Memory) []*types.MemoryEntry {
	if snapshot == nil || strings.TrimSpace(sessionID) == "" {
		return nil
	}

	sourceEntries := snapshot.TranscriptSourceEntries()
	if len(sourceEntries) == 0 {
		return nil
	}

	now := time.Now().UnixMilli()
	output := make([]*types.MemoryEntry, 0, len(sourceEntries))
	for index, entry := range sourceEntries {
		if entry == nil {
			continue
		}
		cloned := *entry
		cloned.ID = 0
		cloned.SessionID = sessionID
		cloned.Sequence = int64(index + 1)
		if cloned.Created == 0 {
			cloned.Created = now + int64(index)
		}
		cloned.Updated = now
		output = append(output, &cloned)
	}
	return output
}

func getSubtaskCompletionResult(dbConn *gorm.DB, taskID uint) (subtaskCompletionResult, error) {
	task, err := database.GetTaskByID(dbConn, taskID)
	if err != nil {
		return subtaskCompletionResult{}, err
	}

	result := subtaskCompletionResult{
		TaskID:     task.ID,
		SessionID:  strings.TrimSpace(task.SessionID),
		Status:     strings.TrimSpace(task.Status),
		Error:      strings.TrimSpace(task.Error),
		WorkDir:    strings.TrimSpace(task.WorkDir),
		Branch:     strings.TrimSpace(task.Branch),
		BaseBranch: strings.TrimSpace(task.BaseBranch),
	}

	if sessionID, answer, err := getTaskFinalAnswer(dbConn, taskID); err == nil {
		if strings.TrimSpace(sessionID) != "" {
			result.SessionID = strings.TrimSpace(sessionID)
		}
		result.Answer = strings.TrimSpace(answer)
	}

	if executions, err := database.GetExecutionsByTaskID(dbConn, taskID, 1); err == nil && len(executions) > 0 {
		result.DurationMs = executions[0].Duration
	}

	baseline := loadSubtaskRepoBaseline(taskID)
	defer clearSubtaskRepoBaseline(taskID)

	if result.WorkDir != "" && coregit.IsGitRepo(result.WorkDir) {
		if baseline != nil {
			result.ModifiedFiles, result.CreatedFiles = subtaskFileChangesSinceBaseline(result.WorkDir, baseline)
		} else if repoState, err := coregit.GetRepoState(result.WorkDir); err == nil && repoState != nil {
			result.ModifiedFiles = normalizeSubtaskPaths(repoState.ModifiedFiles)
			result.CreatedFiles = normalizeSubtaskPaths(repoState.UntrackedFiles)
		}
	}

	result.ModifiedFiles = normalizeSubtaskPaths(result.ModifiedFiles)
	result.CreatedFiles = normalizeSubtaskPaths(result.CreatedFiles)
	result.Summary = formatSubtaskCompletionSummary(result)
	return result, nil
}

func normalizeSubtaskPaths(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	out := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func formatSubtaskCompletionSummary(result subtaskCompletionResult) string {
	lines := []string{
		"子 Worker 调用总结",
		"- 任务 ID: #" + strconv.FormatUint(uint64(result.TaskID), 10),
		"- 状态: " + firstNonEmptySubtask(result.Status, "unknown"),
		"- 执行时长: " + formatSubtaskDuration(result.DurationMs),
	}

	if result.WorkDir != "" {
		lines = append(lines, "- 工作目录: "+result.WorkDir)
	}
	if result.Branch != "" {
		lines = append(lines, "- 分支: "+result.Branch)
	}
	if result.BaseBranch != "" && result.BaseBranch != result.Branch {
		lines = append(lines, "- 基准分支: "+result.BaseBranch)
	}
	if len(result.CreatedFiles) > 0 {
		lines = append(lines, "- 新增文件: "+strings.Join(result.CreatedFiles, ", "))
	}
	if len(result.ModifiedFiles) > 0 {
		lines = append(lines, "- 修改文件: "+strings.Join(result.ModifiedFiles, ", "))
	}
	if strings.TrimSpace(result.Status) == string(models.TaskStatusCancelled) {
		reason := strings.TrimSpace(result.Error)
		if reason == "" {
			reason = models.TaskCancelledByUserMessage
		}
		lines = append(lines, "- 结束原因: "+reason)
	} else if result.Error != "" {
		lines = append(lines, "- 错误: "+result.Error)
	}
	if result.Answer != "" {
		lines = append(lines, "- 最终输出: "+result.Answer)
	}
	if len(result.CreatedFiles) == 0 && len(result.ModifiedFiles) == 0 {
		lines = append(lines, "- 文件变更: 未检测到可汇总的 Git 工作区变更")
	}

	return strings.Join(lines, "\n")
}

func formatSubtaskDuration(durationMs int64) string {
	if durationMs <= 0 {
		return "0ms"
	}
	if durationMs < 1000 {
		return strconv.FormatInt(durationMs, 10) + "ms"
	}
	seconds := float64(durationMs) / 1000
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.1fs", seconds), "0"), ".")
}

func firstNonEmptySubtask(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
