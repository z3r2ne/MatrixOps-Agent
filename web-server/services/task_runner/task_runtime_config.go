package task_runner

import (
	"context"

	"matrixops-agent/llm"
	agentsession "matrixops-agent/session"
	"matrixops-agent/types"

	database "pkgs/db"

	"gorm.io/gorm"
)

type TaskRuntimeConfig struct {
	wsHub WSHub
	db    *gorm.DB
	// Task-level (immutable for a runtime)
	TaskID               uint
	WorkspaceID          string
	ProjectID            string
	ParentTaskID         *uint
	TaskName             string
	SessionID            string
	Branch               string
	NewBranch            string
	BaseBranch           string
	ParentMemorySnapshot *types.Memory
	MemoryLibraryMode string
	MemoryLibraryIDs  []uint
	WorkDir              string
	Emitter              *Emitter
	WorkerConfig         map[string]interface{}

	// Run-level (can change per Start)
	Content string
	// InputSource 本轮用户输入来源：wechat / frontend；决定是否转发 assistant 回复到微信。
	InputSource string
	// QueueItemID 入队时使用的队列项 ID；为空则自动生成。
	QueueItemID string
	// InputParts 与 Content 一并传入 agent（如用户上传的图片/文件，type=file）
	InputParts []*types.Part
	// MessageKind / MessageOrigin 写入会话消息元数据（system 类队列项）
	MessageKind   string
	MessageOrigin string
	FromWorker            string
	ToWorker              string
	Ctx                   context.Context
	OnEnd                 func()
	SkipCreateUserMessage bool
	MergeMessage          bool
	SessionWindow         *agentsession.SessionWindow
	// client
	LLMClient llm.ChatClient

	OnEmitterCreateds []func(emitter *agentsession.Emitter) error

	RunningRuntime *TaskRuntime
}

type TaskRuntimeLLMConfig struct {
	Provider string
	APIKey   string
	Model    string
	BaseURL  string
	Proxy    string
}

type TaskRuntimeConfigOption func(*TaskRuntimeConfig)

func NewTaskRuntimeConfig(options ...TaskRuntimeConfigOption) *TaskRuntimeConfig {
	config := &TaskRuntimeConfig{}
	for _, option := range options {
		option(config)
	}
	if config.db == nil {
		config.db = database.DB
	}
	return config
}

func (c *TaskRuntimeConfig) clone() *TaskRuntimeConfig {
	if c == nil {
		return &TaskRuntimeConfig{}
	}
	cloned := *c
	return &cloned
}

func (c *TaskRuntimeConfig) applyOptions(options ...TaskRuntimeConfigOption) *TaskRuntimeConfig {
	clone := c.clone()
	for _, option := range options {
		option(clone)
	}
	return clone
}

func WithEmitter(emitter *Emitter) TaskRuntimeConfigOption {
	return func(c *TaskRuntimeConfig) {
		c.Emitter = emitter
	}
}

func WithSessionID(sessionID string) TaskRuntimeConfigOption {
	return func(c *TaskRuntimeConfig) {
		c.SessionID = sessionID
	}
}

func WithTaskID(taskID uint) TaskRuntimeConfigOption {
	return func(c *TaskRuntimeConfig) {
		c.TaskID = taskID
	}
}

func WithProjectID(projectID string) TaskRuntimeConfigOption {
	return func(c *TaskRuntimeConfig) {
		c.ProjectID = projectID
	}
}

func WithWorkspaceID(workspaceID string) TaskRuntimeConfigOption {
	return func(c *TaskRuntimeConfig) {
		c.WorkspaceID = workspaceID
	}
}

func WithParentTaskID(parentTaskID *uint) TaskRuntimeConfigOption {
	return func(c *TaskRuntimeConfig) {
		c.ParentTaskID = parentTaskID
	}
}

func WithTaskName(taskName string) TaskRuntimeConfigOption {
	return func(c *TaskRuntimeConfig) {
		c.TaskName = taskName
	}
}

func WithNewBranch(newBranch string) TaskRuntimeConfigOption {
	return func(c *TaskRuntimeConfig) {
		c.NewBranch = newBranch
	}
}

func WithBranch(branch string) TaskRuntimeConfigOption {
	return func(c *TaskRuntimeConfig) {
		c.Branch = branch
	}
}

func WithBaseBranch(baseBranch string) TaskRuntimeConfigOption {
	return func(c *TaskRuntimeConfig) {
		c.BaseBranch = baseBranch
	}
}

func WithParentMemorySnapshot(memory *types.Memory) TaskRuntimeConfigOption {
	return func(c *TaskRuntimeConfig) {
		c.ParentMemorySnapshot = memory
	}
}

func WithMemoryLibraryMode(mode string) TaskRuntimeConfigOption {
	return func(c *TaskRuntimeConfig) {
		c.MemoryLibraryMode = mode
	}
}

func WithMemoryLibraryIDs(ids []uint) TaskRuntimeConfigOption {
	return func(c *TaskRuntimeConfig) {
		c.MemoryLibraryIDs = append([]uint(nil), ids...)
	}
}

func WithWorkDir(path string) TaskRuntimeConfigOption {
	return func(c *TaskRuntimeConfig) {
		c.WorkDir = path
	}
}

func WithOnEnd(onEnd func()) TaskRuntimeConfigOption {
	return func(c *TaskRuntimeConfig) {
		c.OnEnd = onEnd
	}
}

func WithContent(content string) TaskRuntimeConfigOption {
	return func(c *TaskRuntimeConfig) {
		c.Content = content
	}
}

func WithQueueItemID(itemID string) TaskRuntimeConfigOption {
	return func(c *TaskRuntimeConfig) {
		c.QueueItemID = itemID
	}
}

func WithInputSource(source string) TaskRuntimeConfigOption {
	return func(c *TaskRuntimeConfig) {
		c.InputSource = source
	}
}

func WithMessageKind(kind string) TaskRuntimeConfigOption {
	return func(c *TaskRuntimeConfig) {
		c.MessageKind = kind
	}
}

func WithMessageOrigin(origin string) TaskRuntimeConfigOption {
	return func(c *TaskRuntimeConfig) {
		c.MessageOrigin = origin
	}
}

// WithInputParts 附加用户消息中的文件/图片等多段输入（与 WithContent 可同时使用）
func WithInputParts(parts []*types.Part) TaskRuntimeConfigOption {
	return func(c *TaskRuntimeConfig) {
		if len(parts) == 0 {
			return
		}
		c.InputParts = append(c.InputParts, parts...)
	}
}

func WithMergeMessage(mergeMessage bool) TaskRuntimeConfigOption {
	return func(c *TaskRuntimeConfig) {
		c.MergeMessage = mergeMessage
	}
}

func WithSkipCreateUserMessage(skipCreateUserMessage bool) TaskRuntimeConfigOption {
	return func(c *TaskRuntimeConfig) {
		c.SkipCreateUserMessage = skipCreateUserMessage
	}
}

func WithSessionWindow(sessionWindow *agentsession.SessionWindow) TaskRuntimeConfigOption {
	return func(c *TaskRuntimeConfig) {
		c.SessionWindow = sessionWindow
	}
}

func WithOnEmitterCreated(onEmitterCreated ...func(emitter *agentsession.Emitter) error) TaskRuntimeConfigOption {
	return func(c *TaskRuntimeConfig) {
		c.OnEmitterCreateds = append(c.OnEmitterCreateds, onEmitterCreated...)
	}
}

func WithFromWorker(fromWorker string) TaskRuntimeConfigOption {
	return func(c *TaskRuntimeConfig) {
		c.FromWorker = fromWorker
	}
}

func WithToWorker(toWorker string) TaskRuntimeConfigOption {
	return func(c *TaskRuntimeConfig) {
		c.ToWorker = toWorker
	}
}

func WithCtx(ctx context.Context) TaskRuntimeConfigOption {
	return func(c *TaskRuntimeConfig) {
		c.Ctx = ctx
	}
}

func WithLLMClient(client llm.ChatClient) TaskRuntimeConfigOption {
	return func(c *TaskRuntimeConfig) {
		c.LLMClient = client
	}
}

func WithDB(db *gorm.DB) TaskRuntimeConfigOption {
	return func(c *TaskRuntimeConfig) {
		c.db = db
	}
}

func WithWSHub(wsHub WSHub) TaskRuntimeConfigOption {
	return func(c *TaskRuntimeConfig) {
		c.wsHub = wsHub
	}
}

func WithWorkerConfig(cfg map[string]interface{}) TaskRuntimeConfigOption {
	return func(c *TaskRuntimeConfig) {
		c.WorkerConfig = cfg
	}
}

func WithRunningRuntime(runningRuntime *TaskRuntime) TaskRuntimeConfigOption {
	return func(c *TaskRuntimeConfig) {
		c.RunningRuntime = runningRuntime
	}
}
