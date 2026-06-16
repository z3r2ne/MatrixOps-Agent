package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"pkgs/db/storage"
	"strconv"
	"strings"

	agenttypes "matrixops-agent/types"
	coregit "matrixops.local/core_agent/git"
	"matrixops/services"
	"matrixops/services/task_runner"
	database "pkgs/db"
	"pkgs/db/models"
	"pkgs/memorysearch"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"gorm.io/gorm"
)

func getCurrentGitBranch(projectPath string) (string, error) {
	return coregit.CurrentBranch(projectPath)
}

func selectTaskBranches(branch, newBranch, baseBranch, currentBranch string) (string, string, error) {
	branch = strings.TrimSpace(branch)
	newBranch = strings.TrimSpace(newBranch)
	baseBranch = strings.TrimSpace(baseBranch)
	currentBranch = strings.TrimSpace(currentBranch)

	if branch == "" {
		branch = currentBranch
	}

	if newBranch == "" {
		if branch == "" {
			return "", "", fmt.Errorf("当前分支不能为空")
		}
		return branch, branch, nil
	}

	if baseBranch == "" {
		if branch == "" || branch == newBranch {
			baseBranch = currentBranch
		} else {
			baseBranch = branch
		}
	}
	if baseBranch == "" {
		baseBranch = currentBranch
	}
	if baseBranch == "" {
		return "", "", fmt.Errorf("基础分支不能为空")
	}
	return newBranch, baseBranch, nil
}

func taskInputPartsToAgentParts(parts []models.TaskInputPart) []*agenttypes.Part {
	if len(parts) == 0 {
		return nil
	}
	out := make([]*agenttypes.Part, 0, len(parts))
	for _, part := range parts {
		switch strings.TrimSpace(part.Type) {
		case agenttypes.PartTypeText:
			text := strings.TrimSpace(part.Text)
			if text == "" {
				continue
			}
			out = append(out, &agenttypes.Part{Type: agenttypes.PartTypeText, Text: text})
		case "file":
			if strings.TrimSpace(part.Path) == "" && strings.TrimSpace(part.URL) == "" {
				continue
			}
			out = append(out, &agenttypes.Part{
				Type:        "file",
				Path:        strings.TrimSpace(part.Path),
				URL:         strings.TrimSpace(part.URL),
				Mime:        strings.TrimSpace(part.Mime),
				Filename:    strings.TrimSpace(part.Filename),
				InputSource: strings.TrimSpace(part.InputSource),
			})
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func moveTaskQueueItemToFront(queue []models.TaskMessageQueueItem, itemID string) (target *models.TaskMessageQueueItem, next []models.TaskMessageQueueItem) {
	for _, item := range queue {
		if item.ID == itemID {
			cp := item
			target = &cp
			break
		}
	}
	if target == nil {
		return nil, queue
	}

	next = make([]models.TaskMessageQueueItem, 0, len(queue))
	next = append(next, *target)
	for _, item := range queue {
		if item.ID == itemID {
			continue
		}
		next = append(next, item)
	}
	return target, next
}

// TaskHandler 任务处理器
type TaskHandler struct {
	db *gorm.DB
}

// NewTaskHandler 创建任务处理器
func NewTaskHandler(db *gorm.DB) *TaskHandler {
	return &TaskHandler{db: db}
}

var createAndRunTask = task_runner.CreateAndRunTask

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func (h *TaskHandler) resolveWorkspaceTaskProject(workspaceID uint, projectID uint) (*models.Project, error) {
	if projectID == 0 {
		return nil, nil
	}
	_ = workspaceID
	project, err := database.GetProjectByID(h.db, projectID)
	if err != nil {
		return nil, fmt.Errorf("项目不存在")
	}
	return project, nil
}

func hasTaskGitOptions(req models.TaskCreate) bool {
	return strings.TrimSpace(req.Branch) != "" ||
		strings.TrimSpace(req.NewBranch) != "" ||
		strings.TrimSpace(req.BaseBranch) != ""
}

// GetTasksByWorkspace 获取工作区下所有项目的任务
func (h *TaskHandler) GetTasksByWorkspace(c *gin.Context) {
	workspaceID := c.Param("id")

	var wid uint
	fmt.Sscanf(workspaceID, "%d", &wid)

	if _, err := database.GetWorkspaceByID(h.db, wid); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "工作区不存在"})
		return
	}

	tasks, err := database.GetTasksWithProjectByWorkspaceID(h.db, wid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取任务失败"})
		return
	}

	c.JSON(http.StatusOK, tasks)
}

// ReorderWorkspaceTasks 按 taskIds 顺序更新工作区内任务的 list_position（须包含该工作区全部任务）
func (h *TaskHandler) ReorderWorkspaceTasks(c *gin.Context) {
	workspaceID := c.Param("id")
	wid := parseUint(workspaceID)
	if wid == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效工作区 ID"})
		return
	}
	if _, err := database.GetWorkspaceByID(h.db, wid); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "工作区不存在"})
		return
	}
	var body struct {
		TaskIDs []uint `json:"taskIds"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}
	if err := database.ReorderTasksInWorkspace(h.db, wid, body.TaskIDs); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

// RunWorkspaceTask 创建并执行工作区任务
func (h *TaskHandler) RunWorkspaceTask(c *gin.Context) {
	workspaceID := c.Param("id")

	var req models.TaskCreate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	var wid uint
	fmt.Sscanf(workspaceID, "%d", &wid)

	workspace, err := database.GetWorkspaceByID(h.db, wid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "工作区不存在"})
		return
	}

	project, err := h.resolveWorkspaceTaskProject(wid, req.ProjectID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	sessionProjectID := task_runner.WorkspaceSessionProjectID(workspaceID)
	workDir := workspace.Path
	branch := ""
	baseBranch := ""
	newBranch := strings.TrimSpace(req.NewBranch)
	if project != nil {
		if newBranch == "" {
			workDir = project.Path
		} else {
			// worktree 模式由 CreateAndRunTask 创建检出目录，避免误用原项目路径。
			workDir = ""
		}
		sessionProjectID = models.ConvertProjectIDToString(project.ID)
		if coregit.IsGitRepo(project.Path) {
			currentBranch, branchErr := getCurrentGitBranch(project.Path)
			if branchErr != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "获取当前分支失败"})
				return
			}
			branch, baseBranch, err = selectTaskBranches(req.Branch, newBranch, req.BaseBranch, currentBranch)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
		} else if hasTaskGitOptions(req) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "当前项目不是 Git 项目，不能选择分支或创建 worktree"})
			return
		}
	}

	task, err := createAndRunTask(
		task_runner.WithDB(h.db),
		task_runner.WithWSHub(services.GetGlobalWSHub(h.db)),
		task_runner.WithWorkspaceID(workspaceID),
		task_runner.WithProjectID(sessionProjectID),
		task_runner.WithWorkDir(workDir),
		task_runner.WithTaskName(req.Name),
		task_runner.WithContent(req.Content),
		task_runner.WithInputParts(taskInputPartsToAgentParts(req.InputParts)),
		task_runner.WithToWorker(req.WorkerName),
		task_runner.WithBranch(branch),
		task_runner.WithBaseBranch(baseBranch),
		task_runner.WithNewBranch(newBranch),
		task_runner.WithMergeMessage(true),
		task_runner.WithMemoryLibraryMode(req.MemoryLibraryMode),
		task_runner.WithMemoryLibraryIDs(req.MemoryLibraryIDs),
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, task)
}

// CreateWorkspaceTask 创建工作区任务
func (h *TaskHandler) CreateWorkspaceTask(c *gin.Context) {
	workspaceID := c.Param("id")

	var req models.TaskCreate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	var wid uint
	fmt.Sscanf(workspaceID, "%d", &wid)

	workspace, err := database.GetWorkspaceByID(h.db, wid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "工作区不存在"})
		return
	}

	project, err := h.resolveWorkspaceTaskProject(wid, req.ProjectID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	workDir := workspace.Path
	branch := strings.TrimSpace(req.Branch)
	baseBranch := strings.TrimSpace(req.BaseBranch)
	newBranch := strings.TrimSpace(req.NewBranch)
	baseCommitHash := ""
	if project != nil {
		workDir = project.Path
		if coregit.IsGitRepo(project.Path) {
			currentBranch, branchErr := getCurrentGitBranch(project.Path)
			if branchErr != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "获取当前分支失败"})
				return
			}
			branch, baseBranch, err = selectTaskBranches(req.Branch, newBranch, req.BaseBranch, currentBranch)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			if newBranch != "" {
				worktreePath, worktreeErr := createWorktreeForTask(project.Path, project.WorktreePath, newBranch, baseBranch)
				if worktreeErr != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "创建 Worktree 失败: " + worktreeErr.Error()})
					return
				}
				workDir = worktreePath
			}
			baseCommitHash, _ = coregit.RevParse(project.Path, baseBranch)
		} else if hasTaskGitOptions(req) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "当前项目不是 Git 项目，不能选择分支或创建 worktree"})
			return
		}
	}

	task := models.Task{
		WorkspaceID:    parseUint(workspaceID),
		ProjectID:      req.ProjectID,
		ParentTaskID:   req.ParentTaskID,
		Name:           req.Name,
		Content:        req.Content,
		WorkerID:       req.WorkerID,
		WorkerName:     req.WorkerName,
		Status:         "queue",
		Branch:         branch,
		BaseBranch:     baseBranch,
		BaseCommitHash: baseCommitHash,
		WorkDir:        workDir,
		PromptCacheKey: task_runner.NewPromptCacheKey(),
		ListPosition:   0,
	}
	database.PopulateTaskCreateMemoryLibrary(&task, req)

	if err := h.db.Transaction(func(tx *gorm.DB) error {
		if err := database.ShiftTaskListPositionsDown(tx, task.WorkspaceID); err != nil {
			return err
		}
		return database.CreateTask(tx, &task)
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建任务失败"})
		return
	}
	if err := database.ApplyTaskMemoryLibraryAfterCreate(h.db, &task); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	enqueueTaskMemoryLibraryIndexJobs(h.db, &task)

	// if task.StartNow {
	// 	taskRunner.Run(task.ID)
	// }

	c.JSON(http.StatusCreated, task)
}

// createWorktreeForTask 为任务创建 worktree
func createWorktreeForTask(projectPath, projectWorktreePath, newBranch, baseBranch string) (string, error) {
	return coregit.CreateWorktree(projectPath, projectWorktreePath, newBranch, baseBranch)
}

// UpdateTask 更新任务
// GetTask 获取单个任务
func (h *TaskHandler) GetTask(c *gin.Context) {
	id := c.Param("id")

	taskID := parseUint(id)
	task, err := database.GetTaskByID(h.db, taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
		return
	}

	c.JSON(http.StatusOK, task)
}

// GetTaskPrompt 获取任务当前会话最近一次生成的 Prompt
func (h *TaskHandler) GetTaskPrompt(c *gin.Context) {
	id := c.Param("id")
	taskID := parseUint(id)
	if taskID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效任务ID"})
		return
	}

	task, err := database.GetTaskByID(h.db, taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
		return
	}

	sessionID := resolveTaskSessionID(h.db, task)
	if sessionID == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务当前没有可用会话"})
		return
	}

	requestedMessageID := strings.TrimSpace(c.Query("messageId"))
	messageID := requestedMessageID
	partID := ""
	prompt := ""
	rawResponse := ""
	if requestedMessageID != "" {
		snapshotSessionID, resolvedPrompt, resolvedRawResponse, snapshotErr := storage.GetPromptByMessageID(h.db, requestedMessageID)
		if snapshotErr != nil || snapshotSessionID != sessionID {
			c.JSON(http.StatusNotFound, gin.H{"error": "当前任务还没有生成过可展示的 Prompt"})
			return
		}
		prompt = resolvedPrompt
		rawResponse = resolvedRawResponse
	} else {
		var promptErr error
		messageID, partID, prompt, rawResponse, promptErr = storage.GetLatestPromptBySessionID(h.db, sessionID)
		if promptErr != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "当前任务还没有生成过可展示的 Prompt"})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"taskId":      taskID,
		"sessionId":   sessionID,
		"messageId":   messageID,
		"partId":      partID,
		"prompt":      prompt,
		"rawResponse": rawResponse,
	})
}

func (h *TaskHandler) UpdateTask(c *gin.Context) {
	id := c.Param("id")

	taskID := parseUint(id)
	task, err := database.GetTaskByID(h.db, taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
		return
	}

	if task.WorkspaceID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "任务未绑定工作区"})
		return
	}

	var req models.TaskUpdate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	if req.Name != nil {
		task.Name = *req.Name
	}
	if req.Memo != nil {
		task.Memo = *req.Memo
	}
	if req.WorkerID != nil {
		task.WorkerID = req.WorkerID
	}

	if err := database.UpdateTask(h.db, task); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新任务失败"})
		return
	}

	c.JSON(http.StatusOK, task)
}

// DeleteTask 删除任务
func (h *TaskHandler) DeleteTask(c *gin.Context) {
	id := c.Param("id")

	taskID := parseUint(id)
	task, err := database.GetTaskByID(h.db, taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
		return
	}

	if task.WorkspaceID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "任务未绑定工作区"})
		return
	}

	if err := database.DeleteTask(h.db, taskID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除任务失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "任务删除成功"})
}

// GetTaskQueue 获取任务的消息队列
func (h *TaskHandler) GetTaskQueue(c *gin.Context) {
	id := c.Param("id")
	taskID := parseUint(id)
	if taskID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效任务ID"})
		return
	}

	queue, autoSend, err := database.GetTaskQueueSettings(h.db, taskID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取消息队列失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"queue": queue, "autoSend": autoSend})
}

// GetTaskPlan 获取任务关联会话的执行计划
func (h *TaskHandler) GetTaskPlan(c *gin.Context) {
	id := c.Param("id")
	taskID := parseUint(id)
	if taskID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效任务ID"})
		return
	}

	sessionID, err := database.GetTaskSessionID(h.db, taskID)
	if err != nil || strings.TrimSpace(sessionID) == "" {
		c.JSON(http.StatusOK, gin.H{"plan": nil})
		return
	}

	plan, err := storage.GetPlan(h.db, sessionID)
	if err != nil {
		var notFound storage.NotFoundError
		if errors.As(err, &notFound) {
			c.JSON(http.StatusOK, gin.H{"plan": nil})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取任务计划失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"plan": plan.Content.Data})
}

// UpdateTaskQueue 更新任务的消息队列（完整替换）
func (h *TaskHandler) UpdateTaskQueue(c *gin.Context) {
	id := c.Param("id")
	taskID := parseUint(id)
	if taskID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效任务ID"})
		return
	}

	var req struct {
		Queue    []models.TaskMessageQueueItem `json:"queue"`
		AutoSend *bool                         `json:"autoSend"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	if err := database.UpdateTaskQueue(h.db, taskID, req.Queue); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新消息队列失败"})
		return
	}

	autoSend := true
	if req.AutoSend != nil {
		autoSend = *req.AutoSend
		if err := database.SetTaskMessageQueueAutoSend(h.db, taskID, autoSend); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "更新队列自动发送设置失败"})
			return
		}
	} else if settingsQueue, settingsAutoSend, settingsErr := database.GetTaskQueueSettings(h.db, taskID); settingsErr == nil {
		_ = settingsQueue
		autoSend = settingsAutoSend
	}

	hub := services.GetGlobalWSHub(h.db)
	// 广播队列变更
	hub.BroadcastTaskQueue(taskID, req.Queue)

	if autoSend {
		_ = task_runner.TryAutoRunTaskQueue(taskID,
			task_runner.WithDB(h.db),
			task_runner.WithWSHub(hub),
		)
	}

	c.JSON(http.StatusOK, gin.H{"queue": req.Queue, "autoSend": autoSend})
}

// SendNextTaskQueueItem 将队列中的某一条消息作为下一轮用户输入优先发送。
func (h *TaskHandler) SendNextTaskQueueItem(c *gin.Context) {
	id := c.Param("id")
	taskID := parseUint(id)
	if taskID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效任务ID"})
		return
	}

	itemID := c.Param("itemId")
	if itemID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效队列项ID"})
		return
	}

	task, err := database.GetTaskByID(h.db, taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
		return
	}

	targetItem, nextQueue := moveTaskQueueItemToFront(task.MessageQueue, itemID)
	if targetItem == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "队列项不存在"})
		return
	}

	hub := services.GetGlobalWSHub(h.db)

	// 对话循环进行中：标记为 supplement 并移到队首，当前循环下一轮会自动注入
	if task_runner.IsRunning(taskID) {
		targetItem.Supplement = true
		nextQueue[0] = *targetItem
		if err := database.UpdateTaskQueue(h.db, taskID, nextQueue); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "更新队列失败"})
			return
		}
		hub.BroadcastTaskQueue(taskID, nextQueue)
		c.JSON(http.StatusOK, gin.H{"message": "已加入本轮补充", "queue": nextQueue})
		return
	}

	// 非运行中：从队列移除并立即作为下一轮用户输入执行
	remainingQueue := make([]models.TaskMessageQueueItem, 0, len(nextQueue)-1)
	if len(nextQueue) > 1 {
		remainingQueue = append(remainingQueue, nextQueue[1:]...)
	}
	if err := database.UpdateTaskQueue(h.db, taskID, remainingQueue); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新队列失败"})
		return
	}
	hub.BroadcastTaskQueue(taskID, remainingQueue)

	var agentParts []*agenttypes.Part
	if len(targetItem.Parts) > 0 {
		agentParts = taskInputPartsToAgentParts(targetItem.Parts)
	}
	opts := []task_runner.TaskRuntimeConfigOption{
		task_runner.WithContent(targetItem.Content),
		task_runner.WithWSHub(hub),
		task_runner.WithDB(h.db),
	}
	if len(agentParts) > 0 {
		opts = append(opts, task_runner.WithInputParts(agentParts))
	}
	opts = append(opts, task_runner.WithInputSource(task_runner.QueueItemInputSource(targetItem)))
	opts = append(opts, task_runner.WithMessageKind(task_runner.QueueItemMessageKind(targetItem)))
	opts = append(opts, task_runner.WithMessageOrigin(task_runner.QueueItemMessageOrigin(targetItem)))
	if err := task_runner.RunTask(taskID, opts...); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("启动任务失败: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "已发送到下一轮", "queue": remainingQueue})
}

// TaskWebSocket 任务聊天与日志流
func (h *TaskHandler) TaskWebSocket(c *gin.Context) {
	id := c.Param("id")
	taskID := parseUint(id)
	if taskID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效任务ID"})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	client := &services.GlobalWSClient{Send: make(chan []byte, 256)}
	services.GetGlobalWSHub(h.db).Register(client)
	defer services.GetGlobalWSHub(h.db).Unregister(client)

	// 读取客户端消息，直到连接关闭。
	go func() {
		for {
			if err := conn.ReadJSON(new(json.RawMessage)); err != nil {
				return
			}
		}
	}()

	for message := range client.Send {
		if err := conn.WriteMessage(websocket.TextMessage, message); err != nil {
			return
		}
	}
}

// GetTaskExecutions 获取任务的所有执行记录列表
func (h *TaskHandler) GetTaskExecutions(c *gin.Context) {
	id := c.Param("id")
	taskID := parseUint(id)
	if taskID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效任务ID"})
		return
	}

	executions, err := database.GetExecutionsByTaskIDOrdered(h.db, taskID, false)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取执行记录失败"})
		return
	}

	c.JSON(http.StatusOK, executions)
}

// GetTaskExecution 获取单个执行记录详情
func (h *TaskHandler) GetTaskExecution(c *gin.Context) {
	execID := c.Param("execId")
	id := parseUint(execID)
	if id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效执行记录ID"})
		return
	}

	execution, err := database.GetExecutionByID(h.db, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "执行记录不存在"})
		return
	}

	c.JSON(http.StatusOK, execution)
}

// GetExecutionLogs 获取执行的日志列表
func (h *TaskHandler) GetExecutionLogs(c *gin.Context) {
	execID := c.Param("execId")
	id := parseUint(execID)
	if id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效执行记录ID"})
		return
	}

	logs, err := database.GetExecutionLogsByExecutionID(h.db, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取日志失败"})
		return
	}

	c.JSON(http.StatusOK, logs)
}

// RestartTask 重新启动任务（创建新的执行）
func (h *TaskHandler) RestartTask(c *gin.Context) {
	id := c.Param("id")
	taskID := parseUint(id)
	if taskID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效任务ID"})
		return
	}

	task, err := database.GetTaskByID(h.db, taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
		return
	}

	if task.WorkspaceID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "任务未绑定工作区"})
		return
	}

	// 更新任务状态（只更新 status 字段，避免覆盖 session_id）
	database.UpdateTaskStatus(h.db, taskID, "active")

	// 启动执行
	task_runner.RunTask(taskID)

	c.JSON(http.StatusOK, gin.H{"message": "任务已重新启动"})
}

// StopTask 停止任务执行
func (h *TaskHandler) StopTask(c *gin.Context) {
	id := c.Param("id")
	taskID := parseUint(id)
	if taskID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效任务ID"})
		return
	}

	task, err := database.GetTaskByID(h.db, taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
		return
	}

	// 停止任务
	if err := task_runner.CancelTask(taskID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if task.WorkspaceID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "任务未绑定工作区"})
		return
	}

	// 用户主动停止：先标为 cancelled；TaskRuntime.Finish 会再次确认并写入结束原因
	_ = database.UpdateTaskFields(h.db, taskID, map[string]interface{}{
		"status": string(models.TaskStatusCancelled),
		"error":  models.TaskCancelledByUserMessage,
	})
	_ = database.SetTaskMessageQueueAutoSend(h.db, taskID, false)
	hub := services.GetGlobalWSHub(h.db)
	if queue, queueErr := database.GetTaskQueue(h.db, taskID); queueErr == nil {
		hub.BroadcastTaskQueue(taskID, queue)
	}

	// 广播停止消息
	hub.BroadcastTaskMessage(taskID, &models.TaskMessage{Type: "status", Role: "system", Content: "任务已被用户取消"})

	c.JSON(http.StatusOK, gin.H{"message": "任务已停止"})
}

// IsTaskRunning 检查任务是否在运行
func (h *TaskHandler) IsTaskRunning(c *gin.Context) {
	id := c.Param("id")
	taskID := parseUint(id)
	if taskID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效任务ID"})
		return
	}

	running := task_runner.IsRunning(taskID)
	c.JSON(http.StatusOK, gin.H{"running": running})
}

func parseUint(raw string) uint {
	var id uint
	for _, ch := range raw {
		if ch < '0' || ch > '9' {
			return 0
		}
		id = id*10 + uint(ch-'0')
	}
	return id
}

func enqueueTaskMemoryLibraryIndexJobs(db *gorm.DB, task *models.Task) {
	if db == nil || task == nil {
		return
	}
	for _, libraryID := range task.MemoryLibraryIDs {
		if libraryID == 0 {
			continue
		}
		_, _ = memorysearch.EnqueueMemoryLibrarySearchIndexJob(db, libraryID)
	}
}

func (h *TaskHandler) GetTaskLogsV2(c *gin.Context) {
	id := c.Param("id")
	taskID := parseUint(id)
	if taskID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效任务ID"})
		return
	}

	task, err := database.GetTaskByID(h.db, taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
		return
	}

	sessionID := resolveTaskSessionID(h.db, task)

	if sessionID == "" {
		c.JSON(http.StatusOK, gin.H{
			"items":               []models.TaskMessage{},
			"hasMore":             false,
			"nextBeforeMessageId": "",
		})
		return
	}

	limit := 100
	if rawLimit := strings.TrimSpace(c.Query("limit")); rawLimit != "" {
		if parsed, parseErr := strconv.Atoi(rawLimit); parseErr == nil && parsed > 0 {
			if parsed > 200 {
				parsed = 200
			}
			limit = parsed
		}
	}
	beforeMessageID := strings.TrimSpace(c.Query("beforeMessageId"))

	messagesWithParts, hasMore, nextBeforeMessageID, err := storage.GetMessageWithPartsBySessionIDPageLight(h.db, sessionID, limit, beforeMessageID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取日志失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"items":               messagesWithParts,
		"hasMore":             hasMore,
		"nextBeforeMessageId": nextBeforeMessageID,
	})
}

func (h *TaskHandler) RetryLastUserMessage(c *gin.Context) {
	taskID := parseUint(c.Param("id"))
	if taskID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效任务ID"})
		return
	}

	var req struct {
		MessageID string `json:"messageId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	task, err := database.GetTaskByID(h.db, taskID)
	if err != nil || task == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
		return
	}

	sessionID := resolveTaskSessionID(h.db, task)
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "当前任务还没有会话"})
		return
	}

	retryResult, err := storage.RetryFromUserMessage(h.db, sessionID, req.MessageID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	wsHub := services.GetGlobalWSHub(h.db)
	if err := task_runner.RunTask(taskID,
		task_runner.WithDB(h.db),
		task_runner.WithWSHub(wsHub),
		task_runner.WithSessionID(sessionID),
		task_runner.WithContent(retryResult.Text),
		task_runner.WithInputParts(filterRetryMessageInputParts(retryResult.Parts)),
		task_runner.WithInputSource(task_runner.TaskInputSourceFrontend),
	); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "已重新发送用户消息",
		"taskId":    taskID,
		"sessionId": sessionID,
		"content":   retryResult.Text,
	})
}

func filterRetryMessageInputParts(parts []*agenttypes.Part) []*agenttypes.Part {
	if len(parts) == 0 {
		return nil
	}
	out := make([]*agenttypes.Part, 0, len(parts))
	for _, part := range parts {
		if part == nil {
			continue
		}
		out = append(out, part)
	}
	return out
}

func (h *TaskHandler) GetTaskLogs(c *gin.Context) {
	id := c.Param("id")
	taskID := parseUint(id)
	if taskID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效任务ID"})
		return
	}

	// 直接查询该任务的所有执行记录（按时间正序）
	executions, err := database.GetExecutionsByTaskIDOrdered(h.db, taskID, true)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询执行记录失败"})
		return
	}

	if len(executions) == 0 {
		// 没有执行记录，返回空数组
		c.JSON(http.StatusOK, []models.TaskMessage{})
		return
	}

	// 2. 合并所有执行的日志
	allLogs := make([]models.ExecutionLog, 0)
	for _, exec := range executions {
		logs, err := database.GetExecutionLogsByExecutionID(h.db, exec.ID)
		if err != nil {
			log.Printf("[GetSessionLogs] Failed to load logs for execution %d: %v", exec.ID, err)
			continue
		}
		allLogs = append(allLogs, logs...)
	}

	// 3. 转换为 TaskMessage 格式并去重
	messages := make([]models.TaskMessage, 0, len(allLogs))
	seenSessionID := false // 避免重复的 session_id 消息
	// seenMessages := make(map[string]bool) // 用于去重消息（基于内容和类型）

	for _, logEntry := range allLogs {
		// 跳过重复的 session_id 消息（多次执行会有多个）
		if logEntry.MsgType == string(models.LogMsgTypeSessionID) {
			if seenSessionID {
				continue
			}
			seenSessionID = true
		}

		// 跳过 finished 类型的消息
		if logEntry.MsgType == string(models.LogMsgTypeFinished) {
			continue
		}

		msg := models.TaskMessage{
			Type:      logEntry.MsgType,
			Role:      "assistant",
			Content:   logEntry.Content,
			Timestamp: logEntry.CreatedAt.UnixMilli(),
		}

		// 如果是 normalized_entry，解析 EntryJSON
		if logEntry.MsgType == string(models.LogMsgTypeNormalizedEntry) && logEntry.EntryJSON != "" {
			var entry models.NormalizedEntry
			if err := json.Unmarshal([]byte(logEntry.EntryJSON), &entry); err == nil {
				msg.Entry = &entry
				msg.Type = "normalized_entry"

				// 根据 entry_type 设置 role
				if entry.EntryType == models.EntryTypeUserMessage {
					msg.Role = "user"
				}

				// 去重逻辑：对于用户消息和系统初始化消息，根据内容和类型去重
				// if entry.EntryType == models.EntryTypeUserMessage || entry.EntryType == models.EntryTypeSystemMessage {
				// 	dedupeKey := fmt.Sprintf("%s:%s", entry.EntryType, entry.Content)
				// 	if seenMessages[dedupeKey] {
				// 		continue // 跳过重复消息
				// 	}
				// 	seenMessages[dedupeKey] = true
				// }
			}
		}

		messages = append(messages, msg)
	}

	c.JSON(http.StatusOK, messages)
}

// GetSessionLogs 根据 session_id 获取完整会话历史（包括初始执行和所有追问）
func (h *TaskHandler) GetSessionLogs(c *gin.Context) {
	sessionID := c.Param("id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的会话ID"})
		return
	}

	// 1. 查找所有使用该 session_id 的执行记录
	executions, err := database.GetExecutionsBySessionID(h.db, sessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询执行记录失败"})
		return
	}

	if len(executions) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "未找到该会话的执行记录"})
		return
	}

	// 2. 合并所有执行的日志
	allLogs := make([]models.ExecutionLog, 0)
	for _, exec := range executions {
		logs, err := database.GetExecutionLogsByExecutionID(h.db, exec.ID)
		if err != nil {
			log.Printf("[GetSessionLogs] Failed to load logs for execution %d: %v", exec.ID, err)
			continue
		}
		allLogs = append(allLogs, logs...)
	}

	// 3. 转换为 TaskMessage 格式并去重
	messages := make([]models.TaskMessage, 0, len(allLogs))
	seenSessionID := false // 避免重复的 session_id 消息
	// seenMessages := make(map[string]bool) // 用于去重消息（基于内容和类型）

	for _, logEntry := range allLogs {
		// 跳过重复的 session_id 消息（多次执行会有多个）
		if logEntry.MsgType == string(models.LogMsgTypeSessionID) {
			if seenSessionID {
				continue
			}
			seenSessionID = true
		}

		// 跳过 finished 类型的消息
		if logEntry.MsgType == string(models.LogMsgTypeFinished) {
			continue
		}

		msg := models.TaskMessage{
			Type:      logEntry.MsgType,
			Role:      "assistant",
			Content:   logEntry.Content,
			Timestamp: logEntry.CreatedAt.UnixMilli(),
		}

		// 如果是 normalized_entry，解析 EntryJSON
		if logEntry.MsgType == string(models.LogMsgTypeNormalizedEntry) && logEntry.EntryJSON != "" {
			var entry models.NormalizedEntry
			if err := json.Unmarshal([]byte(logEntry.EntryJSON), &entry); err == nil {
				msg.Entry = &entry
				msg.Type = "normalized_entry"

				// 根据 entry_type 设置 role
				if entry.EntryType == models.EntryTypeUserMessage {
					msg.Role = "user"
				}

				// 去重逻辑：对于用户消息和系统初始化消息，根据内容和类型去重
				// if entry.EntryType == models.EntryTypeUserMessage || entry.EntryType == models.EntryTypeSystemMessage {
				// 	dedupeKey := fmt.Sprintf("%s:%s", entry.EntryType, entry.Content)
				// 	if seenMessages[dedupeKey] {
				// 		continue // 跳过重复消息
				// 	}
				// 	seenMessages[dedupeKey] = true
				// }
			}
		}

		messages = append(messages, msg)
	}

	c.JSON(http.StatusOK, messages)
}
