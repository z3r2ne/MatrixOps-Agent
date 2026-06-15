package session

import (
	"fmt"
	"strings"
	"time"

	"matrixops-agent/util"
)

// createUserMessage 创建用户消息
// 处理用户输入，提取文件提及、代码审查提及、worker 提及等
// 返回：用户文本、工具消息列表、错误
func (r *AgentRunner) createUserMessage(runtimeConfig *RuntimeConfig) (string, []WithParts, error) {
	workerName := runtimeConfig.Worker.Name
	if workerName == "" {
		return "", nil, fmt.Errorf("worker not found")
	}

	messageID := util.Ascending("message")

	// 处理输入的 parts，展开并提取提及
	parts := []*Part{}
	fileMentions := []string{}
	reviewMentions := []string{}
	commandMentions := []string{}
	seenMentions := map[string]struct{}{}
	seenReviewMentions := map[string]struct{}{}
	seenCommandMentions := map[string]struct{}{}
	userText := ""

	for _, part := range runtimeConfig.Parts {
		// 展开用户输入的 part（可能包含特殊类型）
		expanded, err := expandUserPart(r.task, part, runtimeConfig.Worker)
		if err != nil {
			return "", nil, fmt.Errorf("expand user part: %w", err)
		}

		for _, item := range expanded {
			partCopy := item
			if partCopy.ID == "" {
				partCopy.ID = util.Ascending("part")
			}
			partCopy.MessageID = messageID
			partCopy.SessionID = r.GetSessionID()

			if partCopy.Type == "text" && partCopy.Time == nil {
				partCopy.Time = &PartTime{
					Start: time.Now().UnixMilli(),
					End:   time.Now().UnixMilli(),
				}
			}

			// 处理文本类型的 part，提取各种提及
			if partCopy.Type == "text" && partCopy.Text != "" {
				// 提取文件提及 file://
				fileURLs, replaced := extractFileMentions(partCopy.Text)
				partCopy.Text = replaced
				for _, url := range fileURLs {
					if _, ok := seenMentions[url]; ok {
						continue
					}
					seenMentions[url] = struct{}{}
					fileMentions = append(fileMentions, url)
				}

				// 提取代码审查提及 review://
				reviewURLs, reviewReplaced := extractReviewMentions(partCopy.Text)
				partCopy.Text = reviewReplaced
				for _, url := range reviewURLs {
					if _, ok := seenReviewMentions[url]; ok {
						continue
					}
					seenReviewMentions[url] = struct{}{}
					reviewMentions = append(reviewMentions, url)
				}

				// 提取 worker 提及 worker://
				_, workerReplaced := extractWorkerMentionsFromText(partCopy.Text)
				partCopy.Text = workerReplaced

				// 提取 skill 提及 skill://
				_, skillReplaced := extractSkillMentionsFromText(partCopy.Text)
				partCopy.Text = skillReplaced

				// 提取 command 提及 command://
				commandNames, commandReplaced := extractCommandMentionsFromText(partCopy.Text)
				partCopy.Text = normalizeCommandMentionText(commandReplaced)
				for _, name := range commandNames {
					normalized := strings.TrimSpace(strings.ToLower(name))
					if normalized == "" {
						continue
					}
					if _, ok := seenCommandMentions[normalized]; ok {
						continue
					}
					seenCommandMentions[normalized] = struct{}{}
					commandMentions = append(commandMentions, normalized)
				}
				userText += partCopy.Text
			}

			parts = append(parts, partCopy)
		}
	}

	messageKind := strings.TrimSpace(runtimeConfig.MessageKind)
	if messageKind == "" {
		messageKind = MessageKindUser
	}
	info := &MessageInfo{
		ID:            messageID,
		SessionID:     r.GetSessionID(),
		Role:          RoleUser,
		MessageKind:   messageKind,
		MessageOrigin: strings.TrimSpace(runtimeConfig.MessageOrigin),
		Worker:        workerName,
		Model:         runtimeConfig.ModelSettings.Name,
		Provider:      runtimeConfig.LLMConfig.Name,
		Time:          MessageTime{Created: time.Now().UnixMilli()},
	}
	if runtimeConfig != nil && runtimeConfig.Assistant != nil && runtimeConfig.Assistant.Time.Created <= info.Time.Created {
		runtimeConfig.Assistant.Time.Created = info.Time.Created + 1
	}

	// 更新消息和 parts
	if _, err := r.emitter.UpdateMessage(info); err != nil {
		return "", nil, fmt.Errorf("update message: %w", err)
	}
	for _, partCopy := range parts {
		if _, err := r.emitter.UpdatePart(partCopy); err != nil {
			return "", nil, fmt.Errorf("update part: %w", err)
		}
	}

	// 构建工具消息（文件提及、代码审查提及）
	results := []WithParts{}

	// 处理文件提及
	for _, fileURL := range fileMentions {
		toolMsg, err := r.buildFileToolMessage(runtimeConfig, fileURL)
		if err != nil {
			return "", nil, fmt.Errorf("build file tool message for %s: %w", fileURL, err)
		}
		results = append(results, toolMsg)
	}

	// 处理代码审查提及
	for _, reviewURL := range reviewMentions {
		toolMsg, err := r.buildReviewToolMessage(runtimeConfig, reviewURL)
		if err != nil {
			return "", nil, fmt.Errorf("build review tool message for %s: %w", reviewURL, err)
		}
		results = append(results, toolMsg)
	}

	userInput := []WithParts{{Info: info, Parts: parts}}
	results = append(userInput, results...)

	enrichUserFilePartURLs(r, parts)
	runtimeConfig.Parts = cloneParts(parts)
	runtimeConfig.SetUserInput(mergePartsToText(runtimeConfig.Parts))
	runtimeConfig.CommandRequestMessageID = messageID
	userText = runtimeConfig.UserInput
	for _, commandName := range commandMentions {
		switch commandName {
		case "compress":
			runtimeConfig.ManualMemoryCompactionRequested = true
			runtimeConfig.ManualMemoryCompactionPrompt = userText
		case "summary":
			runtimeConfig.ManualSessionSummaryRequested = true
			runtimeConfig.ManualSessionSummaryPrompt = userText
		case "new-worktree":
			runtimeConfig.NewWorktreeBranch = userText
		}
	}

	return userText, results, nil
}

func normalizeCommandMentionText(text string) string {
	lines := strings.Split(text, "\n")
	normalized := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.Join(strings.Fields(line), " ")
		if strings.TrimSpace(line) == "" {
			continue
		}
		normalized = append(normalized, line)
	}
	return strings.Join(normalized, "\n")
}

func cloneParts(parts []*Part) []*Part {
	if len(parts) == 0 {
		return nil
	}
	out := make([]*Part, 0, len(parts))
	for _, part := range parts {
		if part == nil {
			continue
		}
		copied := *part
		out = append(out, &copied)
	}
	return out
}
