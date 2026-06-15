package taskctx

import (
	"errors"
	"strconv"

	"matrixops-agent/project"
	"pkgs/db/models"
)

type Context struct {
	Task      *models.Task
	WorkDir   string
	Worktree  string
	ProjectID string
	VCS       string
}

func Resolve(task *models.Task) (Context, error) {
	if task == nil {
		return Context{}, errors.New("task required")
	}
	if task.WorkDir == "" {
		return Context{}, errors.New("task workdir required")
	}
	info, worktree, err := project.FromDirectory(task.WorkDir)
	if err != nil {
		return Context{}, err
	}
	return Context{
		Task:      task,
		WorkDir:   task.WorkDir,
		Worktree:  worktree,
		ProjectID: resolveTaskContextProjectID(task),
		VCS:       info.VCS,
	}, nil
}

func resolveTaskContextProjectID(task *models.Task) string {
	if task == nil {
		return ""
	}
	if task.ProjectID > 0 {
		return strconv.FormatUint(uint64(task.ProjectID), 10)
	}
	if task.WorkspaceID > 0 {
		return "workspace:" + strconv.FormatUint(uint64(task.WorkspaceID), 10)
	}
	return ""
}
