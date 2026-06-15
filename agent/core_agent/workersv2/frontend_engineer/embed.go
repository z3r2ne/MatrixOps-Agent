package frontend_engineer

import _ "embed"

//go:embed frontend_engineer.yaml
var builtinFrontendEngineerYAML []byte

// BuiltinDefinitionYAML 返回嵌入的 frontend_engineer worker 定义（供数据库种子使用）。
func BuiltinDefinitionYAML() []byte {
	return builtinFrontendEngineerYAML
}
