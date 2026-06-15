package models

import "strings"

type TaskListGroupMode string

const (
	TaskListGroupModeNone    TaskListGroupMode = "none"
	TaskListGroupModeDate    TaskListGroupMode = "date"
	TaskListGroupModeProject TaskListGroupMode = "project"
)

const DefaultTaskListGroupMode = TaskListGroupModeProject

func NormalizeTaskListGroupMode(value string) (TaskListGroupMode, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case string(TaskListGroupModeNone):
		return TaskListGroupModeNone, true
	case string(TaskListGroupModeDate):
		return TaskListGroupModeDate, true
	case string(TaskListGroupModeProject):
		return TaskListGroupModeProject, true
	default:
		return "", false
	}
}

func TaskListGroupModeOrDefault(value string) TaskListGroupMode {
	if mode, ok := NormalizeTaskListGroupMode(value); ok {
		return mode
	}
	return DefaultTaskListGroupMode
}
