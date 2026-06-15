package builtin

import "matrixops-agent/skills/builtin/research"

// ReadAll 返回所有内置技能的文件内容。key 为技能目录下的相对路径，如 research/SKILL.md。
func ReadAll() map[string][]byte {
	return map[string][]byte{
		"research/SKILL.md": research.BuiltinSkillMD(),
	}
}
