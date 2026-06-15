# 前端消息更新逻辑优化

## 问题描述

**旧逻辑**：当收到相同ID的消息时，会在整个消息数组中查找并更新第一个匹配的消息。

**问题场景**：
```
消息1-5: id="xx"   → 合并成一组
消息6:   id="aa"   → 单独一组
消息7:   id="xx"   → ❌ 错误地更新到消息1-5组中
```

用户期望消息7应该是新的一组，因为它与消息1-5不连续（中间被id="aa"打断了）。

## 解决方案

**新逻辑**：只检查**最后一条消息**的ID，只有连续相同ID才更新。

```typescript
// 修改前：在整个数组中查找相同ID
const exists = prev.some(m => existingId === msgId)
if (exists) {
  return prev.map(m => existingId === msgId ? message : m)
}

// 修改后：只检查最后一条消息
if (prev.length > 0) {
  const lastMsg = prev[prev.length - 1]
  const lastId = lastMsg.entry?.id
  
  if (lastId === msgId) {
    // 只更新最后一条
    return [...prev.slice(0, -1), message]
  }
}
// ID不同，添加新消息
return [...prev, message]
```

## 修改的文件

### 1. `src/hooks/useTaskWebSocket.ts`

**位置**：第121-136行

**修改内容**：
- ✅ 移除全局查找逻辑
- ✅ 只检查最后一条消息的ID
- ✅ 实现连续相同ID才合并

### 2. `src/hooks/useGlobalWebSocket.ts`

**位置**：第159-176行

**修改内容**：
- ✅ 移除特殊 `thinking` 类型的全局查找逻辑
- ✅ 统一所有消息类型的更新逻辑
- ✅ 只检查最后一条消息的ID

## 效果对比

### 修改前

```
收到消息序列：
1. id="assistant-1", content="你好"
2. id="assistant-1", content="你好，我"
3. id="assistant-1", content="你好，我是AI"
4. id="tool-1", content="执行命令"
5. id="assistant-1", content="任务完成"  ← 错误：更新到消息1

结果：只有3条消息（消息5覆盖了消息3）
```

### 修改后

```
收到消息序列：
1. id="assistant-1", content="你好"
2. id="assistant-1", content="你好，我"       ← 更新消息1
3. id="assistant-1", content="你好，我是AI"   ← 更新消息2
4. id="tool-1", content="执行命令"            ← 新消息（ID不同）
5. id="assistant-1", content="任务完成"       ← 新消息（不连续）

结果：3条消息（消息1-3合并为一条，消息4独立，消息5独立）
```

## 使用场景

### 场景1：流式输出（连续更新）

```
AI回复逐字显示：
→ "正在"
→ "正在思考"
→ "正在思考..."
→ "正在思考...完成"

✅ 只显示1条消息，内容不断更新
```

### 场景2：工具调用打断

```
AI回复 → 工具调用 → AI继续回复：
→ id="assistant-1": "我来帮你"
→ id="tool-1": "执行命令"
→ id="assistant-1": "完成了"

✅ 显示3条独立消息（不会合并第3条到第1条）
```

### 场景3：多轮对话

```
用户1 → AI1 → 用户2 → AI2：
→ id="user-1": "你好"
→ id="assistant-1": "你好"
→ id="user-2": "帮我写代码"
→ id="assistant-2": "好的"

✅ 显示4条独立消息
```

## 技术细节

### ID生成逻辑

```typescript
const msgId = message.entry?.id || 
  `${message.timestamp}-${message.content?.slice(0, 20)}`
```

优先级：
1. `entry.id` - 后端生成的唯一ID
2. 回退到 `timestamp + content` 组合

### 更新判断

```typescript
// 检查最后一条消息
const lastMsg = prev[prev.length - 1]
const lastId = lastMsg.entry?.id

// 只有连续相同才更新
if (lastId === msgId) {
  return [...prev.slice(0, -1), message]
}
```

### 边界情况

1. **第一条消息**：直接添加
2. **没有ID**：使用 timestamp+content 作为回退ID
3. **空数组**：直接添加

## 测试建议

### 手动测试

1. **流式输出测试**
   - 发送消息给AI
   - 观察AI回复是否逐字更新而不是创建多条消息

2. **工具调用测试**
   - 让AI执行命令
   - 确认命令执行前后的AI消息是独立的

3. **多轮对话测试**
   - 连续发送多条消息
   - 确认每条用户消息和AI回复都是独立的

### 自动化测试（建议）

```typescript
describe('Message Update Logic', () => {
  it('should update last message with same ID', () => {
    const messages = [{ entry: { id: 'msg-1' }, content: 'Hello' }]
    const newMessage = { entry: { id: 'msg-1' }, content: 'Hello World' }
    
    // 应该更新最后一条
    expect(result).toHaveLength(1)
    expect(result[0].content).toBe('Hello World')
  })
  
  it('should add new message when ID is different', () => {
    const messages = [{ entry: { id: 'msg-1' }, content: 'Hello' }]
    const newMessage = { entry: { id: 'msg-2' }, content: 'World' }
    
    // 应该添加新消息
    expect(result).toHaveLength(2)
  })
  
  it('should not merge non-consecutive same IDs', () => {
    const messages = [
      { entry: { id: 'msg-1' }, content: 'A' },
      { entry: { id: 'msg-2' }, content: 'B' }
    ]
    const newMessage = { entry: { id: 'msg-1' }, content: 'C' }
    
    // 应该添加为第3条消息，不更新第1条
    expect(result).toHaveLength(3)
    expect(result[2].content).toBe('C')
  })
})
```

## 影响范围

### 受影响的组件

1. ✅ `useTaskWebSocket` - 任务WebSocket hook
2. ✅ `useGlobalWebSocket` - 全局WebSocket hook
3. ✅ `ChatInterface` - 聊天界面（使用上述hooks）
4. ✅ 所有使用实时消息更新的页面

### 不受影响的功能

- ❌ 历史消息加载（只影响实时更新）
- ❌ 消息持久化（后端逻辑）
- ❌ 其他非消息相关功能

## 总结

### 改进点

✅ **更准确的消息分组**：只有真正连续的消息才会合并  
✅ **更好的用户体验**：AI回复和工具调用不会混淆  
✅ **更清晰的对话流**：每轮对话都是独立的  
✅ **更简单的逻辑**：只检查最后一条，性能更好  

### 兼容性

✅ **向后兼容**：不影响现有功能  
✅ **渐进增强**：自动处理新旧格式  
✅ **无需迁移**：立即生效  

---

**修改完成时间**：2026-01-22  
**影响版本**：v2.1.0+
