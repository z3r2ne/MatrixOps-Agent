# core_agent

`core_agent` 是从 `agent/session` 主执行链中抽出来的独立 Go 库。

它的目标不是处理项目自己的数据库、会话存储或记忆落库，而是提供一套可复用的 Agent 运行时能力：

- 注册 action
- 注册工具
- 循环调用大模型
- 流式解析 action JSON
- 流式执行工具调用
- 通过 emitter 持续输出 message / part 更新
- 通过 hooks 让调用方注入 memory、token 持久化、step 后处理等逻辑

## 设计边界

`core_agent` 负责：

- Agent 运行循环
- `StreamV2` 流式 action 解析
- 默认 action: `message` / `answer` / `call_tool`
- 工具注册与执行
- 自定义 action 的注册接口

`core_agent` 不负责：

- 数据库存储
- 具体业务 memory 结构的落库
- 具体前端协议
- 业务特有 action 的语义处理

这些能力应由外层项目通过 adapter / hook 注入。

## 核心对象

- `Runner`: 执行循环入口
- `RunnerConfig`: 运行时配置
- `RunState`: 单次运行状态
- `ActionHandler`: 可注册的自定义 action
- `ToolRegistry`: 工具注册表
- `Emitter`: 输出 message / part 更新的接口

## 最小用法

```go
emitter := myEmitter{}
llmClient := myLLMClient{}
	runner, err := coreagent.NewRunner(coreagent.RunnerConfig{
		Emitter:   emitter,
		LLMClient: llmClient,
		PromptBuilder: func(state *coreagent.RunState) (string, error) {
			return buildPrompt(state), nil
		},
		Hooks: coreagent.RunnerHooks{
			BuildMemory: func(state *coreagent.RunState) (any, error) {
				return currentMemory(), nil
			},
			RecordAction: func(state *coreagent.RunState, rawOutput string, parts []*coreagent.Part) error {
				return recordToOuterMemory(rawOutput, parts)
			},
		},
	})
	if err != nil {
		return err
	}

	state := &coreagent.RunState{
		Context:   ctx,
		SessionID: "session-1",
		Assistant: &coreagent.Message{
			ID:        "msg-1",
			SessionID: "session-1",
			Role:      coreagent.RoleAssistant,
		},
		UserInput: "帮我分析这个问题",
	}

	return runner.Run(state)
```

## 自定义 action

外层可以通过 `RegisterAction` 注册业务特有 action。

典型做法：

1. 在外层定义 action schema
2. 在 handler 内解析 `action.Data`
3. 通过 `ctx.UpdatePart` 持续推送中间过程
4. 在外层处理真正的业务副作用

当前 `matrixops` 项目里的 `memory_organization` 就是这样接入的。

## 当前状态

当前已经稳定承载：

- `processV2` 主循环
- `StreamV2` 流式 action 解析
- `message/answer/call_tool` 流式执行
- 自定义 action 接入模式

如果后续继续库化，可以考虑再补：

- 更完整的示例目录
- 更细粒度的 hook 文档
- 通用的数组型 action helper
