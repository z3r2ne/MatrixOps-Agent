package verification

import _ "embed"

//go:embed verification.yaml
var builtinVerificationYAML []byte

// BuiltinDefinitionYAML 返回嵌入的 verification worker 定义（供数据库种子使用）。
func BuiltinDefinitionYAML() []byte {
	return builtinVerificationYAML
}
