package session

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"matrixops-agent/taskctx"
	"matrixops-agent/tool"
	"pkgs/db/models"
)

func expandUserPart(task *models.Task, part *Part, worker *models.Worker) ([]*Part, error) {
	switch part.Type {
	case "file":
		return expandFilePart(task, part)
	case "agent":
		return expandAgentPart(part, worker)
	default:
		return []*Part{part}, nil
	}
}

func expandAgentPart(part *Part, _ *models.Worker) ([]*Part, error) {
	hint := ""
	return []*Part{
		part,
		{
			Type:      "text",
			Synthetic: true,
			Text:      " Use the above message and context to generate a prompt and call the task tool with subagent: " + part.AgentName + hint,
		},
	}, nil
}

func expandFilePart(task *models.Task, part *Part) ([]*Part, error) {
	if part != nil && strings.Contains(strings.TrimSpace(part.Path), UserInputSubdir) {
		return []*Part{part}, nil
	}
	if part != nil && strings.TrimSpace(part.URL) != "" {
		if parsed, err := url.Parse(part.URL); err == nil && parsed.Scheme == "file" {
			if strings.Contains(parsed.Path, UserInputSubdir) {
				return []*Part{part}, nil
			}
		}
	}
	if part.URL == "" {
		return []*Part{part}, nil
	}
	parsed, err := url.Parse(part.URL)
	if err != nil {
		return []*Part{part}, nil
	}
	switch parsed.Scheme {
	case "data":
		return expandDataURLPart(part)
	case "file":
		return expandFileURLPart(task, part, parsed)
	default:
		return []*Part{part}, nil
	}
}

func expandDataURLPart(part *Part) ([]*Part, error) {
	if part.Mime != "text/plain" {
		return []*Part{part}, nil
	}
	segments := strings.SplitN(part.URL, ",", 2)
	if len(segments) != 2 {
		return []*Part{part}, nil
	}
	data, err := base64.StdEncoding.DecodeString(segments[1])
	if err != nil {
		return []*Part{part}, nil
	}
	return []*Part{
		{
			Type:      "text",
			Synthetic: true,
			Text:      "Called the Read tool with the following input: " + `{"filePath":"` + part.Filename + `"}`,
		},
		{
			Type:      "text",
			Synthetic: true,
			Text:      string(data),
		},
		part,
	}, nil
}

func expandFileURLPart(task *models.Task, part *Part, parsed *url.URL) ([]*Part, error) {
	path := parsed.Path
	if path == "" {
		return []*Part{part}, nil
	}
	path, _ = url.PathUnescape(path)
	if !filepath.IsAbs(path) {
		path = filepath.Join("/", path)
	}
	info, err := os.Stat(path)
	if err != nil {
		return []*Part{
			{
				Type:      "text",
				Synthetic: true,
				Text:      "Read tool failed to read " + path + " with the following error: " + err.Error(),
			},
			part,
		}, nil
	}
	if info.IsDir() {
		part.Mime = "application/x-directory"
	}
	if part.Mime == "application/x-directory" {
		return listDirectoryParts(task, part, path)
	}
	if part.Mime == "text/plain" {
		return readFileParts(task, part, parsed, path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return []*Part{part}, nil
	}
	encoded := base64.StdEncoding.EncodeToString(data)
	filePart := part
	filePart.URL = "data:" + part.Mime + ";base64," + encoded
	return []*Part{
		{
			Type:      "text",
			Synthetic: true,
			Text:      "Called the Read tool with the following input: {\"filePath\":\"" + path + "\"}",
		},
		filePart,
	}, nil
}

func readFileParts(task *models.Task, part *Part, parsed *url.URL, path string) ([]*Part, error) {
	offset, limit := parseRange(parsed)
	args := map[string]interface{}{"path": path}
	if offset != 0 {
		args["offset"] = offset
	}
	if limit != 0 {
		args["limit"] = limit
	}
	readTool := tool.NewReadTool(nil)
	ctx := tool.Context{SessionID: ""}
	if task != nil {
		if resolved, err := taskctx.Resolve(task); err == nil {
			ctx.Directory = resolved.WorkDir
			ctx.Worktree = resolved.Worktree
		}
	} else {
		ctx.Directory = path
		ctx.Worktree = path
	}
	result, err := tool.ExecuteWithOutputTruncation(readTool, ctx, args)
	if err != nil {
		return []*Part{
			{
				Type:      "text",
				Synthetic: true,
				Text:      "Read tool failed to read " + path + " with the following error: " + err.Error(),
			},
			part,
		}, nil
	}
	return []*Part{
		{
			Type:      "text",
			Synthetic: true,
			Text:      "Called the Read tool with the following input: " + fmt.Sprintf("%v", args),
		},
		{
			Type:      "text",
			Synthetic: true,
			Text:      result.Content,
		},
		part,
	}, nil
}

func listDirectoryParts(task *models.Task, part *Part, path string) ([]*Part, error) {
	listTool := tool.ListTool{}
	ctx := tool.Context{SessionID: ""}
	if task != nil {
		if resolved, err := taskctx.Resolve(task); err == nil {
			ctx.Directory = resolved.WorkDir
			ctx.Worktree = resolved.Worktree
		}
	} else {
		ctx.Directory = path
		ctx.Worktree = path
	}
	result, err := tool.ExecuteWithOutputTruncation(listTool, ctx, map[string]interface{}{"path": path})
	if err != nil {
		return []*Part{
			{
				Type:      "text",
				Synthetic: true,
				Text:      "List tool failed to read " + path + " with the following error: " + err.Error(),
			},
			part,
		}, nil
	}
	return []*Part{
		{
			Type:      "text",
			Synthetic: true,
			Text:      "Called the list tool with the following input: " + `{"path":"` + path + `"}`,
		},
		{
			Type:      "text",
			Synthetic: true,
			Text:      result.Content,
		},
		part,
	}, nil
}

func parseRange(parsed *url.URL) (int, int) {
	start := parsed.Query().Get("start")
	end := parsed.Query().Get("end")
	if start == "" {
		return 0, 0
	}
	startInt, err := strconv.Atoi(start)
	if err != nil {
		return 0, 0
	}
	offset := startInt - 1
	if offset < 0 {
		offset = 0
	}
	limit := 0
	if end != "" {
		if endInt, err := strconv.Atoi(end); err == nil {
			limit = endInt - offset
			if limit < 0 {
				limit = 0
			}
		}
	}
	return offset, limit
}

// intentionally left without extra validation; caller owns dependency checks.
