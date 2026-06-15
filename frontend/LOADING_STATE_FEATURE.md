# 加载状态功能（Shimmer 效果）

## 🎯 功能描述

在任务开始执行时显示一个优雅的加载状态（Shimmer 效果），当真实消息数据到来时自动移除，提升用户体验。

## 📊 实现原理

### 消息流程

```
任务开始执行
    ↓
后端发送 loading 消息
    ↓
前端显示 Shimmer 效果
    ↓
真实消息到来（thinking/assistant/tool_use）
    ↓
前端自动移除 loading 消息
    ↓
显示真实内容
```

## 🔧 后端实现

### 1. 消息类型定义

```go
// backend/models/normalized_entry.go
const (
    EntryTypeLoading = "loading"  // ← 加载状态类型
)
```

### 2. 发送加载消息

#### 场景1：初始任务执行

```go
// backend/services/task_runner.go:469-480
func (r *TaskRunner) execute(taskID uint) {
    // 任务开始
    task.Status = "active"
    database.GetDB().Save(&task)
    
    // 发送加载状态消息
    r.hub.BroadcastNormalized(taskID, models.NormalizedEntry{
        ID:        "loading-initial",       // ← 固定ID
        EntryType: models.EntryTypeLoading,
        Content:   "正在执行任务...",
    })
    
    // ... 启动任务执行
}
```

#### 场景2：追问执行

```go
// backend/services/task_runner.go:289-297
func (r *TaskRunner) executeFollowUp(taskID uint, prompt string, sessionID string) {
    // 广播用户消息
    r.hub.Broadcast(taskID, TaskMessage{ ... })
    
    // 发送加载状态消息
    r.hub.BroadcastNormalized(taskID, models.NormalizedEntry{
        ID:        "loading-followup",      // ← 固定ID
        EntryType: models.EntryTypeLoading,
        Content:   "正在处理追问...",
    })
    
    // ... 启动追问执行
}
```

#### 场景3：常驻进程追问

```go
// backend/services/task_runner.go:95-109
func (r *TaskRunner) executeFollowUpWithResidentProcess(...) {
    // 广播用户消息
    r.hub.Broadcast(taskID, TaskMessage{ ... })
    
    // 发送加载状态消息
    r.hub.BroadcastNormalized(taskID, models.NormalizedEntry{
        ID:        "loading-resident",      // ← 固定ID
        EntryType: models.EntryTypeLoading,
        Content:   "正在处理追问...",
    })
    
    // ... 向常驻进程发送消息
}
```

## 🎨 前端实现

### 1. Loading UI 组件（Shimmer 效果）

```typescript
// src/components/workspace/ChatInterface.tsx:231-244
case "loading":
  return (
    <div className="flex w-full items-start gap-2">
      <div className="flex h-6 w-6 shrink-0 items-center justify-center bg-muted mt-0.5">
        <Bot className="h-4 w-4 text-muted-foreground" />
      </div>
      <div className="flex-1 min-w-0 px-3 py-2 bg-muted/50">
        {/* Shimmer 效果 */}
        <div className="space-y-2 animate-pulse">
          <div className="h-3 bg-muted-foreground/20 rounded w-3/4"></div>
          <div className="h-3 bg-muted-foreground/20 rounded w-1/2"></div>
        </div>
      </div>
    </div>
  )
```

**UI 效果**：
- 左侧：Bot 图标（与 AI 消息一致）
- 右侧：两行 Shimmer 动画条（模拟文字加载）
- 背景：灰色背景（与 AI 消息一致）

### 2. 自动移除 Loading 消息

```typescript
// src/components/workspace/ChatInterface.tsx:308-382
const displayEntries = useMemo(() => {
  const entries: NormalizedEntry[] = []
  let hasRealContent = false  // ← 标记是否有真实内容
  
  for (const msg of externalMessages) {
    if (msg.entry) {
      // 检查是否为真实内容（非 loading）
      if (msg.entry.entry_type !== "loading") {
        hasRealContent = true  // ← 发现真实内容
      }
      
      // ... 处理消息
      entries.push(msg.entry)
    }
  }
  
  // 如果有真实内容，移除所有 loading 消息
  if (hasRealContent) {
    return entries.filter(e => e.entry_type !== "loading")  // ← 自动移除
  }
  
  return entries
}, [externalMessages])
```

**移除逻辑**：
- ✅ 只要有一条真实消息（thinking/assistant/tool_use/error），就移除所有 loading
- ✅ 如果只有 loading 消息，继续显示
- ✅ 自动处理，前端无需额外判断

## 📊 完整流程示例

### 场景1：初始任务执行

```
用户发送任务
    ↓
后端：发送 loading-initial
前端：显示 Shimmer 动画 "正在执行任务..."
    ↓ 0.5s
后端：发送第一条 thinking 消息
前端：自动移除 loading，显示思考内容 ✅
    ↓
后端：继续发送 assistant、tool_use 等消息
前端：正常显示
```

### 场景2：追问执行

```
用户发送追问
    ↓
前端：立即显示用户消息
    ↓
后端：发送 loading-followup
前端：显示 Shimmer 动画 "正在处理追问..."
    ↓ 0.5s
后端：发送第一条 thinking 消息
前端：自动移除 loading，显示思考内容 ✅
    ↓
后端：继续发送 assistant 等消息
前端：正常显示
```

### 场景3：历史恢复

```
用户打开任务详情
    ↓
前端：加载历史消息（HTTP API）
    ↓
如果历史中有 loading 消息：
  - 检查是否有真实内容（thinking/assistant/tool_use）
  - 有真实内容 → 自动过滤掉 loading ✅
  - 只有 loading → 显示 loading（任务可能中断）
```

## 🎨 UI 对比

### 修改前

```
加载中... ⏳
```

**问题**：
- ❌ 样式简陋
- ❌ 与 AI 消息风格不一致
- ❌ 不够优雅

### 修改后

```
┌────────────────────────────┐
│ 🤖  ▓▓▓▓▓▓▓▓▓▓▓░░░░        │  ← Shimmer 动画
│     ▓▓▓▓▓▓░░░░              │  ← Shimmer 动画
└────────────────────────────┘
```

**优点**：
- ✅ Shimmer 动画更优雅
- ✅ 与 AI 消息风格一致（同样的背景和布局）
- ✅ 模拟文字加载效果
- ✅ 提供更好的视觉反馈

## 🔍 技术细节

### Loading 消息 ID

| 场景 | ID | 说明 |
|------|----|----|
| 初始执行 | `loading-initial` | 任务首次启动 |
| 追问执行 | `loading-followup` | 使用新进程追问 |
| 常驻进程追问 | `loading-resident` | 使用常驻进程追问 |

### 自动移除条件

```typescript
// 真实内容的定义
const isRealContent = 
  entry_type === "thinking" ||
  entry_type === "assistant_message" ||
  entry_type === "tool_use" ||
  entry_type === "error_message" ||
  entry_type === "user_message"

// loading 不算真实内容
const isNotRealContent = 
  entry_type === "loading" ||
  entry_type === "system_message"
```

只要出现任何真实内容消息，loading 就会被移除。

### Shimmer 动画实现

使用 Tailwind CSS 的 `animate-pulse` 类：

```tsx
<div className="space-y-2 animate-pulse">
  <div className="h-3 bg-muted-foreground/20 rounded w-3/4"></div>
  <div className="h-3 bg-muted-foreground/20 rounded w-1/2"></div>
</div>
```

**效果**：
- 两行不同宽度的灰色条
- 持续的淡入淡出动画
- 模拟文字加载的视觉效果

## 📝 测试场景

### 测试1：初始任务执行

1. 创建新任务并启动
2. **预期**：立即显示 Shimmer 加载动画
3. 等待 0.5-1秒
4. **预期**：loading 消息自动消失，显示 thinking 或 assistant 消息

### 测试2：追问执行

1. 在已完成的任务中发送追问
2. **预期**：用户消息立即显示
3. **预期**：下方显示 Shimmer 加载动画
4. 等待 0.5-1秒
5. **预期**：loading 消息自动消失，显示 AI 回复

### 测试3：历史恢复

1. 刷新页面或切换任务
2. **预期**：从数据库加载历史消息
3. **预期**：如果历史中有 loading 且有真实消息，loading 被自动过滤
4. **预期**：只显示真实内容

### 测试4：任务中断

1. 启动任务后立即停止
2. **预期**：只有 loading 消息（没有真实内容）
3. **预期**：loading 消息继续显示（表示任务未完成）

## 📊 修改的文件

### 后端

1. ✅ `backend/services/task_runner.go`
   - 第472-479行：初始任务执行添加 loading
   - 第289-297行：追问执行添加 loading
   - 第95-109行：常驻进程追问添加 loading

2. ✅ `backend/models/normalized_entry.go`
   - 第15行：已定义 `EntryTypeLoading` 类型

### 前端

1. ✅ `src/components/workspace/ChatInterface.tsx`
   - 第231-244行：改进 loading UI（Shimmer 效果）
   - 第308-382行：自动移除 loading 逻辑

2. ✅ `src/lib/api.ts`
   - 第130行：已定义 `loading` 类型

## ✅ 优势

### 用户体验

- ✅ **即时反馈**：任务启动后立即显示加载状态
- ✅ **视觉优雅**：Shimmer 动画比简单的 spinner 更专业
- ✅ **自动清理**：无需手动管理，真实内容到来时自动消失
- ✅ **风格一致**：与 AI 消息保持相同的视觉风格

### 技术优势

- ✅ **无需额外状态管理**：loading 消息与其他消息统一处理
- ✅ **向后兼容**：历史消息中的 loading 也能自动过滤
- ✅ **简单可靠**：逻辑清晰，易于维护
- ✅ **性能友好**：只在 useMemo 中过滤，不影响渲染性能

## 🎬 实际效果

### 加载中

```
┌─────────────────────────────────────┐
│ 👤 帮我修改代码                       │  ← 用户消息
├─────────────────────────────────────┤
│ 🤖  ▓▓▓▓▓▓▓▓▓▓▓░░░░                 │  ← Shimmer 动画
│     ▓▓▓▓▓▓░░░░                      │  ← Shimmer 动画
└─────────────────────────────────────┘
```

### 加载完成（自动切换）

```
┌─────────────────────────────────────┐
│ 👤 帮我修改代码                       │  ← 用户消息
├─────────────────────────────────────┤
│ 🧠 [思考过程] ▼                      │  ← 真实内容
│    正在分析代码结构...                │
├─────────────────────────────────────┤
│ 🤖 我来帮你修改...                   │  ← AI 回复
└─────────────────────────────────────┘
```

**注意**：loading 消息已自动移除，无缝切换到真实内容！

---

**实现完成时间**：2026-01-22  
**影响版本**：v2.2.0+
