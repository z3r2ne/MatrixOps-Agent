package session

import (
	"context"
	"errors"
	"strings"
	"time"

	agenttool "matrixops-agent/tool"
	"matrixops-agent/types"
	"matrixops-agent/util"
)

const userCancelledConversationReason = "user-cancelled"

func isUserCancelledContext(ctx context.Context, err error) bool {
	if err == nil || !errors.Is(err, context.Canceled) {
		return false
	}
	if ctx == nil {
		return false
	}
	return errors.Is(context.Cause(ctx), agenttool.ErrTaskExecutionCancelledByUser)
}

func newAssistantUserCancelledPart(assistant *MessageInfo) *Part {
	if assistant == nil || strings.TrimSpace(assistant.ID) == "" || strings.TrimSpace(assistant.SessionID) == "" {
		return nil
	}
	now := time.Now().UnixMilli()
	return &Part{
		ID:        util.Ascending("part"),
		MessageID: assistant.ID,
		SessionID: assistant.SessionID,
		Type:      types.PartTypeFinishStep,
		Text:      "已取消对话",
		Reason:    userCancelledConversationReason,
		Time: &PartTime{
			Start:   now,
			End:     now,
			Created: now,
		},
	}
}

func (r *AgentRunner) emitAssistantUserCancelledPart(assistant *MessageInfo) {
	part := newAssistantUserCancelledPart(assistant)
	if part == nil || r == nil || r.emitter == nil {
		return
	}
	_, _ = r.emitter.UpdatePart(part)
}

func (r *AgentRunner) clearAssistantFooterStatus(messageID string) {
	if r == nil || r.emitter == nil || strings.TrimSpace(messageID) == "" {
		return
	}
	r.emitter.Emit(EventAssistantFooterStatus, AssistantFooterStatusEvent{
		MessageID: messageID,
		Text:      "",
		Loading:   false,
	})
}
