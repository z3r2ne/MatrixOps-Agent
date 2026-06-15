package services

import (
	"matrixops/services/task_runner"
	"matrixops/types"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	database "pkgs/db"
	models "pkgs/db/models"
	"pkgs/db/storage"

	"gorm.io/gorm"
)

// WSAction WebSocket 动作类型
type WSAction string

const (
	WSActionSubscribe     WSAction = "subscribe"       // 订阅任务
	WSActionUnsubscribe   WSAction = "unsubscribe"     // 取消订阅
	WSActionSendMessage   WSAction = "send_message"    // 发送消息到任务
	WSActionRestart       WSAction = "restart"         // 重启任务
	WSActionStop          WSAction = "stop"            // 停止任务
	WSActionCancelTool    WSAction = "cancel_tool"     // 取消单个工具调用
	WSActionWaitUserInput WSAction = "wait_user_input" // 等待用户输入
)

// WSMessageType WebSocket 消息类型
type WSMessageType = types.WSMessageType

// WSIncomingMessage 客户端发送的消息
type WSIncomingMessage struct {
	Action  WSAction        `json:"action"`
	TaskID  uint            `json:"taskId"`
	Content string          `json:"content,omitempty"`
	Parts   json.RawMessage `json:"parts,omitempty"`
	Data    json.RawMessage `json:"data,omitempty"`
}

type WSOutgoingMessage = types.WSOutgoingMessage

// GlobalWSClient 全局 WebSocket 客户端
type GlobalWSClient struct {
	ID            string
	Send          chan []byte
	subscriptions map[uint]bool // 订阅的任务 ID
	mu            sync.RWMutex
}

// NewGlobalWSClient 创建新的客户端
func NewGlobalWSClient(id string) *GlobalWSClient {
	return &GlobalWSClient{
		ID:            id,
		Send:          make(chan []byte, 256),
		subscriptions: make(map[uint]bool),
	}
}

// Subscribe 订阅任务
func (c *GlobalWSClient) Subscribe(taskID uint) {
	c.mu.Lock()
	c.subscriptions[taskID] = true
	c.mu.Unlock()
}

// Unsubscribe 取消订阅任务
func (c *GlobalWSClient) Unsubscribe(taskID uint) {
	c.mu.Lock()
	delete(c.subscriptions, taskID)
	c.mu.Unlock()
}

// IsSubscribed 是否订阅了某个任务
func (c *GlobalWSClient) IsSubscribed(taskID uint) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.subscriptions[taskID]
}


// GlobalWSHub 全局 WebSocket Hub
type GlobalWSHub struct {
	db                   *gorm.DB
	wg                   sync.WaitGroup
	mu                   sync.RWMutex
	clients              map[*GlobalWSClient]bool
	broadcast            chan globalBroadcast
	idToAck              map[string]func(result map[string]interface{})
	taskMessageListeners map[uint]map[string]func(*models.TaskMessage)
}

func NewGlobalWSHub(db *gorm.DB) *GlobalWSHub {
	return &GlobalWSHub{
		db:                   db,
		clients:              make(map[*GlobalWSClient]bool),
		broadcast:            make(chan globalBroadcast, 100),
		idToAck:              make(map[string]func(result map[string]interface{})),
		taskMessageListeners: make(map[uint]map[string]func(*models.TaskMessage)),
	}
}

type globalBroadcast struct {
	taskID  uint
	message WSOutgoingMessage
}

var globalWSHub *GlobalWSHub
var globalWSHubOnce sync.Once

// GetGlobalWSHub 获取全局 WebSocket Hub 单例
func GetGlobalWSHub(db *gorm.DB) *GlobalWSHub {
	globalWSHubOnce.Do(func() {
		globalWSHub = NewGlobalWSHub(db)
		globalWSHub.RunAsync()
	})
	return globalWSHub
}

// SetGlobalWSHubForTest 在测试中注入指定 Hub（避免单例绑定到其它测试的 DB）。
func SetGlobalWSHubForTest(h *GlobalWSHub) {
	globalWSHub = h
}

func (h *GlobalWSHub) RunAsync() {
	h.wg.Add(1)
	go func() {
		h.run()
		h.wg.Done()
	}()
}
func (h *GlobalWSHub) run() {
	for msg := range h.broadcast {
		h.mu.RLock()
		data, _ := json.Marshal(msg.message)
		for client := range h.clients {
			select {
			case client.Send <- data:
			default:
				// 缓冲满，跳过
			}
		}
		h.mu.RUnlock()
	}
}

// Register 注册客户端
func (h *GlobalWSHub) Register(client *GlobalWSClient) {
	h.mu.Lock()
	h.clients[client] = true
	h.mu.Unlock()
	log.Printf("[GlobalWSHub] Client registered: %s", client.ID)
}

// Unregister 注销客户端
func (h *GlobalWSHub) Unregister(client *GlobalWSClient) {
	h.mu.Lock()
	delete(h.clients, client)
	h.mu.Unlock()
	log.Printf("[GlobalWSHub] Client unregistered: %s", client.ID)
}

// BroadcastToTask 向订阅了某任务的客户端广播消息
func (h *GlobalWSHub) BroadcastToTask(taskID uint, msg types.WSOutgoingMessage) {
	h.broadcast <- globalBroadcast{taskID: taskID, message: msg}
}

// BroadcastTaskMessage 广播任务消息
func (h *GlobalWSHub) BroadcastTaskMessage(taskID uint, message *models.TaskMessage) {
	h.BroadcastToTask(taskID, types.WSOutgoingMessage{
		TaskID:  taskID,
		Type:    types.WSTypeTaskMessage,
		Message: message,
	})

	// 通知 task message listeners
	h.mu.RLock()
	listeners := h.taskMessageListeners[taskID]
	h.mu.RUnlock()
	for _, listener := range listeners {
		go listener(message)
	}
}

// RegisterTaskMessageListener 注册任务消息监听器
func (h *GlobalWSHub) RegisterTaskMessageListener(taskID uint, key string, listener func(*models.TaskMessage)) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.taskMessageListeners[taskID] == nil {
		h.taskMessageListeners[taskID] = make(map[string]func(*models.TaskMessage))
	}
	h.taskMessageListeners[taskID][key] = listener
}

// UnregisterTaskMessageListener 注销任务消息监听器
func (h *GlobalWSHub) UnregisterTaskMessageListener(taskID uint, key string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.taskMessageListeners[taskID] != nil {
		delete(h.taskMessageListeners[taskID], key)
	}
}

// BroadcastNormalizedEntry 广播规范化条目
func (h *GlobalWSHub) BroadcastNormalizedEntry(taskID uint, entry *models.NormalizedEntry) {
	message := normalizedEntryToTaskMessage(entry)
	h.BroadcastToTask(taskID, types.WSOutgoingMessage{
		TaskID:  taskID,
		Type:    types.WSTypeTaskMessage,
		Message: message,
	})
}

// BroadcastTaskStatus 广播任务状态变化
func (h *GlobalWSHub) BroadcastTaskStatus(taskID uint, status models.TaskStatus, sessionID string, msg string) {
	h.BroadcastToTask(taskID, types.WSOutgoingMessage{
		TaskID:    taskID,
		Type:      types.WSTypeTaskStatus,
		Status:    string(status),
		SessionID: sessionID,
		Error:     msg,
	})
}

// BroadcastTaskStatus 广播任务状态变化
func (h *GlobalWSHub) BroadcastIsWorking(taskID uint) {
	h.BroadcastToTask(taskID, types.WSOutgoingMessage{
		TaskID: taskID,
		Type:   types.WSTypeIsWorking,
	})
}

func (h *GlobalWSHub) BroadcastIsNotWorking(taskID uint) {
	h.BroadcastToTask(taskID, types.WSOutgoingMessage{
		TaskID: taskID,
		Type:   types.WSTypeIsNotWorking,
	})
}


// BroadcastError 广播错误
func (h *GlobalWSHub) BroadcastError(taskID uint, err string) {
	h.BroadcastToTask(taskID, types.WSOutgoingMessage{
		TaskID: taskID,
		Type:   types.WSTypeError,
		Error:  err,
	})
}

func (h *GlobalWSHub) BroadcastSessionTitle(taskID uint, title string) {
	h.BroadcastToTask(taskID, types.WSOutgoingMessage{
		TaskID: taskID,
		Type:   types.WSTypeSessionTitle,
		Data:   title,
	})
}

func (h *GlobalWSHub) BroadcastRetry(taskID uint) {
	h.BroadcastToTask(taskID, types.WSOutgoingMessage{
		TaskID: taskID,
		Type:   types.WSTypeRetry,
	})
}

func (h *GlobalWSHub) taskQueueWSData(taskID uint, queue []models.TaskMessageQueueItem) map[string]interface{} {
	autoSend := true
	if task, err := database.GetTaskByID(h.db, taskID); err == nil && task != nil {
		autoSend = task.MessageQueueAutoSend
	}
	return map[string]interface{}{
		"queue":    queue,
		"autoSend": autoSend,
	}
}

func (h *GlobalWSHub) sendTaskQueueSnapshotToClient(client *GlobalWSClient, taskID uint) {
	queue, err := database.GetTaskQueue(h.db, taskID)
	if err != nil {
		return
	}
	h.SendToClient(client, WSOutgoingMessage{
		TaskID: taskID,
		Type:   types.WSTypeTaskQueue,
		Data:   h.taskQueueWSData(taskID, queue),
	})
}

func (h *GlobalWSHub) BroadcastTaskQueue(taskID uint, queue []models.TaskMessageQueueItem) {
	h.BroadcastToTask(taskID, types.WSOutgoingMessage{
		TaskID: taskID,
		Type:   types.WSTypeTaskQueue,
		Data:   h.taskQueueWSData(taskID, queue),
	})
}

func (h *GlobalWSHub) BroadcastTaskPlan(taskID uint, plan any) {
	h.BroadcastToTask(taskID, types.WSOutgoingMessage{
		TaskID: taskID,
		Type:   types.WSTypeTaskPlan,
		Data:   plan,
	})
}

func (h *GlobalWSHub) sendTaskPlanSnapshotToClient(client *GlobalWSClient, taskID uint) {
	sessionID, err := database.GetTaskSessionID(h.db, taskID)
	if err != nil || strings.TrimSpace(sessionID) == "" {
		return
	}
	plan, err := storage.GetPlan(h.db, sessionID)
	if err != nil {
		return
	}
	h.SendToClient(client, WSOutgoingMessage{
		TaskID: taskID,
		Type:   types.WSTypeTaskPlan,
		Data:   plan.Content.Data,
	})
}

// BroadcastILinkSessionExpired 通知所有客户端：某 iLink 微信账号会话已过期，需重新扫码登录。
func (h *GlobalWSHub) BroadcastILinkSessionExpired(accountID uint, botID, ilinkUserID string) {
	h.BroadcastToTask(0, types.WSOutgoingMessage{
		Type: types.WSTypeILinkSessionExpired,
		Data: map[string]interface{}{
			"accountId":   accountID,
			"botId":       botID,
			"ilinkUserId": ilinkUserID,
		},
	})
}

func (h *GlobalWSHub) BroadcastWaitUserInput(taskID uint, id string, ack func(result map[string]interface{}), question map[string]interface{}) {
	h.idToAck[id] = ack
	h.BroadcastToTask(taskID, types.WSOutgoingMessage{
		TaskID: taskID,
		Type:   types.WSTypeWaitUserInput,
		Data:   map[string]interface{}{"id": id, "question": question},
	})
}

func (h *GlobalWSHub) handleWaitUserInput(taskID uint, content string) error {
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(content), &data); err != nil {
		return fmt.Errorf("消息格式错误")
	}
	id := data["id"].(string)
	result := data["result"].(map[string]interface{})
	ack, ok := h.idToAck[id]
	if !ok {
		return fmt.Errorf("等待用户输入 ID 不存在")
	}
	ack(result)
	delete(h.idToAck, id)
	return nil
}
func normalizedEntryToTaskMessage(entry *models.NormalizedEntry) *models.TaskMessage {
	if entry == nil {
		return nil
	}
	role := "assistant"
	switch entry.EntryType {
	case models.EntryTypeUserMessage:
		role = "user"
	case models.EntryTypeSystemMessage:
		role = "system"
	}
	return &models.TaskMessage{
		Type:      "normalized_entry",
		Role:      role,
		Content:   entry.Content,
		Timestamp: time.Now().UnixMilli(),
		Entry:     entry,
	}
}

// SendToClient 发送消息给特定客户端
func (h *GlobalWSHub) SendToClient(client *GlobalWSClient, msg WSOutgoingMessage) {
	data, _ := json.Marshal(msg)
	select {
	case client.Send <- data:
	default:
	}
}

// HandleMessage 处理客户端消息
func (h *GlobalWSHub) HandleMessage(client *GlobalWSClient, data []byte) error {
	var msg WSIncomingMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return fmt.Errorf("消息格式错误")
	}

	switch msg.Action {
	case WSActionSubscribe:
		return h.handleSubscribe(client, msg.TaskID)

	case WSActionUnsubscribe:
		return h.handleUnsubscribe(client, msg.TaskID)

	case WSActionSendMessage:
		return h.handleSendMessage(msg.TaskID, msg.Content, msg.Parts)

	case WSActionRestart:
		return h.handleRestart(client, msg.TaskID)

	case WSActionStop:
		return h.handleStop(msg.TaskID)

	case WSActionCancelTool:
		return h.handleCancelTool(msg.TaskID, msg.Data)

	case WSActionWaitUserInput:
		return h.handleWaitUserInput(msg.TaskID, msg.Content)

	default:
		return fmt.Errorf("未知的操作类型")
	}
}

func (h *GlobalWSHub) handleSubscribe(client *GlobalWSClient, taskID uint) error {
	if taskID == 0 {
		return fmt.Errorf("无效的任务 ID")
	}

	client.Subscribe(taskID)

	// 发送订阅成功消息
	h.SendToClient(client, WSOutgoingMessage{
		TaskID: taskID,
		Type:   types.WSTypeSubscribed,
	})

	h.sendTaskQueueSnapshotToClient(client, taskID)
	h.sendTaskPlanSnapshotToClient(client, taskID)

	return nil
}

func (h *GlobalWSHub) handleUnsubscribe(client *GlobalWSClient, taskID uint) error {
	client.Unsubscribe(taskID)
	h.SendToClient(client, WSOutgoingMessage{
		TaskID: taskID,
		Type:   types.WSTypeUnsubscribed,
	})
	return nil
}

func (h *GlobalWSHub) handleSendMessage(taskID uint, content string, partsRaw json.RawMessage) error {
	if taskID == 0 {
		return fmt.Errorf("任务 ID 或消息内容为空")
	}
	task, dbErr := database.GetTaskByID(h.db, taskID)
	if dbErr != nil {
		return dbErr
	}
	parts, partsText, err := ParseWSUserParts(strings.TrimSpace(task.WorkDir), taskID, partsRaw)
	if err != nil {
		return err
	}
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		trimmed = partsText
	}
	if trimmed == "" && len(parts) == 0 {
		return fmt.Errorf("任务 ID 或消息内容为空")
	}

	opts := []task_runner.TaskRuntimeConfigOption{
		task_runner.WithContent(trimmed),
		task_runner.WithWSHub(h),
		task_runner.WithDB(h.db),
		task_runner.WithInputSource(task_runner.TaskInputSourceFrontend),
	}
	if len(parts) > 0 {
		opts = append(opts, task_runner.WithInputParts(parts))
	}
	return task_runner.RunTask(taskID, opts...)
}

func (h *GlobalWSHub) handleCancelTool(taskID uint, data json.RawMessage) error {
	if taskID == 0 {
		return fmt.Errorf("无效的任务 ID")
	}

	var payload struct {
		CallID string `json:"callID"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("取消工具参数格式错误")
	}
	if payload.CallID == "" {
		return fmt.Errorf("缺少 callID")
	}
	return task_runner.CancelToolCall(taskID, payload.CallID)
}

func (h *GlobalWSHub) handleRestart(client *GlobalWSClient, taskID uint) error {
	if taskID == 0 {
		return fmt.Errorf("无效的任务 ID")
	}

	return task_runner.RunTask(taskID)
}

func (h *GlobalWSHub) handleStop(taskID uint) error {
	if taskID == 0 {
		return fmt.Errorf("无效的任务 ID")
	}

	log.Printf("[GlobalWSHub] Stopping task %d", taskID)
	err := task_runner.CancelTask(taskID)
	if err != nil {
		h.BroadcastError(taskID, fmt.Sprintf("停止任务失败: %v", err))
		return nil
	}

	h.BroadcastIsNotWorking(taskID)
	return nil
}

func (h *GlobalWSHub) Close() {
	close(h.broadcast)
	h.wg.Wait()
}
