package tool

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type RunWorkerTaskRequest struct {
	WorkerName            string
	Content               string
	TaskName              string
	TaskID                uint // 非 0 时向已有子任务继续发送消息，而非创建新任务
	ParentTaskID          *uint
	MergeMessage          bool
	SkipCreateUserMessage bool
	SessionWindow         interface{}
}

type RunWorkerTaskProgress struct {
	TaskID       uint
	SessionID    string
	ParentTaskID *uint
	WorkerName   string
	TaskName     string
	Content      string
	Status       string
	Answer       string
}

type RunWorkerTaskResult struct {
	TaskID        uint
	SessionID     string
	ParentTaskID  *uint
	WorkerName    string
	TaskName      string
	Content       string
	Status        string
	Answer        string
	Summary       string
	Error         string
	DurationMs    int64
	WorkDir       string
	Branch        string
	BaseBranch    string
	ModifiedFiles []string
	CreatedFiles  []string
}

type RunWorkerTaskFunc func(ctx Context, req RunWorkerTaskRequest, onProgress func(RunWorkerTaskProgress)) (RunWorkerTaskResult, error)

type RunWorkerTaskTool struct {
	run                RunWorkerTaskFunc
	availableWorkersFn func() []string
}

func NewRunWorkerTaskTool(run RunWorkerTaskFunc, availableWorkersFn func() []string) *RunWorkerTaskTool {
	return &RunWorkerTaskTool{
		run:                run,
		availableWorkersFn: availableWorkersFn,
	}
}

func (RunWorkerTaskTool) Name() string { return "run_worker_task" }

func (RunWorkerTaskTool) VerbosName() string { return "执行子任务" }

func (RunWorkerTaskTool) Description() string {
	return "调用指定 worker 创建并执行一个真实子任务；适合把广泛只读探索交给 `explore`、把仓库级代码地图沉淀交给 `code_map`、把前端/UI 问题交给 `frontend_engineer`、把方案设计交给 `plan`、把独立验收交给 `verification`。不传 `task_id` 时创建新子任务；传入上次返回的 `task_id`（metadata 中的 subtaskTaskId）则在同一子任务 session 上续聊，子 worker 可看到此前对话。续聊适用于：① 子任务已结束但还需补充细节、路径、结论或局部核实；② 子任务异常结束（failed/cancelled）需追问、纠错或换思路继续；③ 分多轮逐步收窄范围。换 worker 或全新课题时不要传 `task_id`。完成后返回总结（含状态、结束原因、本子任务期间的文件变更等）。"
}

func (t RunWorkerTaskTool) Schema() map[string]interface{} {
	workerField := map[string]interface{}{
		"type":        "string",
		"description": "要执行子任务的 worker 名称。广泛只读代码探索优先 `explore`；系统生成仓库代码地图优先 `code_map`；前端/UI问题优先 `frontend_engineer`；方案设计优先 `plan`；交付前独立验收优先 `verification`。",
	}
	if names := t.availableWorkers(); len(names) > 0 {
		workerField["enum"] = names
	}
	return ObjectParamSchema(map[string]interface{}{
		"worker": workerField,
		"content": map[string]interface{}{
			"type": "string",
			"description": "发给子 worker 的本轮指令。新建子任务时写清目标、背景、已知线索、排除项和期望输出；续聊（传 task_id）时写清本轮要补充或纠正什么，可引用上次总结中的缺口，无需重复全文。示例：让 `explore` 梳理相关模块与关键文件；续聊时写“上次未给出 Diff 搜索组件路径，请只读补充具体文件与组件名”。",
		},
		"name": map[string]interface{}{
			"type":        "string",
			"description": "可选。仅创建新子任务时使用，简短标题；与 task_id 互斥，续聊时不要传。",
		},
		"task_id": map[string]interface{}{
			"type": "integer",
			"description": "可选。已有子任务 ID（取上次工具结果 metadata 的 subtaskTaskId）。指定时在同一子任务、同一 worker、同一 session 上续聊：子 worker 保留此前对话与记忆。适用于子任务正常结束但信息不够、或 failed/cancelled 等异常结束后需继续追问/纠错/补查；worker 必须与该子任务一致。不传则创建新子任务。",
		},
	}, []string{"worker", "content"})
}

func (t *RunWorkerTaskTool) Execute(ctx Context, input map[string]interface{}) (Result, error) {
	if t == nil || t.run == nil {
		return Result{IsError: true, Name: "run_worker_task"}, errors.New("run_worker_task: missing runner")
	}

	workerName, _ := input["worker"].(string)
	content, _ := input["content"].(string)
	taskName, _ := input["name"].(string)
	taskID, taskIDErr := parseOptionalUintID(input["task_id"])
	if taskIDErr != nil {
		taskID, taskIDErr = parseOptionalUintID(input["taskId"])
	}

	workerName = strings.TrimSpace(workerName)
	content = strings.TrimSpace(content)
	taskName = strings.TrimSpace(taskName)

	if taskIDErr != nil {
		return Result{IsError: true, Name: "run_worker_task"}, fmt.Errorf("run_worker_task: invalid task_id: %w", taskIDErr)
	}
	if workerName == "" {
		return Result{IsError: true, Name: "run_worker_task"}, errors.New("run_worker_task: missing worker")
	}
	if content == "" {
		return Result{IsError: true, Name: "run_worker_task"}, errors.New("run_worker_task: missing content")
	}
	if taskID > 0 && strings.TrimSpace(taskName) != "" {
		return Result{IsError: true, Name: "run_worker_task"}, errors.New("run_worker_task: task_id cannot be used together with name")
	}
	if allowed := t.availableWorkers(); len(allowed) > 0 && !containsString(allowed, workerName) {
		return Result{IsError: true, Name: "run_worker_task"}, fmt.Errorf("run_worker_task: worker %q not allowed; allowed workers: %s", workerName, strings.Join(allowed, ", "))
	}

	progressReporter := func(progress RunWorkerTaskProgress) {
		metadata := map[string]interface{}{
			"subtaskTaskId":       progress.TaskID,
			"subtaskSessionId":    progress.SessionID,
			"subtaskParentTaskId": progress.ParentTaskID,
			"subtaskWorkerName":   progress.WorkerName,
			"subtaskTaskName":     progress.TaskName,
			"subtaskContent":      progress.Content,
			"subtaskStatus":       progress.Status,
			"subtaskAnswer":       progress.Answer,
		}
		ctx.EmitEvent(StreamEvent{
			Status:   "running",
			Title:    fmt.Sprintf("调用子任务 · %s", progress.WorkerName),
			Metadata: metadata,
		})
	}

	result, err := t.run(ctx, RunWorkerTaskRequest{
		WorkerName: workerName,
		Content:    content,
		TaskName:   taskName,
		TaskID:     taskID,
	}, progressReporter)

	metadata := map[string]interface{}{
		"subtaskTaskId":        result.TaskID,
		"subtaskSessionId":     result.SessionID,
		"subtaskParentTaskId":  result.ParentTaskID,
		"subtaskWorkerName":    result.WorkerName,
		"subtaskTaskName":      result.TaskName,
		"subtaskContent":       result.Content,
		"subtaskStatus":        result.Status,
		"subtaskAnswer":        result.Answer,
		"subtaskSummary":       result.Summary,
		"subtaskError":         result.Error,
		"subtaskDurationMs":    result.DurationMs,
		"subtaskWorkDir":       result.WorkDir,
		"subtaskBranch":        result.Branch,
		"subtaskBaseBranch":    result.BaseBranch,
		"subtaskModifiedFiles": result.ModifiedFiles,
		"subtaskCreatedFiles":  result.CreatedFiles,
	}
	contentSummary := strings.TrimSpace(result.Summary)
	if contentSummary == "" {
		contentSummary = strings.TrimSpace(result.Answer)
	}
	toolResult := Result{
		Name:     "run_worker_task",
		Content:  contentSummary,
		Metadata: metadata,
	}
	if contentSummary != "" {
		toolResult.FullContent = contentSummary
	}
	if err != nil {
		toolResult.IsError = true
		return toolResult, err
	}
	if strings.TrimSpace(result.Status) == "cancelled" {
		toolResult.Metadata["subtaskCancelled"] = true
	}
	return toolResult, nil
}

func (t *RunWorkerTaskTool) availableWorkers() []string {
	if t == nil || t.availableWorkersFn == nil {
		return nil
	}
	names := t.availableWorkersFn()
	if len(names) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(names))
	out := make([]string, 0, len(names))
	for _, name := range names {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	sort.Strings(out)
	return out
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func parseOptionalUintID(value interface{}) (uint, error) {
	if value == nil {
		return 0, nil
	}
	switch typed := value.(type) {
	case uint:
		return typed, nil
	case uint64:
		if typed > uint64(^uint(0)) {
			return 0, fmt.Errorf("value %d out of range", typed)
		}
		return uint(typed), nil
	case int:
		if typed < 0 {
			return 0, fmt.Errorf("negative value %d", typed)
		}
		return uint(typed), nil
	case int64:
		if typed < 0 {
			return 0, fmt.Errorf("negative value %d", typed)
		}
		return uint(typed), nil
	case float64:
		if typed < 0 || typed != float64(uint(typed)) {
			return 0, fmt.Errorf("invalid value %v", typed)
		}
		return uint(typed), nil
	case json.Number:
		parsed, err := typed.Int64()
		if err != nil {
			return 0, err
		}
		if parsed < 0 {
			return 0, fmt.Errorf("negative value %d", parsed)
		}
		return uint(parsed), nil
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return 0, nil
		}
		parsed, err := strconv.ParseUint(trimmed, 10, 64)
		if err != nil {
			return 0, err
		}
		return uint(parsed), nil
	default:
		return 0, fmt.Errorf("unsupported type %T", value)
	}
}
