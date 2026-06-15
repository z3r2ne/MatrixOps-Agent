package services

import (
	"log"
	"os/exec"
	"sync"
	"time"
)

// ResidentProcess 常驻进程信息
type ResidentProcess struct {
	TaskID      uint
	Cmd         *exec.Cmd
	Stdin       *StdinWrapper
	LastUsedAt  time.Time
	SessionID   string
	WorkDir     string
}

// StdinWrapper stdin 包装器，支持多次写入
type StdinWrapper struct {
	stdin  interface{ Write([]byte) (int, error) }
	closed bool
	mu     sync.Mutex
}

func NewStdinWrapper(stdin interface{ Write([]byte) (int, error) }) *StdinWrapper {
	return &StdinWrapper{stdin: stdin}
}

func (w *StdinWrapper) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return 0, nil
	}
	return w.stdin.Write(data)
}

func (w *StdinWrapper) Close() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.closed = true
}

// ProcessManager 进程管理器
type ProcessManager struct {
	processes map[uint]*ResidentProcess // taskID -> process
	mu        sync.RWMutex
}

var (
	processManager     *ProcessManager
	processManagerOnce sync.Once
)

// GetProcessManager 获取全局进程管理器
func GetProcessManager() *ProcessManager {
	processManagerOnce.Do(func() {
		processManager = &ProcessManager{
			processes: make(map[uint]*ResidentProcess),
		}
		// 启动清理协程（清理超过1小时未使用的进程）
		go processManager.startCleaner()
	})
	return processManager
}

// Register 注册常驻进程
func (pm *ProcessManager) Register(taskID uint, cmd *exec.Cmd, stdin *StdinWrapper, sessionID string, workDir string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// 如果已有旧进程，先清理
	if old, exists := pm.processes[taskID]; exists {
		log.Printf("[ProcessManager] 清理任务 %d 的旧进程", taskID)
		if old.Cmd != nil && old.Cmd.Process != nil {
			old.Cmd.Process.Kill()
		}
	}

	pm.processes[taskID] = &ResidentProcess{
		TaskID:     taskID,
		Cmd:        cmd,
		Stdin:      stdin,
		LastUsedAt: time.Now(),
		SessionID:  sessionID,
		WorkDir:    workDir,
	}

	log.Printf("[ProcessManager] 注册任务 %d 的常驻进程，session: %s", taskID, sessionID)
}

// Get 获取常驻进程
func (pm *ProcessManager) Get(taskID uint) (*ResidentProcess, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	process, exists := pm.processes[taskID]
	if exists {
		// 检查进程是否还在运行
		if process.Cmd.Process != nil {
			// 尝试发送信号 0 检查进程是否存活
			if err := process.Cmd.Process.Signal(nil); err == nil {
				process.LastUsedAt = time.Now()
				return process, true
			}
			log.Printf("[ProcessManager] 任务 %d 的进程已退出", taskID)
		}
	}
	return nil, false
}

// UpdateLastUsed 更新最后使用时间
func (pm *ProcessManager) UpdateLastUsed(taskID uint) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if process, exists := pm.processes[taskID]; exists {
		process.LastUsedAt = time.Now()
	}
}

// Remove 移除常驻进程（手动停止）
func (pm *ProcessManager) Remove(taskID uint) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if process, exists := pm.processes[taskID]; exists {
		log.Printf("[ProcessManager] 移除任务 %d 的常驻进程", taskID)
		if process.Cmd != nil && process.Cmd.Process != nil {
			process.Cmd.Process.Kill()
		}
		delete(pm.processes, taskID)
	}
}

// IsActive 检查任务是否有活跃的常驻进程
func (pm *ProcessManager) IsActive(taskID uint) bool {
	_, exists := pm.Get(taskID)
	return exists
}

// List 列出所有活跃的常驻进程
func (pm *ProcessManager) List() []uint {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	taskIDs := make([]uint, 0, len(pm.processes))
	for taskID := range pm.processes {
		taskIDs = append(taskIDs, taskID)
	}
	return taskIDs
}

// startCleaner 启动清理协程
func (pm *ProcessManager) startCleaner() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		pm.cleanInactive()
	}
}

// cleanInactive 清理不活跃的进程（超过1小时未使用）
func (pm *ProcessManager) cleanInactive() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	now := time.Now()
	timeout := 1 * time.Hour

	for taskID, process := range pm.processes {
		if now.Sub(process.LastUsedAt) > timeout {
			log.Printf("[ProcessManager] 清理任务 %d 的不活跃进程（超过 %v 未使用）", taskID, timeout)
			if process.Cmd != nil && process.Cmd.Process != nil {
				process.Cmd.Process.Kill()
			}
			delete(pm.processes, taskID)
		}
	}
}

// CleanupAll 清理所有进程（服务器关闭时调用）
func (pm *ProcessManager) CleanupAll() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	log.Printf("[ProcessManager] 开始清理所有常驻进程（共 %d 个）", len(pm.processes))

	for taskID, process := range pm.processes {
		log.Printf("[ProcessManager] 清理任务 %d 的进程", taskID)
		if process.Cmd != nil && process.Cmd.Process != nil {
			if err := process.Cmd.Process.Kill(); err != nil {
				log.Printf("[ProcessManager] 清理任务 %d 的进程失败: %v", taskID, err)
			}
		}
		delete(pm.processes, taskID)
	}

	log.Printf("[ProcessManager] 所有常驻进程已清理完成")
}
