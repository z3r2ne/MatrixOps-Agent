package terminal

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	database "pkgs/db"
	"pkgs/db/models"
	"pkgs/shellutil"

	"github.com/creack/pty"
	"gorm.io/gorm"
)

const (
	maxBufferBytes     = 1 << 20
	defaultTerminalCol = 120
	defaultTerminalRow = 32
)

type SessionSnapshot struct {
	ID      string `json:"id"`
	WorkDir string `json:"workDir"`
	Closed  bool   `json:"closed"`
}

type PollResult struct {
	Output  string `json:"output"`
	Cursor  int64  `json:"cursor"`
	Closed  bool   `json:"closed"`
	WorkDir string `json:"workDir"`
}

type Manager struct {
	db       *gorm.DB
	mu       sync.RWMutex
	sessions map[string]*Session
}

type Session struct {
	id      string
	workDir string
	cmd     *exec.Cmd
	ptyFile *os.File

	mu        sync.Mutex
	buffer    []byte
	base      int64
	closed    bool
	closeOnce sync.Once
}

func NewManager(db *gorm.DB) *Manager {
	return &Manager{
		db:       db,
		sessions: make(map[string]*Session),
	}
}

func (m *Manager) Create(workDir string) (*SessionSnapshot, error) {
	if m == nil {
		return nil, fmt.Errorf("terminal manager is required")
	}
	resolvedWorkDir, err := resolveWorkDir(workDir)
	if err != nil {
		return nil, err
	}
	shellInfo, err := resolveShell(m.db)
	if err != nil {
		return nil, err
	}
	executable := strings.TrimSpace(shellInfo.Path)
	if executable == "" {
		executable = strings.TrimSpace(shellInfo.Command)
	}
	if executable == "" {
		return nil, fmt.Errorf("shell executable is empty")
	}

	cmd := exec.Command(executable, shellutil.InteractiveShellArgs(shellInfo)...)
	cmd.Dir = resolvedWorkDir
	cmd.Env = shellutil.AugmentEnv(append(os.Environ(), "TERM=xterm-256color", "COLORTERM=truecolor"))

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{
		Cols: defaultTerminalCol,
		Rows: defaultTerminalRow,
	})
	if err != nil {
		return nil, err
	}

	session := &Session{
		id:      newSessionID(),
		workDir: resolvedWorkDir,
		cmd:     cmd,
		ptyFile: ptmx,
	}

	m.mu.Lock()
	m.sessions[session.id] = session
	m.mu.Unlock()

	go session.readLoop(m)

	return &SessionSnapshot{
		ID:      session.id,
		WorkDir: session.workDir,
		Closed:  false,
	}, nil
}

func (m *Manager) Poll(sessionID string, cursor int64) (*PollResult, error) {
	session, err := m.get(sessionID)
	if err != nil {
		return nil, err
	}
	return session.poll(cursor), nil
}

func (m *Manager) Write(sessionID, input string) error {
	session, err := m.get(sessionID)
	if err != nil {
		return err
	}
	return session.write(input)
}

func (m *Manager) Resize(sessionID string, cols, rows uint16) error {
	session, err := m.get(sessionID)
	if err != nil {
		return err
	}
	return session.resize(cols, rows)
}

func (m *Manager) Close(sessionID string) error {
	session, err := m.get(sessionID)
	if err != nil {
		return err
	}
	session.close()
	m.mu.Lock()
	delete(m.sessions, sessionID)
	m.mu.Unlock()
	return nil
}

func (m *Manager) CleanupAll() {
	if m == nil {
		return
	}
	m.mu.Lock()
	sessions := make([]*Session, 0, len(m.sessions))
	for _, session := range m.sessions {
		sessions = append(sessions, session)
	}
	m.sessions = make(map[string]*Session)
	m.mu.Unlock()

	for _, session := range sessions {
		session.close()
	}
}

func (m *Manager) get(sessionID string) (*Session, error) {
	m.mu.RLock()
	session, ok := m.sessions[strings.TrimSpace(sessionID)]
	m.mu.RUnlock()
	if !ok || session == nil {
		return nil, fmt.Errorf("terminal session not found")
	}
	return session, nil
}

func (s *Session) readLoop(manager *Manager) {
	buf := make([]byte, 4096)
	for {
		n, err := s.ptyFile.Read(buf)
		if n > 0 {
			s.append(buf[:n])
		}
		if err != nil {
			if err != io.EOF {
				s.append([]byte("\r\n[terminal closed]\r\n"))
			}
			s.markClosed()
			manager.mu.Lock()
			delete(manager.sessions, s.id)
			manager.mu.Unlock()
			return
		}
	}
}

func (s *Session) append(chunk []byte) {
	if len(chunk) == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.buffer = append(s.buffer, chunk...)
	if len(s.buffer) > maxBufferBytes {
		extra := len(s.buffer) - maxBufferBytes
		s.base += int64(extra)
		s.buffer = append([]byte(nil), s.buffer[extra:]...)
	}
}

func (s *Session) poll(cursor int64) *PollResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	if cursor < s.base {
		cursor = s.base
	}
	end := s.base + int64(len(s.buffer))
	startIndex := int(cursor - s.base)
	if startIndex < 0 {
		startIndex = 0
	}
	if startIndex > len(s.buffer) {
		startIndex = len(s.buffer)
	}
	return &PollResult{
		Output:  string(s.buffer[startIndex:]),
		Cursor:  end,
		Closed:  s.closed,
		WorkDir: s.workDir,
	}
}

func (s *Session) write(input string) error {
	s.mu.Lock()
	closed := s.closed
	s.mu.Unlock()
	if closed {
		return fmt.Errorf("terminal session already closed")
	}
	_, err := s.ptyFile.Write([]byte(input))
	return err
}

func (s *Session) resize(cols, rows uint16) error {
	if cols == 0 || rows == 0 {
		return nil
	}
	s.mu.Lock()
	closed := s.closed
	s.mu.Unlock()
	if closed {
		return nil
	}
	return pty.Setsize(s.ptyFile, &pty.Winsize{Cols: cols, Rows: rows})
}

func (s *Session) close() {
	s.closeOnce.Do(func() {
		s.markClosed()
		if s.ptyFile != nil {
			_ = s.ptyFile.Close()
		}
		if s.cmd != nil && s.cmd.Process != nil {
			_ = s.cmd.Process.Kill()
			_, _ = s.cmd.Process.Wait()
		}
	})
}

func (s *Session) markClosed() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
}

func resolveShell(db *gorm.DB) (shellutil.Info, error) {
	selectedShell := ""
	customShell := ""
	if db != nil {
		if item, err := database.GetGlobalConfigByKey(db, models.ConfigKeyDefaultShell); err == nil {
			selectedShell = item.Value
		}
		if item, err := database.GetGlobalConfigByKey(db, models.ConfigKeyCustomShellCommand); err == nil {
			customShell = item.Value
		}
	}
	return shellutil.Resolve(selectedShell, customShell)
}

func newSessionID() string {
	var raw [12]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return fmt.Sprintf("term-%d", os.Getpid())
	}
	return hex.EncodeToString(raw[:])
}

func resolveWorkDir(workDir string) (string, error) {
	trimmed := strings.TrimSpace(workDir)
	if trimmed == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		trimmed = cwd
	}
	absolute, err := filepath.Abs(trimmed)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("workDir is not a directory")
	}
	return absolute, nil
}
