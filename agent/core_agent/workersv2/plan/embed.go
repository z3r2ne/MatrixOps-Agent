package plan

import _ "embed"

//go:embed plan.yaml
var builtinPlanYAML []byte

// BuiltinDefinitionYAML 返回嵌入的 plan worker 定义（供数据库种子使用）。
func BuiltinDefinitionYAML() []byte {
	return builtinPlanYAML
}
