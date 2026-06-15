package process

import (
	"os/exec"
	"sync"
	"time"
)

// ResidentProcess 常驻进程信息
type ResidentProcess struct {
	TaskID     uint
	Cmd        *exec.Cmd
	Stdin      *StdinWrapper
	LastUsedAt time.Time
	SessionID  string
	WorkDir    string
}

// StdinWrapper stdin 包装器，支持多次写入
type StdinWrapper struct {
	stdin  interface{ Write([]byte) (int, error) }
	closed bool
	mu     sync.Mutex
}

// NewStdinWrapper 创建 stdin 包装器
func NewStdinWrapper(stdin interface{ Write([]byte) (int, error) }) *StdinWrapper {
	return &StdinWrapper{stdin: stdin}
}

// Write 写入数据
func (w *StdinWrapper) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return 0, nil
	}
	return w.stdin.Write(data)
}

// Close 关闭包装器
func (w *StdinWrapper) Close() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.closed = true
}
