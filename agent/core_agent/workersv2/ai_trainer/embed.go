package ai_trainer

import _ "embed"

//go:embed ai_trainer.yaml
var builtinAITrainerYAML []byte

// BuiltinDefinitionYAML 返回嵌入的 ai_trainer worker 定义（供数据库种子使用）。
func BuiltinDefinitionYAML() []byte {
	return builtinAITrainerYAML
}
