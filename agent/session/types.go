package session

import (
	"matrixops-agent/global"
	"matrixops-agent/taskctx"
	"matrixops-agent/types"
	"fmt"
	"path/filepath"
	"pkgs/db/models"
)

type Info = types.Info
type Summary = types.Summary
type ShareInfo = types.ShareInfo
type TimeInfo = types.TimeInfo
type RevertInfo = types.RevertInfo
type SessionEvent = types.SessionEvent
type SessionDiffEvent = types.SessionDiffEvent
type SessionErrorEvent = types.SessionErrorEvent
type MessageEvent = types.MessageEvent
type MessageRemovedEvent = types.MessageRemovedEvent
type PartEvent = types.PartEvent
type PartRemovedEvent = types.PartRemovedEvent
type PluginVarSetEvent = types.PluginVarSetEvent
type WaitUserInputEvent = types.WaitUserInputEvent

func PlanPath(task *models.Task, session *Info) string {
	ctx, err := taskctx.Resolve(task)
	if err == nil && ctx.VCS == "git" {
		return filepath.Join(ctx.Worktree, global.ConfigDirName, "plans", planFilename(session))
	}
	return filepath.Join(global.Path.Data, "plans", planFilename(session))
}

func planFilename(session *Info) string {
	return fmt.Sprintf("%d-%s.md", session.Time.Created, session.Slug)
}
