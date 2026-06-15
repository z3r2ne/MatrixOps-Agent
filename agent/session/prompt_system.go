package session

import (
	"fmt"
	"strings"

	database "pkgs/db"
)

// buildWorkerPrompt 构建 worker 的完整提示词
// 合并 worker 提示词、模型提示词、职业提示词
func (r *AgentRunner) buildWorkerPrompt(workerName string) (string, error) {
	worker, err := database.GetWorkerByName(r.db, workerName)
	if err != nil {
		return "", fmt.Errorf("prompt: buildWorkerPrompt worker: %w", err)
	}

	modelSettings, _ := database.GetModelSettingsForWorker(r.db, worker)

	occupation, err := database.GetOccupationByCode(r.db, worker.Occupation)
	if err != nil {
		return "", fmt.Errorf("prompt: buildWorkerPrompt occupation: %w", err)
	}

	pieces := []string{}
	workspacePath := r.resolveSessionWorkspacePath(r.db, r.GetSessionID())

	// 按优先级添加提示词
	if worker.SystemPrompt != "" {
		pieces = append(pieces, replaceAIWorkspacePlaceholder(worker.SystemPrompt, workspacePath))
	}

	if modelSettings != nil && modelSettings.Prompt != "" {
		pieces = append(pieces, replaceAIWorkspacePlaceholder(modelSettings.Prompt, workspacePath))
	}

	if occupation != nil && occupation.Prompt != "" {
		pieces = append(pieces, replaceAIWorkspacePlaceholder(occupation.Prompt, workspacePath))
	}

	return strings.Join(pieces, "\n"), nil
}
