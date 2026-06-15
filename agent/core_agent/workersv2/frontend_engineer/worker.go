// Package frontend_engineer 提供面向「前端设计 / UI 实现」场景的 generic Worker 装配：主循环使用 promptbuilder.FrontendEngineerPromptBuilderName。
package frontend_engineer

import (
	coreagent "matrixops.local/core_agent"
	"matrixops.local/core_agent/promptbuilder"
	"matrixops.local/core_agent/workersv2/generic"
)

// WorkerName 建议与数据库 worker.name、YAML 配置中的标识一致。
const WorkerName = "frontend_engineer"

// PromptBuilderName 对应 promptbuilder 注册名。
const PromptBuilderName = promptbuilder.FrontendEngineerPromptBuilderName

// BaseOptions 返回绑定本 worker 主循环模板与逻辑名的 Option（不含 DB、LLM、Emitter 等）。
// loopOpts 可传入 ContextInfoBuilder 等与 coreagent.MustCreatePromptBuilder 一致的选项。
func BaseOptions(loopOpts coreagent.PromptBuilderOptions) []generic.Option {
	return []generic.Option{
		generic.WithName(WorkerName),
		generic.WithLoop(nil, PromptBuilderName, loopOpts),
	}
}

// New 先应用 BaseOptions(loopOpts)，再应用 opts，最后走 generic.New 的默认补齐与校验。
func New(loopOpts coreagent.PromptBuilderOptions, opts ...generic.Option) (*generic.Worker, error) {
	all := append(BaseOptions(loopOpts), opts...)
	return generic.New(all...)
}
