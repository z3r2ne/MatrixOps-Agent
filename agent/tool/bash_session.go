package tool

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/creack/pty"
)

const (
	bashSessionStatusRunning   = "running"
	bashSessionStatusCompleted = "completed"
	bashSessionStatusFailed    = "failed"
	bashSessionStatusCancelled = "cancelled"

	defaultBashOutputMaxBytes = 2 * 1024 * 1024
)

var globalBashSessionManager = &BashSessionManager{}

type BashSessionManager struct {
	mu       sync.RWMutex
	sessions map[string]*BashSession
}

type BashSession struct {
	ID       string
	ScopeKey string
	Command  string
	Workdir  string

	mu      sync.RWMutex
	status  string
	buffer  *terminalOutputBuffer
	stdin   io.WriteCloser
	cmd     *exec.Cmd
	cancel  context.CancelCauseFunc
	done    chan struct{}
	result  Result
	execErr error
}

func bashScopeKey(sessionID string) string {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return "global"
	}
	return sessionID
}

func sessionMapKey(scopeKey, jobID string) string {
	return scopeKey + ":" + strings.TrimSpace(jobID)
}

func newBashJobID() string {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return fmt.Sprintf("bash-%d", time.Now().UnixNano())
	}
	return "bash-" + hex.EncodeToString(buf[:])
}

func isAsyncBashExecution(ctx Context) bool {
	if ctx.Values == nil {
		return false
	}
	value, ok := ctx.Values[ToolContextAsyncBashKey].(bool)
	return ok && value
}

func signalBashJobReady(ctx Context, jobID string) {
	if ctx.Values == nil {
		return
	}
	raw, ok := ctx.Values[ToolContextBashJobReadyKey]
	if !ok {
		return
	}
	ch, ok := raw.(chan string)
	if !ok || ch == nil {
		return
	}
	select {
	case ch <- jobID:
	default:
	}
}

func (m *BashSessionManager) Start(scopeKey string, toolCtx Context, command, workdir string) (*BashSession, error) {
	scopeKey = strings.TrimSpace(scopeKey)
	if scopeKey == "" {
		scopeKey = "global"
	}
	command = strings.TrimSpace(command)
	if command == "" {
		return nil, errors.New("bash: missing command")
	}

	session := &BashSession{
		ID:       newBashJobID(),
		ScopeKey: scopeKey,
		Command:  command,
		Workdir:  workdir,
		status:   bashSessionStatusRunning,
		buffer:   &terminalOutputBuffer{},
		done:     make(chan struct{}),
	}

	m.mu.Lock()
	if m.sessions == nil {
		m.sessions = map[string]*BashSession{}
	}
	m.sessions[sessionMapKey(scopeKey, session.ID)] = session
	m.mu.Unlock()

	go session.run(toolCtx)
	return session, nil
}

func (m *BashSessionManager) Get(scopeKey, jobID string) (*BashSession, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.sessions == nil {
		return nil, false
	}
	session, ok := m.sessions[sessionMapKey(bashScopeKey(scopeKey), jobID)]
	return session, ok
}

func (m *BashSessionManager) ReadOutput(scopeKey, jobID string, maxBytes int) (output string, status string, err error) {
	session, ok := m.Get(scopeKey, jobID)
	if !ok {
		return "", "", fmt.Errorf("bash session %q not found", jobID)
	}
	if maxBytes <= 0 {
		maxBytes = 8000
	}
	session.mu.RLock()
	defer session.mu.RUnlock()
	return session.buffer.Tail(maxBytes), session.status, nil
}

func (m *BashSessionManager) SendInput(scopeKey, jobID, command string) error {
	session, ok := m.Get(scopeKey, jobID)
	if !ok {
		return fmt.Errorf("bash session %q not found", jobID)
	}
	command = strings.TrimSpace(command)
	if command == "" {
		return errors.New("send_bash_command: missing command")
	}
	if !strings.HasSuffix(command, "\n") {
		command += "\n"
	}

	session.mu.RLock()
	status := session.status
	stdin := session.stdin
	session.mu.RUnlock()

	if status != bashSessionStatusRunning {
		return fmt.Errorf("bash session %q is not running (status=%s)", jobID, status)
	}
	if stdin == nil {
		return fmt.Errorf("bash session %q does not accept input", jobID)
	}
	if _, err := stdin.Write([]byte(command)); err != nil {
		return fmt.Errorf("send_bash_command: %w", err)
	}
	return nil
}

func (m *BashSessionManager) Stop(scopeKey, jobID string) error {
	session, ok := m.Get(scopeKey, jobID)
	if !ok {
		return fmt.Errorf("bash session %q not found", jobID)
	}
	return session.stop(ErrToolExecutionCancelledByUser)
}

func (s *BashSession) wait() (Result, error) {
	<-s.done
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.result, s.execErr
}

func (s *BashSession) stop(cause error) error {
	s.mu.RLock()
	cmd := s.cmd
	stdin := s.stdin
	cancel := s.cancel
	status := s.status
	s.mu.RUnlock()
	if status != bashSessionStatusRunning {
		return fmt.Errorf("bash session %q is not running (status=%s)", s.ID, status)
	}
	if cause == nil {
		cause = ErrToolExecutionCancelledByUser
	}
	if cancel != nil {
		cancel(cause)
	}
	if stdin != nil {
		_ = stdin.Close()
	}
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
	return nil
}

func (s *BashSession) finish(status string, result Result, execErr error) {
	s.mu.Lock()
	s.status = status
	s.result = result
	s.execErr = execErr
	s.mu.Unlock()
	close(s.done)
}

func (s *BashSession) run(toolCtx Context) {
	parentCtx := toolCtx.Context
	if parentCtx == nil {
		parentCtx = context.Background()
	}
	execCtx, cancel := context.WithCancelCause(parentCtx)
	s.mu.Lock()
	s.cancel = cancel
	s.mu.Unlock()
	defer cancel(nil)

	result := Result{
		Name: "bash",
		Metadata: map[string]interface{}{
			"bashJobId":  s.ID,
			"async":      true,
			"tty":        runtime.GOOS != "windows",
			"streamMode": "terminal",
			"cancelable": true,
			"command":    s.Command,
			"workdir":    s.Workdir,
		},
	}

	toolCtx.EmitEvent(StreamEvent{
		Status: "running",
		Metadata: map[string]interface{}{
			"bashJobId":  s.ID,
			"async":      true,
			"tty":        runtime.GOOS != "windows",
			"streamMode": "terminal",
			"cancelable": true,
			"command":    s.Command,
			"workdir":    s.Workdir,
		},
	})

	signalBashJobReady(toolCtx, s.ID)

	var execErr error
	if runtime.GOOS == "windows" {
		execErr = s.runWithPipes(execCtx, toolCtx, &result)
	} else {
		execErr = s.runWithPTY(execCtx, toolCtx, &result)
	}

	status := bashSessionStatusCompleted
	if execErr != nil {
		if errors.Is(execCtx.Err(), context.Canceled) {
			status = bashSessionStatusCancelled
			result.IsError = true
			result.Metadata["cancelled"] = true
			if cause := context.Cause(execCtx); cause != nil && cause != context.Canceled {
				result.Metadata["cancelCause"] = cause.Error()
				if errors.Is(cause, ErrToolExecutionCancelledByUser) {
					result.Metadata["cancelledBy"] = "user"
				}
			}
		} else {
			status = bashSessionStatusFailed
			result.IsError = true
		}
	}

	result.Content = s.buffer.Value()
	s.finish(status, result, execErr)
}

func (s *BashSession) runWithPTY(execCtx context.Context, toolCtx Context, result *Result) error {
	cmd := buildCommand(execCtx, s.Command)
	cmd.Dir = s.Workdir
	cmd.Env = appendTerminalEnv(os.Environ())

	ptmx, err := pty.Start(cmd)
	if err != nil {
		result.IsError = true
		return err
	}

	s.mu.Lock()
	s.cmd = cmd
	s.stdin = ptmx
	s.mu.Unlock()

	defer func() { _ = ptmx.Close() }()

	readErrCh := make(chan error, 1)
	go func() {
		defer close(readErrCh)
		streamReader := NewDeltaStreamReader(ptmx, DefaultStreamReaderInterval, func(delta string) error {
			s.buffer.Append(delta)
			toolCtx.EmitEvent(StreamEvent{
				Status:  "running",
				Stream:  "terminal",
				Content: delta,
				Metadata: map[string]interface{}{
					"bashJobId":  s.ID,
					"async":      true,
					"tty":        true,
					"streamMode": "terminal",
					"cancelable": true,
				},
			})
			return nil
		})
		_, readErr := io.ReadAll(streamReader)
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			readErrCh <- readErr
			return
		}
		readErrCh <- nil
	}()

	waitErr := cmd.Wait()
	_ = ptmx.Close()
	readErr := <-readErrCh
	if readErr != nil && waitErr == nil {
		waitErr = readErr
	}

	if exitErr, ok := waitErr.(*exec.ExitError); ok {
		result.Metadata["exitCode"] = exitErr.ExitCode()
	} else if waitErr == nil {
		result.Metadata["exitCode"] = 0
	}

	if errors.Is(execCtx.Err(), context.Canceled) {
		return execCtx.Err()
	}
	return waitErr
}

func (s *BashSession) runWithPipes(execCtx context.Context, toolCtx Context, result *Result) error {
	cmd := buildCommand(execCtx, s.Command)
	cmd.Dir = s.Workdir
	cmd.Env = appendTerminalEnv(os.Environ())

	stdin, err := cmd.StdinPipe()
	if err != nil {
		result.IsError = true
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		result.IsError = true
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		result.IsError = true
		return err
	}
	if err := cmd.Start(); err != nil {
		result.IsError = true
		return err
	}

	s.mu.Lock()
	s.cmd = cmd
	s.stdin = stdin
	s.mu.Unlock()

	var readWG sync.WaitGroup
	readErrCh := make(chan error, 2)
	emitDelta := func(stream string) func(delta string) error {
		return func(delta string) error {
			s.buffer.Append(delta)
			toolCtx.EmitEvent(StreamEvent{
				Status:  "running",
				Stream:  stream,
				Content: delta,
				Metadata: map[string]interface{}{
					"bashJobId":  s.ID,
					"async":      true,
					"tty":        false,
					"streamMode": "terminal",
					"cancelable": true,
				},
			})
			return nil
		}
	}
	streamReader := func(stream string, reader io.Reader) {
		defer readWG.Done()
		deltaReader := NewDeltaStreamReader(reader, DefaultStreamReaderInterval, emitDelta(stream))
		_, readErr := io.ReadAll(deltaReader)
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			readErrCh <- readErr
		}
	}

	readWG.Add(2)
	go streamReader("stdout", stdout)
	go streamReader("stderr", stderr)

	waitErr := cmd.Wait()
	_ = stdin.Close()
	readWG.Wait()
	close(readErrCh)
	for readErr := range readErrCh {
		if waitErr == nil {
			waitErr = readErr
		}
	}

	if exitErr, ok := waitErr.(*exec.ExitError); ok {
		result.Metadata["exitCode"] = exitErr.ExitCode()
	} else if waitErr == nil {
		result.Metadata["exitCode"] = 0
	}

	if errors.Is(execCtx.Err(), context.Canceled) {
		return execCtx.Err()
	}
	return waitErr
}

func runAsyncBashSession(ctx Context, command, workdir string) (Result, error) {
	scopeKey := bashScopeKey(ctx.SessionID)
	session, err := globalBashSessionManager.Start(scopeKey, ctx, command, workdir)
	if err != nil {
		return Result{IsError: true, Name: "bash"}, err
	}
	result, execErr := session.wait()
	return result, execErr
}

func parseBashJobID(input map[string]interface{}) (string, error) {
	raw, ok := input["bash_job_id"]
	if !ok {
		raw = input["job_id"]
	}
	value, ok := raw.(string)
	if !ok {
		return "", errors.New("missing bash_job_id")
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "", errors.New("missing bash_job_id")
	}
	return value, nil
}
