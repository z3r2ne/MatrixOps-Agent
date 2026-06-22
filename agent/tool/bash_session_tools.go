package tool

import (
	"errors"
	"fmt"
)

type ReadBashOutputTool struct{}

func (ReadBashOutputTool) Name() string { return "read_bash_output" }

func (ReadBashOutputTool) VerbosName() string { return "读取 Bash 输出" }

func (ReadBashOutputTool) Description() string {
	return "读取异步 bash 会话的最近输出。需要提供 bash_job_id（从 bash async=true 启动返回的 metadata 中获取）。"
}

func (ReadBashOutputTool) Schema() map[string]interface{} {
	return ObjectParamSchema(map[string]interface{}{
		"bash_job_id": map[string]interface{}{
			"type":        "string",
			"description": "bash 会话 ID，取自 bash 异步启动返回的 metadata.bashJobId。",
		},
		"max_bytes": map[string]interface{}{
			"type":        "integer",
			"description": "可选。返回输出尾部最大字节数，默认 8000。",
		},
	}, []string{"bash_job_id"})
}

func (ReadBashOutputTool) Execute(ctx Context, input map[string]interface{}) (Result, error) {
	jobID, err := parseBashJobID(input)
	if err != nil {
		return Result{IsError: true, Name: "read_bash_output"}, fmt.Errorf("read_bash_output: %w", err)
	}

	maxBytes := 8000
	if raw, ok := input["max_bytes"]; ok {
		if parsed, parseErr := parseOptionalUintID(raw); parseErr == nil && parsed > 0 {
			maxBytes = int(parsed)
		}
	}
	if maxBytes > defaultBashOutputMaxBytes {
		maxBytes = defaultBashOutputMaxBytes
	}

	output, status, err := globalBashSessionManager.ReadOutput(bashScopeKey(ctx.SessionID), jobID, maxBytes)
	if err != nil {
		return Result{IsError: true, Name: "read_bash_output"}, fmt.Errorf("read_bash_output: %w", err)
	}

	return Result{
		Name:    "read_bash_output",
		Content: output,
		Metadata: map[string]interface{}{
			"bashJobId": jobID,
			"status":    status,
			"maxBytes":  maxBytes,
		},
	}, nil
}

type SendBashCommandTool struct{}

func (SendBashCommandTool) Name() string { return "send_bash_command" }

func (SendBashCommandTool) VerbosName() string { return "向 Bash 发送命令" }

func (SendBashCommandTool) Description() string {
	return "向正在运行的异步 bash 会话发送命令（写入 stdin）。需要提供 bash_job_id。"
}

func (SendBashCommandTool) Schema() map[string]interface{} {
	return ObjectParamSchema(map[string]interface{}{
		"bash_job_id": map[string]interface{}{
			"type":        "string",
			"description": "bash 会话 ID，取自 bash 异步启动返回的 metadata.bashJobId。",
		},
		"command": map[string]interface{}{
			"type":        "string",
			"description": "要发送到 bash 会话的命令。",
		},
	}, []string{"bash_job_id", "command"})
}

func (SendBashCommandTool) Execute(ctx Context, input map[string]interface{}) (Result, error) {
	jobID, err := parseBashJobID(input)
	if err != nil {
		return Result{IsError: true, Name: "send_bash_command"}, fmt.Errorf("send_bash_command: %w", err)
	}
	command, ok := input["command"].(string)
	if !ok || command == "" {
		return Result{IsError: true, Name: "send_bash_command"}, errors.New("send_bash_command: missing command")
	}

	if err := globalBashSessionManager.SendInput(bashScopeKey(ctx.SessionID), jobID, command); err != nil {
		return Result{IsError: true, Name: "send_bash_command"}, fmt.Errorf("send_bash_command: %w", err)
	}

	return Result{
		Name:    "send_bash_command",
		Content: fmt.Sprintf("已向 bash 会话 %s 发送命令", jobID),
		Metadata: map[string]interface{}{
			"bashJobId": jobID,
			"command":   command,
		},
	}, nil
}

type StopBashTool struct{}

func (StopBashTool) Name() string { return "stop_bash" }

func (StopBashTool) VerbosName() string { return "结束 Bash 会话" }

func (StopBashTool) Description() string {
	return "结束一个正在运行的异步 bash 会话。需要提供 bash_job_id。"
}

func (StopBashTool) Schema() map[string]interface{} {
	return ObjectParamSchema(map[string]interface{}{
		"bash_job_id": map[string]interface{}{
			"type":        "string",
			"description": "要结束的 bash 会话 ID。",
		},
	}, []string{"bash_job_id"})
}

func (StopBashTool) Execute(ctx Context, input map[string]interface{}) (Result, error) {
	jobID, err := parseBashJobID(input)
	if err != nil {
		return Result{IsError: true, Name: "stop_bash"}, fmt.Errorf("stop_bash: %w", err)
	}

	if err := globalBashSessionManager.Stop(bashScopeKey(ctx.SessionID), jobID); err != nil {
		return Result{IsError: true, Name: "stop_bash"}, fmt.Errorf("stop_bash: %w", err)
	}

	return Result{
		Name:    "stop_bash",
		Content: fmt.Sprintf("已请求结束 bash 会话 %s", jobID),
		Metadata: map[string]interface{}{
			"bashJobId": jobID,
		},
	}, nil
}
