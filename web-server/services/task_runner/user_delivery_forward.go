package task_runner

import (
	"fmt"
	"strings"

	agentsession "matrixops-agent/session"
	agenttool "matrixops-agent/tool"
	"pkgs/db/storage"
)

func (r *TaskRuntime) wrapDeliverUserMessage() agenttool.DeliverUserMessageFunc {
	return func(ctx agenttool.Context, params agenttool.UserDeliveryParams) error {
		emitter := r.sessionEmitter
		if emitter == nil {
			return fmt.Errorf("message: 会话不可用")
		}
		workDir := r.workDir
		if strings.TrimSpace(ctx.Directory) != "" {
			workDir = ctx.Directory
		}
		messageID, err := agentsession.DeliverUserMessage(r.db, emitter, workDir, params)
		if err != nil {
			return err
		}
		if !r.turnForwardToWeChat.Load() || strings.TrimSpace(messageID) == "" {
			return nil
		}
		wp, loadErr := storage.GetMessageWithPartsLight(r.db, messageID)
		if loadErr != nil || wp == nil {
			return nil
		}
		r.maybeForwardAssistantMessage(wp)
		return nil
	}
}
