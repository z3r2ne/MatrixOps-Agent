package session

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"matrixops-agent/llm"
	"matrixops-agent/tool"
	coreagent "matrixops.local/core_agent"
	mcppkg "pkgs/mcp"
	"pkgs/db/models"
)

func parseProgressSubtaskTaskID(metadata map[string]interface{}) (uint, bool) {
	if metadata == nil {
		return 0, false
	}
	raw, ok := metadata["subtaskTaskId"]
	if !ok {
		raw = metadata["subtaskTaskID"]
	}
	switch value := raw.(type) {
	case float64:
		if value > 0 {
			return uint(value), true
		}
	case int:
		if value > 0 {
			return uint(value), true
		}
	case uint:
		if value > 0 {
			return value, true
		}
	case json.Number:
		parsed, err := value.Int64()
		if err == nil && parsed > 0 {
			return uint(parsed), true
		}
	}
	return 0, false
}

func isAsyncEligibleTool(toolName string) bool {
	toolName = strings.TrimSpace(toolName)
	if toolName == "" {
		return false
	}
	if _, _, ok := mcppkg.ParseToolFullName(toolName); ok {
		return false
	}
	return tool.IsAsyncEligibleBuiltinTool(toolName)
}

func formatAsyncToolResultMessage(toolName string, params map[string]interface{}, duration time.Duration, status string, result coreagent.ToolResult, execErr error) string {
	toolName = strings.TrimSpace(toolName)
	status = strings.TrimSpace(status)
	if status == "" {
		status = "completed"
	}
	paramsText := tool.ParamsJSONString(params)
	content := strings.TrimSpace(result.Content)
	if content == "" && execErr != nil {
		content = execErr.Error()
	}
	if result.IsError && content == "" {
		content = "tool execution failed"
	}
	body := strings.TrimSpace(fmt.Sprintf(
		"工具：%s\n参数：%s\n执行时长：%.1fs\n状态：%s\n结果：\n%s",
		toolName,
		paramsText,
		duration.Seconds(),
		status,
		content,
	))
	return fmt.Sprintf("<async_tool_result>\n%s\n</async_tool_result>", body)
}

func formatAsyncToolStartUserVisibleBody(callID string, subtaskTaskID uint) string {
	callID = strings.TrimSpace(callID)
	lines := []string{"异步任务已启动"}
	if callID != "" {
		lines = append(lines, "call_id: "+callID)
	}
	if subtaskTaskID > 0 {
		lines = append(lines, fmt.Sprintf("task_id: %d", subtaskTaskID))
	}
	return strings.Join(lines, "\n")
}

func (r *AgentRunner) executeSessionToolCallWithAsync(
	runtimeConfig *RuntimeConfig,
	actionCtx *coreagent.ActionContext,
	toolInstance coreagent.Tool,
	call coreagent.ToolCall,
	toolCtx coreagent.ToolContext,
	execute func(coreagent.ToolCall, coreagent.ToolContext) (coreagent.ToolResult, error),
) (coreagent.ToolResult, error) {
	async, strippedArgs := tool.ParseAsyncFlag(call.Arguments)
	if !async {
		return execute(call, toolCtx)
	}
	if !isAsyncEligibleTool(call.Name) {
		return coreagent.ToolResult{
			IsError: true,
			Name:    call.Name,
			Content: fmt.Sprintf("tool %q does not support async execution", call.Name),
		}, nil
	}
	return r.startAsyncToolCall(runtimeConfig, call, strippedArgs, toolCtx, execute)
}

func (r *AgentRunner) startAsyncToolCall(
	runtimeConfig *RuntimeConfig,
	call coreagent.ToolCall,
	args map[string]interface{},
	toolCtx coreagent.ToolContext,
	execute func(coreagent.ToolCall, coreagent.ToolContext) (coreagent.ToolResult, error),
) (coreagent.ToolResult, error) {
	callID := strings.TrimSpace(call.ID)
	if callID == "" {
		callID = fmt.Sprintf("async-tool-%d", time.Now().UnixNano())
	}
	toolName := strings.TrimSpace(call.Name)

	startedAt := time.Now()
	asyncCall := call
	asyncCall.Arguments = args
	asyncCall.ID = callID
	asyncToolCtx := toolCtx
	if asyncToolCtx.Values == nil {
		asyncToolCtx.Values = map[string]interface{}{}
	}

	// Special-case run_worker_task: wait for the first progress event to learn subtask task_id,
	// then persist critical info and emit the async start system message containing that id.
	var (
		subtaskTaskIDCh chan uint
		subtaskTaskID   uint
	)
	if strings.TrimSpace(toolName) == "run_worker_task" {
		subtaskTaskIDCh = make(chan uint, 1)
		if rawHandler, ok := asyncToolCtx.Values["tool_event_handler"]; ok {
			if handler, ok := rawHandler.(func(tool.StreamEvent)); ok {
				asyncToolCtx.Values["tool_event_handler"] = func(ev tool.StreamEvent) {
					handler(ev)
					if id, ok := parseProgressSubtaskTaskID(ev.Metadata); ok {
						select {
						case subtaskTaskIDCh <- id:
						default:
						}
					}
				}
			}
		}
	}

	go func() {
		toolExecCtx, _, cleanup := tool.DeriveToolCallContext(asyncToolCtx.Context, callID, toolName)
		defer cleanup()
		asyncToolCtx.Context = toolExecCtx

		result, execErr := execute(asyncCall, asyncToolCtx)
		status := "completed"
		if execErr != nil || result.IsError {
			status = "failed"
		}
		if toolExecCtx.Err() != nil {
			status = "cancelled"
			if execErr == nil {
				execErr = context.Cause(toolExecCtx)
			}
			if strings.TrimSpace(result.Content) == "" {
				result = coreagent.ToolResult{
					IsError: true,
					Name:    toolName,
					Content: "[Tool Cancelled]: async tool execution was cancelled by user",
					Metadata: map[string]interface{}{
						"cancelled":   true,
						"cancelledBy": "user",
					},
				}
			}
		}
		r.finishAsyncToolCall(runtimeConfig, callID, toolName, args, startedAt, status, result, execErr)
	}()

	if subtaskTaskIDCh != nil {
		select {
		case subtaskTaskID = <-subtaskTaskIDCh:
		case <-time.After(3 * time.Second):
		}
	}

	placeholder := fmt.Sprintf(
		"工具 %q 已异步启动（call_id=%s）。完成后将以 async_tool_result 补充消息告知结果；进行中的任务可在侧栏查看或手动结束。",
		toolName,
		callID,
	)

	item := newAsyncToolCriticalInfoItem(callID, toolName, args, subtaskTaskID, placeholder)
	if err := r.upsertCriticalInfoItem(item); err != nil {
		return coreagent.ToolResult{IsError: true, Name: toolName, Content: err.Error()}, err
	}

	if runtimeConfig != nil {
		_ = r.deliverSupplementUserMessage(runtimeConfig, models.TaskMessageQueueItem{
			ID:         fmt.Sprintf("async-tool-start-%s", callID),
			Type:       models.TaskMessageQueueTypeSystem,
			Content:    formatAsyncToolStartUserVisibleBody(callID, subtaskTaskID),
			Source:     "async_tool_start",
			Supplement: true,
			CreatedAt:  time.Now().UnixMilli(),
		})
		if runtimeConfig.MemoryState != nil {
			// 关键信息仍以详细系统消息形式进入上下文（用于记忆压缩后重注入）。
			appendCriticalInfoMessageToMemory(runtimeConfig.MemoryState, item.Message)
		}
	}

	return coreagent.ToolResult{
		Name:    toolName,
		Content: placeholder,
		Metadata: map[string]interface{}{
			"async":  true,
			"callId": callID,
			"status": "running",
		},
	}, nil
}

func (r *AgentRunner) finishAsyncToolCall(
	runtimeConfig *RuntimeConfig,
	callID string,
	toolName string,
	params map[string]interface{},
	startedAt time.Time,
	status string,
	result coreagent.ToolResult,
	execErr error,
) {
	if r == nil {
		return
	}
	callID = strings.TrimSpace(callID)
	toolName = strings.TrimSpace(toolName)
	duration := time.Since(startedAt)
	message := formatAsyncToolResultMessage(toolName, params, duration, status, result, execErr)

	if r.messageQueue != nil {
		item := models.TaskMessageQueueItem{
			ID:         fmt.Sprintf("async-tool-result-%s", callID),
			Type:       models.TaskMessageQueueTypeSystem,
			Content:    message,
			Source:     models.TaskMessageQueueSourceAsyncToolResult,
			Supplement: true,
			Metadata: map[string]interface{}{
				"callID":   callID,
				"toolName": toolName,
				"status":   status,
			},
			CreatedAt: time.Now().UnixMilli(),
		}
		_ = r.prependSupplementQueueItem(item)
	}

	_ = r.removeCriticalInfoItem(formatCriticalInfoID(callID))
	r.emitSessionInfoUpdated()
	_ = runtimeConfig
	_ = execErr
}

func parseAsyncSubtaskTaskID(metadata map[string]interface{}) (uint, bool) {
	if metadata == nil {
		return 0, false
	}
	raw, ok := metadata["subtaskTaskId"]
	if !ok {
		raw = metadata["taskId"]
	}
	switch value := raw.(type) {
	case float64:
		if value > 0 {
			return uint(value), true
		}
	case int:
		if value > 0 {
			return uint(value), true
		}
	case uint:
		if value > 0 {
			return value, true
		}
	case json.Number:
		parsed, err := value.Int64()
		if err == nil && parsed > 0 {
			return uint(parsed), true
		}
	}
	return 0, false
}

func (r *AgentRunner) buildSessionToolExecutor(
	runtimeConfig *RuntimeConfig,
) func(actionCtx *coreagent.ActionContext, toolInstance coreagent.Tool, call coreagent.ToolCall, toolCtx coreagent.ToolContext) (coreagent.ToolResult, error) {
	return func(actionCtx *coreagent.ActionContext, toolInstance coreagent.Tool, call coreagent.ToolCall, toolCtx coreagent.ToolContext) (coreagent.ToolResult, error) {
		if actionCtx != nil {
			part := actionCtx.GetToolPart(call.ID)
			if part != nil {
				if toolCtx.Values == nil {
					toolCtx.Values = map[string]interface{}{}
				}
				toolCtx.Values["tool_event_handler"] = newCoreToolProgressReporter(actionCtx, part)
			}
			if call.ID != "" && actionCtx.IsToolPreAuthorized(call.ID) {
				if toolCtx.Values == nil {
					toolCtx.Values = map[string]interface{}{}
				}
				toolCtx.Values[tool.ToolContextSkipAuthorizeKey] = true
			}
		}
		execute := func(execCall coreagent.ToolCall, execCtx coreagent.ToolContext) (coreagent.ToolResult, error) {
			result, execErr := toolInstance.Execute(execCtx, execCall.Arguments)
			if execErr != nil {
				return result, execErr
			}
			return result, nil
		}
		return r.executeSessionToolCallWithAsync(runtimeConfig, actionCtx, toolInstance, call, toolCtx, execute)
	}
}

func sessionToolCallFromCore(call coreagent.ToolCall) llm.ToolCall {
	return llm.ToolCall{
		ID:        call.ID,
		Name:      call.Name,
		Arguments: call.Arguments,
	}
}
