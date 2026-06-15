package services

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	database "pkgs/db"
	"pkgs/db/models"

	"gorm.io/gorm"
)

// MsgStore 消息存储 (对应 vibe-kanban 的 MsgStore)
// 支持内存广播 + 数据库持久化
type MsgStore struct {
	db          *gorm.DB
	executionID uint
	mu          sync.RWMutex
	history     []models.LogMsg             // 内存历史（用于快速获取）
	sequence    atomic.Uint64               // 序号生成器
	subscribers map[chan models.LogMsg]bool // 订阅者
	sessionID   string                      // Agent 会话 ID
	finished    bool                        // 是否已完成
	maxHistory  int                         // 最大历史条数

	// 持久化管道
	persistChan chan *models.LogMsg // 带缓冲的持久化通道
	persistDone chan struct{}       // 持久化完成信号
}

// NewMsgStore 创建消息存储
func NewMsgStore(executionID uint) *MsgStore {
	store := &MsgStore{
		executionID: executionID,
		history:     make([]models.LogMsg, 0, 100),
		subscribers: make(map[chan models.LogMsg]bool),
		maxHistory:  500,                             // 内存最多保留 500 条
		persistChan: make(chan *models.LogMsg, 1000), // 带缓冲的持久化通道
		persistDone: make(chan struct{}),
	}
	// 启动持久化 goroutine
	go store.persistWorker()
	return store
}

// persistWorker 持久化工作协程，按顺序消费消息并写入数据库
func (s *MsgStore) persistWorker() {
	for msg := range s.persistChan {
		if msg == nil {
			continue
		}
		logEntry := models.ExecutionLogFromLogMsg(s.executionID, msg)
		if err := database.CreateExecutionLog(s.db, logEntry); err != nil {
			log.Printf("[MsgStore] Failed to persist log for execution %d: %v", s.executionID, err)
		}
	}
	close(s.persistDone)
}

// NewMsgStoreFromDB 从数据库恢复消息存储（用于客户端重连）
func NewMsgStoreFromDB(executionID uint) *MsgStore {
	store := &MsgStore{
		executionID: executionID,
		history:     make([]models.LogMsg, 0, 100),
		subscribers: make(map[chan models.LogMsg]bool),
		maxHistory:  500,
		persistChan: make(chan *models.LogMsg, 1000),
		persistDone: make(chan struct{}),
	}
	// 启动持久化 goroutine
	go store.persistWorker()

	// 从数据库加载历史日志
	if logs, err := database.GetExecutionLogsByExecutionID(executionID); err == nil {

		for _, logEntry := range logs {
			msg := logEntry.ToLogMsg()
			store.history = append(store.history, *msg)

			// 更新序号
			if uint64(msg.Sequence) >= store.sequence.Load() {
				store.sequence.Store(uint64(msg.Sequence) + 1)
			}

			// 提取 session_id
			if msg.Type == models.LogMsgTypeSessionID {
				store.sessionID = msg.SessionID
			}

			// 检查是否已完成
			if msg.Type == models.LogMsgTypeFinished {
				store.finished = true
			}
		}
	}

	return store
}

// Push 推送消息（广播 + 持久化）
func (s *MsgStore) Push(msgType models.LogMsgType, content string, entry *models.NormalizedEntry) {
	msg := models.LogMsg{
		Type:     msgType,
		Content:  content,
		Entry:    entry,
		Sequence: uint(s.sequence.Add(1)),
	}

	s.mu.Lock()
	// 添加到历史
	s.history = append(s.history, msg)
	if len(s.history) > s.maxHistory {
		s.history = s.history[len(s.history)-s.maxHistory:]
	}

	// 广播给订阅者
	for ch := range s.subscribers {
		select {
		case ch <- msg:
		default:
			// 订阅者缓冲满，跳过
		}
	}
	s.mu.Unlock()

	// 发送到持久化通道（按顺序存储）
	s.enqueuePersist(&msg)
}

// enqueuePersist 将消息加入持久化队列
func (s *MsgStore) enqueuePersist(msg *models.LogMsg) {
	select {
	case s.persistChan <- msg:
	default:
		// 通道满，记录警告但不阻塞
		log.Printf("[MsgStore] Persist channel full for execution %d, message may be lost", s.executionID)
	}
}

// PushSessionID 推送会话 ID
func (s *MsgStore) PushSessionID(sessionID string) {

	s.mu.Lock()
	s.sessionID = sessionID
	s.mu.Unlock()

	msg := models.LogMsg{
		Type:      models.LogMsgTypeSessionID,
		SessionID: sessionID,
		Sequence:  uint(s.sequence.Add(1)),
	}

	s.mu.Lock()
	s.history = append(s.history, msg)
	for ch := range s.subscribers {
		select {
		case ch <- msg:
		default:
		}
	}
	s.mu.Unlock()

	// 发送到持久化通道
	s.enqueuePersist(&msg)

	// 同步更新 TaskExecution 表（确保 session_id 立即可查询）
	execID := s.executionID
	if err := database.UpdateExecutionSessionID(execID, sessionID); err != nil {
		log.Printf("[MsgStore] Failed to save session_id to TaskExecution %d: %v", execID, err)
	}
}

// PushFinished 推送完成信号
func (s *MsgStore) PushFinished() {
	s.mu.Lock()
	if s.finished {
		s.mu.Unlock()
		return // 已经完成，避免重复调用
	}
	s.finished = true
	s.mu.Unlock()

	msg := models.LogMsg{
		Type:     models.LogMsgTypeFinished,
		Sequence: uint(s.sequence.Add(1)),
	}

	s.mu.Lock()
	s.history = append(s.history, msg)
	for ch := range s.subscribers {
		select {
		case ch <- msg:
		default:
		}
		close(ch)
	}
	s.subscribers = make(map[chan models.LogMsg]bool)
	s.mu.Unlock()

	// 发送完成消息到持久化通道，然后关闭
	s.enqueuePersist(&msg)
	close(s.persistChan)

	// 等待持久化完成（最多等 5 秒）
	select {
	case <-s.persistDone:
		log.Printf("[MsgStore] Persist worker finished for execution %d", s.executionID)
	case <-time.After(5 * time.Second):
		log.Printf("[MsgStore] Persist worker timeout for execution %d", s.executionID)
	}
}

// GetHistory 获取历史消息
func (s *MsgStore) GetHistory() []models.LogMsg {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]models.LogMsg, len(s.history))
	copy(result, s.history)
	return result
}

// GetSessionID 获取会话 ID
func (s *MsgStore) GetSessionID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sessionID
}

// IsFinished 是否已完成
func (s *MsgStore) IsFinished() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.finished
}

// Subscribe 订阅实时消息
func (s *MsgStore) Subscribe() chan models.LogMsg {
	ch := make(chan models.LogMsg, 100)
	s.mu.Lock()
	s.subscribers[ch] = true
	s.mu.Unlock()
	return ch
}

// Unsubscribe 取消订阅
func (s *MsgStore) Unsubscribe(ch chan models.LogMsg) {
	s.mu.Lock()
	delete(s.subscribers, ch)
	s.mu.Unlock()
}

// GetNextSequence 获取下一个序号（用于 EntryIndexProvider）
func (s *MsgStore) GetNextSequence() uint {
	return uint(s.sequence.Add(1))
}

// EntryIndexProvider 条目索引提供器（确保索引唯一性）
type EntryIndexProvider struct {
	counter atomic.Uint64
}

// NewEntryIndexProvider 创建索引提供器
func NewEntryIndexProvider() *EntryIndexProvider {
	return &EntryIndexProvider{}
}

// StartFrom 从现有历史中初始化（找到最大索引）
func (p *EntryIndexProvider) StartFrom(history []models.LogMsg) {
	var maxIndex uint64
	for _, msg := range history {
		if msg.Entry != nil {
			// 从 entry ID 中提取索引（如果有的话）
			// 这里简单处理，实际可能需要解析 ID
		}
		if uint64(msg.Sequence) > maxIndex {
			maxIndex = uint64(msg.Sequence)
		}
	}
	p.counter.Store(maxIndex)
}

// Next 获取下一个索引
func (p *EntryIndexProvider) Next() uint {
	return uint(p.counter.Add(1))
}

// MsgStoreManager 管理所有执行的 MsgStore
type MsgStoreManager struct {
	mu     sync.RWMutex
	stores map[uint]*MsgStore
}

var globalMsgStoreManager *MsgStoreManager
var msgStoreManagerOnce sync.Once

// GetMsgStoreManager 获取全局 MsgStore 管理器
func GetMsgStoreManager() *MsgStoreManager {
	msgStoreManagerOnce.Do(func() {
		globalMsgStoreManager = &MsgStoreManager{
			stores: make(map[uint]*MsgStore),
		}
	})
	return globalMsgStoreManager
}

// GetOrCreate 获取或创建 MsgStore
func (m *MsgStoreManager) GetOrCreate(executionID uint) *MsgStore {
	m.mu.Lock()
	defer m.mu.Unlock()

	if store, ok := m.stores[executionID]; ok {
		return store
	}

	// 尝试从数据库恢复
	store := NewMsgStoreFromDB(executionID)
	m.stores[executionID] = store
	return store
}

// Get 获取 MsgStore（不自动创建）
func (m *MsgStoreManager) Get(executionID uint) *MsgStore {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.stores[executionID]
}

// Remove 移除 MsgStore（执行完成后清理内存）
func (m *MsgStoreManager) Remove(executionID uint) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.stores, executionID)
}

// TaskMessageToLogMsg 将旧的 TaskMessage 转换为 LogMsg（兼容）
func TaskMessageToLogMsg(msg *models.TaskMessage) models.LogMsg {
	logMsg := models.LogMsg{
		Content: fmt.Sprintf("%v", msg.Content),
	}

	switch msg.Type {
	case "stdout":
		logMsg.Type = models.LogMsgTypeStdout
	case "stderr":
		logMsg.Type = models.LogMsgTypeStderr
	case "normalized_entry":
		logMsg.Type = models.LogMsgTypeNormalizedEntry
		logMsg.Entry = msg.Entry
	default:
		logMsg.Type = models.LogMsgTypeStdout
	}

	return logMsg
}

// LogMsgToJSON 转换为 WebSocket 消息格式
func LogMsgToJSON(msg models.LogMsg) []byte {
	bytes, _ := json.Marshal(msg)
	return bytes
}
