package clawbot

import _ "embed"

//go:embed clawbot.yaml
var builtinYAML []byte

func BuiltinDefinitionYAML() []byte {
	return builtinYAML
}
