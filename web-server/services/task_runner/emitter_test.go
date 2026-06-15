package task_runner

import (
	"testing"
	"time"

	agentsession "matrixops-agent/session"
	agenttypes "matrixops-agent/types"
	servicesTypes "matrixops/types"
	database "pkgs/db"
	"pkgs/db/models"
	"pkgs/db/storage"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type taskStatusBroadcast struct {
	taskID    uint
	status    models.TaskStatus
	sessionID string
	msg       string
}

type stubWSHub struct {
	statuses []taskStatusBroadcast
	errors   []string
	messages []servicesTypes.WSOutgoingMessage
}

func (h *stubWSHub) BroadcastToTask(taskID uint, msg servicesTypes.WSOutgoingMessage) {
	h.messages = append(h.messages, msg)
}

func (h *stubWSHub) BroadcastTaskMessage(taskID uint, message *models.TaskMessage) {}

func (h *stubWSHub) BroadcastNormalizedEntry(taskID uint, entry *models.NormalizedEntry) {}

func (h *stubWSHub) BroadcastTaskStatus(taskID uint, status models.TaskStatus, sessionID string, msg string) {
	h.statuses = append(h.statuses, taskStatusBroadcast{
		taskID:    taskID,
		status:    status,
		sessionID: sessionID,
		msg:       msg,
	})
}

func (h *stubWSHub) BroadcastIsWorking(taskID uint) {}

func (h *stubWSHub) BroadcastIsNotWorking(taskID uint) {}

func (h *stubWSHub) BroadcastError(taskID uint, err string) {
	h.errors = append(h.errors, err)
}

func (h *stubWSHub) BroadcastSessionTitle(taskID uint, title string) {}

func (h *stubWSHub) BroadcastRetry(taskID uint) {}

func (h *stubWSHub) BroadcastWaitUserInput(taskID uint, id string, ack func(result map[string]interface{}), question map[string]interface{}) {
}

func (h *stubWSHub) BroadcastTaskQueue(taskID uint, queue []models.TaskMessageQueueItem) {}

func (h *stubWSHub) BroadcastTaskPlan(taskID uint, plan any) {}

func setupEmitterTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	if err := db.AutoMigrate(&models.Task{}, &models.TaskExecution{}); err != nil {
		t.Fatalf("migrate tables: %v", err)
	}
	if err := storage.InitStorage(db); err != nil {
		t.Fatalf("init storage: %v", err)
	}

	return db
}

func createRunningTask(t *testing.T, db *gorm.DB) *models.Task {
	t.Helper()

	task := &models.Task{
		ProjectID:  1,
		Name:       "memory-test",
		Content:    "inspect memory manager",
		WorkerName: "coder",
		Status:     string(models.TaskStatusRunning),
		WorkDir:    "/tmp",
	}
	if err := db.Create(task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}
	return task
}

func TestEmitterSessionIDBroadcastsImmediatelyAndIsReusedForTaskStatus(t *testing.T) {
	db := setupEmitterTestDB(t)
	task := createRunningTask(t, db)
	hub := &stubWSHub{}

	emitter := NewEmitter(hub, db, task.ID)
	emitter.EmitSessionID("session-live")

	refreshedTask, err := database.GetTaskByID(db, task.ID)
	if err != nil {
		t.Fatalf("reload task: %v", err)
	}
	if refreshedTask.SessionID != "session-live" {
		t.Fatalf("task session_id = %q, want %q", refreshedTask.SessionID, "session-live")
	}

	if len(hub.statuses) != 1 {
		t.Fatalf("broadcast count = %d, want 1", len(hub.statuses))
	}
	if hub.statuses[0].taskID != task.ID {
		t.Fatalf("broadcast task id = %d, want %d", hub.statuses[0].taskID, task.ID)
	}
	if hub.statuses[0].status != models.TaskStatusRunning {
		t.Fatalf("broadcast status = %q, want %q", hub.statuses[0].status, models.TaskStatusRunning)
	}
	if hub.statuses[0].sessionID != "session-live" {
		t.Fatalf("broadcast session id = %q, want %q", hub.statuses[0].sessionID, "session-live")
	}

	emitter.EmitTaskStatus(models.TaskStatusDone, "任务执行完成")

	if len(hub.statuses) != 2 {
		t.Fatalf("broadcast count after status update = %d, want 2", len(hub.statuses))
	}
	if hub.statuses[1].sessionID != "session-live" {
		t.Fatalf("follow-up broadcast session id = %q, want %q", hub.statuses[1].sessionID, "session-live")
	}
	if hub.statuses[1].status != models.TaskStatusDone {
		t.Fatalf("follow-up broadcast status = %q, want %q", hub.statuses[1].status, models.TaskStatusDone)
	}
}

func TestOnEmitterCreatedPersistsSessionIDToRuntimeAndExecution(t *testing.T) {
	db := setupEmitterTestDB(t)
	task := createRunningTask(t, db)
	execution := models.TaskExecution{
		TaskID:    task.ID,
		Status:    string(models.TaskStatusRunning),
		StartedAt: time.Now(),
	}
	if err := db.Create(&execution).Error; err != nil {
		t.Fatalf("create execution: %v", err)
	}

	hub := &stubWSHub{}
	runtime := &TaskRuntime{
		db:        db,
		taskID:    task.ID,
		emitter:   NewEmitter(hub, db, task.ID),
		config:    &TaskRuntimeConfig{SessionID: ""},
		execution: execution,
	}

	agentEmitter := agentsession.NewEmitter(db, "")
	if err := runtime.onEmitterCreated(agentEmitter); err != nil {
		t.Fatalf("register emitter listeners: %v", err)
	}

	agentEmitter.Emit(agentsession.EventSessionCreated, agentsession.SessionEvent{
		Info: &agentsession.Info{ID: "session-created"},
	})

	if runtime.sessionID != "session-created" {
		t.Fatalf("runtime session id = %q, want %q", runtime.sessionID, "session-created")
	}
	if runtime.config.SessionID != "session-created" {
		t.Fatalf("runtime config session id = %q, want %q", runtime.config.SessionID, "session-created")
	}
	if runtime.execution.AgentSessionID != "session-created" {
		t.Fatalf("runtime execution session id = %q, want %q", runtime.execution.AgentSessionID, "session-created")
	}

	storedExecution, err := database.GetExecutionByID(db, execution.ID)
	if err != nil {
		t.Fatalf("reload execution: %v", err)
	}
	if storedExecution.AgentSessionID != "session-created" {
		t.Fatalf("execution session id = %q, want %q", storedExecution.AgentSessionID, "session-created")
	}

	if len(hub.statuses) != 1 {
		t.Fatalf("broadcast count = %d, want 1", len(hub.statuses))
	}
	if hub.statuses[0].sessionID != "session-created" {
		t.Fatalf("broadcast session id = %q, want %q", hub.statuses[0].sessionID, "session-created")
	}
	if hub.statuses[0].status != models.TaskStatusRunning {
		t.Fatalf("broadcast status = %q, want %q", hub.statuses[0].status, models.TaskStatusRunning)
	}
}

func TestEmitterSuppressesDuplicateErrorsWithinWindow(t *testing.T) {
	db := setupEmitterTestDB(t)
	task := createRunningTask(t, db)
	hub := &stubWSHub{}

	emitter := NewEmitter(hub, db, task.ID)
	emitter.EmitError(assertError("context canceled"))
	emitter.EmitError(assertError("context canceled"))
	emitter.EmitError(assertError("context canceled"))

	if len(hub.errors) != 1 {
		t.Fatalf("error broadcasts = %d, want 1", len(hub.errors))
	}
	if hub.errors[0] != "context canceled" {
		t.Fatalf("unexpected error payload: %q", hub.errors[0])
	}
}

func TestOnEmitterCreatedBroadcastsMessagePartsInStoredOrder(t *testing.T) {
	db := setupEmitterTestDB(t)
	task := createRunningTask(t, db)
	hub := &stubWSHub{}

	runtime := &TaskRuntime{
		db:      db,
		taskID:  task.ID,
		emitter: NewEmitter(hub, db, task.ID),
	}

	agentEmitter := agentsession.NewEmitter(db, "session-order")
	if err := runtime.onEmitterCreated(agentEmitter); err != nil {
		t.Fatalf("register emitter listeners: %v", err)
	}

	message := &agentsession.MessageInfo{
		ID:        "message-order",
		SessionID: "session-order",
		Role:      agenttypes.RoleAssistant,
		Time:      agenttypes.MessageTime{Created: 100},
	}
	if _, err := agentEmitter.UpdateMessage(message); err != nil {
		t.Fatalf("UpdateMessage: %v", err)
	}

	textPart := &agentsession.Part{
		ID:        "part-text",
		MessageID: message.ID,
		SessionID: message.SessionID,
		Type:      agenttypes.PartTypeText,
		Text:      "我先快速梳理了仓库结构。",
		Time:      &agenttypes.PartTime{Start: 101, Created: 101, End: 102},
	}
	if _, err := agentEmitter.UpdatePart(textPart); err != nil {
		t.Fatalf("UpdatePart(text): %v", err)
	}

	toolPart := &agentsession.Part{
		ID:        "part-tool",
		MessageID: message.ID,
		SessionID: message.SessionID,
		Type:      agenttypes.PartTypeTool,
		Time:      &agenttypes.PartTime{Start: 103, Created: 103},
		Tool: &agenttypes.ToolPart{
			Name:   "read",
			CallID: "call-1",
			State: agenttypes.ToolState{
				Status: "running",
				Input:  map[string]interface{}{"path": "README.md"},
				Time:   agenttypes.PartTime{Start: 103, Created: 103},
			},
		},
	}
	if _, err := agentEmitter.UpdatePart(toolPart); err != nil {
		t.Fatalf("UpdatePart(tool): %v", err)
	}

	var lastMessage *agenttypes.WithParts
	for _, message := range hub.messages {
		if message.Type != servicesTypes.WSTypeMessageV2 || message.Data == nil {
			continue
		}
		if wp, ok := message.Data.(*agenttypes.WithParts); ok {
			lastMessage = wp
		}
	}
	if lastMessage == nil {
		t.Fatal("expected message_v2 broadcast")
	}
	if len(lastMessage.Parts) != 2 {
		t.Fatalf("expected 2 parts, got %d", len(lastMessage.Parts))
	}
	if lastMessage.Parts[0].Type != agenttypes.PartTypeText {
		t.Fatalf("expected first part text, got %q", lastMessage.Parts[0].Type)
	}
	if lastMessage.Parts[1].Type != agenttypes.PartTypeTool {
		t.Fatalf("expected second part tool, got %q", lastMessage.Parts[1].Type)
	}
}

type staticError string

func (e staticError) Error() string {
	return string(e)
}

func assertError(message string) error {
	return staticError(message)
}
