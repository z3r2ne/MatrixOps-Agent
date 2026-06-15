package tool

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
)

func NewDefaultRegistry(db *gorm.DB, setVar func(key string, value any)) *Registry {
	registry := NewRegistry()
	registry.Register(GlobTool{})
	registry.Register(GrepTool{})
	registry.Register(RGTool{})
	registry.Register(HTTPHeadTool{})
	registry.Register(HTTPDeleteTool{})
	registry.Register(FetchURLTool{})
	registry.Register(ListTool{})
	registry.Register(TreeTool{})
	registry.Register(BashTool{})
	registry.Register(LoadSkillTool{db: db})
	registry.Register(DiffTool{db: db})
	registry.Register(GetPlanTool{db: db})
	registry.Register(WritePlanTool{db: db})
	registry.Register(UpdatePlanTool{db: db})
	// registry.Register(NewAnswerTool(setVar))
	return registry
}

func NewDefaultRegistryWithQuestion(opts *DefaultRegistryOptions) *Registry {
	if opts == nil {
		opts = &DefaultRegistryOptions{}
	}

	appendFileRecord := normalizeAppendFileRecord(opts.AppendFileRecord)

	registry := NewRegistry()
	registry.Register(NewReadTool(appendFileRecord))
	// registry.Register(NewReadWholeTool(appendFileRecord))
	registry.Register(NewDeleteTool(appendFileRecord))
	registry.Register(NewWriteTool(appendFileRecord))
	registry.Register(NewEditTool(appendFileRecord))
	registry.Register(NewPatchTool(appendFileRecord))
	registry.Register(GlobTool{})
	registry.Register(GrepTool{})
	registry.Register(RGTool{})
	registry.Register(HTTPHeadTool{})
	registry.Register(HTTPDeleteTool{})
	registry.Register(FetchURLTool{})
	registry.Register(ListTool{})
	registry.Register(TreeTool{})
	registry.Register(BashTool{})
	registry.Register(LoadSkillTool{db: opts.DB})
	registry.Register(DiffTool{db: opts.DB})
	registry.Register(GetPlanTool{db: opts.DB})
	registry.Register(WritePlanTool{db: opts.DB})
	registry.Register(UpdatePlanTool{db: opts.DB})
	if opts.RunWorkerTask != nil {
		registry.Register(NewRunWorkerTaskTool(opts.RunWorkerTask, opts.AvailableWorkerNames))
	}
	registry.Register(NewRemindTool(opts.DB, opts.TaskID))
	registry.Register(NewGetTaskContentTool(opts.DB, opts.TaskID))
	registry.Register(SetToolStallTimeoutTool{})
	registry.Register(NewMessageTool(func(ctx Context, params UserDeliveryParams) error {
		if opts.DeliverUserMessage == nil {
			return fmt.Errorf("message: 消息投递不可用")
		}
		return opts.DeliverUserMessage(ctx, params)
	}))
	// registry.Register(NewAnswerTool(opts.SetVar))
	registry.Register(NewQuestionTool(opts.WaitUserInput))
	if opts.StopWorkerTask != nil {
		registry.Register(NewStopWorkerTaskTool(opts.StopWorkerTask))
	}
	if opts.GetWorkerTaskProgress != nil {
		registry.Register(NewGetWorkerTaskProgressTool(opts.GetWorkerTaskProgress))
	}
	if opts.SendMessageToWorker != nil {
		registry.Register(NewSendMessageToWorkerTool(opts.SendMessageToWorker, opts.AvailableWorkerNames))
	}
	return registry
}

// registerCatalogRunWorkerTask 为工具目录/权限配置 UI 注册 run_worker_task 元数据。
// 使用 stub runner：仅用于展示与权限项，不会在 catalog 构建时执行子任务。
func registerCatalogRunWorkerTask(registry *Registry) {
	if registry == nil {
		return
	}
	if _, err := registry.Get("run_worker_task"); err == nil {
		return
	}
	registry.Register(NewRunWorkerTaskTool(
		func(Context, RunWorkerTaskRequest, func(RunWorkerTaskProgress)) (RunWorkerTaskResult, error) {
			return RunWorkerTaskResult{}, errors.New("run_worker_task: unavailable outside task runtime")
		},
		nil,
	))
}

type DefaultRegistryOptions struct {
	DB                    *gorm.DB
	TaskID                uint
	SetVar                func(key string, value any)
	AppendFileRecord      func(*FileOpRecord)
	WaitUserInput         func(map[string]interface{}) (map[string]any, error)
	SessionID             string
	RunWorkerTask         RunWorkerTaskFunc
	AvailableWorkerNames  func() []string
	DeliverUserMessage    DeliverUserMessageFunc
	StopWorkerTask        StopWorkerTaskFunc
	GetWorkerTaskProgress GetWorkerTaskProgressFunc
	SendMessageToWorker   RunWorkerTaskFunc
}

func normalizeAppendFileRecord(appendFileRecord func(*FileOpRecord)) func(*FileOpRecord) {
	if appendFileRecord != nil {
		return appendFileRecord
	}
	return func(*FileOpRecord) {}
}

func recordFileOp(appendFileRecord func(*FileOpRecord), record *FileOpRecord) {
	if appendFileRecord == nil {
		return
	}
	appendFileRecord(record)
}

type FileOpRecord struct {
	Path    string
	Offset  int
	Limit   int
	IsWhole bool
	Old     string
	New     string
	Action  FileOpRecordAction
	Content string
	Patch   string
}

type FileOpRecordAction string

const (
	FileOpRecordActionRead   FileOpRecordAction = "read"
	FileOpRecordActionDelete FileOpRecordAction = "delete"
	FileOpRecordActionWrite  FileOpRecordAction = "write"
	FileOpRecordActionEdit   FileOpRecordAction = "edit"
	FileOpRecordActionPatch  FileOpRecordAction = "patch"
	FileOpRecordActionGlob   FileOpRecordAction = "glob"
	FileOpRecordActionGrep   FileOpRecordAction = "grep"
	FileOpRecordActionRG     FileOpRecordAction = "rg"
	FileOpRecordActionList   FileOpRecordAction = "list"
)
