package session

import (
	"matrixops-agent/llm"
	"matrixops-agent/permission"
	"matrixops-agent/plugin"
	"matrixops-agent/tool"
	"matrixops-agent/types"
	coreagent "matrixops.local/core_agent"
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	database "pkgs/db"
	"pkgs/db/models"
	"pkgs/db/storage"
	mcppkg "pkgs/mcp"
	"pkgs/taskqueue"
	"time"

	"matrixops-agent/util"
	"pkgs/sizewg"

	"gorm.io/gorm"
)

type AgentRunner struct {
	// 会话信息（包含 sessionID 和 projectID）
	session *types.Info

	// 任务信息
	task *models.Task

	// 基础设施
	db            *gorm.DB
	emitter       *Emitter
	perms         *permission.Manager
	pluginManager *plugin.Manager

	// 内部状态
	wg           *sizewg.SizedWaitGroup
	isNewSession bool

	messageQueue *taskqueue.Queue

	// toolPermissionMu serializes project tool permission prompts so concurrent tool
	// calls do not emit overlapping wait_user_input requests.
	toolPermissionMu sync.Mutex
}

// GetSessionID 获取会话ID
func (r *AgentRunner) GetSessionID() string {
	if r.session != nil {
		return r.session.ID
	}
	return ""
}

// GetProjectID 获取项目ID
func (r *AgentRunner) GetProjectID() string {
	if r.session != nil {
		return r.session.ProjectID
	}
	return ""
}

// GetDirectory 获取工作目录
func (r *AgentRunner) GetDirectory() string {
	if r.session != nil {
		return r.session.Directory
	}
	return ""
}

// GetSession 获取会话信息
func (r *AgentRunner) GetSession() *types.Info {
	return r.session
}

// GetTask 获取任务信息
func (r *AgentRunner) GetTask() *models.Task {
	return r.task
}

func NewAgentRunner(options ...AgentRunnerOption) (*AgentRunner, error) {
	// 1. 构建 AgentRunnerConfig
	config := NewAgentRunnerConfig(options...)

	// 2. 验证必需的基础设施配置
	// 3. 创建 Emitter（如果没有提供）
	var sessionID string
	var isNewSession bool

	if config.SessionID == "" {
		if config.Session != nil && config.Session.ID != "" {
			sessionID = config.Session.ID
			config.SessionID = config.Session.ID
			isNewSession = false
		} else if config.db != nil {
			isNewSession = true
			seedLibraryIDs := resolveSeedMemoryLibraryIDs(config)
			sess, err := storage.NewSessionWithMemoryLibraries(config.db, config.ProjectID, config.Directory, seedLibraryIDs)
			if err != nil {
				return nil, fmt.Errorf("failed to create new session: %w", err)
			}
			sessionID = sess.ID
			config.SessionID = sess.ID
			config.Session = sess
		} else {
			isNewSession = true
			now := time.Now().UnixMilli()
			sessionID = util.Descending("session")
			config.SessionID = sessionID
			config.Session = &types.Info{
				ID:        sessionID,
				Slug:      util.Slug(),
				ProjectID: config.ProjectID,
				Directory: config.Directory,
				Title:     createDefaultTitle(false),
				Version:   util.Version(),
				Time: types.TimeInfo{
					Created: now,
					Updated: now,
				},
			}
			if config.TaskID > 0 {
				if root, workspace, err := database.NewTaskWorkspace(config.TaskID, "", config.Directory); err == nil {
					config.Session.WorkspaceRoot = root
					config.Session.WorkspacePath = workspace
				}
			} else {
				if root, workspace, err := database.NewSessionWorkspace("", config.Directory); err == nil {
					config.Session.WorkspaceRoot = root
					config.Session.WorkspacePath = workspace
				}
			}
		}
	} else {
		// 使用现有会话
		isNewSession = false
		sessionID = config.SessionID
		if config.Session == nil {
			if config.db == nil {
				config.Session = &types.Info{
					ID:        config.SessionID,
					ProjectID: config.ProjectID,
					Directory: config.Directory,
					Title:     createDefaultTitle(false),
					Version:   util.Version(),
					Time: types.TimeInfo{
						Created: time.Now().UnixMilli(),
						Updated: time.Now().UnixMilli(),
					},
				}
			} else {
				sess, err := storage.GetSession(config.db, config.SessionID)
				if err != nil {
					return nil, fmt.Errorf("failed to get session by ID %s: %w", config.SessionID, err)
				}
				config.Session = sess
			}
		}
	}

	// 4. 创建或更新 Emitter
	if config.Emitter == nil {
		config.Emitter = NewEmitter(config.db, sessionID)
	}

	// 5. 触发 Emitter 创建回调
	for _, onEmitterCreated := range config.OnEmitterCreateds {
		if err := onEmitterCreated(config.Emitter); err != nil {
			return nil, fmt.Errorf("failed to execute emitter created callback: %w", err)
		}
	}

	// 6. 发送会话创建事件
	config.Emitter.Emit(EventSessionCreated, SessionEvent{Info: config.Session})

	// 7. 加载 Task（如果提供了 TaskID）
	var task *models.Task
	if config.TaskID > 0 {
		if config.db == nil {
			return nil, errors.New("db is required when task id is set")
		}
		var err error
		task, err = database.GetTaskByID(config.db, config.TaskID)
		if err != nil {
			return nil, fmt.Errorf("failed to get task by ID %d: %w", config.TaskID, err)
		}
		config.Task = task
	} else if config.Task != nil {
		task = config.Task
	}

	// 8. 创建 Tools（如果没有提供）
	if config.Tools == nil {
		opts := &tool.DefaultRegistryOptions{
			DB: config.db,
			TaskID: config.TaskID,
			SetVar: func(key string, value any) {
				config.Emitter.SetPluginVar(key, value)
			},
			SessionID: sessionID,
			WaitUserInput: func(questions map[string]interface{}) (map[string]any, error) {
				return config.Emitter.WaitUserInput(questions)
			},
			AvailableWorkerNames: func() []string {
				return loadCallableWorkerNames(config.db)
			},
			RunWorkerTask: buildRunWorkerTaskFunc(config.db, config.TaskID, config.newTaskHandler, func() *types.Memory {
				return cloneProcessV2Memory(config.baseMemory)
			}),
			DeliverUserMessage: buildDeliverUserMessage(config),
		}
		config.Tools = tool.NewDefaultRegistryWithQuestion(opts)
		tool.RegisterMcpTools(config.Tools, mcppkg.GetManager())
		tool.RegisterSearchTools(config.Tools, config.db)
		tool.RegisterMemorySearchTools(config.Tools, config.db, sessionID)
	}

	// 9. 验证 session 对象的完整性
	if config.Session.ID == "" {
		return nil, errors.New("session ID is empty")
	}
	if config.Session.ProjectID == "" {
		return nil, errors.New("session projectID is empty")
	}

	// 10. 消息队列（绑定任务时实例化）
	taskID := uint(0)
	if task != nil {
		taskID = task.ID
	} else if config.TaskID > 0 {
		taskID = config.TaskID
	}
	var messageQueue *taskqueue.Queue
	if config.messageQueue != nil {
		messageQueue = config.messageQueue
	} else if config.db != nil && taskID > 0 {
		messageQueue = taskqueue.New(config.db, taskID, config.queueBroadcast)
	}
	config.messageQueue = messageQueue

	// 11. 构建 AgentRunner
	return &AgentRunner{
		session:              config.Session,
		task:                 task,
		db:                   config.db,
		emitter:              config.Emitter,
		perms:                config.Perms,
		pluginManager:        config.PluginManager,
		wg:                   sizewg.New(10),
		isNewSession:         isNewSession,
		messageQueue: config.messageQueue,
	}, nil
}

func toStringValue(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return ""
	}
}

type AgentRunnerConfig struct {
	db            *gorm.DB
	LLM           llm.ChatClient
	Tools         *tool.Registry
	Perms         *permission.Manager
	PluginManager *plugin.Manager
	WorkerName    string
	Worker        *models.Worker
	LLMConfig     *models.LLMConfig
	ModelSettings *models.ModelSettings

	Emitter           *Emitter
	SessionID         string
	Ctx               context.Context
	MessageID         string
	ToolConfig        map[string]bool
	System            string
	Variant           string
	Parts             []*Part
	OnEmitterCreateds []func(emitter *Emitter) error

	forceContinue  bool
	newTaskHandler func(args map[string]interface{}) (map[string]interface{}, error)

	ProjectID string
	Directory string
	TaskID    uint

	Task *models.Task

	Session *types.Info

	skipCreateUserMessage bool
	mergeMessage          bool
	messageKind           string
	messageOrigin         string

	sessionWindow *SessionWindow

	enableCallToolReason bool

	baseMemory     *types.Memory
	processV2Hooks *ProcessV2Hooks
	actionSchemas  []coreagent.ActionSchema

	stallWatchdogTimeout          time.Duration
	queueBroadcast                taskqueue.BroadcastFunc
	messageQueue                  *taskqueue.Queue
	deliverUserMessage            tool.DeliverUserMessageFunc
}

type SessionWindow struct {
	Roles     []string
	IsAllRole bool
	Me        string
	Prompt    string
	History   []*WithParts
}

type AgentRunnerOption func(*AgentRunnerConfig)

func NewAgentRunnerConfig(options ...AgentRunnerOption) *AgentRunnerConfig {
	config := &AgentRunnerConfig{
		Perms: permission.NewManager(nil),
	}
	for _, option := range options {
		option(config)
	}
	ensureConfigDefaults(config)
	return config
}

func WithLLM(client llm.ChatClient) AgentRunnerOption {
	return func(c *AgentRunnerConfig) {
		c.LLM = client
	}
}

func WithTools(tools *tool.Registry) AgentRunnerOption {
	return func(c *AgentRunnerConfig) {
		c.Tools = tools
	}
}

func WithSessionWindow(window *SessionWindow) AgentRunnerOption {
	return func(c *AgentRunnerConfig) {
		c.sessionWindow = window
	}
}

func WithSkipCreateUserMessage(skipCreateUserMessage bool) AgentRunnerOption {
	return func(c *AgentRunnerConfig) {
		c.skipCreateUserMessage = skipCreateUserMessage
	}
}

func WithMergeMessage(mergeMessage bool) AgentRunnerOption {
	return func(c *AgentRunnerConfig) {
		c.mergeMessage = mergeMessage
	}
}

func WithEnableCallToolReason(enable bool) AgentRunnerOption {
	return func(c *AgentRunnerConfig) {
		c.enableCallToolReason = enable
	}
}

func WithPerms(perms *permission.Manager) AgentRunnerOption {
	return func(c *AgentRunnerConfig) {
		c.Perms = perms
	}
}

func WithPluginManager(manager *plugin.Manager) AgentRunnerOption {
	return func(c *AgentRunnerConfig) {
		c.PluginManager = manager
	}
}

func WithDB(db *gorm.DB) AgentRunnerOption {
	return func(c *AgentRunnerConfig) {
		c.db = db
	}
}

func WithProjectID(projectID string) AgentRunnerOption {
	return func(c *AgentRunnerConfig) {
		c.ProjectID = projectID
	}
}

func WithCtx(ctx context.Context) AgentRunnerOption {
	return func(c *AgentRunnerConfig) {
		c.Ctx = ctx
	}
}

func WithDirectory(directory string) AgentRunnerOption {
	return func(c *AgentRunnerConfig) {
		c.Directory = directory
	}
}

// func WithSessionBus(sessionBus *SessionBusEvent) AgentRunnerOption {
// 	return func(c *AgentRunnerConfig) {
// 		c.SessionBus = sessionBus
// 	}
// }

func WithSessionID(sessionID string) AgentRunnerOption {
	return func(c *AgentRunnerConfig) {
		c.SessionID = sessionID
		// if c.SessionBus != nil {
		// 	c.SessionBus.SessionID = sessionID
		// }
	}
}

func WithWorker(workerName string) AgentRunnerOption {
	return func(c *AgentRunnerConfig) {
		c.WorkerName = workerName
	}
}

func WithWorkerModel(worker *models.Worker) AgentRunnerOption {
	return func(c *AgentRunnerConfig) {
		c.Worker = worker
		if worker != nil && c.WorkerName == "" {
			c.WorkerName = worker.Name
		}
	}
}

func WithLLMConfigModel(cfg *models.LLMConfig) AgentRunnerOption {
	return func(c *AgentRunnerConfig) {
		c.LLMConfig = cfg
	}
}

func WithModelSettings(settings *models.ModelSettings) AgentRunnerOption {
	return func(c *AgentRunnerConfig) {
		c.ModelSettings = settings
	}
}

func WithForceContinue(forceContinue bool) AgentRunnerOption {
	return func(c *AgentRunnerConfig) {
		c.forceContinue = forceContinue
	}
}

func WithInputText(inputText string) AgentRunnerOption {
	return func(c *AgentRunnerConfig) {
		c.Parts = append(c.Parts, &Part{
			Type: types.PartTypeText,
			Text: inputText,
		})
	}
}

func WithInputParts(inputParts []*Part) AgentRunnerOption {
	return func(c *AgentRunnerConfig) {
		c.Parts = append(c.Parts, inputParts...)
	}
}

func WithMessageKind(kind string) AgentRunnerOption {
	return func(c *AgentRunnerConfig) {
		c.messageKind = strings.TrimSpace(kind)
	}
}

func WithMessageOrigin(origin string) AgentRunnerOption {
	return func(c *AgentRunnerConfig) {
		c.messageOrigin = strings.TrimSpace(origin)
	}
}

func WithMessageID(messageID string) AgentRunnerOption {
	return func(c *AgentRunnerConfig) {
		c.MessageID = messageID
	}
}

func WithSession(session *types.Info) AgentRunnerOption {
	return func(c *AgentRunnerConfig) {
		c.Session = session
		if session != nil && c.SessionID == "" {
			c.SessionID = session.ID
		}
	}
}

func WithEmitter(emitter *Emitter) AgentRunnerOption {
	return func(c *AgentRunnerConfig) {
		c.Emitter = emitter
	}
}

func WithNewTaskHandler(newTaskHandler func(args map[string]interface{}) (map[string]interface{}, error)) AgentRunnerOption {
	return func(c *AgentRunnerConfig) {
		c.newTaskHandler = newTaskHandler
	}
}

func WithToolConfig(toolConfig map[string]bool) AgentRunnerOption {
	return func(c *AgentRunnerConfig) {
		if toolConfig == nil {
			c.ToolConfig = nil
			return
		}
		c.ToolConfig = copyToolConfig(toolConfig)
	}
}

func WithTaskID(taskID uint) AgentRunnerOption {
	return func(c *AgentRunnerConfig) {
		c.TaskID = taskID
	}
}

func WithQueueBroadcaster(broadcast taskqueue.BroadcastFunc) AgentRunnerOption {
	return func(c *AgentRunnerConfig) {
		c.queueBroadcast = broadcast
	}
}

// WithMessageQueue 注入已构造的消息队列（主要用于测试）。
func WithMessageQueue(queue *taskqueue.Queue) AgentRunnerOption {
	return func(c *AgentRunnerConfig) {
		c.messageQueue = queue
	}
}

func WithDeliverUserMessage(fn tool.DeliverUserMessageFunc) AgentRunnerOption {
	return func(c *AgentRunnerConfig) {
		c.deliverUserMessage = fn
	}
}

func buildDeliverUserMessage(config *AgentRunnerConfig) tool.DeliverUserMessageFunc {
	if config == nil {
		return nil
	}
	if config.deliverUserMessage != nil {
		return config.deliverUserMessage
	}
	return func(ctx tool.Context, params tool.UserDeliveryParams) error {
		workDir := config.Directory
		if strings.TrimSpace(ctx.Directory) != "" {
			workDir = ctx.Directory
		}
		_, err := DeliverUserMessage(config.db, config.Emitter, workDir, params)
		return err
	}
}

func WithSystemPrompt(system string) AgentRunnerOption {
	return func(c *AgentRunnerConfig) {
		c.System = system
	}
}

func WithBaseInstructions(instructions string) AgentRunnerOption {
	return func(c *AgentRunnerConfig) {
		c.System = instructions
	}
}

func WithVariant(variant string) AgentRunnerOption {
	return func(c *AgentRunnerConfig) {
		c.Variant = variant
	}
}

func WithBaseMemory(memory *types.Memory) AgentRunnerOption {
	return func(c *AgentRunnerConfig) {
		c.baseMemory = memory
	}
}

func WithProcessV2Hooks(hooks *ProcessV2Hooks) AgentRunnerOption {
	return func(c *AgentRunnerConfig) {
		c.processV2Hooks = hooks
	}
}

func WithActionSchemas(schemas []coreagent.ActionSchema) AgentRunnerOption {
	return func(c *AgentRunnerConfig) {
		c.actionSchemas = append([]coreagent.ActionSchema(nil), schemas...)
	}
}

func WithOnEmitterCreated(onEmitterCreated ...func(emitter *Emitter) error) AgentRunnerOption {
	return func(c *AgentRunnerConfig) {
		c.OnEmitterCreateds = append(c.OnEmitterCreateds, onEmitterCreated...)
	}
}

func copyToolConfig(toolConfig map[string]bool) map[string]bool {
	if toolConfig == nil {
		return nil
	}
	cloned := make(map[string]bool, len(toolConfig))
	for key, value := range toolConfig {
		cloned[key] = value
	}
	return cloned
}

func resolveSeedMemoryLibraryIDs(config *AgentRunnerConfig) []uint {
	// 会话种子只注入项目关联的记忆库（非 RAG），在 seedSessionMemoryLibraries 中解析。
	return nil
}

func ensureConfigDefaults(cfg *AgentRunnerConfig) {
	if cfg.Perms == nil {
		cfg.Perms = permission.NewManager(nil)
	}
	if cfg.Emitter == nil {
		cfg.Emitter = NewEmitter(cfg.db, cfg.SessionID)
	}
	if cfg.PluginManager == nil {
		cfg.PluginManager = plugin.NewManager()
	}
}
