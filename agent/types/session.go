package types

import (
	"matrixops-agent/permission"
)

type Info struct {
	ID            string             `json:"id"`
	Slug          string             `json:"slug"`
	ProjectID     string             `json:"projectID"`
	Directory     string             `json:"directory"`
	WorkspaceRoot string             `json:"workspaceRoot,omitempty"`
	WorkspacePath string             `json:"workspacePath,omitempty"`
	EnabledSkills []string           `json:"enabledSkills,omitempty"`
	ParentID      string             `json:"parentID,omitempty"`
	Summary       *Summary           `json:"summary,omitempty"`
	Share         *ShareInfo         `json:"share,omitempty"`
	Title         string             `json:"title"`
	Version       string             `json:"version"`
	Time          TimeInfo           `json:"time"`
	Permission    permission.Ruleset `json:"permission,omitempty"`
	Revert        *RevertInfo        `json:"revert,omitempty"`
	StartSnapshot string             `json:"startSnapshot,omitempty"`
	Tokens        *MessageTokens     `json:"tokens,omitempty"`
}

type Summary struct {
	Additions int        `json:"additions"`
	Deletions int        `json:"deletions"`
	Files     int        `json:"files"`
	Diffs     []FileDiff `json:"diffs,omitempty"`
}

type ShareInfo struct {
	URL string `json:"url"`
}

type TimeInfo struct {
	Created    int64 `json:"created"`
	Updated    int64 `json:"updated"`
	Compacting int64 `json:"compacting,omitempty"`
	Archived   int64 `json:"archived,omitempty"`
}

type RevertInfo struct {
	MessageID string `json:"messageID"`
	PartID    string `json:"partID,omitempty"`
	Snapshot  string `json:"snapshot,omitempty"`
	Diff      string `json:"diff,omitempty"`
}

type SessionEvent struct {
	Info *Info `json:"info"`
}

type SessionDiffEvent struct {
	SessionID string     `json:"sessionID"`
	Diff      []FileDiff `json:"diff"`
}

type FileDiff struct {
	File      string `json:"file"`
	Before    string `json:"before"`
	After     string `json:"after"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
}

type SessionErrorEvent struct {
	SessionID string        `json:"sessionID,omitempty"`
	Error     *MessageError `json:"error"`
}

type PluginVarSetEvent struct {
	Key   string `json:"key"`
	Value any    `json:"value"`
}

type WaitUserInputEvent struct {
	Questions map[string]interface{} `json:"questions"`
}

type MessageEvent struct {
	Info *MessageInfo `json:"info"`
}

type MessageRemovedEvent struct {
	SessionID string `json:"sessionID"`
	MessageID string `json:"messageID"`
}

type PartEvent struct {
	Part  *Part  `json:"part"`
	Delta string `json:"delta,omitempty"`
}

type PartRemovedEvent struct {
	SessionID string `json:"sessionID"`
	MessageID string `json:"messageID"`
	PartID    string `json:"partID"`
}
