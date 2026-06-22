package session

import (
	"fmt"
	"strings"
	"time"

	"matrixops-agent/tool"
	"matrixops-agent/types"
	coreagent "matrixops.local/core_agent"
	sessionmemory "matrixops-agent/session/memory"
	"pkgs/db/storage"
)

const (
	criticalInfoTypeAsyncTool = "async_tool"
	criticalInfoMarkerPrefix  = "[[critical_info:"
	criticalInfoMarkerSuffix  = "]]"
)

func criticalInfoMarker(id string) string {
	id = strings.TrimSpace(id)
	return criticalInfoMarkerPrefix + id + criticalInfoMarkerSuffix
}

func formatCriticalInfoID(callID string) string {
	callID = strings.TrimSpace(callID)
	if callID == "" {
		return "async_tool:unknown"
	}
	return "async_tool:" + callID
}

func formatAsyncToolStartBody(toolName string, params map[string]interface{}, callID string, subtaskTaskID uint, bashJobID string) string {
	toolName = strings.TrimSpace(toolName)
	callID = strings.TrimSpace(callID)
	bashJobID = strings.TrimSpace(bashJobID)
	paramsText := tool.ParamsJSONString(params)
	subtaskLine := ""
	if subtaskTaskID > 0 {
		subtaskLine = fmt.Sprintf("\ntask_id: %d", subtaskTaskID)
	}
	bashJobLine := ""
	if bashJobID != "" {
		bashJobLine = "\nbash_job_id: " + bashJobID
	}
	return strings.TrimSpace(fmt.Sprintf(
		"异步工具任务已启动：%s\n参数：%s\ncall_id: %s%s%s",
		toolName,
		paramsText,
		callID,
		subtaskLine,
		bashJobLine,
	))
}

func formatAsyncToolStartMessage(toolName string, params map[string]interface{}, callID string, subtaskTaskID uint, bashJobID string) string {
	return coreagent.FormatSystemSupplementMessage(formatAsyncToolStartBody(toolName, params, callID, subtaskTaskID, bashJobID))
}

func criticalInfoPresentInTranscript(transcript string, item types.CriticalInfoItem) bool {
	transcript = strings.TrimSpace(transcript)
	if transcript == "" {
		return false
	}

	for _, source := range item.MatchSources {
		switch strings.TrimSpace(source.Kind) {
		case "tool_call":
			name := strings.TrimSpace(source.ToolName)
			params := strings.TrimSpace(source.ParamsJSON)
			output := strings.TrimSpace(source.Output)
			if name == "" || params == "" || output == "" {
				continue
			}
			if !strings.Contains(transcript, name) {
				continue
			}
			if !strings.Contains(transcript, params) {
				continue
			}
			if !strings.Contains(transcript, output) {
				continue
			}
			return true
		}
	}

	return false
}

func (r *AgentRunner) loadSessionCriticalInfo() *types.CriticalInfo {
	if r == nil {
		return nil
	}
	if r.session != nil && r.session.CriticalInfo != nil {
		return r.session.CriticalInfo
	}
	sessionID := r.GetSessionID()
	if sessionID == "" || r.db == nil {
		return nil
	}
	info, err := storage.GetSessionCriticalInfo(r.db, sessionID)
	if err != nil || info == nil {
		return nil
	}
	if r.session != nil {
		r.session.CriticalInfo = info
	}
	return info
}

func (r *AgentRunner) persistSessionCriticalInfo(criticalInfo *types.CriticalInfo) error {
	if r == nil || r.db == nil {
		return nil
	}
	sessionID := r.GetSessionID()
	if sessionID == "" {
		return nil
	}
	updated, err := storage.UpdateSessionCriticalInfo(r.db, sessionID, criticalInfo)
	if err != nil {
		return err
	}
	if r.session != nil && updated != nil {
		r.session.CriticalInfo = updated.CriticalInfo
	}
	r.emitSessionInfoUpdated()
	return nil
}

func (r *AgentRunner) emitSessionInfoUpdated() {
	if r == nil || r.emitter == nil || r.session == nil {
		return
	}
	r.emitter.Emit(EventSessionUpdated, SessionEvent{Info: r.session})
}

func (r *AgentRunner) ensureCriticalInfoInContext(runtimeConfig *RuntimeConfig) error {
	if r == nil || runtimeConfig == nil {
		return nil
	}
	criticalInfo := r.loadSessionCriticalInfo()
	if criticalInfo == nil || len(criticalInfo.Items) == 0 {
		return nil
	}
	if err := r.ensureRuntimeMemoryState(runtimeConfig); err != nil {
		return err
	}
	if runtimeConfig.MemoryState == nil {
		return nil
	}
	transcript := runtimeConfig.MemoryState.Snapshot().PromptContent()
	for _, item := range criticalInfo.Items {
		if criticalInfoPresentInTranscript(transcript, item) {
			continue
		}
		message := strings.TrimSpace(item.Message)
		if message == "" {
			continue
		}
		appendCriticalInfoMessageToMemory(runtimeConfig.MemoryState, message)
		transcript = runtimeConfig.MemoryState.Snapshot().PromptContent()
	}
	return nil
}

func appendCriticalInfoMessageToMemory(memoryState *ProcessV2MemoryState, message string) {
	if memoryState == nil || strings.TrimSpace(message) == "" {
		return
	}
	snapshot := memoryState.Snapshot()
	now := time.Now().UnixMilli()
	if len(snapshot.Entries) > 0 {
		entries := sessionmemory.CloneMemoryEntries(snapshot.Entries)
		entries = append(entries, &types.MemoryEntry{
			Role:      "user",
			Content:   message,
			Synthetic: true,
			Created:   now,
			Updated:   now,
		})
		memoryState.ReplaceEntries(entries)
		return
	}
	memoryState.AppendUserText(message, nil)
}

func (r *AgentRunner) upsertCriticalInfoItem(item types.CriticalInfoItem) error {
	criticalInfo := r.loadSessionCriticalInfo()
	if criticalInfo == nil {
		criticalInfo = &types.CriticalInfo{}
	}
	found := false
	for index, existing := range criticalInfo.Items {
		if existing.ID == item.ID {
			criticalInfo.Items[index] = item
			found = true
			break
		}
	}
	if !found {
		criticalInfo.Items = append(criticalInfo.Items, item)
	}
	return r.persistSessionCriticalInfo(criticalInfo)
}

func (r *AgentRunner) removeCriticalInfoItem(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil
	}
	criticalInfo := r.loadSessionCriticalInfo()
	if criticalInfo == nil || len(criticalInfo.Items) == 0 {
		return nil
	}
	next := make([]types.CriticalInfoItem, 0, len(criticalInfo.Items))
	for _, item := range criticalInfo.Items {
		if item.ID == id {
			continue
		}
		next = append(next, item)
	}
	if len(next) == 0 {
		return r.persistSessionCriticalInfo(nil)
	}
	return r.persistSessionCriticalInfo(&types.CriticalInfo{Items: next})
}

func newAsyncToolCriticalInfoItem(callID, toolName string, params map[string]interface{}, subtaskTaskID uint, bashJobID string, launchOutput string) types.CriticalInfoItem {
	now := time.Now().UnixMilli()
	id := formatCriticalInfoID(callID)
	paramsCopy := map[string]interface{}{}
	for key, value := range params {
		paramsCopy[key] = value
	}
	paramsJSON := tool.ParamsJSONString(paramsCopy)
	matchSources := []types.CriticalInfoMatchSource{
		{
			Kind:       "tool_call",
			ToolName:   strings.TrimSpace(toolName),
			ParamsJSON: paramsJSON,
			Output:     strings.TrimSpace(launchOutput),
		},
	}
	return types.CriticalInfoItem{
		ID:           id,
		Type:         criticalInfoTypeAsyncTool,
		Marker:       "",
		Message:      formatAsyncToolStartMessage(toolName, paramsCopy, callID, subtaskTaskID, bashJobID),
		MatchSources: matchSources,
		CreatedAt:    now,
		AsyncTask: &types.AsyncToolTaskMeta{
			CallID:    callID,
			ToolName:  toolName,
			Params:    paramsCopy,
			Status:    "running",
			StartedAt: now,
			TaskID:    subtaskTaskID,
			BashJobID: strings.TrimSpace(bashJobID),
		},
	}
}
