package models

import "strings"

type WorkspaceType string

const (
	WorkspaceTypeCode WorkspaceType = "code"
	WorkspaceTypeTest WorkspaceType = "test"
	WorkspaceTypeClaw WorkspaceType = "claw"
)

const DefaultWorkspaceType = WorkspaceTypeCode

func NormalizeWorkspaceType(value string) (WorkspaceType, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case string(WorkspaceTypeCode):
		return WorkspaceTypeCode, true
	case string(WorkspaceTypeTest):
		return WorkspaceTypeTest, true
	case string(WorkspaceTypeClaw):
		return WorkspaceTypeClaw, true
	default:
		return "", false
	}
}

func WorkspaceTypeOrDefault(value string) WorkspaceType {
	if t, ok := NormalizeWorkspaceType(value); ok {
		return t
	}
	return DefaultWorkspaceType
}
