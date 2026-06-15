package builtin

import (
	"matrixops.local/core_agent/workersv2/ai_trainer"
	"matrixops.local/core_agent/workersv2/clawbot"
	"matrixops.local/core_agent/workersv2/code_map"
	"matrixops.local/core_agent/workersv2/compaction"
	"matrixops.local/core_agent/workersv2/explore"
	"matrixops.local/core_agent/workersv2/frontend_engineer"
	"matrixops.local/core_agent/workersv2/generic"
	"matrixops.local/core_agent/workersv2/leader"
	"matrixops.local/core_agent/workersv2/plan"
	"matrixops.local/core_agent/workersv2/verification"
)

// ReadAll 返回所有内置 Worker 的 YAML 内容。key 仅用于排序与排查，无稳定语义承诺。
func ReadAll() map[string][]byte {
	return map[string][]byte{
		"ai_trainer/ai_trainer.yaml":               ai_trainer.BuiltinDefinitionYAML(),
		"clawbot/clawbot.yaml":                     clawbot.BuiltinDefinitionYAML(),
		"code_map/code_map.yaml":                   code_map.BuiltinDefinitionYAML(),
		"compaction/compaction.yaml":               compaction.BuiltinDefinitionYAML(),
		"explore/explore.yaml":                     explore.BuiltinDefinitionYAML(),
		"frontend_engineer/frontend_engineer.yaml": frontend_engineer.BuiltinDefinitionYAML(),
		"generic/chat.yaml":                        generic.BuiltinChatYAML(),
		"leader/leader.yaml":                       leader.BuiltinDefinitionYAML(),
		"plan/plan.yaml":                           plan.BuiltinDefinitionYAML(),
		"verification/verification.yaml":           verification.BuiltinDefinitionYAML(),
	}
}
