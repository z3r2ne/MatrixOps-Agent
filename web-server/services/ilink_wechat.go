package services

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"log"
	"strings"
	"time"

	"matrixops-agent/ilink"
	"matrixops-agent/types"
	"matrixops/services/task_runner"
)

func (rt *ilinkAccount) startTypingLoop() {
	rt.stopTypingLoop()

	ctx, cancel := context.WithCancel(context.Background())
	rt.typingMu.Lock()
	rt.typingCancel = cancel
	rt.typingMu.Unlock()

	go func() {
		sendTyping := func() {
			rt.msgMu.Lock()
			userID := rt.lastFromUserID
			token := rt.lastCtxToken
			rt.msgMu.Unlock()
			if userID == "" {
				return
			}
			tctx, tcancel := context.WithTimeout(context.Background(), 10*time.Second)
			if err := rt.bot.SendTyping(tctx, userID, token); err != nil {
				log.Printf("[ilink] send typing failed: %v", err)
			}
			tcancel()
		}

		sendTyping()
		ticker := time.NewTicker(4 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				rt.msgMu.Lock()
				userID := rt.lastFromUserID
				token := rt.lastCtxToken
				rt.msgMu.Unlock()
				if userID != "" {
					cancelCtx, cancelFn := context.WithTimeout(context.Background(), 5*time.Second)
					if err := rt.bot.CancelTyping(cancelCtx, userID, token); err != nil {
						log.Printf("[ilink] cancel typing failed: %v", err)
					}
					cancelFn()
				}
				return
			case <-ticker.C:
				sendTyping()
			}
		}
	}()
}

func (rt *ilinkAccount) stopTypingLoop() {
	rt.typingMu.Lock()
	cancel := rt.typingCancel
	rt.typingCancel = nil
	rt.typingMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func wechatAttachmentsToInputParts(attachments []ilink.InboundAttachment) []*types.Part {
	if len(attachments) == 0 {
		return nil
	}
	parts := make([]*types.Part, 0, len(attachments))
	for _, attachment := range attachments {
		mimeType := strings.TrimSpace(attachment.MimeType)
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}
		filename := strings.TrimSpace(attachment.Filename)
		if filename == "" {
			filename = "attachment"
		}
		dataURL := "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(attachment.Data)
		parts = append(parts, &types.Part{
			Type:     "file",
			URL:      dataURL,
			Mime:     mimeType,
			Filename: filename,
		})
	}
	return parts
}

func parseAssistantAttachmentPayload(content any) (task_runner.AssistantAttachmentPayload, bool) {
	switch value := content.(type) {
	case task_runner.AssistantAttachmentPayload:
		if strings.TrimSpace(value.URL) == "" {
			return task_runner.AssistantAttachmentPayload{}, false
		}
		return value, true
	case map[string]interface{}:
		raw, err := json.Marshal(value)
		if err != nil {
			return task_runner.AssistantAttachmentPayload{}, false
		}
		var payload task_runner.AssistantAttachmentPayload
		if err := json.Unmarshal(raw, &payload); err != nil {
			return task_runner.AssistantAttachmentPayload{}, false
		}
		if strings.TrimSpace(payload.URL) == "" {
			return task_runner.AssistantAttachmentPayload{}, false
		}
		return payload, true
	default:
		return task_runner.AssistantAttachmentPayload{}, false
	}
}

func (rt *ilinkAccount) sendAttachmentToWeChat(ctx context.Context, payload task_runner.AssistantAttachmentPayload) error {
	rt.msgMu.Lock()
	toUserID := rt.lastFromUserID
	ctxToken := rt.lastCtxToken
	rt.msgMu.Unlock()
	if toUserID == "" {
		return nil
	}

	url := strings.TrimSpace(payload.URL)
	filename := strings.TrimSpace(payload.Filename)
	switch {
	case strings.HasPrefix(url, "data:"):
		return rt.bot.SendMediaFromDataURL(ctx, toUserID, url, filename, ctxToken)
	case strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://"):
		return rt.bot.SendMediaFromURL(ctx, toUserID, url, ctxToken)
	default:
		log.Printf("[ilink] unsupported attachment url scheme: %q", url)
		return nil
	}
}
