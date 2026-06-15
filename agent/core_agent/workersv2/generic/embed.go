package generic

import _ "embed"

//go:embed chat.yaml
var builtinChatYAML []byte

// BuiltinChatYAML 返回嵌入的 chat worker 定义（供数据库种子使用）。
func BuiltinChatYAML() []byte {
	return builtinChatYAML
}
