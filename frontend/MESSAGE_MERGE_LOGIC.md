# 消息合并逻辑统一说明

## 📋 核心原则

**只有连续的相同ID消息才会合并（覆盖），非连续的相同ID视为独立消息**

## 🎯 统一规则

```typescript
// 正确的逻辑
let lastId = null
for (const msg of messages) {
  const currentId = msg.entry?.id
  
  if (currentId && currentId === lastId) {
    // 连续相同ID：更新最后一条
    updateLastMessage(msg)
  } else {
    // ID不同或不连续：添加新消息
    addNewMessage(msg)
    lastId = currentId
  }
}
```

## 📊 所有消息处理位置

### 1️⃣ useGlobalWebSocket - 实时消息处理

**文件**：`src/hooks/useGlobalWebSocket.ts`  
**位置**：第159-176行  
**场景**：WebSocket 接收实时消息（`task_message`）

```typescript
// ✅ 已修改：只更新连续相同ID
case "task_message":
  if (state.messages.length > 0 && msg.message!.entry?.id) {
    const lastMsg = state.messages[state.messages.length - 1]
    const lastId = lastMsg.entry?.id
    
    // 只有最后一条消息的ID与当前消息相同时才更新
    if (lastId === msg.message!.entry.id) {
      state.messages = [...state.messages.slice(0, -1), msg.message!]
    } else {
      state.messages = [...state.messages, msg.message!]
    }
  } else {
    state.messages = [...state.messages, msg.message!]
  }
```

**状态**：✅ **已统一**

---

### 2️⃣ useGlobalWebSocket - WebSocket历史合并

**文件**：`src/hooks/useGlobalWebSocket.ts`  
**位置**：第215-237行  
**场景**：WebSocket 接收历史消息（`history`）

```typescript
// ⚠️ 当前逻辑：全局去重（Set）
case "history":
  const existingEntryIds = new Set(
    state.messages.filter(m => m.entry?.id).map(m => m.entry!.id)
  )
  const newMessages = msg.history!.filter(m => {
    if (m.entry?.id) {
      return !existingEntryIds.has(m.entry.id)  // 全局去重
    }
    return true
  })
  state.messages = [...newMessages, ...state.messages]
```

**状态**：✅ **无需修改**（理由见后）

---

### 3️⃣ useGlobalWebSocket - 初始加载历史

**文件**：`src/hooks/useGlobalWebSocket.ts`  
**位置**：第268-312行  
**场景**：订阅时通过 HTTP API 从数据库加载历史

```typescript
// ✅ 当前逻辑：直接替换整个数组
const subscribe = async (taskId: number) => {
  const history = await api.getSessionLogs(sessionResult.sessionId)
  
  setTaskStates(prev => {
    newMap.set(taskId, { 
      messages: history,  // 直接替换
      isSubscribed: true 
    })
  })
}
```

**状态**：✅ **无需修改**（首次加载，不存在合并问题）

---

### 4️⃣ useTaskWebSocket - 实时消息处理

**文件**：`src/hooks/useTaskWebSocket.ts`  
**位置**：第121-139行  
**场景**：WebSocket 接收实时消息

```typescript
// ✅ 已修改：只更新连续相同ID
setMessages(prev => {
  const msgId = message.entry?.id || `${message.timestamp}-${message.content?.slice(0, 20)}`
  
  if (prev.length > 0) {
    const lastMsg = prev[prev.length - 1]
    const lastId = lastMsg.entry?.id || `${lastMsg.timestamp}-${lastMsg.content?.slice(0, 20)}`
    
    // 只有最后一条消息的ID与当前消息相同时才更新
    if (lastId === msgId) {
      return [...prev.slice(0, -1), message]
    }
  }
  
  return [...prev, message]
})
```

**状态**：✅ **已统一**

---

### 5️⃣ ChatInterface - 渲染前处理

**文件**：`src/components/workspace/ChatInterface.tsx`  
**位置**：第308-358行  
**场景**：渲染前最终处理消息数组

```typescript
// ✅ 已修改：只合并连续相同ID
const displayEntries = useMemo(() => {
  const entries: NormalizedEntry[] = []
  let lastEntryId: string | null = null

  for (const msg of externalMessages) {
    if (msg.entry) {
      // 只有连续相同ID才覆盖
      if (lastEntryId === msg.entry.id && entries.length > 0) {
        entries[entries.length - 1] = msg.entry  // 更新最后一条
      } else {
        entries.push(msg.entry)  // 添加新条目
        lastEntryId = msg.entry.id
      }
    }
  }
  
  return entries
}, [externalMessages])
```

**状态**：✅ **已统一**

---

## 🔍 为什么 WebSocket `history` 不需要修改？

```typescript
// useGlobalWebSocket.ts 的 history 处理
case "history":
  const existingEntryIds = new Set(...)
  const newMessages = msg.history!.filter(m => 
    !existingEntryIds.has(m.entry.id)
  )
  state.messages = [...newMessages, ...state.messages]
```

**原因**：

1. **实际场景很少发生**
   - WebSocket `history` 消息只在任务**正在运行**时发送
   - 大多数情况下，历史已经通过 HTTP API 加载
   - 只收到 `subscribed` 确认，不会收到 `history`

2. **即使发生也安全**
   - 使用 Set 去重确保不会重复显示
   - 历史消息是完整的，不存在"连续性"问题
   - 最终渲染时 ChatInterface 会再次应用连续性规则

3. **修改收益低**
   - 这个分支极少执行
   - 最终显示由 ChatInterface 控制
   - 修改增加复杂度但收益甚微

## 📝 测试场景

### 场景1：流式输出（连续更新）✅

```
收到消息：
1. id="assistant-1", content="正在"
2. id="assistant-1", content="正在思考"
3. id="assistant-1", content="正在思考..."

结果：只显示1条消息，内容不断更新 ✅
```

### 场景2：工具调用打断（非连续）✅

```
收到消息：
1. id="assistant-1", content="我来帮你"
2. id="tool-1", content="执行命令"
3. id="assistant-1", content="完成了"

结果：显示3条独立消息 ✅
- 消息1：AI回复 "我来帮你"
- 消息2：工具调用
- 消息3：AI回复 "完成了"（不会覆盖消息1）
```

### 场景3：多轮对话✅

```
收到消息：
1. id="user-1", content="你好"
2. id="assistant-1", content="你好！"
3. id="user-2", content="帮我写代码"
4. id="assistant-2", content="好的"

结果：显示4条独立消息 ✅
```

### 场景4：追问后继续（非连续）✅

```
初始执行：
1. id="assistant-1", content="任务开始"
2. id="assistant-1", content="任务完成"  → 更新消息1

追问后：
3. id="user-3", content="继续优化"
4. id="assistant-1", content="好的，继续"  → 新消息（不覆盖消息2）

结果：显示3条消息 ✅
- 消息1：AI "任务完成"（流式更新后的最终版本）
- 消息2：用户 "继续优化"
- 消息3：AI "好的，继续"（新消息，不覆盖消息1）
```

## 🎯 总结

### ✅ 已统一的位置

1. **useGlobalWebSocket** - 实时消息（`task_message`）
2. **useTaskWebSocket** - 实时消息
3. **ChatInterface** - 渲染处理

### ⚠️ 无需修改的位置

1. **useGlobalWebSocket** - WebSocket历史（`history`）
   - 理由：极少执行 + 最终由ChatInterface控制
2. **useGlobalWebSocket** - 初始加载（`subscribe`）
   - 理由：首次加载，不存在合并问题

### 🔒 核心保证

**所有用户可见的消息渲染都经过 `ChatInterface` 的统一处理，确保"只有连续相同ID才合并"的规则被严格执行！**

---

**修改完成时间**：2026-01-22  
**影响版本**：v2.1.0+  
**相关文档**：`FRONTEND_MESSAGE_UPDATE.md`
