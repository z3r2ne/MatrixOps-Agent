package tool

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"

	"github.com/creack/pty"
	"pkgs/shellutil"
)

type BashTool struct{}

var _ Tool = (*BashTool)(nil)

func (BashTool) Name() string {
	return "bash"
}

func (BashTool) VerbosName() string {
	return "命令执行"
}

func (BashTool) Description() string {
	return "执行命令行指令"
}

func (BashTool) Schema() map[string]interface{} {
	return ObjectParamSchema(map[string]interface{}{
		"command": map[string]interface{}{
			"type":        "string",
			"description": "The command to execute",
		},
		"workdir": map[string]interface{}{
			"type":        "string",
			"description": "The working directory for the command",
		},
	}, []string{"command"})
}

func (BashTool) Execute(ctx Context, input map[string]interface{}) (Result, error) {
	command, ok := input["command"].(string)
	if !ok || command == "" {
		return Result{IsError: true}, errors.New("bash: missing command")
	}
	workdir := ctx.Directory
	if dir, ok := input["workdir"].(string); ok && dir != "" {
		workdir = resolvePath(ctx.Directory, dir)
	}
	return runBashCommand(ctx, command, workdir)
}

func runBashCommand(ctx Context, command string, workdir string) (Result, error) {
	execCtx := ctx.Context
	if execCtx == nil {
		execCtx = context.Background()
	}

	result := Result{
		Name: "bash",
		Metadata: map[string]interface{}{
			"tty":        runtime.GOOS != "windows",
			"streamMode": "terminal",
			"cancelable": true,
			"command":    command,
			"workdir":    workdir,
		},
	}
	ctx.EmitEvent(StreamEvent{
		Status: "running",
		Metadata: map[string]interface{}{
			"tty":        runtime.GOOS != "windows",
			"streamMode": "terminal",
			"cancelable": true,
			"command":    command,
			"workdir":    workdir,
		},
	})

	if runtime.GOOS == "windows" {
		return runBashCommandWithPipes(execCtx, ctx, command, workdir, result)
	}
	return runBashCommandWithPTY(execCtx, ctx, command, workdir, result)
}

func runBashCommandWithPTY(execCtx context.Context, toolCtx Context, command string, workdir string, result Result) (Result, error) {
	cmd := buildCommand(execCtx, command)
	cmd.Dir = workdir
	cmd.Env = appendTerminalEnv(os.Environ())

	ptmx, err := pty.Start(cmd)
	if err != nil {
		result.IsError = true
		return result, err
	}
	defer func() { _ = ptmx.Close() }()

	buffer := &terminalOutputBuffer{}
	readErrCh := make(chan error, 1)

	go func() {
		defer close(readErrCh)
		streamReader := NewDeltaStreamReader(ptmx, DefaultStreamReaderInterval, func(delta string) error {
			buffer.Append(delta)
			toolCtx.EmitEvent(StreamEvent{
				Status:  "running",
				Stream:  "terminal",
				Content: delta,
				Metadata: map[string]interface{}{
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
	readErr := <-readErrCh
	if readErr != nil && waitErr == nil {
		waitErr = readErr
	}
	result.Content = buffer.Value()

	if exitErr, ok := waitErr.(*exec.ExitError); ok {
		result.Metadata["exitCode"] = exitErr.ExitCode()
	} else if waitErr == nil {
		result.Metadata["exitCode"] = 0
	}

	if errors.Is(execCtx.Err(), context.Canceled) {
		result.IsError = true
		result.Metadata["cancelled"] = true
		if cause := context.Cause(execCtx); cause != nil && cause != context.Canceled {
			result.Metadata["cancelCause"] = cause.Error()
			if errors.Is(cause, ErrToolExecutionCancelledByUser) {
				result.Metadata["cancelledBy"] = "user"
			}
		}
		return result, execCtx.Err()
	}
	if waitErr != nil {
		result.IsError = true
		return result, waitErr
	}
	return result, nil
}

func runBashCommandWithPipes(execCtx context.Context, toolCtx Context, command string, workdir string, result Result) (Result, error) {
	cmd := buildCommand(execCtx, command)
	cmd.Dir = workdir
	cmd.Env = appendTerminalEnv(os.Environ())

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		result.IsError = true
		return result, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		result.IsError = true
		return result, err
	}
	if err := cmd.Start(); err != nil {
		result.IsError = true
		return result, err
	}

	buffer := &terminalOutputBuffer{}
	var readWG sync.WaitGroup
	readErrCh := make(chan error, 2)
	emitTerminalDelta := func(stream string, tty bool) func(delta string) error {
		return func(delta string) error {
			buffer.Append(delta)
			toolCtx.EmitEvent(StreamEvent{
				Status:  "running",
				Stream:  stream,
				Content: delta,
				Metadata: map[string]interface{}{
					"tty":        tty,
					"streamMode": "terminal",
					"cancelable": true,
				},
			})
			return nil
		}
	}
	streamReader := func(stream string, reader io.Reader) {
		defer readWG.Done()
		deltaReader := NewDeltaStreamReader(reader, DefaultStreamReaderInterval, emitTerminalDelta(stream, false))
		_, readErr := io.ReadAll(deltaReader)
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			readErrCh <- readErr
		}
	}

	readWG.Add(2)
	go streamReader("stdout", stdout)
	go streamReader("stderr", stderr)
	waitErr := cmd.Wait()
	readWG.Wait()
	close(readErrCh)
	for readErr := range readErrCh {
		if waitErr == nil {
			waitErr = readErr
		}
	}
	result.Content = buffer.Value()

	if exitErr, ok := waitErr.(*exec.ExitError); ok {
		result.Metadata["exitCode"] = exitErr.ExitCode()
	} else if waitErr == nil {
		result.Metadata["exitCode"] = 0
	}

	if errors.Is(execCtx.Err(), context.Canceled) {
		result.IsError = true
		result.Metadata["cancelled"] = true
		if cause := context.Cause(execCtx); cause != nil && cause != context.Canceled {
			result.Metadata["cancelCause"] = cause.Error()
			if errors.Is(cause, ErrToolExecutionCancelledByUser) {
				result.Metadata["cancelledBy"] = "user"
			}
		}
		return result, execCtx.Err()
	}
	if waitErr != nil {
		result.IsError = true
		return result, waitErr
	}
	return result, nil
}

func buildCommand(ctx context.Context, command string) *exec.Cmd {
	info := shellutil.Current()
	cmdName, args, err := shellutil.WrapCommand(info, command)
	if err == nil {
		return buildExecCommand(ctx, cmdName, args...)
	}
	if runtime.GOOS == "windows" {
		return buildExecCommand(ctx, "cmd", "/C", command)
	}
	return buildExecCommand(ctx, "sh", "-lc", command)
}

func buildExecCommand(ctx context.Context, name string, args ...string) *exec.Cmd {
	if ctx != nil {
		return exec.CommandContext(ctx, name, args...)
	}
	return exec.Command(name, args...)
}

func appendTerminalEnv(env []string) []string {
	hasTerm := false
	hasGitPager := false
	hasPager := false
	hasLess := false
	for _, entry := range env {
		if strings.HasPrefix(entry, "TERM=") {
			hasTerm = true
		}
		if strings.HasPrefix(entry, "GIT_PAGER=") {
			hasGitPager = true
		}
		if strings.HasPrefix(entry, "PAGER=") {
			hasPager = true
		}
		if strings.HasPrefix(entry, "LESS=") {
			hasLess = true
		}
	}
	if !hasTerm {
		env = append(env, "TERM=xterm-256color")
	}
	if !hasGitPager {
		env = append(env, "GIT_PAGER=cat")
	}
	if !hasPager {
		env = append(env, "PAGER=cat")
	}
	if !hasLess {
		env = append(env, "LESS=FRX")
	}
	return env
}

type terminalOutputBuffer struct {
	mu      sync.Mutex
	content string
}

func (b *terminalOutputBuffer) Append(chunk string) string {
	b.mu.Lock()
	defer b.mu.Unlock()
	if chunk == "" {
		return b.content
	}
	b.content += chunk
	return b.content
}

func (b *terminalOutputBuffer) Value() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.content
}
