package models

// 本包定义了存储层使用的模型和常量

// 存储键的结构定义
// Session: ["session", projectID, sessionID]
// Message: ["message", sessionID, messageID]
// Part: ["part", messageID, partID]
// SessionDiff: ["session_diff", sessionID]
// Todo: ["todo", sessionID]
// Share: ["share", sessionID]

const (
	// KeyPrefixSession 会话键前缀
	KeyPrefixSession = "session"
	// KeyPrefixMessage 消息键前缀
	KeyPrefixMessage = "message"
	// KeyPrefixPart 部件键前缀
	KeyPrefixPart = "part"
	// KeyPrefixSessionDiff 会话差异键前缀
	KeyPrefixSessionDiff = "session_diff"
	// KeyPrefixTodo 待办事项键前缀
	KeyPrefixTodo = "todo"
	// KeyPrefixShare 分享键前缀
	KeyPrefixShare = "share"
)
