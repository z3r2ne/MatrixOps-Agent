package task_runner

import (
	"errors"
	"fmt"
	"log"
	"pkgs/db/models"
	"strconv"
	"strings"
	"sync"

	coregit "matrixops.local/core_agent/git"
	database "pkgs/db"
	"pkgs/memorysearch"

	"gorm.io/gorm"

	"matrixops-agent/types"
)

var taskRuntimes = make(map[uint]*TaskRuntime)
var taskRuntimeMutex sync.RWMutex
var globalTaskWg sync.WaitGroup
var wgMap = make(map[uint]*sync.WaitGroup)

func addTaskRuntime(runtime *TaskRuntime) {
	taskRuntimeMutex.Lock()
	defer taskRuntimeMutex.Unlock()
	taskRuntimes[runtime.taskID] = runtime
}

func removeTaskRuntime(taskID uint) {
	taskRuntimeMutex.Lock()
	defer taskRuntimeMutex.Unlock()
	delete(taskRuntimes, taskID)
}

func getTaskRuntime(taskID uint) *TaskRuntime {
	taskRuntimeMutex.RLock()
	defer taskRuntimeMutex.RUnlock()
	return taskRuntimes[taskID]
}

// RunTask 启动或续跑任务；若任务已在运行且带有用户输入，则写入消息队列。
func RunTask(taskID uint, opts ...TaskRuntimeConfigOption) error {
	mergedOpts, config, err := prepareTaskRun(taskID, opts...)
	if err != nil {
		return err
	}
	if IsRunning(taskID) && hasEnqueueableUserInput(config) {
		return enqueueTaskUserMessage(config)
	}
	return launchTaskAsync(taskID, mergedOpts, config)
}

func prepareTaskRun(taskID uint, opts ...TaskRuntimeConfigOption) ([]TaskRuntimeConfigOption, *TaskRuntimeConfig, error) {
	defaultOpts := []TaskRuntimeConfigOption{
		WithTaskID(taskID),
	}

	peekConfig := NewTaskRuntimeConfig(opts...)
	db := peekConfig.db
	if db == nil {
		db = database.DB
	}

	task, err := database.GetTaskByID(db, taskID)
	if err != nil {
		return nil, nil, fmt.Errorf("获取任务失败: %w", err)
	}
	if task == nil {
		return nil, nil, fmt.Errorf("任务不存在: %d", taskID)
	}

	defaultOpts = append(defaultOpts, WithWorkDir(task.WorkDir))
	defaultOpts = append(defaultOpts, WithSessionID(task.SessionID))
	defaultOpts = append(defaultOpts, WithToWorker(task.WorkerName))
	defaultOpts = append(defaultOpts, WithWorkspaceID(fmt.Sprintf("%d", task.WorkspaceID)))
	if task.ProjectID > 0 {
		defaultOpts = append(defaultOpts, WithProjectID(models.ConvertProjectIDToString(task.ProjectID)))
	} else {
		defaultOpts = append(defaultOpts, WithProjectID(WorkspaceSessionProjectID(fmt.Sprintf("%d", task.WorkspaceID))))
	}
	defaultOpts = append(defaultOpts, WithOnEnd(func() {
		workDir := task.WorkDir
		if workDir == "" {
			return
		}
		currentBranch, err := coregit.CurrentBranch(workDir)
		if err != nil {
			return
		}
		if currentBranch != "" && currentBranch != task.Branch {
			database.UpdateTaskFields(db, task.ID, map[string]interface{}{
				"branch": currentBranch,
			})
		}
	}))
	mergedOpts := append(defaultOpts, opts...)
	config := NewTaskRuntimeConfig(mergedOpts...)
	if config.wsHub == nil {
		return nil, nil, fmt.Errorf("wsHub is nil")
	}
	if config.db == nil {
		return nil, nil, fmt.Errorf("run task db is nil")
	}
	return mergedOpts, config, nil
}

func launchTaskAsync(taskID uint, opts []TaskRuntimeConfigOption, config *TaskRuntimeConfig) error {
	db := config.db
	wsHub := config.wsHub

	existingRuntime := getTaskRuntime(taskID)
	needLaunchGuard := existingRuntime == nil
	if needLaunchGuard && !markTaskLaunching(taskID) {
		return nil
	}

	optsToRun := opts
	if existingRuntime != nil {
		optsToRun = append(opts, WithRunningRuntime(existingRuntime))
	}
	if task, err := database.GetTaskByID(db, taskID); err == nil && task != nil && task.ParentTaskID != nil && *task.ParentTaskID > 0 {
		storeSubtaskRepoBaselineForTask(db, taskID)
	}
	globalTaskWg.Add(1)
	wgMap[taskID] = &sync.WaitGroup{}
	wg := wgMap[taskID]
	wg.Add(1)
	go func() {
		if needLaunchGuard {
			defer unmarkTaskLaunching(taskID)
		}
		defer wg.Done()
		defer globalTaskWg.Done()
		runtime, err := runTask(optsToRun...)
		if err != nil {
			wsHub.BroadcastError(taskID, err.Error())
			return
		}
		_ = runtime
		if needLaunchGuard {
			_ = TryConsumeAppendQueue(taskID, WithDB(db), WithWSHub(wsHub))
			_ = TryAutoRunTaskQueue(taskID, WithDB(db), WithWSHub(wsHub))
		}
	}()
	return nil
}

// CancelTask 取消正在运行的任务
func CancelTask(taskID uint) error {
	runtime := getTaskRuntime(taskID)
	if runtime == nil {
		return fmt.Errorf("任务 %d 未在运行", taskID)
	}

	runtime.cancelCurrentRun()
	log.Printf("[TaskRunner] Stop requested for task %d", taskID)
	return nil
}

func CancelToolCall(taskID uint, callID string) error {
	runtime := getTaskRuntime(taskID)
	if runtime == nil {
		return fmt.Errorf("任务 %d 未在运行", taskID)
	}
	if err := runtime.cancelToolCall(callID); err != nil {
		return err
	}
	log.Printf("[TaskRunner] Tool cancel requested for task %d, call %s", taskID, callID)
	return nil
}

func WaitAllTasks() {
	globalTaskWg.Wait()
}

func WaitTask(taskID uint) error {
	wg := wgMap[taskID]
	if wg == nil {
		return fmt.Errorf("任务 %d 未在运行", taskID)
	}
	wg.Wait()
	return nil
}

// // WaitTask 等待任务执行完成
// func WaitTask(taskID uint) error {
// 	runtime := getTaskRuntime(taskID)
// 	if runtime == nil {
// 		return errors.New("任务未在运行")
// 	}

// 	// 创建一个通道来等待任务完成
// 	done := make(chan struct{})

// 	// 启动一个 goroutine 定期检查任务是否还在运行
// 	go func() {
// 		ticker := time.NewTicker(100 * time.Millisecond)
// 		defer ticker.Stop()

// 		for {
// 			<-ticker.C
// 			if getTaskRuntime(taskID) == nil {
// 				close(done)
// 				return
// 			}
// 		}
// 	}()

// 	// 等待任务完成
// 	<-done
// 	return nil
// }

func IsRunning(taskID uint) bool {
	runtime := getTaskRuntime(taskID)
	return runtime != nil
}

// runTask 执行任务的核心逻辑（同步函数，返回 runtime 和 error）
func runTask(opts ...TaskRuntimeConfigOption) (*TaskRuntime, error) {
	config := NewTaskRuntimeConfig(opts...)

	if config.db == nil {
		return nil, fmt.Errorf("db不能为nil")
	}

	defaultOpts := []TaskRuntimeConfigOption{}

	if config.FromWorker != "" {
		defaultOpts = append(defaultOpts, WithFromWorker("__USER__"))
	}

	opts = append(defaultOpts, opts...)

	if config.RunningRuntime != nil {
		runtime := config.RunningRuntime
		if err := runtime.startLoop(opts...); err != nil {
			runtime.Finish(-1, err)
			return runtime, nil
		}
		return runtime, nil
	} else {
		runtime, err := NewTaskRuntime(opts...)
		if err != nil {
			return nil, err
		}

		// 将 runtime 添加到运行时列表
		addTaskRuntime(runtime)
		defer removeTaskRuntime(runtime.taskID)
		// 更新为 active
		runtime.updateTaskStatus(models.TaskStatusRunning, "任务已开始执行")

		// 启动进程
		if err := runtime.startLoop(opts...); err != nil {
			runtime.Finish(-1, err)
			return runtime, nil
		}
		runtime.Finish(0, nil)
		return runtime, nil
	}
}

func taskQueueItemToInputParts(item *models.TaskMessageQueueItem) []*types.Part {
	if item == nil || len(item.Parts) == 0 {
		return nil
	}
	parts := make([]*types.Part, 0, len(item.Parts))
	for _, p := range item.Parts {
		switch strings.TrimSpace(p.Type) {
		case types.PartTypeText:
			text := strings.TrimSpace(p.Text)
			if text == "" {
				continue
			}
			parts = append(parts, &types.Part{Type: types.PartTypeText, Text: text})
		case "file":
			if strings.TrimSpace(p.Path) == "" && strings.TrimSpace(p.URL) == "" {
				continue
			}
			parts = append(parts, &types.Part{
				Type:     "file",
				Path:     strings.TrimSpace(p.Path),
				URL:      strings.TrimSpace(p.URL),
				Mime:     p.Mime,
				Filename: p.Filename,
				InputSource: strings.TrimSpace(p.InputSource),
			})
		}
	}
	if len(parts) == 0 {
		return nil
	}
	return parts
}

func (r *TaskRuntime) startLoop(opts ...TaskRuntimeConfigOption) error {
	storeSubtaskRepoBaselineForTask(r.db, r.taskID)
	if err := r.Start(opts...); err != nil {
		return err
	}

	for {
		task, err := database.GetTaskByID(r.db, r.taskID)
		if err != nil || task == nil {
			return err
		}
		if !task.MessageQueueAutoSend {
			return nil
		}

		if err := r.consumeAppendQueueHead(task); err != nil {
			return err
		}

		nextItem, err := r.dequeueNextMessageQueueItem()
		if err != nil {
			r.wsHub.BroadcastError(r.taskID, fmt.Sprintf("自动执行队列任务失败: %v", err))
			return nil
		}
		if nextItem == nil {
			return nil
		}

		nextOpts := []TaskRuntimeConfigOption{
			WithContent(nextItem.Content),
			WithWSHub(r.wsHub),
			WithDB(r.db),
		}
		if parts := taskQueueItemToInputParts(nextItem); len(parts) > 0 {
			nextOpts = append(nextOpts, WithInputParts(parts))
		}
		nextOpts = append(nextOpts, WithInputSource(QueueItemInputSource(nextItem)))
		nextOpts = append(nextOpts, WithMessageKind(QueueItemMessageKind(nextItem)))
		nextOpts = append(nextOpts, WithMessageOrigin(QueueItemMessageOrigin(nextItem)))
		if err := r.Start(nextOpts...); err != nil {
			return err
		}
	}
}

// 辅助函数
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func CreateAndRunTask(opts ...TaskRuntimeConfigOption) (*models.Task, error) {
	config := NewTaskRuntimeConfig(opts...)
	if err := assertCreateAndRunTaskConfig(config); err != nil {
		return nil, err
	}
	db := config.db
	wsHub := config.wsHub
	if wsHub == nil {
		return nil, fmt.Errorf("wsHub is nil")
	}
	if db == nil {
		return nil, fmt.Errorf("create and run task db is nil")
	}
	workspaceID := config.WorkspaceID
	parsedWorkspaceID, err := strconv.ParseUint(workspaceID, 10, 64)
	if err != nil || parsedWorkspaceID == 0 {
		return nil, errors.New("无效工作区 ID")
	}
	workspace, err := database.GetWorkspaceByID(db, uint(parsedWorkspaceID))
	if err != nil {
		return nil, errors.New("工作区不存在")
	}

	var project *models.Project
	if parsedProjectID, parseErr := strconv.ParseUint(config.ProjectID, 10, 64); parseErr == nil && parsedProjectID > 0 {
		project, err = database.GetProjectByID(db, uint(parsedProjectID))
		if err != nil {
			return nil, errors.New("项目不存在")
		}
	}

	var workDir string
	var branch string

	// worktree 模式优先：当同时指定 NewBranch 和 BaseBranch 时，优先使用 worktree
	if config.NewBranch != "" && config.BaseBranch != "" {
		shouldCreateWorktree := config.WorkDir == ""
		if !shouldCreateWorktree && project != nil {
			providedWorkDir := strings.TrimSpace(config.WorkDir)
			if providedWorkDir == strings.TrimSpace(project.Path) {
				shouldCreateWorktree = true
			}
		}
		if !shouldCreateWorktree {
			// 调用方已提供 workDir（可能已是创建好的 worktree），直接使用
			workDir = config.WorkDir
		} else {
			if project == nil {
				return nil, errors.New("未选择项目，无法创建 worktree")
			}
			worktreePath, err := createWorktreeForTask(project.Path, project.WorktreePath, config.NewBranch, config.BaseBranch)
			if err != nil {
				return nil, fmt.Errorf("创建 Worktree 失败: %w", err)
			}
			workDir = worktreePath
		}
		branch = config.NewBranch
	} else if config.WorkDir != "" {
		workDir = config.WorkDir
		branch = config.Branch
		if branch == "" {
			branch = config.BaseBranch
		}
	} else {
		return nil, errors.New("必须指定 WorkDir 或启用 Worktree 模式（NewBranch + BaseBranch）")
	}

	baseCommitHash := ""
	if config.BaseBranch != "" {
		baseCommitHash, _ = coregit.RevParse(workDir, config.BaseBranch)
	}

	task := models.Task{
		WorkspaceID:       workspace.ID,
		ProjectID:         0,
		ParentTaskID:      config.ParentTaskID,
		Name:              config.TaskName,
		Content:           config.Content,
		WorkerName:        config.ToWorker,
		Status:            "queue",
		Branch:            branch,
		BaseBranch:        config.BaseBranch,
		BaseCommitHash:    baseCommitHash,
		WorkDir:           workDir,
		PromptCacheKey:    NewPromptCacheKey(),
		ListPosition:      0,
		MemoryLibraryMode: models.NormalizeTaskMemoryLibraryMode(config.MemoryLibraryMode),
		MemoryLibraryIDs:  models.UintSlice(append([]uint(nil), config.MemoryLibraryIDs...)),
	}
	if project != nil {
		task.ProjectID = project.ID
	}

	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := database.ShiftTaskListPositionsDown(tx, workspace.ID); err != nil {
			return err
		}
		return database.CreateTask(tx, &task)
	}); err != nil {
		return nil, errors.New("创建任务失败")
	}
	if err := database.ApplyTaskMemoryLibraryAfterCreate(db, &task); err != nil {
		return &task, err
	}
	for _, libraryID := range task.MemoryLibraryIDs {
		if libraryID == 0 {
			continue
		}
		_, _ = memorysearch.EnqueueMemoryLibrarySearchIndexJob(db, libraryID)
	}
	if err := seedTaskSessionMemorySnapshot(db, task.SessionID, config.ParentMemorySnapshot); err != nil {
		return &task, fmt.Errorf("初始化子任务记忆失败: %w", err)
	}

	newOpts := []TaskRuntimeConfigOption{
		WithSessionID(task.SessionID),
		WithTaskID(task.ID),
		WithWorkDir(workDir),
	}
	taskOpts := append(opts, newOpts...)

	if err := RunTask(task.ID, taskOpts...); err != nil {
		_ = database.UpdateTaskStatus(config.db, task.ID, string(models.TaskStatusFailed))
		task.Status = string(models.TaskStatusFailed)
		return &task, err
	}

	return &task, nil
}

// createWorktreeForTask 为任务创建 worktree
func createWorktreeForTask(projectPath, projectWorktreePath, newBranch, baseBranch string) (string, error) {
	return coregit.CreateWorktree(projectPath, projectWorktreePath, newBranch, baseBranch)
}
