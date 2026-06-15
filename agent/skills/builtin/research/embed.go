package research

import _ "embed"

//go:embed SKILL.md
var builtinSkillMD []byte

// BuiltinSkillMD 返回嵌入的调研技能定义。
func BuiltinSkillMD() []byte {
	return builtinSkillMD
}
