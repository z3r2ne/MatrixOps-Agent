package compaction

import _ "embed"

//go:embed compaction.yaml
var builtinDefinitionYAML []byte

// BuiltinDefinitionYAML 返回嵌入的 compaction worker 定义（供数据库种子使用）。
func BuiltinDefinitionYAML() []byte {
	return append([]byte(nil), builtinDefinitionYAML...)
}
