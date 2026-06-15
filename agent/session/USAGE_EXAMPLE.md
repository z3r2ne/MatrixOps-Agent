# Agent Session 使用示例

本文档描述当前 `agent/session` 的主执行链。旧版 `WorkflowService` 状态机已移除，普通任务现在直接进入 V2 执行内核。

## 基本使用

### 普通对话 / 任务执行

```go
input := "帮我实现一个用户登录功能"

_, err := session.Prompt(
    session.WithDB(db),
    session.WithSessionID(sessionID),
    session.WithDirectory(workDir),
    session.WithWorker("chat"),
    session.WithInputText(input),
)
if err != nil {
    return err
}
```

**执行链：**

```text
Prompt
  -> buildRuntimeConfig
  -> createUserMessage
  -> runTaskV2
  -> processV2
  -> generic.Worker.Chat
  -> core_agent.Runner
```

## 当前主流程

### 1. 构建运行时配置

`Prompt(options...)` 先通过 `NewAgentRunnerConfig` 和 `buildRuntimeConfig` 解析运行环境：

- 当前 session
- worker / model / model config
- project / workdir
- tools
- memory state
- 输入文本和附件 parts

### 2. 创建用户消息

`createUserMessage` 会：

- 创建用户 `message`
- 创建对应 `part`
- 展开输入附件
- 解析 mention
- 将用户输入写入会话消息表

支持的 mention 类型包括：

- `file://...`
- `review://...`
- `worker://...`
- `skill://...`
- `command://...`

### 3. 特殊 command 处理

当前 `command://default?name=compress` 会被解析为手动记忆压缩请求。

例如用户输入：

```text
@compress 压缩后不要丢失目标
```

后端会解析为：

- command: `compress`
- 用户附加压缩要求: `压缩后不要丢失目标`

然后设置：

```go
runtimeConfig.ManualMemoryCompactionRequested = true
runtimeConfig.ManualMemoryCompactionPrompt = userText
```

`Prompt` 会先执行新的记忆压缩逻辑，并通过 `compaction` part 把进度和结果推给前端。

### 4. 直接进入 V2 执行

非 command-only 请求会直接执行：

```go
r.runTaskV2(runtimeConfig)
```

`runTaskV2` 会调用：

```go
r.runTaskV2(runtimeConfig, ProcessInputV2{
    MaxSteps:            1000,
    ExecuteOnce:         false,
    UserPrompt:          runtimeConfig.UserInput,
    UserLLMContentParts: userImagePartsForLLM(runtimeConfig.Parts),
})
```

### 5. V2 执行内核

`processV2` 使用 `generic.Worker` 和 `core_agent.Runner` 执行模型循环。

每一轮主要步骤：

```text
PrepareMemory
BuildMemory
BuildPrompt
Stream model output
Parse actions/tool calls
Execute tools
Record action/memory
Persist tokens
AfterStep
OnAnswer
```

## 工具调用

工具由 worker 配置和默认 registry 决定。常见工具包括：

- `read`
- `write`
- `edit`
- `patch`
- `bash`
- `rg`
- `glob`
- `diff`
- `load_skill`
- `question`

模型输出 action 后，`core_agent.Runner` 会将 action 派发到对应 tool handler，tool 执行结果会更新为 `part` 并通过 emitter 推送。

## 记忆机制

当前记忆以 `memory_entries` 为主，构建 prompt 时会转为 messages 传入模型。

### 自动压缩

自动压缩检查点：

- 每轮模型调用前
- 每轮模型调用后

触发条件：

```text
当前 token >= 模型配置 contextLimit * 80%
```

触发后会压缩较早的 30% 记忆，并写回 `memory_entries`。

### 手动压缩

通过 `@compress` 触发。

手动压缩不会继续普通模型回复，只执行压缩并显示 `compaction` 卡片。

## 前端回流

执行过程中的消息更新通过 emitter 传给 task runner，再经 WebSocket 推送给前端。

主要事件：

- `EventMessageUpdated`
- `EventPartUpdated`
- `EventSessionCreated`
- `EventSessionError`
- `EventAssistantFooterStatus`

前端收到后展示：

- 普通文本
- reasoning
- tool call
- compaction
- memory organization
- error
- finish step

## 任务执行入口

### 新建任务

```text
frontend
  -> POST /projects/:id/tasks/run
  -> task_runner.CreateAndRunTask
  -> task_runner.RunTask
  -> TaskRuntime.Start
  -> runMatrixopsAgentPrompt
  -> session.Prompt
```

### 继续对话

```text
frontend WebSocket send_message
  -> GlobalWSHub.handleSendMessage
  -> task_runner.RunTask
  -> TaskRuntime.Start
  -> runMatrixopsAgentPrompt
  -> session.Prompt
```

## 当前推荐扩展点

### 新增用户命令

建议新增 command mention，而不是在前端直接调用业务接口。

推荐流程：

```text
前端插入 command://default?name=xxx
  -> createUserMessage 解析 command mention
  -> 设置 runtimeConfig 标志或参数
  -> Prompt 根据标志执行对应逻辑
  -> 通过 message/part 回流前端
```

### 新增工具

工具应注册到 `tool.Registry`，并通过 worker 的 enabled tools 控制是否可用。

### 新增 worker

worker 应通过数据库配置或内置 worker YAML 初始化，并绑定模型配置。

## 故障排查

### 模型没有继续执行

检查是否被 command-only 请求拦截，例如 `@compress` 会只执行压缩。

### 前端没有看到实时更新

检查：

- task 是否已订阅 WebSocket
- emitter 是否触发 `EventMessageUpdated`
- `TaskRuntime.onEmitterCreated` 是否成功读取 `message + parts`

### 自动压缩不触发

检查：

- 当前模型配置 `contextLimit`
- 当前 session tokens
- 是否达到 `contextLimit * 80%`

## 总结

当前 `agent/session` 的主执行模式已经简化为：

```text
用户输入
  -> createUserMessage
  -> command/mention preprocessing
  -> runTaskV2
  -> processV2
  -> core_agent.Runner
  -> tool/action execution
  -> message/part emitter
```

旧版 `WorkflowService` 状态机已不再参与主执行链。
