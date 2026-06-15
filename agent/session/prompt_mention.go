package session

import (
	"fmt"
	"time"

	"matrixops-agent/taskctx"
	"matrixops-agent/tool"
	"matrixops-agent/types"
	"matrixops-agent/util"
	corementions "matrixops.local/core_agent/mentions"
)

func extractFileMentions(text string) ([]string, string) {
	mentions, replaced := corementions.ExtractFileMentions(text)
	urls := make([]string, 0, len(mentions))
	for _, mention := range mentions {
		urls = append(urls, mention.RawURL)
	}
	return urls, replaced
}

func extractReviewMentions(text string) ([]string, string) {
	mentions, replaced := corementions.ExtractReviewMentions(text)
	urls := make([]string, 0, len(mentions))
	for _, mention := range mentions {
		urls = append(urls, mention.RawURL)
	}
	return urls, replaced
}

func extractWorkerMentions(parts []*Part) []string {
	texts := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == nil || part.Type != "text" || part.Text == "" {
			continue
		}
		texts = append(texts, part.Text)
	}
	return corementions.CollectWorkerMentionNames(texts)
}

func extractWorkerMentionsFromText(text string) ([]string, string) {
	mentions, replaced := corementions.ExtractWorkerMentionsFromText(text)
	names := make([]string, 0, len(mentions))
	for _, mention := range mentions {
		names = append(names, mention.Name)
	}
	return names, replaced
}

func extractSkillMentionsFromText(text string) ([]string, string) {
	mentions, replaced := corementions.ExtractSkillMentionsFromText(text)
	names := make([]string, 0, len(mentions))
	for _, mention := range mentions {
		names = append(names, mention.Name)
	}
	return names, replaced
}

func extractCommandMentionsFromText(text string) ([]string, string) {
	mentions, replaced := corementions.ExtractCommandMentionsFromText(text)
	names := make([]string, 0, len(mentions))
	for _, mention := range mentions {
		names = append(names, mention.Name)
	}
	return names, replaced
}

func (r *AgentRunner) buildFileToolMessage(runtimeConfig *RuntimeConfig, fileURL string) (WithParts, error) {
	path := corementions.FileURLToPath(fileURL)
	if path == "" {
		return WithParts{}, fmt.Errorf("invalid file url: %s", fileURL)
	}

	readTool := tool.NewReadTool(nil)
	ctx := tool.Context{SessionID: r.GetSessionID()}
	if r.task != nil {
		if resolved, err := taskctx.Resolve(r.task); err == nil {
			ctx.Directory = resolved.WorkDir
			ctx.Worktree = resolved.Worktree
		}
	} else {
		ctx.Directory = path
		ctx.Worktree = path
	}

	result, err := tool.ExecuteWithOutputTruncation(readTool, ctx, map[string]interface{}{"path": path})
	status := "completed"
	output := ""
	toolErr := ""
	if err != nil {
		status = "error"
		toolErr = err.Error()
	} else {
		output = result.Content
	}

	msgID := util.Ascending("message")
	msg := &MessageInfo{ID: msgID, SessionID: r.GetSessionID(), Role: RoleAssistant, Worker: runtimeConfig.Worker.Name, Occupation: runtimeConfig.Worker.Occupation, ProviderID: runtimeConfig.LLMConfig.Name, ModelID: runtimeConfig.ModelSettings.Name, Time: MessageTime{Created: time.Now().UnixMilli()}, State: "completed"}
	if _, err := r.emitter.UpdateMessage(msg); err != nil {
		return WithParts{}, err
	}

	now := time.Now().UnixMilli()
	part := Part{ID: util.Ascending("part"), MessageID: msgID, SessionID: r.GetSessionID(), Type: types.PartTypeTool, Time: &PartTime{Start: now, End: now, Created: now}, Tool: &ToolPart{Name: "read", CallID: util.Ascending("call"), State: ToolState{Status: status, Input: map[string]interface{}{"path": path}, Output: output, Error: toolErr, FullOutput: result.FullContent, MemoryMetadata: cloneAnyMap(result.MemoryMetadata), Time: PartTime{Start: now, End: now, Created: now}}}}
	if _, err := r.emitter.UpdatePart(&part); err != nil {
		return WithParts{}, err
	}
	return WithParts{Info: msg, Parts: []*Part{&part}}, nil
}

func (r *AgentRunner) buildReviewToolMessage(runtimeConfig *RuntimeConfig, reviewURL string) (WithParts, error) {
	params, ok := corementions.ReviewURLToParams(reviewURL)
	if !ok {
		return WithParts{}, fmt.Errorf("invalid review url: %s", reviewURL)
	}

	diffTool := tool.DiffTool{}
	ctx := tool.Context{SessionID: r.GetSessionID()}
	if r.task != nil {
		if resolved, err := taskctx.Resolve(r.task); err == nil {
			ctx.Directory = resolved.WorkDir
			ctx.Worktree = resolved.Worktree
		}
	}

	result, err := tool.ExecuteWithOutputTruncation(diffTool, ctx, map[string]interface{}{
		"fromType": params.FromType,
		"from":     params.From,
		"toType":   params.ToType,
		"to":       params.To,
	})
	status := "completed"
	output := ""
	toolErr := ""
	if err != nil {
		status = "error"
		toolErr = err.Error()
	} else {
		output = result.Content
	}

	msgID := util.Ascending("message")
	msg := &MessageInfo{ID: msgID, SessionID: r.GetSessionID(), Role: RoleAssistant, Worker: runtimeConfig.Worker.Name, Occupation: runtimeConfig.Worker.Occupation, ProviderID: runtimeConfig.LLMConfig.Name, ModelID: runtimeConfig.ModelSettings.Name, Time: MessageTime{Created: time.Now().UnixMilli()}, State: "completed"}
	if _, err := r.emitter.UpdateMessage(msg); err != nil {
		return WithParts{}, err
	}

	now := time.Now().UnixMilli()
	part := Part{ID: util.Ascending("part"), MessageID: msgID, SessionID: r.GetSessionID(), Type: types.PartTypeTool, Time: &PartTime{Start: now, End: now, Created: now}, Tool: &ToolPart{Name: "diff", CallID: util.Ascending("call"), State: ToolState{Status: status, Input: map[string]interface{}{"fromType": params.FromType, "from": params.From, "toType": params.ToType, "to": params.To}, Output: output, Error: toolErr, Metadata: filterToolDisplayMetadata(cloneAnyMap(result.Metadata)), MemoryMetadata: cloneAnyMap(result.MemoryMetadata), FullOutput: result.FullContent, Time: PartTime{Start: now, End: now, Created: now}}}}
	if _, err := r.emitter.UpdatePart(&part); err != nil {
		return WithParts{}, err
	}
	return WithParts{Info: msg, Parts: []*Part{&part}}, nil
}
