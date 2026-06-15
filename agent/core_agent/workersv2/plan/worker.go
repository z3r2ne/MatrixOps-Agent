// Package plan 提供面向「实施方案设计 / 关键文件梳理 / 风险与依赖拆解」场景的 generic Worker 装配：主循环使用默认任务模板。
package plan

import (
	coreagent "matrixops.local/core_agent"
	"matrixops.local/core_agent/workersv2/generic"
)

// WorkerName 建议与数据库 worker.name、YAML 配置中的标识一致。
const WorkerName = "plan"

// BaseOptions 返回绑定本 worker 逻辑名与默认主循环模板的 Option（不含 DB、LLM、Emitter 等）。
func BaseOptions(loopOpts coreagent.PromptBuilderOptions) []generic.Option {
	return []generic.Option{
		generic.WithName(WorkerName),
		generic.WithLoop(nil, coreagent.DefaultPromptBuilderName, loopOpts),
	}
}

// New 先应用 BaseOptions(loopOpts)，再应用 opts，最后走 generic.New 的默认补齐与校验。
func New(loopOpts coreagent.PromptBuilderOptions, opts ...generic.Option) (*generic.Worker, error) {
	all := append(BaseOptions(loopOpts), opts...)
	return generic.New(all...)
}
