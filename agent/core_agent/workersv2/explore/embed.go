package explore

import _ "embed"

//go:embed explore.yaml
var builtinExploreYAML []byte

// BuiltinDefinitionYAML 返回嵌入的 explore worker 定义（供数据库种子使用）。
func BuiltinDefinitionYAML() []byte {
	return builtinExploreYAML
}
