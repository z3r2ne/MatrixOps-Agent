package session

import (
	"fmt"
	"strings"
	"time"

	"matrixops-agent/llm"
	"matrixops-agent/taskctx"
	"matrixops-agent/types"
	"matrixops-agent/util"
	database "pkgs/db"
	dbmodels "pkgs/db/models"
	"pkgs/db/storage"
)

const (
	defaultSessionSummaryTitle = "项目介绍"
	sessionSummaryToolName     = "summary"
)

func (r *AgentRunner) runManualSessionSummary(runtimeConfig *RuntimeConfig) error {
	if runtimeConfig == nil || runtimeConfig.LLMClient == nil || runtimeConfig.LLMConfig == nil {
		return fmt.Errorf("summary command requires llm runtime")
	}
	sessionID := r.GetSessionID()
	if sessionID == "" || r.db == nil {
		return fmt.Errorf("summary command requires session db context")
	}

	historyMessages, err := r.buildSessionSummaryMessages(runtimeConfig)
	if err != nil {
		return err
	}
	if len(historyMessages) == 0 {
		return fmt.Errorf("当前会话没有可总结的历史内容")
	}

	model, temperature, topP, maxOutputTokens := taskWorkerModelRequest(runtimeConfig)
	instruction := r.buildSessionSummaryInstruction(runtimeConfig)
	userPrompt := buildSessionSummaryUserPrompt(runtimeConfig)
	summaryResult, err := r.summarizeMemoryWithStream(runtimeConfig, llm.ChatRequest{
		Context: runtimeConfig.Ctx,
		Messages: append(historyMessages, &llm.ModelMessage{
			Role:    "user",
			Content: userPrompt,
		}),
		ProviderOptions: runtimeConfig.LLMConfig,
		Model:           model,
		Temperature:     temperature,
		TopP:            topP,
		MaxOutputTokens: maxOutputTokens,
		ExtraOptions: map[string]interface{}{
			"instructions": instruction,
		},
	}, nil)
	if err != nil {
		return err
	}
	summary := strings.TrimSpace(summaryResult.Summary)
	if summary == "" {
		return fmt.Errorf("summary result is empty")
	}

	title, err := r.generateSessionSummaryTitle(runtimeConfig, summary)
	if err != nil {
		return err
	}

	title = normalizeSummaryLibraryTitle(title)
	library, err := r.saveSessionSummaryLibrary(runtimeConfig, title, summary)
	if err != nil {
		return err
	}

	return r.emitSessionSummaryResult(runtimeConfig, library, summary)
}

func (r *AgentRunner) buildSessionSummaryMessages(runtimeConfig *RuntimeConfig) ([]*llm.ModelMessage, error) {
	history, err := storage.GetSessionMessageParts(r.db, r.GetSessionID())
	if err != nil {
		return nil, err
	}
	filtered := make([]*WithParts, 0, len(history))
	for _, message := range history {
		if message == nil || message.Info == nil {
			continue
		}
		if strings.TrimSpace(message.Info.ID) == strings.TrimSpace(runtimeConfig.CommandRequestMessageID) {
			continue
		}
		filtered = append(filtered, (*WithParts)(message))
	}
	return ToModelMessages(filtered), nil
}

func (r *AgentRunner) buildSessionSummaryInstruction(runtimeConfig *RuntimeConfig) string {
	lines := []string{
		"你正在为当前项目生成一份将被长期保存到记忆库、供其他人快速了解项目的“项目介绍”。",
		"这份内容的目标读者是不熟悉该项目的新成员，他们需要先理解这个项目是什么、做什么、当前有哪些稳定事实与约束。",
		"请优先总结项目层面的长期信息：项目定位、核心功能/目标、关键模块、稳定的架构与技术选型、关键约束、依赖、集成关系、重要环境信息，以及与项目本身长期相关的事实。",
		"请特别保留当前项目的环境信息，例如工作目录、worktree、是否 git 仓库、系统平台、shell 和日期；如果对理解项目长期有效，也可概括项目运行/开发环境。",
		"不要把前面对话中的具体任务过程、临时调试步骤、一次性的修 bug 细节、短期待办、零散操作记录直接写进项目介绍，除非它们已经沉淀为项目本身的稳定事实或长期约束。",
		"只基于历史会话中的已知事实与环境信息总结，不要编造未出现的结果。",
		"输出必须是简洁、结构化、可读的 Markdown 正文，不要输出 JSON、不要写“总结如下”之类前言，也不要附加解释。",
	}

	if r.task != nil {
		if ctx, err := taskctx.Resolve(r.task); err == nil {
			workspacePath := r.resolveSessionWorkspacePath(r.db, r.GetSessionID())
			if workspacePath == "" {
				workspacePath = fallbackAIWorkspacePath(ctx.WorkDir, "")
			}
			lines = append(lines, "", buildStandardEnvironmentPrompt(ctx, workspacePath, time.Now()))
		}
	}

	if runtimeConfig != nil && runtimeConfig.Project != nil {
		lines = append(lines, "", "当前项目名称："+strings.TrimSpace(runtimeConfig.Project.Name))
	}

	return strings.Join(lines, "\n")
}

func buildSessionSummaryUserPrompt(runtimeConfig *RuntimeConfig) string {
	lines := []string{
		"请基于以上历史会话，生成一份适合长期保存到记忆库、面向其他人的项目介绍。",
	}
	if runtimeConfig != nil && strings.TrimSpace(runtimeConfig.ManualSessionSummaryPrompt) != "" {
		lines = append(lines, "用户附加要求："+strings.TrimSpace(runtimeConfig.ManualSessionSummaryPrompt))
	}
	return strings.Join(lines, "\n")
}

func buildSessionSummaryTitleInstruction() string {
	return "请为用户提供的项目介绍生成一个简短、事实性、适合作为记忆库标题的中文标题。标题应更像项目资料名或项目介绍标题，而不是任务总结标题。优先概括项目名称、系统领域或核心能力，不要突出某一次具体任务、修复动作或临时操作。只输出标题本身，不要加引号、序号、Markdown 或解释。"
}

func (r *AgentRunner) generateSessionSummaryTitle(runtimeConfig *RuntimeConfig, summary string) (string, error) {
	model, temperature, topP, _ := taskWorkerModelRequest(runtimeConfig)
	titleResult, err := r.summarizeMemoryWithStream(runtimeConfig, llm.ChatRequest{
		Context: runtimeConfig.Ctx,
		Messages: []*llm.ModelMessage{
			{Role: "user", Content: summary},
		},
		ProviderOptions: runtimeConfig.LLMConfig,
		Model:           model,
		Temperature:     temperature,
		TopP:            topP,
		MaxOutputTokens: 128,
		ExtraOptions: map[string]interface{}{
			"instructions": buildSessionSummaryTitleInstruction(),
		},
	}, nil)
	if err != nil {
		return "", err
	}
	title := strings.TrimSpace(titleResult.Summary)
	if title == "" {
		return "", fmt.Errorf("summary title is empty")
	}
	return title, nil
}

func normalizeSummaryLibraryTitle(title string) string {
	title = strings.TrimSpace(title)
	title = strings.Trim(title, "#*`\"' \n\t")
	if title == "" {
		title = defaultSessionSummaryTitle
	}
	title = strings.Join(strings.Fields(title), " ")
	if len([]rune(title)) > 60 {
		title = string([]rune(title)[:60])
	}
	return title
}

func (r *AgentRunner) saveSessionSummaryLibrary(runtimeConfig *RuntimeConfig, title string, summary string) (*dbmodels.MemoryLibrary, error) {
	name := title
	if strings.TrimSpace(name) == "" {
		name = defaultSessionSummaryTitle
	}
	library := &dbmodels.MemoryLibrary{
		Name:    name,
		Content: summary,
	}
	if err := database.CreateMemoryLibrary(r.db, library); err != nil {
		library.Name = fmt.Sprintf("%s %s", name, time.Now().Format("20060102-150405"))
		if err := database.CreateMemoryLibrary(r.db, library); err != nil {
			return nil, err
		}
	}

	project := runtimeConfig.Project
	if project == nil && r.task != nil && r.task.ProjectID > 0 {
		if loaded, err := database.GetProjectByID(r.db, r.task.ProjectID); err == nil {
			project = loaded
		}
	}
	if project != nil {
		nextIDs := append([]uint{}, project.MemoryLibraryIDs.Slice()...)
		found := false
		for _, id := range nextIDs {
			if id == library.ID {
				found = true
				break
			}
		}
		if !found {
			nextIDs = append(nextIDs, library.ID)
			project.MemoryLibraryIDs = dbmodels.UintSlice(nextIDs)
			if err := database.UpdateProject(r.db, project); err != nil {
				return nil, err
			}
		}
	}

	return library, nil
}

func (r *AgentRunner) emitSessionSummaryResult(runtimeConfig *RuntimeConfig, library *dbmodels.MemoryLibrary, summary string) error {
	assistant := runtimeConfig.Assistant
	if assistant == nil {
		assistant = &MessageInfo{
			ID:        util.Ascending("message"),
			SessionID: r.GetSessionID(),
			Role:      RoleAssistant,
			Time:      MessageTime{Created: time.Now().UnixMilli()},
			State:     "completed",
		}
		runtimeConfig.Assistant = assistant
	}

	content := fmt.Sprintf("已将当前会话总结保存到记忆库《%s》。\n\n%s", library.Name, summary)
	part := &Part{
		ID:        util.Ascending("part"),
		MessageID: assistant.ID,
		SessionID: r.GetSessionID(),
		Type:      types.PartTypeText,
		Text:      content,
		Time: &PartTime{
			Start:   time.Now().UnixMilli(),
			End:     time.Now().UnixMilli(),
			Created: time.Now().UnixMilli(),
		},
	}
	if _, err := r.emitter.UpdatePart(part); err != nil {
		return err
	}
	return nil
}
