package session

import (
	"strings"
	"time"

	"matrixops-agent/types"
	"matrixops-agent/util"
)

func newAssistantErrorPart(assistant *MessageInfo, messageError *MessageError) *Part {
	if assistant == nil || strings.TrimSpace(assistant.ID) == "" || strings.TrimSpace(assistant.SessionID) == "" || messageError == nil {
		return nil
	}
	return &Part{
		ID:        util.Ascending("part"),
		MessageID: assistant.ID,
		SessionID: assistant.SessionID,
		Type:      types.PartTypeError,
		Text:      strings.TrimSpace(messageError.Message),
		Error:     messageError,
		Time: &PartTime{
			Start: time.Now().UnixMilli(),
			End:   time.Now().UnixMilli(),
		},
	}
}

func (r *AgentRunner) emitAssistantErrorPart(assistant *MessageInfo, messageError *MessageError) {
	part := newAssistantErrorPart(assistant, messageError)
	if part == nil || r == nil || r.emitter == nil {
		return
	}
	_, _ = r.emitter.UpdatePart(part)
}
