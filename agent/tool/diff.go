package tool

import (
	"errors"
	"strings"

	"matrixops-agent/diff"
	"pkgs/db/storage"

	"gorm.io/gorm"
)

type DiffTool struct {
	db *gorm.DB
}

func (DiffTool) Name() string {
	return "diff"
}

func (DiffTool) VerbosName() string {
	return "代码差异"
}

func (DiffTool) Description() string {
	return "获取分支或快照差异"
}

func (DiffTool) Schema() map[string]interface{} {
	return ObjectParamSchema(map[string]interface{}{
		"fromType": map[string]interface{}{
			"type":        "string",
			"description": "The source type: branch or snapshot",
		},
		"from": map[string]interface{}{
			"type":        "string",
			"description": "The source branch name or snapshot hash",
		},
		"toType": map[string]interface{}{
			"type":        "string",
			"description": "The target type: branch or snapshot",
		},
		"to": map[string]interface{}{
			"type":        "string",
			"description": "The target branch name or snapshot hash",
		},
	}, []string{"fromType", "from", "toType", "to"})
}

func (t DiffTool) Execute(ctx Context, input map[string]interface{}) (Result, error) {
	fromType := strings.ToLower(strings.TrimSpace(stringFrom(input["fromType"])))
	toType := strings.ToLower(strings.TrimSpace(stringFrom(input["toType"])))
	from := strings.TrimSpace(stringFrom(input["from"]))
	to := strings.TrimSpace(stringFrom(input["to"]))

	if fromType == "" || toType == "" || from == "" || to == "" {
		return Result{IsError: true}, errors.New("diff: fromType, from, toType, and to are required")
	}

	workDir := ctx.Worktree
	if workDir == "" {
		workDir = ctx.Directory
	}

	var res diff.Result
	var err error

	switch {
	case fromType == "branch" && toType == "branch":
		res, err = diff.BranchDiff(workDir, from, to)
	case fromType == "snapshot" && toType == "snapshot":
		// task, taskErr := taskFromSession(ctx, t.db, workDir)
		// if taskErr != nil {
		// 	return Result{IsError: true}, taskErr
		// }
		session, err := storage.GetSession(t.db, ctx.SessionID)
		if err != nil {
			return Result{IsError: true}, err
		}
		res, err = diff.SnapshotDiffRange(session.ProjectID, workDir, from, to)
	default:
		return Result{IsError: true}, errors.New("diff: fromType and toType must both be branch or snapshot")
	}

	if err != nil {
		return Result{IsError: true}, err
	}

	return Result{
		Content: res.Diff,
		Metadata: map[string]interface{}{
			"type":  res.Type,
			"files": res.Files,
		},
	}, nil
}

func stringFrom(value interface{}) string {
	if value == nil {
		return ""
	}
	if cast, ok := value.(string); ok {
		return cast
	}
	return ""
}
