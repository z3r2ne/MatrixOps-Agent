package worker

import (
	"errors"
	"os/exec"
	"runtime"
	"strings"

	database "pkgs/db"
	"pkgs/db/models"
	"pkgs/shellutil"
)

// BuildCommand 构建 Worker 执行命令
func BuildCommand(worker models.Worker, cfg map[string]interface{}) (string, []string, error) {
	commandText, err := buildCommandText(worker, cfg)
	if err != nil {
		return "", nil, err
	}
	shellInfo, err := resolveShell(cfg)
	if err != nil {
		return "", nil, err
	}
	return shellutil.WrapCommand(shellInfo, commandText)
}

func buildCommandText(worker models.Worker, cfg map[string]interface{}) (string, error) {
	base := ToString(cfg["base_command_override"])
	if base == "" {
		base = defaultBaseCommand(worker.Provider)
	}
	base = strings.TrimSpace(base)
	if base == "" {
		return "", errors.New("无效执行命令")
	}

	commandText := base

	// 根据 vibe-kanban-main 的默认参数补齐
	if worker.Provider == "cursor" && strings.Contains(base, "cursor-agent") {
		commandText = appendShellArgs(commandText, "-p", "--output-format=stream-json")

		// 添加 --force 参数（强制允许命令执行）
		if force, ok := cfg["force"].(bool); ok && force {
			commandText = appendShellArgs(commandText, "--force")
		}

		// 添加 --model 参数（模型选择）
		if model := ToString(cfg["model"]); model != "" && model != "auto" {
			commandText = appendShellArgs(commandText, "--model", model)
		}
	}

	if extra, ok := cfg["additional_params"].([]interface{}); ok {
		for _, v := range extra {
			if value := ToString(v); value != "" {
				commandText = appendShellArgs(commandText, value)
			}
		}
	}

	return commandText, nil
}

// defaultBaseCommand 获取 provider 的默认命令
func defaultBaseCommand(provider string) string {
	switch provider {
	case "cursor":
		if _, err := exec.LookPath("cursor-agent"); err == nil {
			return "cursor-agent"
		}
		return "cursor"
	case "codex":
		if _, err := exec.LookPath("codex"); err == nil {
			return "codex"
		}
		return "npx -y @openai/codex@0.86.0"
	default:
		return provider
	}
}

func resolveShell(cfg map[string]interface{}) (shellutil.Info, error) {
	selectedShell := ToString(cfg["shell"])
	customShell := ToString(cfg["custom_shell_command"])
	if selectedShell == "" && database.DB != nil {
		if item, err := database.GetGlobalConfigByKey(database.DB, models.ConfigKeyDefaultShell); err == nil {
			selectedShell = item.Value
		}
	}
	if customShell == "" && database.DB != nil {
		if item, err := database.GetGlobalConfigByKey(database.DB, models.ConfigKeyCustomShellCommand); err == nil {
			customShell = item.Value
		}
	}
	return shellutil.Resolve(selectedShell, customShell)
}

func appendShellArgs(command string, args ...string) string {
	parts := []string{strings.TrimSpace(command)}
	for _, arg := range args {
		if strings.TrimSpace(arg) == "" {
			continue
		}
		parts = append(parts, shellQuote(arg))
	}
	return strings.Join(parts, " ")
}

func shellQuote(value string) string {
	if runtime.GOOS == "windows" {
		if strings.ContainsAny(value, " \t\"") {
			return `"` + strings.ReplaceAll(value, `"`, `\"`) + `"`
		}
		return value
	}
	return `'` + strings.ReplaceAll(value, `'`, `'\''`) + `'`
}
