package session

import "matrixops-agent/types"

// 类型别名，保持向后兼容
type Role = types.Role
type MessageInfo = types.MessageInfo
type MessagePath = types.MessagePath
type MessageTime = types.MessageTime
type Part = types.Part
type PartTime = types.PartTime
type ToolPart = types.ToolPart
type ToolState = types.ToolState
type WithParts = types.WithParts
type MessageTokens = types.MessageTokens
type TokenCache = types.TokenCache
type MessageError = types.MessageError
type TextRange = types.TextRange
type LSPPosition = types.LSPPosition
type LSPRange = types.LSPRange
type FilePartSource = types.FilePartSource

// 常量
const (
	RoleUser      = types.RoleUser
	RoleAssistant = types.RoleAssistant

	MessageKindUser   = types.MessageKindUser
	MessageKindSystem = types.MessageKindSystem
)
