package task_runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	coregit "matrixops.local/core_agent/git"
	database "pkgs/db"
	"pkgs/db/models"

	"gorm.io/gorm"

	agentconfig "matrixops-agent/config"
	agentpermission "matrixops-agent/permission"
	agentproject "matrixops-agent/project"
	agentprovider "matrixops-agent/provider"
	agentsession "matrixops-agent/session"
)

var matrixopsAgentMu sync.Mutex

func (r *TaskRuntime) resolveChildTaskBaseBranch(explicitBaseBranch string, candidateBranches ...string) string {
	candidates := make([]string, 0, len(candidateBranches)+3)
	candidates = append(candidates, explicitBaseBranch)
	if r != nil && r.config != nil {
		candidates = append(candidates, r.config.BaseBranch, r.config.Branch)
	}
	candidates = append(candidates, candidateBranches...)

	for _, candidate := range candidates {
		if trimmed := strings.TrimSpace(candidate); trimmed != "" {
			return trimmed
		}
	}

	seenDirs := map[string]struct{}{}
	tryResolveDir := func(dir string) string {
		dir = strings.TrimSpace(dir)
		if dir == "" {
			return ""
		}
		if _, ok := seenDirs[dir]; ok {
			return ""
		}
		seenDirs[dir] = struct{}{}
		branch, err := coregit.CurrentBranch(dir)
		if err != nil {
			return ""
		}
		return strings.TrimSpace(branch)
	}

	if r != nil {
		if branch := tryResolveDir(r.workDir); branch != "" {
			return branch
		}
		if r.config != nil {
			if branch := tryResolveDir(r.config.WorkDir); branch != "" {
				return branch
			}
		}
	}

	if r != nil && r.db != nil && r.config != nil && strings.TrimSpace(r.config.ProjectID) != "" {
		project, err := database.GetProjectByStringID(r.db, r.config.ProjectID)
		if err == nil && project != nil {
			if branch := tryResolveDir(project.Path); branch != "" {
				return branch
			}
		}
	}

	return ""
}

type matrixopsAgentConfig struct {
	Key     string `json:"key"`
	Model   string `json:"model"`
	BaseURL string `json:"baseurl"`
	Proxy   string `json:"proxy"`
}

type matrixopsAgentResult struct {
	sessionID string
	messages  []agentsession.WithParts
	err       error
}

// func (r *TaskRuntime) startMatrixopsAgent(opts ...TaskRuntimeConfigOption) error {
// 	runConfig := NewTaskRuntimeConfig(opts...)
// 	if runConfig.Content == "" {
// 		return errors.New("缺少任务内容")
// 	}
// 	content := runConfig.Content
// 	agentName, modelRef := matrixopsAgentOptions(r.cfg)
// 	cmdName, cmdArgs := matrixopsAgentCommandSpec(agentName, modelRef)

// 	cmdLogger := GetCommandLogger()
// 	promptPreview := content
// 	if len(promptPreview) > 50 {
// 		promptPreview = promptPreview[:50] + "..."
// 	}
// 	sourceName := fmt.Sprintf("Task #%d: %s", r.taskID, promptPreview)

// 	cmdLogger.LogCommand(models.CommandLogCreate{
// 		Source:     "task_runner_matrixops_agent",
// 		SourceID:   &r.taskID,
// 		SourceName: sourceName,
// 		Command:    cmdName,
// 		Args:       cmdArgs,
// 		WorkDir:    r.workDir,
// 		StdinData:  content,
// 	})

// 	// r.agentResultChan = make(chan matrixopsAgentResult, 1)
// 	err := r.runMatrixopsAgentPrompt(r.workDir, content, r.config.TaskInfo.SessionID, agentName, modelRef, runConfig)
// 	if err != nil {
// 		return err
// 	}
// 	return nil
// }

func (r *TaskRuntime) runMatrixopsAgentPrompt(runCtx context.Context, sessionID string, taskID uint, content string, Worker string, opts ...TaskRuntimeConfigOption) error {
	chatConfig := NewTaskRuntimeConfig(opts...)
	if strings.TrimSpace(content) == "" && len(chatConfig.InputParts) == 0 {
		return errors.New("缺少任务内容")
	}

	cmdName, cmdArgs := matrixopsAgentCommandSpec(Worker, "", "")

	cmdLogger := GetCommandLogger(r.db)
	promptPreview := strings.TrimSpace(content)
	if promptPreview == "" && len(chatConfig.InputParts) > 0 {
		promptPreview = fmt.Sprintf("[附件×%d]", len(chatConfig.InputParts))
	}
	if len(promptPreview) > 50 {
		promptPreview = promptPreview[:50] + "..."
	}
	sourceName := fmt.Sprintf("Task #%d: %s", taskID, promptPreview)

	cmdLogger.LogCommand(models.CommandLogCreate{
		Source:     "task_runner_matrixops_agent",
		SourceID:   &taskID,
		SourceName: sourceName,
		Command:    cmdName,
		Args:       cmdArgs,
		WorkDir:    r.workDir,
		StdinData:  strings.TrimSpace(content),
		Fields: models.BuildCommandLogFields(
			models.NewCommandLogField("prompt", "任务输入", strings.TrimSpace(content), "default"),
			models.NewCommandLogField("worker", "执行 Worker", Worker, "default"),
		),
	})

	// r.emitter.EmitIsWorking()
	// defer r.emitter.EmitIsNotWorking()

	// var agentCfg *matrixopsAgentConfig
	// if r.LLM != nil {
	// 	agentCfg = &matrixopsAgentConfig{
	// 		Key:     r.LLM.APIKey,
	// 		Model:   r.LLM.Model,
	// 		BaseURL: r.LLM.BaseURL,
	// 		Proxy:   r.LLM.Proxy,
	// 	}
	// } else {
	// 	if globalCfg, ok, err := loadMatrixopsAgentConfig(); err == nil && ok {
	// 		agentCfg = &globalCfg
	// 	}
	// }

	err := agentproject.Provide(r.workDir, nil, func() error {
		// cfg, err := r.buildMatrixopsAgentConfig(r.LLM)
		// if err != nil {
		// 	return err
		// }

		runnerOptions := []agentsession.AgentRunnerOption{
			agentsession.WithCtx(runCtx),
			// agentsession.WithTools(agenttool.NewDefaultRegistry(r.db)),
			agentsession.WithPerms(agentpermission.NewManager(nil)),
			agentsession.WithOnEmitterCreated(r.onEmitterCreated),
			agentsession.WithOnEmitterCreated(r.config.OnEmitterCreateds...),
			agentsession.WithDeliverUserMessage(r.wrapDeliverUserMessage()),
			agentsession.WithDB(r.db),
			agentsession.WithProjectID(r.config.ProjectID),
			agentsession.WithDirectory(r.workDir),
			agentsession.WithTaskID(r.config.TaskID),
			agentsession.WithQueueBroadcaster(r.wsHub.BroadcastTaskQueue),
			agentsession.WithQueueAutoRun(func() {
				_ = TryAutoRunTaskQueue(r.taskID, WithDB(r.db), WithWSHub(r.wsHub))
			}),
			// agentsession.WithTask(r.toTaskModel()),
			agentsession.WithMergeMessage(r.config.MergeMessage),
			agentsession.WithSessionID(r.config.SessionID),
		}

		if r.config.LLMClient != nil {
			runnerOptions = append(runnerOptions, agentsession.WithLLM(r.config.LLMClient))
		} else {
			runnerOptions = append(runnerOptions, agentsession.WithLLM(agentprovider.NewGenericClient()))
		}

		chatOptions := []agentsession.AgentRunnerOption{
			agentsession.WithCtx(runCtx),
			agentsession.WithSessionID(sessionID),
			agentsession.WithWorker(Worker),
			agentsession.WithSkipCreateUserMessage(r.config.SkipCreateUserMessage),
			// agentsession.WithTask(r.toTaskModel()),
			agentsession.WithNewTaskHandler(r.newTaskHandler(
				WithDB(r.db),
				WithWSHub(r.wsHub),
				WithLLMClient(r.config.LLMClient),
				WithCtx(runCtx),
				WithWorkspaceID(r.config.WorkspaceID),
				WithProjectID(r.config.ProjectID),
				WithBaseBranch(r.config.BaseBranch),
			)),
			agentsession.WithMergeMessage(chatConfig.MergeMessage),
			agentsession.WithSkipCreateUserMessage(chatConfig.SkipCreateUserMessage),
			agentsession.WithOnEmitterCreated(chatConfig.OnEmitterCreateds...),
			agentsession.WithSessionWindow(chatConfig.SessionWindow),
		}
		if trimmed := strings.TrimSpace(content); trimmed != "" {
			chatOptions = append(chatOptions, agentsession.WithInputText(trimmed))
		}
		if len(chatConfig.InputParts) > 0 {
			chatOptions = append(chatOptions, agentsession.WithInputParts(chatConfig.InputParts))
		}
		if kind := strings.TrimSpace(chatConfig.MessageKind); kind != "" {
			chatOptions = append(chatOptions, agentsession.WithMessageKind(kind))
		}
		if origin := strings.TrimSpace(chatConfig.MessageOrigin); origin != "" {
			chatOptions = append(chatOptions, agentsession.WithMessageOrigin(origin))
		}
		opts := append(runnerOptions, chatOptions...)
		_, err := agentsession.Prompt(opts...)
		if err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		return err
	}

	return nil
}

func (r *TaskRuntime) newTaskHandler(defaultOpts ...TaskRuntimeConfigOption) func(args map[string]interface{}) (map[string]interface{}, error) {
	return func(args map[string]interface{}) (map[string]interface{}, error) {
		workerName := toString(args["workerName"])
		fromWorker := toString(args["fromWorker"])
		inputText := toString(args["inputText"])
		taskName := toString(args["taskName"])
		newBranch := toString(args["newBranch"])
		baseBranch := toString(args["baseBranch"])
		// reminder := toString(args["reminder"])
		taskID, ok := toUint(args["taskId"])
		if !ok {
			taskID = 0
		}
		parentTaskID, ok := toUint(args["parentTaskId"])
		var parentTaskIDPtr *uint
		if ok && parentTaskID > 0 {
			parentTaskIDPtr = &parentTaskID
		}
		mergeMessage := toBool(args["mergeMessage"])
		skipCreateUserMessage := toBool(args["skipCreateUserMessage"])
		var sessionWindow *agentsession.SessionWindow
		if args["sessionWindow"] != nil {
			sessionWindow = args["sessionWindow"].(*agentsession.SessionWindow)
		}
		var onSubtaskEmitterCreated func(emitter *agentsession.Emitter) error
		if value, ok := args["onSubtaskEmitterCreated"].(func(emitter *agentsession.Emitter) error); ok {
			onSubtaskEmitterCreated = value
		}
		parentToolCtx, _ := args["parentContext"].(context.Context)
		subtaskRunCtx, cleanupSubtaskRunCtx := buildSubtaskRunContext(r.ctx, parentToolCtx)
		setVar, _ := args["setVar"].(func(key string, value any))
		if setVar == nil {
			setVar = func(key string, value any) {}
		}

		// if inputText == "" {
		// 	return nil, errors.New("缺少 worker 或任务内容")
		// }

		content := inputText
		// if strings.TrimSpace(reminder) != "" {
		// 	content = content + "\n\n提醒事项:\n" + reminder
		// }

		if workerName == "" {
			cleanupSubtaskRunCtx()
			return nil, errors.New("缺少 worker")
		}

		inheritedWorkDir := ""
		inheritedBranch := ""
		if parentTaskIDPtr != nil {
			parentTask, err := database.GetTaskByID(r.db, *parentTaskIDPtr)
			if err != nil {
				cleanupSubtaskRunCtx()
				return nil, fmt.Errorf("获取父任务失败: %w", err)
			}
			if parentTask != nil {
				if strings.TrimSpace(baseBranch) == "" {
					baseBranch = strings.TrimSpace(parentTask.BaseBranch)
					if baseBranch == "" {
						baseBranch = strings.TrimSpace(parentTask.Branch)
					}
				}
				parentBaseBranch := strings.TrimSpace(parentTask.BaseBranch)
				parentBranch := strings.TrimSpace(parentTask.Branch)
				if strings.TrimSpace(newBranch) == "" && (strings.TrimSpace(baseBranch) == "" || strings.TrimSpace(baseBranch) == parentBaseBranch || strings.TrimSpace(baseBranch) == parentBranch) {
					inheritedWorkDir = strings.TrimSpace(parentTask.WorkDir)
					inheritedBranch = parentBranch
				}
			}
		}
		baseBranch = r.resolveChildTaskBaseBranch(baseBranch, inheritedBranch)

		opts := []TaskRuntimeConfigOption{
			WithFromWorker(fromWorker),
			WithToWorker(workerName),
			WithTaskName(taskName),
			WithContent(content),
			WithSkipCreateUserMessage(skipCreateUserMessage),
			WithMergeMessage(mergeMessage),
			WithSessionWindow(sessionWindow),
			WithOnEmitterCreated(func(emitter *agentsession.Emitter) error {
				emitter.On(agentsession.EventPluginVarSet, func(args ...interface{}) {
					event := args[0].(agentsession.PluginVarSetEvent)
					setVar(event.Key, event.Value)
				})
				return nil
			}),
		}
		if onSubtaskEmitterCreated != nil {
			opts = append(opts, WithOnEmitterCreated(onSubtaskEmitterCreated))
		}
		if parentTaskIDPtr != nil {
			opts = append(opts, WithParentTaskID(parentTaskIDPtr))
		}
		if strings.TrimSpace(inheritedWorkDir) != "" {
			opts = append(opts, WithWorkDir(strings.TrimSpace(inheritedWorkDir)))
		}
		if strings.TrimSpace(inheritedBranch) != "" {
			opts = append(opts, WithBranch(strings.TrimSpace(inheritedBranch)))
		}
		if strings.TrimSpace(newBranch) != "" {
			opts = append(opts, WithNewBranch(strings.TrimSpace(newBranch)))
		}
		if strings.TrimSpace(baseBranch) != "" {
			opts = append(opts, WithBaseBranch(strings.TrimSpace(baseBranch)))
		}
		opts = append(opts, WithCtx(subtaskRunCtx))
		opts = append(defaultOpts, opts...)
		if taskID == 0 {
			task, err := CreateAndRunTask(opts...)
			if err != nil {
				cleanupSubtaskRunCtx()
				return nil, err
			}

			return map[string]interface{}{
				"taskId":    task.ID,
				"sessionId": task.SessionID,
				"waitTask": func() error {
					return WaitTask(task.ID)
				},
				"waitResult": func() (map[string]interface{}, error) {
					defer cleanupSubtaskRunCtx()
					return r.waitSubtaskResult(subtaskRunCtx, task.ID)
				},
			}, nil
		} else {
			err := RunTask(taskID, opts...)
			if err != nil {
				cleanupSubtaskRunCtx()
				return nil, err
			}

			return map[string]interface{}{
				"taskId": taskID,
				"waitTask": func() error {
					return WaitTask(taskID)
				},
				"waitResult": func() (map[string]interface{}, error) {
					defer cleanupSubtaskRunCtx()
					return r.waitSubtaskResult(subtaskRunCtx, taskID)
				},
			}, nil
		}

	}
}

func toBool(value interface{}) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	}
	return false
}

func toUint(value interface{}) (uint, bool) {
	switch typed := value.(type) {
	case *uint:
		if typed == nil {
			return 0, false
		}
		return toUint(*typed)
	case *uint64:
		if typed == nil {
			return 0, false
		}
		return toUint(*typed)
	case *int:
		if typed == nil {
			return 0, false
		}
		return toUint(*typed)
	case *float64:
		if typed == nil {
			return 0, false
		}
		return toUint(*typed)
	case *float32:
		if typed == nil {
			return 0, false
		}
		return toUint(*typed)
	case uint:
		return typed, true
	case uint64:
		return uint(typed), true
	case int:
		if typed < 0 {
			return 0, false
		}
		return uint(typed), true
	case float64:
		if typed < 0 {
			return 0, false
		}
		return uint(typed), true
	case float32:
		if typed < 0 {
			return 0, false
		}
		return uint(typed), true
	default:
		return 0, false
	}
}

func loadMatrixopsAgentConfig(db *gorm.DB) (matrixopsAgentConfig, bool, error) {
	config, err := database.GetGlobalConfigByKey(db, models.ConfigKeyMatrixopsAgent)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return matrixopsAgentConfig{}, false, nil
		}
		return matrixopsAgentConfig{}, false, err
	}
	decoded := matrixopsAgentConfig{}
	if err := json.Unmarshal([]byte(config.Value), &decoded); err != nil {
		return matrixopsAgentConfig{}, false, err
	}
	return decoded, true, nil
}

func applyMatrixopsAgentConfig(cfg *agentconfig.Config, agentConfig matrixopsAgentConfig) {
	if cfg == nil {
		return
	}
	if cfg.Provider == nil {
		cfg.Provider = map[string]agentconfig.ProviderConfig{}
	}
	openai := cfg.Provider["openai"]
	if openai.Options == nil {
		openai.Options = map[string]interface{}{}
	}

	if agentConfig.Key != "" {
		openai.Options["apiKey"] = agentConfig.Key
		openai.Options["key"] = agentConfig.Key
	} else {
		delete(openai.Options, "apiKey")
		delete(openai.Options, "key")
	}

	if agentConfig.BaseURL != "" {
		openai.Options["baseURL"] = agentConfig.BaseURL
		openai.Options["baseurl"] = agentConfig.BaseURL
	} else {
		delete(openai.Options, "baseURL")
		delete(openai.Options, "baseurl")
	}

	if agentConfig.Proxy != "" {
		openai.Options["proxy"] = agentConfig.Proxy
		cfg.Proxy = agentConfig.Proxy
	} else {
		delete(openai.Options, "proxy")
		cfg.Proxy = ""
	}

	if agentConfig.Model != "" {
		cfg.Model = agentConfig.Model
	} else {
		cfg.Model = ""
	}

	cfg.Provider["openai"] = openai
}

func matrixopsAgentToolStatus(status string) models.ToolStatus {
	switch status {
	case "completed":
		return models.ToolStatusSuccess
	case "error":
		return models.ToolStatusFailed
	default:
		return models.ToolStatusCreated
	}
}

func matrixopsAgentCommandSpec(workerName string, provider string, model string) (string, []string) {
	args := []string{}
	if workerName != "" {
		args = append(args, "--worker", workerName)
	}
	if provider != "" {
		args = append(args, "--provider", provider)
	}
	if model != "" {
		args = append(args, "--model", model)
	}
	return "matrixops-agent", args
}
