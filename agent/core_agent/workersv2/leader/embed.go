package leader

import _ "embed"

//go:embed leader.yaml
var builtinLeaderYAML []byte

// BuiltinDefinitionYAML 返回嵌入的 leader worker 定义（供数据库种子使用）。
func BuiltinDefinitionYAML() []byte {
	return builtinLeaderYAML
}
