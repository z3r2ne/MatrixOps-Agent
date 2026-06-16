package session

import (
	"fmt"
	"strings"

	"matrixops-agent/llm"
	agenttool "matrixops-agent/tool"
	"pkgs/db/models"
	mcppkg "pkgs/mcp"
)

type WorkerToolDisabledError struct {
	WorkerName string
	ToolName   string
}

func (e WorkerToolDisabledError) Error() string {
	return fmt.Sprintf("worker %s has not enabled tool %s", e.WorkerName, e.ToolName)
}

type ProjectToolDeniedError struct {
	ProjectName string
	ToolName    string
}

func (e ProjectToolDeniedError) Error() string {
	return fmt.Sprintf("project %s denied tool %s", e.ProjectName, e.ToolName)
}

type ProjectToolRejectedError struct {
	ProjectName string
	ToolName    string
	Reason      string
}

func (e ProjectToolRejectedError) Error() string {
	return fmt.Sprintf("project %s rejected tool %s", e.ProjectName, e.ToolName)
}

func buildToolOverrides(
	registry *agenttool.Registry,
	workerEnabledTools map[string]struct{},
	hasWorkerEnabledTools bool,
	project *models.Project,
	projectToolPermissions map[string]string,
) map[string]bool {
	if registry == nil {
		return nil
	}

	overrides := make(map[string]bool)
	for _, name := range registry.Names() {
		if hasWorkerEnabledTools && !mcppkg.IsMcpToolFullName(name) && !agenttool.IsWebSearchTool(name) && !agenttool.IsMemorySearchTool(name) {
			if _, ok := workerEnabledTools[name]; !ok {
				overrides[name] = false
				continue
			}
		}

		if project != nil && !project.YoloMode {
			if projectToolPermissions[name] == models.ProjectToolPermissionDeny {
				overrides[name] = false
			}
		}
	}

	if len(overrides) == 0 {
		return nil
	}
	return overrides
}

func isWorkerToolEnabled(toolName string, workerEnabledTools map[string]struct{}, hasWorkerEnabledTools bool) bool {
	if mcppkg.IsMcpToolFullName(toolName) || agenttool.IsWebSearchTool(toolName) || agenttool.IsMemorySearchTool(toolName) {
		return true
	}
	if !hasWorkerEnabledTools {
		return true
	}
	_, ok := workerEnabledTools[toolName]
	return ok
}

func projectToolAction(project *models.Project, projectToolPermissions map[string]string, toolName string) string {
	if project == nil {
		return models.ProjectToolPermissionAllow
	}
	if project.YoloMode {
		return models.ProjectToolPermissionAllow
	}
	if action, ok := projectToolPermissions[toolName]; ok && models.IsValidProjectToolPermission(action) {
		return action
	}
	if agenttool.IsPermissionExemptTool(toolName) {
		return models.ProjectToolPermissionAllow
	}
	return models.ProjectToolPermissionAsk
}

func buildProjectToolApprovalPayload(
	project *models.Project,
	worker *models.Worker,
	call llm.ToolCall,
	toolInstance agenttool.Tool,
) map[string]interface{} {
	payload := map[string]interface{}{
		"kind": "project_tool_permission",
		"tool": map[string]interface{}{
			"name":        call.Name,
			"label":       toolInstance.VerbosName(),
			"description": toolInstance.Description(),
		},
		"worker": map[string]interface{}{
			"name": worker.Name,
		},
	}

	if project != nil {
		payload["project"] = map[string]interface{}{
			"id":   project.ID,
			"name": project.Name,
		}
	}

	request := map[string]interface{}{}
	if path, ok := call.Arguments["path"].(string); ok && strings.TrimSpace(path) != "" {
		request["path"] = path
	}
	if command, ok := call.Arguments["command"].(string); ok && strings.TrimSpace(command) != "" {
		request["command"] = command
	}
	if content, ok := call.Arguments["content"].(string); ok && strings.TrimSpace(content) != "" {
		request["contentPreview"] = truncatePreview(content, 240)
	}
	if patch, ok := call.Arguments["patch"].(string); ok && strings.TrimSpace(patch) != "" {
		request["patchPreview"] = truncatePreview(patch, 240)
	}
	if len(request) > 0 {
		payload["request"] = request
	}

	return payload
}

func truncatePreview(value string, limit int) string {
	trimmed := strings.TrimSpace(value)
	if limit <= 0 || len(trimmed) <= limit {
		return trimmed
	}
	return trimmed[:limit] + "..."
}

func (r *AgentRunner) authorizeToolCall(
	runtimeConfig *RuntimeConfig,
	worker *models.Worker,
	call llm.ToolCall,
	toolInstance agenttool.Tool,
) error {
	if !isWorkerToolEnabled(call.Name, runtimeConfig.WorkerEnabledTools, runtimeConfig.HasWorkerEnabledTools) {
		return WorkerToolDisabledError{
			WorkerName: worker.Name,
			ToolName:   call.Name,
		}
	}

	action := projectToolAction(runtimeConfig.Project, runtimeConfig.ProjectToolPermissions, call.Name)
	switch action {
	case models.ProjectToolPermissionAllow:
		return nil
	case models.ProjectToolPermissionDeny:
		projectName := ""
		if runtimeConfig.Project != nil {
			projectName = runtimeConfig.Project.Name
		}
		return ProjectToolDeniedError{
			ProjectName: projectName,
			ToolName:    call.Name,
		}
	case models.ProjectToolPermissionAsk:
		if r.emitter == nil {
			projectName := ""
			if runtimeConfig.Project != nil {
				projectName = runtimeConfig.Project.Name
			}
			return ProjectToolDeniedError{
				ProjectName: projectName,
				ToolName:    call.Name,
			}
		}
		r.toolPermissionMu.Lock()
		defer r.toolPermissionMu.Unlock()
		result, err := r.emitter.WaitUserInput(buildProjectToolApprovalPayload(runtimeConfig.Project, worker, call, toolInstance))
		if err != nil {
			return err
		}
		decision, _ := result["decision"].(string)
		reason, _ := result["reason"].(string)
		if decision != "allow" {
			projectName := ""
			if runtimeConfig.Project != nil {
				projectName = runtimeConfig.Project.Name
			}
			return ProjectToolRejectedError{
				ProjectName: projectName,
				ToolName:    call.Name,
				Reason:      strings.TrimSpace(reason),
			}
		}
		return nil
	default:
		return nil
	}
}
