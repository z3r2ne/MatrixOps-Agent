package session

import (
	"strings"

	"pkgs/promptdefs"
)

const taskLoopGuidanceMarker = promptdefs.TaskLoopGuidanceMarker

var taskLoopGuidancePrompt = promptdefs.TaskLoopGuidanceText()

func taskLoopGuidanceText() string {
	return promptdefs.TaskLoopGuidanceText()
}

func promptHasTaskLoopGuidance(prompt string) bool {
	return strings.Contains(strings.TrimSpace(prompt), taskLoopGuidanceMarker)
}

func appendTaskLoopGuidance(prompt string) string {
	trimmed := strings.TrimSpace(prompt)
	if promptHasTaskLoopGuidance(trimmed) {
		return trimmed
	}

	guidance := taskLoopGuidanceText()
	if trimmed == "" {
		return guidance
	}
	return trimmed + "\n\n" + guidance
}
