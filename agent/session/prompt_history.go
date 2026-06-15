package session

import (
	"encoding/json"
	"strings"

	"matrixops-agent/taskctx"
	"matrixops-agent/types"
	database "pkgs/db"
	"pkgs/db/models"
	"pkgs/db/storage"

	"gorm.io/gorm"
)

// getMemory 获取会话的记忆（包含环境信息、提示词、历史等）
func (r *AgentRunner) getMemory(task *models.Task, db *gorm.DB, sessionID, workerName string) (*types.Memory, error) {
	ctx, err := taskctx.Resolve(task)
	if err != nil {
		return nil, err
	}

	aiWorkspaceDir := r.resolveSessionWorkspacePath(db, sessionID)
	if aiWorkspaceDir == "" && task.ProjectID > 0 {
		project, projectErr := database.GetProjectByID(db, task.ProjectID)
		if projectErr == nil {
			aiWorkspaceDir = resolveProjectAIWorkspacePath(ctx.WorkDir, project.Path, project.WorktreePath)
		}
	}
	if aiWorkspaceDir == "" {
		aiWorkspaceDir = fallbackAIWorkspacePath(ctx.WorkDir, "")
	}
	// 获取各层级的提示词
	globalPrompt, err := database.GetGlobalPrompt(db)
	if err != nil {
		return nil, err
	}
	globalPrompt = replaceAIWorkspacePlaceholder(globalPrompt, aiWorkspaceDir)
	globalPrompt = appendTaskLoopGuidance(globalPrompt)

	worker, err := database.GetWorkerByName(db, workerName)
	if err != nil {
		return nil, err
	}

	// 模型提示词
	var modelPrompt string
	modelSettings, _ := database.GetModelSettingsForWorker(db, worker)
	if modelSettings != nil && modelSettings.Prompt != "" {
		modelPrompt = replaceAIWorkspacePlaceholder(modelSettings.Prompt, aiWorkspaceDir)
	}
	// 职业提示词
	var occupationPrompt string
	if worker.Occupation != "" {
		occupation, err := database.GetOccupationByCode(db, worker.Occupation)
		if err == nil && occupation.Prompt != "" {
			occupationPrompt = replaceAIWorkspacePlaceholder(occupation.Prompt, aiWorkspaceDir)
		}
	}

	// 项目提示词
	var projectPrompt string
	if task.ProjectID > 0 {
		project, err := database.GetProjectByID(db, task.ProjectID)
		if err == nil && project.Prompt != "" {
			projectPrompt = replaceAIWorkspacePlaceholder(project.Prompt, aiWorkspaceDir)
		}
	}

	// Worker 提示词
	var workerPrompt string
	if worker.SystemPrompt != "" {
		workerPrompt = replaceAIWorkspacePlaceholder(worker.SystemPrompt, aiWorkspaceDir)
	}
	layers := buildDynamicPromptLayers(db, worker, ctx, aiWorkspaceDir)

	// 项目文件提示词
	var projectFilePrompt []types.FilePrompt
	customPrompts, err := getCustomPromptsWithSource(task)
	if err != nil {
		return nil, err
	}
	for _, prompt := range customPrompts {
		projectFilePrompt = append(projectFilePrompt, types.FilePrompt{
			Path:   prompt[0],
			Prompt: prompt[1],
		})
	}
	installedSkillsPrompt := strings.TrimSpace(installedSkillSummaryInstruction())

	skillPrompts, err := buildWorkerSkillPrompts(worker.EnabledSkills)
	if err != nil {
		return nil, err
	}
	if err := ensureWorkerSkillsInSession(db, sessionID, worker); err != nil {
		return nil, err
	}

	memoryEntries, chatHistory, latestToolCall, err := r.buildSessionMemory(sessionID, db)
	if err != nil {
		return nil, err
	}

	memory := &types.Memory{
		GlobalPrompt:          globalPrompt,
		OccupationPrompt:      occupationPrompt,
		ProjectPrompt:         projectPrompt,
		ModelPrompt:           modelPrompt,
		WorkerPrompt:          workerPrompt,
		SessionGuidancePrompt: layers.sessionGuidance,
		OutputStylePrompt:     layers.outputStyle,
		ToolPriorityPrompt:    layers.toolPriority,
		ProjectFilePrompt:     projectFilePrompt,
		SkillPrompts:          skillPrompts,
		InstalledSkillsPrompt: installedSkillsPrompt,
		EnvPrompt:             layers.environment,
		Entries:               memoryEntries,
		History:               chatHistory,
		LatestToolCall:        latestToolCall,
	}

	return memory, nil
}

func (r *AgentRunner) buildSessionMemory(sessionID string, db *gorm.DB) ([]*types.MemoryEntry, []*types.ChatHistoryItem, *types.LatestToolCall, error) {
	memoryEntries, err := storage.ListMemoryEntriesBySession(db, sessionID)
	if err != nil {
		return nil, nil, nil, err
	}

	chatHistory := memoryEntriesToChatHistory(memoryEntries)
	latestToolCall := buildLatestToolCall(chatHistory)
	return memoryEntries, chatHistory, latestToolCall, nil
}

func formatToolCallResultForHistory(part *types.Part) string {
	if part == nil || part.Tool == nil {
		return ""
	}

	output := strings.TrimSpace(part.Tool.State.Output)
	if output != "" {
		return strings.TrimSpace(output)
	}

	if errText := strings.TrimSpace(part.Tool.State.Error); errText != "" {
		return errText
	}

	return "工具输出为空"
}

func buildLatestToolCallLegacyDefault(role string, content string) *types.LatestToolCall {
	if content == "" {
		return nil
	}
	return &types.LatestToolCall{
		Role:        role,
		Content:     content,
		Instruction: "以下是最后一条消息，请继续。",
	}
}

func latestToolNameFromHistoryItem(item *types.ChatHistoryItem, chatHistory []*types.ChatHistoryItem) string {
	if item != nil {
		if value := strings.TrimSpace(item.ToolName); value != "" {
			return value
		}
		if len(item.NativeToolCalls) > 0 {
			return strings.TrimSpace(item.NativeToolCalls[len(item.NativeToolCalls)-1].Name)
		}
		if role := strings.TrimSpace(item.Role); strings.HasPrefix(role, "call_tool_") {
			return strings.TrimPrefix(role, "call_tool_")
		}
	}
	return latestToolName(chatHistory)
}

func buildLatestToolCall(chatHistory []*types.ChatHistoryItem) *types.LatestToolCall {
	if len(chatHistory) == 0 {
		return nil
	}

	last := chatHistory[len(chatHistory)-1]
	if last == nil {
		return nil
	}

	role := strings.TrimSpace(last.Role)
	content := last.RenderContent()

	switch role {
	case "assistant":
		if len(last.NativeToolCalls) > 0 {
			toolName := strings.TrimSpace(last.NativeToolCalls[len(last.NativeToolCalls)-1].Name)
			instruction := "你刚刚申请了调用工具，工具已经执行成功。请基于工具结果继续选择你的动作。"
			if toolName != "" {
				instruction = "你刚刚申请了调用工具 " + toolName + "，工具已经执行成功。请基于工具结果继续选择你的动作。"
			}
			return &types.LatestToolCall{
				Role:        role,
				ToolName:    toolName,
				Instruction: instruction,
			}
		}
		if content == "" {
			return nil
		}
		return &types.LatestToolCall{
			Role:        role,
			Content:     content,
			Instruction: "你刚刚输出了以下内容，现在继续。",
		}
	case "user":
		if content == "" {
			return nil
		}
		return &types.LatestToolCall{
			Role:        role,
			Content:     content,
			Instruction: "用户刚刚说了以下内容，需要你作答。",
		}
	case "tool":
		toolName := latestToolNameFromHistoryItem(last, chatHistory)
		instruction := "你刚刚申请了调用工具，工具已经执行成功。请基于工具结果继续选择你的动作。"
		if toolName != "" {
			instruction = "你刚刚申请了调用工具 " + toolName + "，工具已经执行成功。请基于工具结果继续选择你的动作。"
		}
		return &types.LatestToolCall{
			Role:        role,
			ToolName:    toolName,
			Content:     content,
			Instruction: instruction,
		}
	default:
		if role == "tool_call" || strings.HasPrefix(role, "call_tool") {
			toolName := latestToolNameFromHistoryItem(last, chatHistory)
			instruction := "你刚刚申请了调用工具，工具已经执行成功。请基于工具结果继续选择你的动作。"
			if toolName != "" {
				instruction = "你刚刚申请了调用工具 " + toolName + "，工具已经执行成功。请基于工具结果继续选择你的动作。"
			}
			return &types.LatestToolCall{
				Role:        role,
				ToolName:    toolName,
				Instruction: instruction,
			}
		}
		return buildLatestToolCallLegacyDefault(role, content)
	}
}

func latestToolName(chatHistory []*types.ChatHistoryItem) string {
	if len(chatHistory) < 2 {
		return ""
	}

	prev := chatHistory[len(chatHistory)-2]
	if prev == nil {
		return ""
	}
	if len(prev.NativeToolCalls) > 0 {
		return strings.TrimSpace(prev.NativeToolCalls[len(prev.NativeToolCalls)-1].Name)
	}

	content := prev.RenderContent()
	if content == "" {
		return ""
	}

	var flat struct {
		CallTool string          `json:"call_tool"`
		Params   json.RawMessage `json:"params"`
	}
	if err := json.Unmarshal([]byte(content), &flat); err != nil {
		return ""
	}
	if strings.TrimSpace(flat.CallTool) != "call_tool" {
		return ""
	}
	return toolNameFromCallToolParams(flat.Params)
}

// filterMessagesByWorker 根据 worker 名称过滤消息
// from 和 to 参数指定对话的双方
// func _filterMessagesByWorker(messages []*WithParts, from, to string) []*WithParts {
// 	if to == "" {
// 		return messages
// 	}

// 	filtered := make([]*WithParts, 0, len(messages))

// 	copyMessages := func(msg *WithParts) *WithParts {
// 		copy := *msg
// 		info := *copy.Info
// 		copy.Info = &info
// 		return &copy
// 	}

// 	for _, msg := range messages {
// 		if msg.Info == nil {
// 			continue
// 		}

// 		// 目标 worker 的消息作为 assistant
// 		if msg.Info.Name == to {
// 			newMsg := copyMessages(msg)
// 			newMsg.Info.Role = RoleAssistant
// 			filtered = append(filtered, newMsg)
// 		}

// 		// from 的消息作为 user
// 		if msg.Info.Name == from {
// 			newMsg := copyMessages(msg)
// 			newMsg.Info.Role = RoleUser
// 			filtered = append(filtered, newMsg)
// 		}
// 	}

// 	return filtered
// }
