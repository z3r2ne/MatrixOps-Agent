package promptdefs

import (
	"embed"
	"strings"
)

const TaskLoopGuidanceMarker = "当前对话只是整个任务流程中的一轮"

//go:embed prompts/global.md
var embeddedGlobalPrompt string

//go:embed prompts/occupations/*.md
var embeddedOccupationPrompts embed.FS

func TaskLoopGuidanceText() string {
	return strings.TrimSpace(embeddedGlobalPrompt)
}

func DefaultGlobalPrompt() string {
	return TaskLoopGuidanceText()
}

func DefaultOccupationPrompt(code string) string {
	name := strings.TrimSpace(code)
	if name == "" {
		return ""
	}
	content, err := embeddedOccupationPrompts.ReadFile("prompts/occupations/" + name + ".md")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(content))
}
