package session

import (
	"fmt"
	"io"
	"regexp"
	"strings"

	"matrixops-agent/types"
	"pkgs/db/storage"

	coreagent "matrixops.local/core_agent"
)

var thinkRegex = regexp.MustCompile(`(?s)<think>.*?</think>`)

const autoTitleMaxRunes = 100

// ensureTitle 确保会话有标题
func (a *AgentRunner) ensureTitle(runtimeConfig *RuntimeConfig) error {
	if a == nil || runtimeConfig == nil {
		return nil
	}
	if a.db == nil || a.session == nil || a.emitter == nil {
		return nil
	}
	if !shouldAutoGenerateSessionTitle(a.session) {
		return nil
	}

	userInput, err := a.loadTitleSourceUserInput()
	if err != nil {
		return fmt.Errorf("ensure title: load title source: %w", err)
	}
	if userInput == "" {
		return nil
	}

	title, err := a.generateSessionTitle(runtimeConfig, userInput)
	if err != nil {
		return fmt.Errorf("ensure title: generate title: %w", err)
	}
	if title == "" {
		return nil
	}

	if err := storage.UpdateSessionTitle(a.db, a.GetSessionID(), title); err != nil {
		return fmt.Errorf("ensure title: persist title: %w", err)
	}
	a.session.Title = title
	if err := a.emitter.UpdateSessionTitle(a.GetSessionID(), title); err != nil {
		return fmt.Errorf("ensure title: emit title update: %w", err)
	}
	return nil
}

func shouldAutoGenerateSessionTitle(sessionInfo *Info) bool {
	if sessionInfo == nil {
		return false
	}
	if strings.TrimSpace(sessionInfo.ParentID) != "" {
		return false
	}
	current := strings.TrimSpace(sessionInfo.Title)
	if current == "" {
		return true
	}
	return strings.HasPrefix(current, parentTitlePrefix) || strings.HasPrefix(current, childTitlePrefix)
}

func (a *AgentRunner) loadTitleSourceUserInput() (string, error) {
	history, err := storage.GetSessionMessageParts(a.db, a.GetSessionID())
	if err != nil {
		return "", err
	}

	firstRealIdx := -1
	realUserCount := 0
	userInput := ""
	for idx, msg := range history {
		if msg == nil || msg.Info == nil || msg.Info.Role != RoleUser {
			continue
		}
		if msg.Info.MessageKind == MessageKindSystem {
			continue
		}
		text, ok := hasRealUserParts(msg.Parts)
		if !ok {
			continue
		}
		realUserCount++
		if firstRealIdx == -1 {
			firstRealIdx = idx
			userInput = text
		}
	}
	if firstRealIdx == -1 || realUserCount != 1 {
		return "", nil
	}
	return userInput, nil
}

func hasRealUserParts(parts []*Part) (string, bool) {
	texts := make([]string, 0, len(parts))
	nonTextCount := 0
	for _, part := range parts {
		if part == nil || part.Synthetic || part.Ignored {
			continue
		}
		if part.Type == types.PartTypeText {
			if trimmed := strings.TrimSpace(part.Text); trimmed != "" {
				texts = append(texts, trimmed)
			}
			continue
		}
		if trimmed := strings.TrimSpace(part.Filename); trimmed != "" {
			texts = append(texts, "附件: "+trimmed)
		} else if trimmed := strings.TrimSpace(part.Mime); trimmed != "" {
			texts = append(texts, "附件: "+trimmed)
		} else {
			nonTextCount++
		}
	}
	if len(texts) > 0 {
		return strings.Join(texts, "\n"), true
	}
	if nonTextCount > 0 {
		return "用户上传了附件，请基于本次输入生成会话标题", true
	}
	return "", false
}

func (a *AgentRunner) generateSessionTitle(runtimeConfig *RuntimeConfig, userInput string) (string, error) {
	if runtimeConfig == nil || runtimeConfig.LLMConfig == nil || !runtimeConfig.LLMConfig.NativeOpenAIToolCalls {
		return "", nil
	}
	model := strings.TrimSpace(runtimeConfig.Model)
	if runtimeConfig.Worker != nil {
		if workerModel := strings.TrimSpace(runtimeConfig.Worker.Model); workerModel != "" {
			model = workerModel
		}
	}
	if model == "" {
		return "", nil
	}

	httpClient := a.ensureLLMHTTPClient(runtimeConfig)
	placement := coreagent.NormalizeSystemPromptPlacement("")
	if runtimeConfig.ModelSettings != nil {
		placement = coreagent.NormalizeSystemPromptPlacement(runtimeConfig.ModelSettings.SystemPromptPlacement)
	}
	promptPayload := coreagent.PrepareFullPromptPayload(buildSessionTitleInstruction(), placement, userInput)
	streamIn := coreagent.StreamInput{
		Context:         runtimeConfig.Ctx,
		Model:           model,
		Prompt:          fmt.Sprintf("这是用户输入内容：<userInput>%s</userInput>", userInput),
		SystemPrompt:    promptPayload.SystemPrompt,
		Instruction:     promptPayload.Instruction,
		MaxOutputTokens: a.effectiveMaxOutputTokens(runtimeConfig),
		ProviderOptions: runtimeConfig.LLMConfig,
		HTTPClient:      httpClient,
		ThinkingType:    runtimeConfig.ThinkingType,
		EnableThinking:  runtimeConfig.EnableThinking,
		BudgetTokens:    runtimeConfig.BudgetTokens,
	}
	out, err := coreagent.StreamV2OpenAINativeOnce(streamIn)
	if err != nil {
		return "", err
	}
	title, err := collectStreamV2AnswerText(out)
	if err != nil {
		return "", err
	}
	return normalizeSessionTitle(title), nil
}

func collectStreamV2AnswerText(out *coreagent.StreamOutput) (string, error) {
	if out == nil {
		return "", nil
	}
	var b strings.Builder
	if out.Wait != nil {
		if err := out.Wait(); err != nil {
			return "", err
		}
	}
	if out.NativeAssistantTextFinishesTurn {
		if out.RawTextReader != nil {
			buf, err := io.ReadAll(out.RawTextReader)
			if err != nil {
				return "", err
			}
			b.Write(buf)
		} else if out.ContentReader != nil {
			buf, err := io.ReadAll(out.ContentReader)
			if err != nil {
				return "", err
			}
			b.Write(buf)
		}
	}
	text := strings.TrimSpace(b.String())
	if text != "" {
		return text, nil
	}
	// 部分 provider 在无 tools 时仅填充 RawTextReader（或 ContentReader 已被提前读空）。
	if out.RawTextReader != nil {
		buf, err := io.ReadAll(out.RawTextReader)
		if err != nil {
			return "", err
		}
		if trimmed := strings.TrimSpace(string(buf)); trimmed != "" {
			return trimmed, nil
		}
	}
	if out.ContentReader != nil {
		buf, err := io.ReadAll(out.ContentReader)
		if err != nil {
			return "", err
		}
		if trimmed := strings.TrimSpace(string(buf)); trimmed != "" {
			return trimmed, nil
		}
	}
	return "", nil
}

func buildSessionTitleInstruction() string {
	return "请基于用户本轮首次输入，生成一个简短、明确、适合作为会话/任务标题的中文标题。标题应概括用户想做的核心工作，不要写成完整句子，不要带引号、序号、Markdown、解释、前后缀或表情。尽量控制在 8 到 24 个汉字内。只输出标题本身。"
}

func normalizeSessionTitle(title string) string {
	title = strings.TrimSpace(thinkRegex.ReplaceAllString(title, ""))
	title = strings.TrimSpace(strings.Split(title, "\n")[0])
	title = strings.Trim(title, "#*`\"'[]（）()<>「」『』，。；：、 \t\n")
	title = strings.Join(strings.Fields(title), " ")
	if title == "" {
		return ""
	}
	runes := []rune(title)
	if len(runes) > autoTitleMaxRunes {
		return string(runes[:autoTitleMaxRunes])
	}
	return title
}
