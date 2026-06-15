package session

import (
	"fmt"
	"strings"

	coreagent "matrixops.local/core_agent"
	"matrixops-agent/types"
	"pkgs/db/models"
	"pkgs/db/storage"
)

func queueItemToSessionParts(item models.TaskMessageQueueItem) []*Part {
	parts := make([]*Part, 0, 1+len(item.Parts))
	if content := strings.TrimSpace(item.Content); content != "" {
		if item.IsSystem() || item.IsAppend() {
			content = coreagent.FormatSystemSupplementMessage(content)
		}
		parts = append(parts, &Part{
			Type: types.PartTypeText,
			Text: content,
		})
	}
	for _, p := range item.Parts {
		if strings.TrimSpace(p.Type) != "file" || strings.TrimSpace(p.URL) == "" {
			continue
		}
		parts = append(parts, &Part{
			Type:     "file",
			URL:      p.URL,
			Mime:     p.Mime,
			Filename: p.Filename,
		})
	}
	return parts
}

// deliverSupplementUserMessage 将 supplement/append 作为会话消息写入，并刷新 runtime memory。
func (r *AgentRunner) deliverSupplementUserMessage(runtimeConfig *RuntimeConfig, item models.TaskMessageQueueItem) error {
	return r.deliverImmediateQueueItem(runtimeConfig, item)
}

func (r *AgentRunner) deliverImmediateQueueItem(runtimeConfig *RuntimeConfig, item models.TaskMessageQueueItem) error {
	if r == nil || runtimeConfig == nil {
		return nil
	}
	parts := queueItemToSessionParts(item)
	if len(parts) == 0 {
		return nil
	}

	savedParts := runtimeConfig.Parts
	savedUserInput := runtimeConfig.UserInput
	savedKind := runtimeConfig.MessageKind
	savedOrigin := runtimeConfig.MessageOrigin
	runtimeConfig.Parts = parts
	if item.IsAppend() || item.IsSystem() {
		runtimeConfig.MessageKind = MessageKindSystem
		runtimeConfig.MessageOrigin = item.ResolvedSource()
	} else {
		runtimeConfig.MessageKind = MessageKindUser
		runtimeConfig.MessageOrigin = ""
	}
	defer func() {
		runtimeConfig.Parts = savedParts
		runtimeConfig.UserInput = savedUserInput
		runtimeConfig.MessageKind = savedKind
		runtimeConfig.MessageOrigin = savedOrigin
	}()

	userText, toolMessages, err := r.createUserMessage(runtimeConfig)
	if err != nil {
		return fmt.Errorf("create supplement user message: %w", err)
	}
	_ = toolMessages

	if err := r.ensureRuntimeMemoryState(runtimeConfig); err != nil {
		return fmt.Errorf("ensure runtime memory after supplement: %w", err)
	}
	sessionID := r.GetSessionID()
	if sessionID == "" || r.db == nil {
		runtimeConfig.MemoryState.AppendUserText(userText, nil)
		return nil
	}
	entries, err := storage.ListMemoryEntriesBySession(r.db, sessionID)
	if err != nil {
		return fmt.Errorf("reload session memory after supplement: %w", err)
	}
	runtimeConfig.MemoryState.ReplaceEntries(entries)
	runtimeConfig.SetUserInput(userText)
	return nil
}
