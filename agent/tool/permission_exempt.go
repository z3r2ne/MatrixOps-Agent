package tool

import "strings"

// permissionExemptTools 不参与项目「询问」流程的内部/元工具。
// 未在项目权限表中显式配置时视为 allow，避免与并发工具权限弹窗互相阻塞。
var permissionExemptTools = map[string]struct{}{
	setToolStallTimeoutToolName: {},
	"question":                  {},
}

// IsPermissionExemptTool reports whether a tool should skip project permission prompts by default.
func IsPermissionExemptTool(name string) bool {
	_, ok := permissionExemptTools[strings.TrimSpace(name)]
	return ok
}
