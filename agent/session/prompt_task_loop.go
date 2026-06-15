package session

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"matrixops-agent/llm"
	"matrixops-agent/project"
	"matrixops-agent/provider"
	"matrixops-agent/snapshot"
	"matrixops-agent/tool"
	"matrixops-agent/types"
	database "pkgs/db"
	"pkgs/db/models"
	"pkgs/db/storage"
	mcppkg "pkgs/mcp"

	coreagent "matrixops.local/core_agent"

	"gorm.io/gorm"
)

// RuntimeConfig 运行时配置（任务级别）
type RuntimeConfig struct {
	// 上下文
	Ctx context.Context

	// 配置引用
	// Config *AgentRunnerConfig

	// LLM 配置
	LLMClient                llm.ChatClient
	LLMHTTPClient            *http.Client
	llmHTTPClientOnce        sync.Once
	Worker                   *models.Worker
	LLMConfig                *models.LLMConfig
	ModelSettings            *models.ModelSettings
	Model                    string
	ReasoningEffort          string
	TextVerbosity            string
	EnableEncryptedReasoning bool
	ParallelToolCalls        bool
	SilentToolWatchdogEnabled bool
	PromptCacheKey           string
	// ThinkingType 来自模型配置（ModelSettings），用于原生请求 thinking.type。
	ThinkingType string
	// EnableThinking 来自 ModelSettings；非 nil 时请求体顶层 enable_thinking（与 ThinkingType 独立）。
	EnableThinking *bool
	// BudgetTokens 来自 ModelSettings；用于 Anthropic 原生请求 thinking.budget_tokens。
	BudgetTokens *int

	// 用户输入
	UserInput     string
	Parts         []*Part
	MessageKind   string
	MessageOrigin string

	// 工具
	Tools        []llm.ToolDefinition
	ToolRegistry *tool.Registry

	// 消息处理
	Assistant *MessageInfo

	// 运行时选项
	MergeMessage          bool
	SkipCreateUserMessage bool
	SessionWindow         *SessionWindow

	// 工具访问控制
	Project                *models.Project
	ProjectToolPermissions map[string]string
	WorkerEnabledTools     map[string]struct{}
	HasWorkerEnabledTools  bool

	// 快照
	CurrentSnapshot string

	// 强制继续
	ForceContinue bool

	// 是否要求 call_tool 带 reason
	EnableCallToolReason bool

	BaseMemory     *types.Memory
	MemoryState    *ProcessV2MemoryState
	ProcessV2Hooks *ProcessV2Hooks
	ActionSchemas  []coreagent.ActionSchema

	AutoCompressionLimitTokens      int
	SessionTokens                   *MessageTokens
	ManualMemoryCompactionRequested bool
	ManualMemoryCompactionPrompt    string
	ManualSessionSummaryRequested   bool
	ManualSessionSummaryPrompt      string
	CommandRequestMessageID         string
	NewWorktreeBranch               string
	// StallWatchdogTimeout 为工具 stall watchdog 触发超时。≤0 表示禁用。
	StallWatchdogTimeout time.Duration
}

// clone 创建 RuntimeConfig 的副本
func (rc *RuntimeConfig) clone() *RuntimeConfig {
	if rc == nil {
		return nil
	}

	// 创建新的 RuntimeConfig
	newRC := &RuntimeConfig{
		// 上下文
		Ctx: rc.Ctx,

		// LLM 配置（指针类型，共享引用）
		LLMClient:                rc.LLMClient,
		LLMHTTPClient:            rc.LLMHTTPClient,
		Worker:                   rc.Worker,
		LLMConfig:                rc.LLMConfig,
		ModelSettings:            rc.ModelSettings,
		Model:                    rc.Model,
		ReasoningEffort:          rc.ReasoningEffort,
		TextVerbosity:            rc.TextVerbosity,
		EnableEncryptedReasoning: rc.EnableEncryptedReasoning,
		ParallelToolCalls:         rc.ParallelToolCalls,
		SilentToolWatchdogEnabled: rc.SilentToolWatchdogEnabled,
		PromptCacheKey:            rc.PromptCacheKey,
		ThinkingType:             rc.ThinkingType,
		EnableThinking:           rc.EnableThinking,
		BudgetTokens:             rc.BudgetTokens,

		// 用户输入
		UserInput:     rc.UserInput,
		MessageKind:   rc.MessageKind,
		MessageOrigin: rc.MessageOrigin,

		// 工具（指针类型，共享引用）
		ToolRegistry: rc.ToolRegistry,

		// 消息处理（指针类型，共享引用）
		Assistant: rc.Assistant,

		// 运行时选项
		MergeMessage:          rc.MergeMessage,
		SkipCreateUserMessage: rc.SkipCreateUserMessage,
		SessionWindow:         rc.SessionWindow,

		// 工具访问控制
		Project:                rc.Project,
		ProjectToolPermissions: rc.ProjectToolPermissions,
		WorkerEnabledTools:     rc.WorkerEnabledTools,
		HasWorkerEnabledTools:  rc.HasWorkerEnabledTools,

		// 快照
		CurrentSnapshot: rc.CurrentSnapshot,

		// call_tool reason 开关
		EnableCallToolReason:            rc.EnableCallToolReason,
		BaseMemory:                      rc.BaseMemory,
		MemoryState:                     rc.MemoryState,
		ProcessV2Hooks:                  rc.ProcessV2Hooks,
		ActionSchemas:                   append([]coreagent.ActionSchema(nil), rc.ActionSchemas...),
		AutoCompressionLimitTokens:      rc.AutoCompressionLimitTokens,
		SessionTokens:                   rc.SessionTokens,
		ManualMemoryCompactionRequested: rc.ManualMemoryCompactionRequested,
		ManualMemoryCompactionPrompt:    rc.ManualMemoryCompactionPrompt,
		ManualSessionSummaryRequested:   rc.ManualSessionSummaryRequested,
		ManualSessionSummaryPrompt:      rc.ManualSessionSummaryPrompt,
		CommandRequestMessageID:         rc.CommandRequestMessageID,
		NewWorktreeBranch:               rc.NewWorktreeBranch,
		StallWatchdogTimeout:            rc.StallWatchdogTimeout,
	}

	// 复制 Parts 切片（浅拷贝）
	if rc.Parts != nil {
		newRC.Parts = make([]*Part, len(rc.Parts))
		copy(newRC.Parts, rc.Parts)
	}

	return newRC
}

func (r *RuntimeConfig) SetWorker(db *gorm.DB, workerName string) error {
	if db == nil {
		return errors.New("db is required to resolve worker by name")
	}
	m, err := database.LoadWorkerModelContext(db, workerName)
	if err != nil {
		return err
	}
	worker := m.Worker
	r.Worker = worker
	r.LLMConfig = m.LLMConfig
	r.ModelSettings = m.ModelSettings
	r.Model = worker.Model

	workerEnabledTools, hasWorkerEnabledTools, err := models.ParseEnabledTools(worker.EnabledTools)
	if err != nil {
		return fmt.Errorf("parse enabled tools failed: %w", err)
	}
	r.WorkerEnabledTools = workerEnabledTools
	r.HasWorkerEnabledTools = hasWorkerEnabledTools
	r.Tools = resolveTools(
		buildToolOverrides(
			r.ToolRegistry,
			workerEnabledTools,
			hasWorkerEnabledTools,
			r.Project,
			r.ProjectToolPermissions,
		),
		r.ToolRegistry,
	)
	return nil
}

func (r *RuntimeConfig) SetUserInput(userInput string) {
	r.UserInput = userInput
}

// buildRuntimeConfig 构建运行时配置
// 创建任务执行所需的运行时上下文
func (r *AgentRunner) buildRuntimeConfig(cfg *AgentRunnerConfig) (*RuntimeConfig, error) {
	dbConn := r.db
	if cfg != nil && cfg.db != nil {
		dbConn = cfg.db
	}

	// 验证必需的任务级别配置
	workerName := cfg.WorkerName
	if workerName == "" {
		workerName = "chat"
	}

	// 获取 worker 信息
	worker := cfg.Worker
	var err error
	if worker == nil {
		if dbConn == nil {
			return nil, errors.New("worker config is required when db is not provided")
		}
		worker, err = database.GetWorkerByName(dbConn, workerName)
		if err != nil {
			return nil, fmt.Errorf("get worker failed: %w", err)
		}
	}

	llmConfig := cfg.LLMConfig
	if worker.LLMConfigID != nil && dbConn != nil {
		dbCfg, dbErr := database.GetLLMConfigByID(dbConn, *worker.LLMConfigID)
		if dbErr != nil {
			return nil, fmt.Errorf("get llm config failed: %w", dbErr)
		}
		if llmConfig == nil {
			llmConfig = dbCfg
		} else {
			// 调用方可能注入未含库表字段的 LLMConfig；OpenAI 原生 tools、代理等以数据库为准。
			llmConfig.NativeOpenAIToolCalls = dbCfg.NativeOpenAIToolCalls
			llmConfig.Proxy = dbCfg.Proxy
		}
	}

	modelSettings := cfg.ModelSettings
	if modelSettings == nil && dbConn != nil {
		modelSettings, _ = database.GetModelSettingsForWorker(dbConn, worker)
	}
	if modelSettings != nil && llmConfig != nil {
		llmConfig.NativeOpenAIToolCalls = modelSettings.NativeOpenAIToolCalls
	}
	deriv := computeModelSettingsRuntimeDerivatives(modelSettings, cfg.TaskID, dbConn)
	reasoningEffort := deriv.ReasoningEffort
	textVerbosity := deriv.TextVerbosity
	enableEncryptedReasoning := deriv.EnableEncryptedReasoning
	parallelToolCalls := deriv.ParallelToolCalls
	silentToolWatchdogEnabled := deriv.SilentToolWatchdogEnabled
	promptCacheKey := deriv.PromptCacheKey
	thinkingType := deriv.ThinkingType
	var enableThinking *bool
	if modelSettings != nil {
		enableThinking = modelSettings.EnableThinking
	}
	var budgetTokens *int
	if modelSettings != nil {
		budgetTokens = modelSettings.BudgetTokens
	}

	// 拍摄快照（使用方法访问 session 信息）
	var currentSnapshot string
	if r.GetDirectory() != "" && r.GetProjectID() != "" {
		if err := project.Provide(r.GetDirectory(), nil, func() error {
			snapshotValue, trackErr := snapshot.Track(r.GetProjectID(), r.GetDirectory())
			if trackErr != nil {
				return trackErr
			}
			currentSnapshot = snapshotValue
			return nil
		}); err != nil {
			return nil, fmt.Errorf("track current snapshot: %w", err)
		}
	}

	// 构建 assistant 消息（使用方法访问 sessionID）
	assistant := MessageInfo{
		ID:         generateMessageID(),
		SessionID:  r.GetSessionID(),
		Role:       RoleAssistant,
		Worker:     cfg.WorkerName,
		ModelID:    worker.Model,
		Occupation: worker.Occupation,
		Time:       MessageTime{Created: currentTimeMillis()},
		State:      "loading",
		Snapshot:   currentSnapshot,
	}
	if llmConfig != nil {
		assistant.ProviderID = llmConfig.Name
	}

	var currentProject *models.Project
	projectID := strings.TrimSpace(cfg.ProjectID)
	if projectID == "" {
		projectID = strings.TrimSpace(r.GetProjectID())
	}
	if projectID != "" && dbConn != nil {
		parsedProjectID, parseErr := strconv.ParseUint(projectID, 10, 64)
		if parseErr == nil && parsedProjectID > 0 {
			currentProject, err = database.GetProjectByID(dbConn, uint(parsedProjectID))
			if err != nil {
				return nil, fmt.Errorf("get project failed: %w", err)
			}
		}
	}

	projectToolPermissions := map[string]string{}
	if currentProject != nil {
		projectToolPermissions, err = models.ParseProjectToolPermissions(currentProject.ToolPermissions)
		if err != nil {
			return nil, fmt.Errorf("parse project tool permissions failed: %w", err)
		}
	}

	workerEnabledTools, hasWorkerEnabledTools, err := models.ParseEnabledTools(worker.EnabledTools)
	if err != nil {
		return nil, fmt.Errorf("parse enabled tools failed: %w", err)
	}

	llmClient := cfg.LLM
	if llmClient == nil {
		llmClient = provider.NewGenericClient()
	}

	baseMemory := cfg.baseMemory
	if baseMemory == nil && strings.TrimSpace(cfg.System) != "" {
		baseMemory = &types.Memory{
			GlobalPrompt: strings.TrimSpace(cfg.System),
		}
	}
	if baseMemory != nil && !promptHasTaskLoopGuidance(baseMemory.GlobalPrompt) {
		baseMemory.GlobalPrompt = appendTaskLoopGuidance(baseMemory.GlobalPrompt)
	}
	var memoryState *ProcessV2MemoryState
	if baseMemory == nil && dbConn != nil && r.GetSessionID() != "" && worker != nil {
		taskForMemory := r.task
		if taskForMemory != nil {
			if loaded, loadErr := r.getMemory(taskForMemory, dbConn, r.GetSessionID(), worker.Name); loadErr == nil {
				baseMemory = loaded
			}
		}
	}
	memoryState = NewProcessV2MemoryState(baseMemory)

	toolRegistry := cfg.Tools
	if toolRegistry == nil {
		toolRegistry = tool.NewDefaultRegistryWithQuestion(&tool.DefaultRegistryOptions{
			DB:     dbConn,
			TaskID: cfg.TaskID,
			SetVar: func(key string, value any) {
				r.emitter.SetPluginVar(key, value)
			},
			SessionID: r.GetSessionID(),
			WaitUserInput: func(questions map[string]interface{}) (map[string]any, error) {
				return r.emitter.WaitUserInput(questions)
			},
			AvailableWorkerNames: func() []string {
				return loadCallableWorkerNames(dbConn)
			},
			RunWorkerTask: buildRunWorkerTaskFunc(dbConn, cfg.TaskID, cfg.newTaskHandler, func() *types.Memory {
				if memoryState != nil {
					return memoryState.Snapshot()
				}
				return cloneProcessV2Memory(baseMemory)
			}),
			StopWorkerTask:        buildStopWorkerTaskFunc(dbConn, cfg.TaskID),
			GetWorkerTaskProgress: buildGetWorkerTaskProgressFunc(dbConn, cfg.TaskID),
			SendMessageToWorker:   buildRunWorkerTaskFunc(dbConn, cfg.TaskID, cfg.newTaskHandler, func() *types.Memory {
				if memoryState != nil {
					return memoryState.Snapshot()
				}
				return cloneProcessV2Memory(baseMemory)
			}),
			DeliverUserMessage: buildDeliverUserMessage(cfg),
		})
	}
	tool.RegisterMcpTools(toolRegistry, mcppkg.GetManager())
	tool.RegisterSearchTools(toolRegistry, dbConn)
	tool.RegisterMemorySearchTools(toolRegistry, dbConn, r.GetSessionID())

	tools := resolveTools(
		buildToolOverrides(
			toolRegistry,
			workerEnabledTools,
			hasWorkerEnabledTools,
			currentProject,
			projectToolPermissions,
		),
		toolRegistry,
	)

	stallWatchdogTimeout := cfg.stallWatchdogTimeout
	if stallWatchdogTimeout <= 0 && dbConn != nil {
		stallWatchdogTimeout = database.GetStallWatchdogTimeout(dbConn)
	}

	return &RuntimeConfig{
		Ctx:                           cfg.Ctx,
		LLMClient:                     llmClient,
		Worker:                        worker,
		LLMConfig:                     llmConfig,
		ModelSettings:                 modelSettings,
		Model:                         worker.Model,
		ReasoningEffort:               reasoningEffort,
		TextVerbosity:                 textVerbosity,
		EnableEncryptedReasoning:      enableEncryptedReasoning,
		ParallelToolCalls:             parallelToolCalls,
		SilentToolWatchdogEnabled:     silentToolWatchdogEnabled,
		PromptCacheKey:                promptCacheKey,
		ThinkingType:                  thinkingType,
		EnableThinking:                enableThinking,
		BudgetTokens:                  budgetTokens,
		UserInput:                     extractUserInput(cfg.Parts),
		Parts:                         cfg.Parts,
		MessageKind:                   cfg.messageKind,
		MessageOrigin:                 cfg.messageOrigin,
		ToolRegistry:                  toolRegistry,
		Tools:                         tools,
		Assistant:                     &assistant,
		MergeMessage:                  cfg.mergeMessage,
		SkipCreateUserMessage:         cfg.skipCreateUserMessage,
		SessionWindow:                 cfg.sessionWindow,
		Project:                       currentProject,
		ProjectToolPermissions:        projectToolPermissions,
		WorkerEnabledTools:            workerEnabledTools,
		HasWorkerEnabledTools:         hasWorkerEnabledTools,
		CurrentSnapshot:               currentSnapshot,
		EnableCallToolReason:          cfg.enableCallToolReason,
		BaseMemory:                    baseMemory,
		ProcessV2Hooks:                cfg.processV2Hooks,
		MemoryState:                   memoryState,
		ActionSchemas:                 append([]coreagent.ActionSchema(nil), cfg.actionSchemas...),
		AutoCompressionLimitTokens:    deriv.AutoCompressionLimitTokens,
		SessionTokens:                 configSessionTokens(cfg, r.GetSession()),
		StallWatchdogTimeout:            stallWatchdogTimeout,
	}, nil
}

func normalizeReasoningLevel(value *string) string {
	if value == nil {
		return ""
	}
	switch strings.TrimSpace(*value) {
	case "low", "medium", "high", "xhigh", "none", "max":
		return strings.TrimSpace(*value)
	default:
		return ""
	}
}

// modelSettingsRuntimeDerivatives 将 ModelSettings 映射为任务循环使用的标量（供单测逐项校验“生效”）。
type modelSettingsRuntimeDerivatives struct {
	ReasoningEffort            string
	TextVerbosity              string
	EnableEncryptedReasoning   bool
	ParallelToolCalls           bool
	SilentToolWatchdogEnabled   bool
	PromptCacheKey              string
	ThinkingType               string
	BudgetTokens               *int
	AutoCompressionLimitTokens int
}

func computeModelSettingsRuntimeDerivatives(modelSettings *models.ModelSettings, taskID uint, dbConn *gorm.DB) modelSettingsRuntimeDerivatives {
	var d modelSettingsRuntimeDerivatives
	if modelSettings == nil {
		return d
	}
	d.ReasoningEffort = normalizeReasoningLevel(modelSettings.ReasoningEffort)
	d.TextVerbosity = normalizeReasoningLevel(modelSettings.TextVerbosity)
	d.EnableEncryptedReasoning = modelSettings.EnableEncryptedReason != nil && *modelSettings.EnableEncryptedReason
	d.ParallelToolCalls = modelSettings.ParallelToolCalls != nil && *modelSettings.ParallelToolCalls
	d.SilentToolWatchdogEnabled = models.SilentToolWatchdogEnabled(modelSettings)
	if modelSettings.EnablePromptCacheKey != nil && *modelSettings.EnablePromptCacheKey && taskID > 0 && dbConn != nil {
		if task, taskErr := database.GetTaskByID(dbConn, taskID); taskErr == nil && task != nil {
			d.PromptCacheKey = strings.TrimSpace(task.PromptCacheKey)
		}
	}
	d.ThinkingType = models.EffectiveThinkingType(modelSettings)
	d.BudgetTokens = modelSettings.BudgetTokens
	d.AutoCompressionLimitTokens = resolveAutoCompressionLimitTokens(modelSettings)
	return d
}

func resolveAutoCompressionLimitTokens(modelSettings *models.ModelSettings) int {
	if modelSettings == nil {
		return 0
	}
	if modelSettings.ContextLimit > 0 {
		return modelSettings.ContextLimit
	}
	return 0
}

func configSessionTokens(cfg *AgentRunnerConfig, sessionInfo *types.Info) *MessageTokens {
	if cfg != nil && cfg.Session != nil && cfg.Session.Tokens != nil {
		tokens := *cfg.Session.Tokens
		return &tokens
	}
	if sessionInfo != nil && sessionInfo.Tokens != nil {
		tokens := *sessionInfo.Tokens
		return &tokens
	}
	return nil
}

func getLastAssistantMessage(db *gorm.DB, sessionID string) (*WithParts, error) {
	parts, err := storage.GetSessionMessageParts(db, sessionID)
	if err != nil {
		return nil, err
	}

	// 从后往前查找最后一个 assistant 消息
	for i := len(parts) - 1; i >= 0; i-- {
		if parts[i].Info != nil && parts[i].Info.Role == RoleAssistant {
			return parts[i], nil
		}
	}

	return nil, nil
}

func getLastAssistantMessageText(db *gorm.DB, sessionID string) (string, error) {
	message, err := getLastAssistantMessage(db, sessionID)
	if err != nil {
		return "", err
	}
	return joinTextParts(message.Parts), nil
}

// extractUserInput 从 Parts 提取用户输入文本
func extractUserInput(parts []*Part) string {
	if len(parts) == 0 {
		return ""
	}
	return mergePartsToText(parts)
}

// mergePartsToText 合并文本 part 为纯文本（不含文件/图片占位符）。
func mergePartsToText(parts []*Part) string {
	return UserInputTextOnly(parts)
}
