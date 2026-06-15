package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"

	agentllm "matrixops-agent/llm"
	agenttypes "matrixops-agent/types"
	apppkg "matrixops/pkg/app"
	taskr "matrixops/services/task_runner"
	wstypes "matrixops/types"
	database "pkgs/db"
	"pkgs/db/models"
	"pkgs/db/storage"

	"github.com/spf13/cobra"
	"gorm.io/gorm"
)

type cliChatRunOptions struct {
	SessionID  string
	WorkerName string
	ProjectID  string
	WorkDir    string
	Prompt     string
	LLMClient  agentllm.ChatClient
	Debug      bool
}

type cliChatHub struct {
	out   io.Writer
	err   io.Writer
	in    *bufio.Reader
	debug bool

	mu                   sync.Mutex
	renderedByMessageID  map[string]string
	lastAssistantMessage string
	printedAnyText       bool
	sessionID            string
	sessionIDAnnounced   bool
	lastError            string
	lastStatus           models.TaskStatus
	lastStatusMessage    string
	lastSessionTitle     string
	toolStateByPartID    map[string]string
	footerStateByMsgID   map[string]string
	subtaskStateByPartID map[string]string
	taskHeaderPrinted    bool
}

func newChatCommand() *cobra.Command {
	var (
		sessionID string
		worker    string
		projectID string
		workDir   string
	)

	cmd := &cobra.Command{
		Use:   "chat [flags] <message>",
		Short: "通过 CLI 向 AI 会话发送一句话",
		Long: `通过 CLI 向当前项目中的 AI 会话发送一句话，并将 AI 输出实时打印到终端。
如果不传 session id，会自动创建一个新会话；后续使用同一个 session id 即可继续对话。`,
		Example: `  # 新建会话并输出 AI 回复
  matrixops chat "你好，帮我看看这个项目"

  # 继续同一个会话
  matrixops chat --session-id session_123 "继续上一步"

  # 指定 worker
  matrixops chat --worker chat "帮我总结当前目录结构"`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			prompt := strings.TrimSpace(strings.Join(args, " "))
			if prompt == "" {
				return fmt.Errorf("消息不能为空")
			}

			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			log.SetOutput(io.Discard)

			app, err := apppkg.NewAppWithOptions(apppkg.AppOptions{
				Quiet:     true,
				LogWriter: cmd.ErrOrStderr(),
			})
			if err != nil {
				return err
			}
			defer cleanupCLIApp(app)

			hub := newCLIChatHub(cmd.OutOrStdout(), cmd.ErrOrStderr(), cmd.InOrStdin(), verbose)
			sessionID, err = executeCLIChatTurn(ctx, app.DB, hub, cliChatRunOptions{
				SessionID:  sessionID,
				WorkerName: worker,
				ProjectID:  projectID,
				WorkDir:    workDir,
				Prompt:     prompt,
				Debug:      verbose,
			})
			hub.Finish()
			if err != nil {
				return err
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&sessionID, "session-id", "s", "", "已有会话 ID；为空时自动创建新会话")
	cmd.Flags().StringVarP(&worker, "worker", "w", "", "使用的 worker（默认沿用会话最近一次 worker，或 chat）")
	cmd.Flags().StringVar(&projectID, "project-id", "", "项目 ID；仅在新建会话时需要，默认根据当前目录自动匹配")
	cmd.Flags().StringVar(&workDir, "workdir", "", "工作目录；默认当前目录")

	return cmd
}

func executeCLIChatTurn(ctx context.Context, db *gorm.DB, hub *cliChatHub, opts cliChatRunOptions) (string, error) {
	if db == nil {
		return "", fmt.Errorf("db is nil")
	}
	if hub == nil {
		return "", fmt.Errorf("cli hub is nil")
	}

	prompt := strings.TrimSpace(opts.Prompt)
	if prompt == "" {
		return "", fmt.Errorf("消息不能为空")
	}

	task, err := resolveCLIChatTask(db, opts)
	if err != nil {
		return "", err
	}

	if opts.Debug {
		project, _ := database.GetProjectByID(db, task.ProjectID)
		projectName := ""
		if project != nil {
			projectName = strings.TrimSpace(project.Name)
		}
		hub.PrintTaskHeader(projectName, task.WorkDir, task.WorkerName, strings.TrimSpace(task.SessionID))
	}

	hub.SetSessionID(task.SessionID)

	runOpts := []taskr.TaskRuntimeConfigOption{
		taskr.WithContent(prompt),
		taskr.WithWSHub(hub),
		taskr.WithDB(db),
		taskr.WithCtx(ctx),
	}
	if opts.LLMClient != nil {
		runOpts = append(runOpts, taskr.WithLLMClient(opts.LLMClient))
	}

	if err := taskr.RunTask(task.ID, runOpts...); err != nil {
		return "", err
	}
	if err := taskr.WaitTask(task.ID); err != nil {
		return "", err
	}

	refreshedTask, err := database.GetTaskByID(db, task.ID)
	if err != nil {
		return strings.TrimSpace(task.SessionID), err
	}

	sessionID := strings.TrimSpace(refreshedTask.SessionID)
	if sessionID == "" {
		sessionID = strings.TrimSpace(hub.SessionID())
	}
	if sessionID == "" {
		sessionID = strings.TrimSpace(task.SessionID)
	}

	if status := models.TaskStatus(strings.TrimSpace(refreshedTask.Status)); status == models.TaskStatusFailed || status == models.TaskStatusCancelled {
		if msg := strings.TrimSpace(refreshedTask.Error); msg != "" {
			return sessionID, errors.New(msg)
		}
		if msg := strings.TrimSpace(hub.LastError()); msg != "" {
			return sessionID, errors.New(msg)
		}
		if status == models.TaskStatusCancelled {
			return sessionID, fmt.Errorf("任务已被用户取消")
		}
		return sessionID, fmt.Errorf("任务执行失败")
	}

	return sessionID, nil
}

func resolveCLIChatTask(db *gorm.DB, opts cliChatRunOptions) (*models.Task, error) {
	if strings.TrimSpace(opts.SessionID) != "" {
		return resolveExistingCLIChatTask(db, opts)
	}
	return createNewCLIChatTask(db, opts)
}

func resolveExistingCLIChatTask(db *gorm.DB, opts cliChatRunOptions) (*models.Task, error) {
	sessionID := strings.TrimSpace(opts.SessionID)
	sessionInfo, err := storage.GetSession(db, sessionID)
	if err != nil {
		return nil, fmt.Errorf("会话不存在: %w", err)
	}

	task, err := findLatestTaskBySessionID(db, sessionID)
	switch {
	case err == nil:
		workerName := strings.TrimSpace(opts.WorkerName)
		if workerName == "" {
			workerName = strings.TrimSpace(task.WorkerName)
		}
		if workerName == "" {
			workerName, _ = latestSessionWorkerName(db, sessionID)
		}
		if workerName == "" {
			workerName = "chat"
		}

		changed := false
		if strings.TrimSpace(task.WorkerName) != workerName {
			task.WorkerName = workerName
			changed = true
		}
		if strings.TrimSpace(task.WorkDir) == "" && strings.TrimSpace(sessionInfo.Directory) != "" {
			task.WorkDir = strings.TrimSpace(sessionInfo.Directory)
			changed = true
		}
		if changed {
			if err := database.UpdateTask(db, task); err != nil {
				return nil, err
			}
		}
		return task, nil
	case !errors.Is(err, gorm.ErrRecordNotFound):
		return nil, err
	}

	project, err := projectForSession(db, sessionInfo)
	if err != nil {
		return nil, err
	}

	workerName := strings.TrimSpace(opts.WorkerName)
	if workerName == "" {
		workerName, _ = latestSessionWorkerName(db, sessionID)
	}
	if workerName == "" {
		workerName = "chat"
	}

	workDir := strings.TrimSpace(sessionInfo.Directory)
	if workDir == "" {
		workDir = preferredProjectPath(project)
	}

	return createCLIChatTaskRecord(db, project, workDir, workerName, sessionID, opts.Prompt)
}

func createNewCLIChatTask(db *gorm.DB, opts cliChatRunOptions) (*models.Task, error) {
	workDir, err := resolveCLIWorkDir(opts.WorkDir)
	if err != nil {
		return nil, err
	}

	project, err := resolveProjectForCLI(db, strings.TrimSpace(opts.ProjectID), workDir)
	if err != nil {
		return nil, err
	}

	workerName := strings.TrimSpace(opts.WorkerName)
	if workerName == "" {
		workerName = "chat"
	}

	return createCLIChatTaskRecord(db, project, workDir, workerName, "", opts.Prompt)
}

func createCLIChatTaskRecord(db *gorm.DB, project *models.Project, workDir, workerName, sessionID, prompt string) (*models.Task, error) {
	if project == nil {
		return nil, fmt.Errorf("项目不存在")
	}

	task := &models.Task{
		ProjectID:    project.ID,
		Name:         buildCLIChatTaskName(sessionID),
		Content:      strings.TrimSpace(prompt),
		WorkerName:   strings.TrimSpace(workerName),
		Status:       string(models.TaskStatusQueue),
		WorkDir:      strings.TrimSpace(workDir),
		SessionID:    strings.TrimSpace(sessionID),
		ListPosition: 0,
	}
	if task.WorkDir == "" {
		task.WorkDir = preferredProjectPath(project)
	}
	if task.WorkerName == "" {
		task.WorkerName = "chat"
	}

	if err := database.CreateTask(db, task); err != nil {
		return nil, err
	}
	return task, nil
}

func resolveCLIWorkDir(input string) (string, error) {
	workDir := strings.TrimSpace(input)
	if workDir == "" {
		var err error
		workDir, err = os.Getwd()
		if err != nil {
			return "", err
		}
	}
	workDir, err := filepath.Abs(workDir)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(workDir)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("工作目录不是目录: %s", workDir)
	}
	return filepath.Clean(workDir), nil
}

func resolveProjectForCLI(db *gorm.DB, explicitProjectID, workDir string) (*models.Project, error) {
	if strings.TrimSpace(explicitProjectID) != "" {
		project, err := database.GetProjectByStringID(db, explicitProjectID)
		if err != nil {
			return nil, fmt.Errorf("获取项目失败: %w", err)
		}
		return project, nil
	}

	projects, err := database.GetAllProjects(db)
	if err != nil {
		return nil, err
	}

	var matched *models.Project
	bestLen := -1
	for index := range projects {
		project := projects[index]
		candidates := []string{
			strings.TrimSpace(project.Path),
			strings.TrimSpace(project.WorktreePath),
		}
		for _, candidate := range candidates {
			if candidate == "" {
				continue
			}
			candidateAbs, err := filepath.Abs(candidate)
			if err != nil {
				continue
			}
			candidateAbs = filepath.Clean(candidateAbs)
			if !pathContains(candidateAbs, workDir) {
				continue
			}
			if l := len(candidateAbs); l > bestLen {
				bestLen = l
				projectCopy := project
				matched = &projectCopy
			}
		}
	}

	if matched == nil {
		return nil, fmt.Errorf("当前目录未匹配到项目，请使用 --project-id 指定项目")
	}
	return matched, nil
}

func pathContains(base, target string) bool {
	base = filepath.Clean(strings.TrimSpace(base))
	target = filepath.Clean(strings.TrimSpace(target))
	if base == "" || target == "" {
		return false
	}
	if base == target {
		return true
	}
	baseWithSep := base
	if !strings.HasSuffix(baseWithSep, string(os.PathSeparator)) {
		baseWithSep += string(os.PathSeparator)
	}
	return strings.HasPrefix(target, baseWithSep)
}

func projectForSession(db *gorm.DB, sessionInfo *agenttypes.Info) (*models.Project, error) {
	if sessionInfo == nil {
		return nil, fmt.Errorf("会话不存在")
	}
	projectID := strings.TrimSpace(sessionInfo.ProjectID)
	if projectID == "" {
		return nil, fmt.Errorf("会话缺少 project_id")
	}
	project, err := database.GetProjectByStringID(db, projectID)
	if err != nil {
		return nil, fmt.Errorf("获取会话项目失败: %w", err)
	}
	return project, nil
}

func preferredProjectPath(project *models.Project) string {
	if project == nil {
		return ""
	}
	if path := strings.TrimSpace(project.WorktreePath); path != "" {
		return path
	}
	return strings.TrimSpace(project.Path)
}

func findLatestTaskBySessionID(db *gorm.DB, sessionID string) (*models.Task, error) {
	var task models.Task
	err := db.Where("session_id = ?", strings.TrimSpace(sessionID)).
		Order("updated_at DESC, id DESC").
		First(&task).Error
	if err != nil {
		return nil, err
	}
	return &task, nil
}

func latestSessionWorkerName(db *gorm.DB, sessionID string) (string, error) {
	messages, err := storage.GetMessageWithPartsBySessionIDLight(db, sessionID)
	if err != nil {
		return "", err
	}
	for index := len(messages) - 1; index >= 0; index-- {
		msg := messages[index]
		if msg == nil || msg.Info == nil {
			continue
		}
		if worker := strings.TrimSpace(msg.Info.Worker); worker != "" {
			return worker, nil
		}
	}
	return "", nil
}

func buildCLIChatTaskName(sessionID string) string {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return "CLI Chat"
	}
	return "CLI Chat " + sessionID
}

func newCLIChatHub(out, err io.Writer, in io.Reader, debug bool) *cliChatHub {
	if out == nil {
		out = io.Discard
	}
	if err == nil {
		err = io.Discard
	}
	if in == nil {
		in = strings.NewReader("")
	}
	return &cliChatHub{
		out:                  out,
		err:                  err,
		in:                   bufio.NewReader(in),
		debug:                debug,
		renderedByMessageID:  make(map[string]string),
		toolStateByPartID:    make(map[string]string),
		footerStateByMsgID:   make(map[string]string),
		subtaskStateByPartID: make(map[string]string),
	}
}

func (h *cliChatHub) BroadcastToTask(_ uint, msg wstypes.WSOutgoingMessage) {
	if msg.Type != wstypes.WSTypeMessageV2 {
		return
	}

	var message *agenttypes.WithParts
	switch value := msg.Data.(type) {
	case *agenttypes.WithParts:
		message = value
	case agenttypes.WithParts:
		message = &value
	default:
		return
	}
	h.renderMessage(message)
}

func (h *cliChatHub) BroadcastTaskMessage(_ uint, _ *models.TaskMessage) {}

func (h *cliChatHub) BroadcastNormalizedEntry(_ uint, _ *models.NormalizedEntry) {}

func (h *cliChatHub) BroadcastTaskStatus(_ uint, status models.TaskStatus, sessionID string, msg string) {
	h.mu.Lock()
	h.lastStatus = status
	h.lastStatusMessage = strings.TrimSpace(msg)
	if strings.TrimSpace(sessionID) != "" {
		h.sessionID = strings.TrimSpace(sessionID)
	}
	shouldAnnounce := h.sessionID != "" && !h.sessionIDAnnounced
	announceID := h.sessionID
	if shouldAnnounce {
		h.sessionIDAnnounced = true
	}
	h.mu.Unlock()

	if shouldAnnounce {
		fmt.Fprintf(h.err, "session_id: %s\n", announceID)
	}
}

func (h *cliChatHub) BroadcastIsWorking(_ uint) {}

func (h *cliChatHub) BroadcastIsNotWorking(_ uint) {}

func (h *cliChatHub) BroadcastStartLoading(_ uint) {}

func (h *cliChatHub) BroadcastEndLoading(_ uint) {}

func (h *cliChatHub) BroadcastError(_ uint, err string) {
	h.mu.Lock()
	h.lastError = strings.TrimSpace(err)
	h.mu.Unlock()
}

func (h *cliChatHub) BroadcastSessionTitle(_ uint, title string) {
	h.mu.Lock()
	h.lastSessionTitle = strings.TrimSpace(title)
	h.mu.Unlock()
}

func (h *cliChatHub) BroadcastRetry(_ uint) {}

func (h *cliChatHub) BroadcastTaskQueue(_ uint, _ []models.TaskMessageQueueItem) {}

func (h *cliChatHub) BroadcastTaskPlan(_ uint, _ any) {}

func (h *cliChatHub) BroadcastWaitUserInput(_ uint, _ string, ack func(result map[string]interface{}), question map[string]interface{}) {
	result := h.askUserInput(question)
	ack(result)
}

func (h *cliChatHub) renderMessage(message *agenttypes.WithParts) {
	if message == nil || message.Info == nil || message.Info.Role != agenttypes.RoleAssistant {
		return
	}

	h.renderFooterStatus(message.Info)
	h.renderToolTrace(message.Parts)
	text := renderAssistantText(message.Parts)

	h.mu.Lock()
	previousMessageID := h.lastAssistantMessage
	if previousMessageID != "" && previousMessageID != message.Info.ID && h.printedAnyText {
		fmt.Fprintln(h.out)
	}

	previousText := h.renderedByMessageID[message.Info.ID]
	delta := ""
	if strings.HasPrefix(text, previousText) {
		delta = text[len(previousText):]
	} else if len(text) > len(previousText) {
		delta = text[len(previousText):]
	}

	if delta != "" {
		fmt.Fprint(h.out, delta)
		h.printedAnyText = true
		h.renderedByMessageID[message.Info.ID] = text
	} else if _, ok := h.renderedByMessageID[message.Info.ID]; !ok {
		h.renderedByMessageID[message.Info.ID] = text
	}
	h.lastAssistantMessage = message.Info.ID
	h.mu.Unlock()
}

func (h *cliChatHub) renderFooterStatus(info *agenttypes.MessageInfo) {
	if !h.debug || info == nil {
		return
	}
	messageID := strings.TrimSpace(info.ID)
	if messageID == "" {
		return
	}

	next := ""
	if info.FooterStatus != nil {
		text := strings.TrimSpace(info.FooterStatus.Text)
		if text != "" || info.FooterStatus.Loading {
			next = fmt.Sprintf("%s|%t", text, info.FooterStatus.Loading)
		}
	}

	h.mu.Lock()
	previous := h.footerStateByMsgID[messageID]
	if previous == next {
		h.mu.Unlock()
		return
	}
	h.footerStateByMsgID[messageID] = next
	h.mu.Unlock()

	if next == "" {
		return
	}
	text := strings.TrimSpace(info.FooterStatus.Text)
	if text == "" && info.FooterStatus.Loading {
		text = "处理中…"
	}
	fmt.Fprintf(h.err, "[status] %s\n", text)
}

func (h *cliChatHub) renderToolTrace(parts []*agenttypes.Part) {
	if !h.debug || len(parts) == 0 {
		return
	}
	for _, part := range parts {
		if part == nil || part.Type != agenttypes.PartTypeTool || part.Tool == nil {
			continue
		}
		partID := strings.TrimSpace(part.ID)
		if partID == "" {
			continue
		}
		status := strings.TrimSpace(part.Tool.State.Status)
		name := strings.TrimSpace(part.Tool.Name)
		if status == "" || name == "" {
			continue
		}
		key := name + "|" + status

		h.mu.Lock()
		previous := h.toolStateByPartID[partID]
		if previous == key {
			h.mu.Unlock()
			continue
		}
		h.toolStateByPartID[partID] = key
		h.mu.Unlock()

		fmt.Fprintf(h.err, "[tool] %s [%s]\n", name, status)
		h.renderSubtaskTrace(partID, part.Tool.State.Metadata)
	}
}

func (h *cliChatHub) renderSubtaskTrace(partID string, metadata map[string]interface{}) {
	if !h.debug || strings.TrimSpace(partID) == "" || len(metadata) == 0 {
		return
	}

	workerName, _ := metadata["subtaskWorkerName"].(string)
	if strings.TrimSpace(workerName) == "" {
		return
	}
	status, _ := metadata["subtaskStatus"].(string)
	taskName, _ := metadata["subtaskTaskName"].(string)
	taskID := anyToString(metadata["subtaskTaskId"])

	key := strings.TrimSpace(workerName) + "|" + strings.TrimSpace(status) + "|" + strings.TrimSpace(taskID)
	h.mu.Lock()
	previous := h.subtaskStateByPartID[partID]
	if previous == key {
		h.mu.Unlock()
		return
	}
	h.subtaskStateByPartID[partID] = key
	h.mu.Unlock()

	if strings.TrimSpace(taskName) != "" && strings.TrimSpace(taskID) != "" {
		fmt.Fprintf(h.err, "[subtask] %s #%s %s [%s]\n", strings.TrimSpace(workerName), strings.TrimSpace(taskID), strings.TrimSpace(taskName), strings.TrimSpace(status))
		return
	}
	if strings.TrimSpace(taskID) != "" {
		fmt.Fprintf(h.err, "[subtask] %s #%s [%s]\n", strings.TrimSpace(workerName), strings.TrimSpace(taskID), strings.TrimSpace(status))
		return
	}
	fmt.Fprintf(h.err, "[subtask] %s [%s]\n", strings.TrimSpace(workerName), strings.TrimSpace(status))
}

func renderAssistantText(parts []*agenttypes.Part) string {
	if len(parts) == 0 {
		return ""
	}
	var builder strings.Builder
	for _, part := range parts {
		if part == nil || part.Ignored {
			continue
		}
		if part.Type == agenttypes.PartTypeText {
			builder.WriteString(part.Text)
		}
	}
	return builder.String()
}

func (h *cliChatHub) askUserInput(question map[string]interface{}) map[string]interface{} {
	if kind, _ := question["kind"].(string); kind == "project_tool_permission" {
		return h.askProjectToolPermission(question)
	}
	return h.askGenericQuestions(question)
}

func (h *cliChatHub) askProjectToolPermission(question map[string]interface{}) map[string]interface{} {
	projectName := nestedString(question, "project", "name")
	workerName := nestedString(question, "worker", "name")
	toolLabel := nestedString(question, "tool", "label")
	toolName := nestedString(question, "tool", "name")
	description := nestedString(question, "tool", "description")
	path := nestedString(question, "request", "path")
	command := nestedString(question, "request", "command")
	contentPreview := nestedString(question, "request", "contentPreview")
	patchPreview := nestedString(question, "request", "patchPreview")

	if projectName == "" {
		projectName = "当前项目"
	}
	if toolLabel == "" {
		toolLabel = toolName
	}
	if toolLabel == "" {
		toolLabel = "tool"
	}
	if workerName == "" {
		workerName = "worker"
	}

	fmt.Fprintf(h.err, "\n[approval] %s 请求调用 %s", workerName, toolLabel)
	if projectName != "" {
		fmt.Fprintf(h.err, "（项目：%s）", projectName)
	}
	fmt.Fprintln(h.err)
	if description != "" {
		fmt.Fprintf(h.err, "说明: %s\n", description)
	}
	if path != "" {
		fmt.Fprintf(h.err, "路径: %s\n", path)
	}
	if command != "" {
		fmt.Fprintf(h.err, "命令: %s\n", command)
	}
	if contentPreview != "" {
		fmt.Fprintf(h.err, "内容预览: %s\n", contentPreview)
	}
	if patchPreview != "" {
		fmt.Fprintf(h.err, "补丁预览: %s\n", patchPreview)
	}

	answer := strings.ToLower(strings.TrimSpace(h.readLine("允许执行？[y/N]: ")))
	if answer == "y" || answer == "yes" {
		return map[string]interface{}{"decision": "allow"}
	}

	reason := strings.TrimSpace(h.readLine("拒绝原因（可留空）: "))
	return map[string]interface{}{
		"decision": "reject",
		"reason":   reason,
	}
}

func (h *cliChatHub) askGenericQuestions(question map[string]interface{}) map[string]interface{} {
	items := extractQuestionItems(question)
	if len(items) == 0 {
		answer := strings.TrimSpace(h.readLine("\n请输入回复（留空表示拒绝）: "))
		if answer == "" {
			return map[string]interface{}{"refused": true}
		}
		return map[string]interface{}{"answer": answer}
	}

	result := make(map[string]interface{}, len(items))
	for index, item := range items {
		key := strings.TrimSpace(item["key"])
		text := strings.TrimSpace(item["question"])
		if key == "" {
			if text != "" {
				key = text
			} else {
				key = fmt.Sprintf("question_%d", index)
			}
		}
		fmt.Fprintf(h.err, "\n%s\n", text)
		if options := strings.TrimSpace(item["options"]); options != "" {
			fmt.Fprintf(h.err, "可选: %s\n", options)
		}
		answer := strings.TrimSpace(h.readLine("> "))
		result[key] = answer
	}

	if len(result) == 0 {
		return map[string]interface{}{"refused": true}
	}
	return result
}

func extractQuestionItems(question map[string]interface{}) []map[string]string {
	rawItems := make([]interface{}, 0)
	switch value := question["questions"].(type) {
	case []interface{}:
		rawItems = value
	default:
		if value != nil {
			rawItems = append(rawItems, value)
		}
	}
	if len(rawItems) == 0 {
		if rawArray, ok := any(question).([]interface{}); ok {
			rawItems = rawArray
		}
	}

	items := make([]map[string]string, 0, len(rawItems))
	usedKeys := make(map[string]struct{}, len(rawItems))
	for index, rawItem := range rawItems {
		item, ok := rawItem.(map[string]interface{})
		if !ok {
			continue
		}
		text := strings.TrimSpace(anyToString(item["question"]))
		if text == "" {
			text = fmt.Sprintf("问题 %d", index+1)
		}
		key := text
		if _, exists := usedKeys[key]; exists {
			key = fmt.Sprintf("question_%d", index)
		}
		usedKeys[key] = struct{}{}

		optionTexts := make([]string, 0)
		if values, ok := item["options"].([]interface{}); ok {
			for _, rawOption := range values {
				if option := strings.TrimSpace(anyToString(rawOption)); option != "" {
					optionTexts = append(optionTexts, option)
				}
			}
		}

		items = append(items, map[string]string{
			"key":      key,
			"question": text,
			"options":  strings.Join(optionTexts, " / "),
		})
	}
	return items
}

func nestedString(input map[string]interface{}, keys ...string) string {
	current := input
	for index, key := range keys {
		value, ok := current[key]
		if !ok {
			return ""
		}
		if index == len(keys)-1 {
			return anyToString(value)
		}
		next, ok := value.(map[string]interface{})
		if !ok {
			return ""
		}
		current = next
	}
	return ""
}

func anyToString(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	case uint:
		return strconv.FormatUint(uint64(typed), 10)
	case uint64:
		return strconv.FormatUint(typed, 10)
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	default:
		return ""
	}
}

func (h *cliChatHub) readLine(prompt string) string {
	fmt.Fprint(h.err, prompt)
	line, err := h.in.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return ""
	}
	return strings.TrimRight(line, "\r\n")
}

func (h *cliChatHub) SetSessionID(sessionID string) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return
	}
	h.mu.Lock()
	h.sessionID = sessionID
	shouldAnnounce := !h.sessionIDAnnounced
	h.sessionIDAnnounced = true
	h.mu.Unlock()
	if shouldAnnounce {
		fmt.Fprintf(h.err, "session_id: %s\n", sessionID)
	}
}

func (h *cliChatHub) SessionID() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.sessionID
}

func (h *cliChatHub) LastError() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.lastError
}

func (h *cliChatHub) Finish() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.printedAnyText {
		fmt.Fprintln(h.out)
	}
}

func (h *cliChatHub) PrintTaskHeader(projectName, workDir, workerName, sessionID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.taskHeaderPrinted {
		return
	}
	h.taskHeaderPrinted = true
	if strings.TrimSpace(projectName) != "" {
		fmt.Fprintf(h.err, "project: %s\n", strings.TrimSpace(projectName))
	}
	if strings.TrimSpace(workDir) != "" {
		fmt.Fprintf(h.err, "workdir: %s\n", strings.TrimSpace(workDir))
	}
	if strings.TrimSpace(workerName) != "" {
		fmt.Fprintf(h.err, "worker: %s\n", strings.TrimSpace(workerName))
	}
	if strings.TrimSpace(sessionID) != "" && !h.sessionIDAnnounced {
		h.sessionID = strings.TrimSpace(sessionID)
		h.sessionIDAnnounced = true
		fmt.Fprintf(h.err, "session_id: %s\n", strings.TrimSpace(sessionID))
	}
}

func cleanupCLIApp(app *apppkg.App) {
	if app == nil {
		return
	}
	app.Cleanup()
	if app.DB == nil {
		return
	}
	sqlDB, err := app.DB.DB()
	if err == nil && sqlDB != nil {
		_ = sqlDB.Close()
	}
}
