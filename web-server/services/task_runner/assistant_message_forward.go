package task_runner

import (
	"strings"
	"time"

	"matrixops-agent/types"
	"pkgs/db/models"
)

const TaskMessageTypeAssistantAttachment = "assistant_attachment"

type AssistantAttachmentPayload struct {
	URL      string `json:"url"`
	Filename string `json:"filename"`
	Mime     string `json:"mime"`
}

func shouldForwardAssistantMessage(info *types.MessageInfo) bool {
	if info == nil {
		return false
	}
	if info.Time.Completed > 0 {
		return true
	}
	return strings.TrimSpace(info.Finish) == "step-finish"
}

func assistantForwardDedupeKey(messageID, text string) string {
	return strings.TrimSpace(messageID) + "\x00" + strings.TrimSpace(text)
}

func collectAssistantFileParts(parts []*types.Part) []*types.Part {
	if len(parts) == 0 {
		return nil
	}
	out := make([]*types.Part, 0, len(parts))
	for _, part := range parts {
		if part == nil || part.Type != "file" {
			continue
		}
		if strings.TrimSpace(part.URL) == "" {
			continue
		}
		out = append(out, part)
	}
	return out
}

func assistantAttachmentDedupeKey(messageID, partKey string) string {
	return strings.TrimSpace(messageID) + "\x00" + strings.TrimSpace(partKey)
}

func attachmentPartKey(part *types.Part) string {
	if part == nil {
		return ""
	}
	if id := strings.TrimSpace(part.ID); id != "" {
		return id
	}
	return strings.TrimSpace(part.URL)
}

func (r *TaskRuntime) maybeForwardAssistantMessage(messageInfoWithParts *types.WithParts) {
	if r == nil || r.emitter == nil || messageInfoWithParts == nil || messageInfoWithParts.Info == nil {
		return
	}
	info := messageInfoWithParts.Info
	if info.Role != types.RoleAssistant {
		return
	}
	if !r.turnForwardToWeChat.Load() {
		return
	}
	if !shouldForwardAssistantMessage(info) {
		return
	}

	text := collectAssistantAnswerText(messageInfoWithParts.Parts)
	if text != "" {
		key := assistantForwardDedupeKey(info.ID, text)
		r.forwardMu.Lock()
		if r.forwardedAssistantKeys == nil {
			r.forwardedAssistantKeys = make(map[string]struct{})
		}
		shouldEmit := false
		if _, exists := r.forwardedAssistantKeys[key]; !exists {
			r.forwardedAssistantKeys[key] = struct{}{}
			shouldEmit = true
		}
		r.forwardMu.Unlock()
		if shouldEmit {
			r.emitter.EmitTaskMessage(&models.TaskMessage{
				Type:      "message",
				Role:      "assistant",
				Content:   text,
				Timestamp: time.Now().UnixMilli(),
			})
		}
	}

	for _, part := range collectAssistantFileParts(messageInfoWithParts.Parts) {
		partKey := attachmentPartKey(part)
		if partKey == "" {
			continue
		}
		key := assistantAttachmentDedupeKey(info.ID, partKey)
		r.forwardMu.Lock()
		if r.forwardedAssistantAttachmentKeys == nil {
			r.forwardedAssistantAttachmentKeys = make(map[string]struct{})
		}
		shouldEmit := false
		if _, exists := r.forwardedAssistantAttachmentKeys[key]; !exists {
			r.forwardedAssistantAttachmentKeys[key] = struct{}{}
			shouldEmit = true
		}
		r.forwardMu.Unlock()
		if !shouldEmit {
			continue
		}
		r.emitter.EmitTaskMessage(&models.TaskMessage{
			Type: TaskMessageTypeAssistantAttachment,
			Role: "assistant",
			Content: AssistantAttachmentPayload{
				URL:      part.URL,
				Filename: part.Filename,
				Mime:     part.Mime,
			},
			Timestamp: time.Now().UnixMilli(),
		})
	}
}
