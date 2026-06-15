package code_map

import _ "embed"

//go:embed code_map.yaml
var builtinCodeMapYAML []byte

// BuiltinDefinitionYAML 返回嵌入的 code_map worker 定义（供数据库种子使用）。
func BuiltinDefinitionYAML() []byte {
	return builtinCodeMapYAML
}
