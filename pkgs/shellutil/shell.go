package shellutil

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	DefaultShellKey       = "default_shell"
	CustomShellCommandKey = "custom_shell_command"
	ShellCustom           = "custom"
)

type Info struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Command     string `json:"command"`
	Path        string `json:"path,omitempty"`
	IsAvailable bool   `json:"isAvailable"`
	IsCurrent   bool   `json:"isCurrent,omitempty"`
	IsCustom    bool   `json:"isCustom,omitempty"`
}

type CurrentResponse struct {
	Current Info   `json:"current"`
	Options []Info `json:"options"`
}

type spec struct {
	ID         string
	Name       string
	Command    string
	CommandWin string
}

var knownShells = []spec{
	{ID: "zsh", Name: "Zsh", Command: "zsh"},
	{ID: "bash", Name: "Bash", Command: "bash"},
	{ID: "fish", Name: "Fish", Command: "fish"},
	{ID: "sh", Name: "POSIX sh", Command: "sh"},
	{ID: "pwsh", Name: "PowerShell", Command: "pwsh", CommandWin: "pwsh.exe"},
	{ID: "powershell", Name: "Windows PowerShell", Command: "powershell", CommandWin: "powershell.exe"},
	{ID: "cmd", Name: "Command Prompt", Command: "cmd", CommandWin: "cmd.exe"},
}

func currentEnvShellPath() string {
	if runtime.GOOS == "windows" {
		if value := strings.TrimSpace(os.Getenv("COMSPEC")); value != "" {
			return value
		}
		return ""
	}
	return strings.TrimSpace(os.Getenv("SHELL"))
}

func Current() Info {
	if raw := currentEnvShellPath(); raw != "" {
		info := infoFromPath(raw)
		info.IsCurrent = true
		return info
	}

	return firstAvailableShell()
}

func Options() []Info {
	current := Current()
	seen := map[string]struct{}{}
	options := make([]Info, 0, len(knownShells)+1)

	for _, item := range knownShells {
		command := item.Command
		if runtime.GOOS == "windows" && item.CommandWin != "" {
			command = item.CommandWin
		}
		path, err := exec.LookPath(command)
		info := Info{
			ID:          item.ID,
			Name:        item.Name,
			Command:     command,
			Path:        path,
			IsAvailable: err == nil,
			IsCurrent:   current.ID == item.ID,
		}
		options = append(options, info)
		seen[item.ID] = struct{}{}
	}

	if current.IsCustom {
		if _, exists := seen[ShellCustom]; !exists {
			options = append(options, Info{
				ID:          ShellCustom,
				Name:        "自定义",
				Command:     current.Command,
				Path:        current.Path,
				IsAvailable: current.IsAvailable,
				IsCurrent:   true,
				IsCustom:    true,
			})
		}
	}

	return options
}

func firstAvailableShell() Info {
	for _, item := range knownShells {
		command := item.Command
		if runtime.GOOS == "windows" && item.CommandWin != "" {
			command = item.CommandWin
		}
		path, err := exec.LookPath(command)
		if err == nil {
			return Info{
				ID:          item.ID,
				Name:        item.Name,
				Command:     command,
				Path:        path,
				IsAvailable: true,
				IsCurrent:   true,
			}
		}
	}

	return Info{
		ID:          fallbackID(),
		Name:        fallbackName(),
		Command:     fallbackCommand(),
		Path:        fallbackCommand(),
		IsAvailable: false,
		IsCurrent:   true,
	}
}

func Resolve(selectedShell, customCommand string) (Info, error) {
	selectedShell = strings.TrimSpace(selectedShell)
	customCommand = strings.TrimSpace(customCommand)

	if selectedShell == "" {
		return Current(), nil
	}
	if selectedShell == ShellCustom {
		if customCommand == "" {
			return Info{}, errors.New("自定义 shell 不能为空")
		}
		info := infoFromPath(customCommand)
		info.IsCustom = true
		return info, nil
	}

	for _, item := range Options() {
		if item.ID == selectedShell {
			return item, nil
		}
	}

	return Info{}, errors.New("未找到 shell 配置")
}

func NormalizeForInit(selectedShell, customCommand string) (string, string) {
	selectedShell = strings.TrimSpace(selectedShell)
	customCommand = strings.TrimSpace(customCommand)
	if selectedShell != "" {
		return selectedShell, customCommand
	}

	current := Current()
	if current.IsCustom {
		return ShellCustom, current.Path
	}
	return current.ID, customCommand
}

func WrapCommand(info Info, command string) (string, []string, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return "", nil, errors.New("shell command is empty")
	}

	executable := strings.TrimSpace(info.Path)
	if executable == "" {
		executable = strings.TrimSpace(info.Command)
	}
	if executable == "" {
		return "", nil, errors.New("shell executable is empty")
	}
	if !isExecutable(executable) && !lookPathExists(executable) {
		return "", nil, errors.New("shell executable is not available")
	}

	switch normalizeID(info) {
	case "cmd":
		return executable, []string{"/C", command}, nil
	case "pwsh", "powershell":
		return executable, []string{"-Command", command}, nil
	default:
		return executable, []string{"-lc", command}, nil
	}
}

func infoFromPath(path string) Info {
	base := strings.ToLower(filepath.Base(strings.TrimSpace(path)))
	base = strings.TrimSuffix(base, ".exe")

	for _, item := range knownShells {
		if base == item.ID || base == strings.TrimSuffix(item.Command, ".exe") || base == strings.TrimSuffix(item.CommandWin, ".exe") {
			return Info{
				ID:          item.ID,
				Name:        item.Name,
				Command:     firstNonEmpty(item.CommandWinIfWindows(), item.Command),
				Path:        path,
				IsAvailable: isExecutable(path),
			}
		}
	}

	return Info{
		ID:          ShellCustom,
		Name:        "自定义",
		Command:     base,
		Path:        path,
		IsAvailable: isExecutable(path) || lookPathExists(path),
		IsCustom:    true,
	}
}

func (s spec) CommandWinIfWindows() string {
	if runtime.GOOS == "windows" && s.CommandWin != "" {
		return s.CommandWin
	}
	return ""
}

func normalizeID(info Info) string {
	if info.ID != "" {
		return info.ID
	}
	base := strings.ToLower(filepath.Base(info.Command))
	base = strings.TrimSuffix(base, ".exe")
	return base
}

func isExecutable(path string) bool {
	if path == "" {
		return false
	}
	stat, err := os.Stat(path)
	return err == nil && !stat.IsDir()
}

func lookPathExists(value string) bool {
	if value == "" {
		return false
	}
	_, err := exec.LookPath(value)
	return err == nil
}

func fallbackID() string {
	if runtime.GOOS == "windows" {
		return "cmd"
	}
	return "sh"
}

func fallbackName() string {
	if runtime.GOOS == "windows" {
		return "Command Prompt"
	}
	return "POSIX sh"
}

func fallbackCommand() string {
	if runtime.GOOS == "windows" {
		return "cmd"
	}
	return "sh"
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
