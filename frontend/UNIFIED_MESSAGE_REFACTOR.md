# 统一消息架构重构（v3.0）

## 🎯 重构目标

**彻底统一任务执行和追问的交互方式，所有操作都通过"发送消息"完成。**

### 核心原则

1. **前后端不再区分"立即执行"和"追问"**
2. **立即执行 = 创建任务 + 发送首条消息**
3. **追问 = 发送消息到已有会话**
4. **"立即开始"开关只是前端UI**，后端不感知

## 📊 架构对比

### 旧架构（v2.x）

```
前端：
  立即执行按钮 → POST /tasks (startNow=true) → 后端自动启动
  追问按钮     → POST /tasks/:id/follow-up → 后端追问

后端：
  CreateTask API：检查 startNow，自动调用 taskRunner.Run()
  FollowUp API：  调用 taskRunner.RunFollowUp()
```

**问题**：
- ❌ 两套逻辑，代码重复
- ❌ 前端无法控制首次消息内容（总是使用 task.Content）
- ❌ API 不一致，难以理解

### 新架构（v3.0）

```
前端：
  立即执行 → 1. POST /tasks (startNow=false)
            2. POST /tasks/:id/messages { content }
  
  追问     → POST /tasks/:id/messages { content }

后端：
  SendMessage API（统一入口）:
    - 没有 session_id → 首次执行（启动进程 + 发送消息）
    - 有 session_id   → 追问（恢复会话 + 发送消息）
```

**优势**：
- ✅ 单一入口，逻辑统一
- ✅ 前端完全控制消息内容
- ✅ API 简洁直观

## 🔧 详细实现

### 1️⃣ 后端修改

#### 修改 1: TaskRunner 支持自定义内容

**文件**: `backend/services/task_runner.go`

```go
// 新增方法：使用自定义内容启动任务
func (r *TaskRunner) RunWithContent(taskID uint, content string) {
    go r.execute(taskID, content)
}

// 修改 execute 方法签名
func (r *TaskRunner) execute(taskID uint, content string) {
    // 确定使用的内容：优先使用传入的 content，否则使用 task.Content
    executionContent := content
    if executionContent == "" {
        executionContent = task.Content
    }
    
    // 所有使用 task.Content 的地方都改为 executionContent
    userEntry := models.NormalizedEntry{
        Content: executionContent,  // ← 使用传入的内容
    }
    
    stdin.Write([]byte(executionContent + "\n"))  // ← 发送给进程
}
```

**修改位置**：
- 第171-174行：添加 `RunWithContent` 方法
- 第461-468行：修改 `execute` 签名和参数处理
- 第581-591行：使用 `executionContent` 而不是 `task.Content`
- 第609-620行：命令日志使用 `executionContent`
- 第657行：发送给 stdin 使用 `executionContent`

#### 修改 2: MessageService 使用新的启动方式

**文件**: `backend/internal/service/task/message_service.go`

```go
func (s *MessageService) SendMessage(taskID uint, req SendMessageRequest) (string, error) {
    // ... 判断是否有现有会话
    
    if isNewSession {
        // 新会话：使用请求中的 content 启动任务
        s.taskRunner.RunWithContent(taskID, req.Content)  // ← 使用自定义内容
        return "", nil
    } else {
        // 现有会话：追问
        s.taskRunner.RunFollowUp(taskID, req.Content, sessionID)
        return sessionID, nil
    }
}
```

**修改位置**：
- 第70-127行：使用 `RunWithContent` 而不是 `Run`
- 移除了对 `time` 包的依赖

### 2️⃣ 前端修改

#### 修改 1: 添加统一消息API

**文件**: `src/lib/api.ts`

```typescript
// 🆕 统一消息API（推荐使用）
async sendTaskMessage(taskId: number, params: {
  content: string;
  retryFromExecId?: number;
  forceWhenDirty?: boolean;
}): Promise<{ message: string; sessionId: string }> {
  return this.request<{ message: string; sessionId: string }>(
    `/tasks/${taskId}/messages`, 
    {
      method: 'POST',
      body: JSON.stringify(params),
    }
  );
}
```

#### 修改 2: 创建任务逻辑

**文件**: `src/pages/ProjectDetailPage.tsx` 和 `WorkspaceDetailPage.tsx`

```typescript
const handleCreateTask = async () => {
  // ... 验证输入
  
  // 1. 创建任务（startNow 总是 false）
  const newTask = await api.createTask(projectIdNum, {
    content: taskContent,
    workerId: worker?.id,
    workerName: worker?.name,
    startNow: false,  // ← 总是 false
    // ...
  })
  
  toast.success("任务已创建")
  const savedContent = taskContent  // 保存内容
  setIsCreateOpen(false)
  setTaskContent("")
  loadData()
  
  // 2. 如果选择了"立即执行"，发送消息启动
  if (startNow) {
    setSelectedTaskId(newTask.id)
    
    try {
      await api.sendTaskMessage(newTask.id, {
        content: savedContent  // ← 使用保存的内容
      })
      toast.success("任务已启动")
    } catch (error: any) {
      toast.error("启动任务失败: " + error.message)
    }
  }
}
```

**修改位置**：
- `ProjectDetailPage.tsx`: 第274-310行
- `WorkspaceDetailPage.tsx`: 第429-467行

## 🔄 完整流程

### 场景1：创建任务并立即执行

```
用户操作：
1. 填写任务内容: "帮我写代码"
2. 选择 Worker
3. 勾选"立即开始" ✅
4. 点击"创建"

前端流程：
1. POST /tasks (startNow=false)           → 创建任务（不启动）
2. POST /tasks/:id/messages { content }  → 发送消息启动

后端流程：
1. 创建任务记录（status="queue"）
2. 收到消息请求，检查无 session_id
3. 调用 RunWithContent(taskID, "帮我写代码")
4. 启动进程，发送"帮我写代码"给 cursor-agent
5. 发送 loading 消息
6. 接收 cursor-agent 输出并广播

用户看到：
1. "任务已创建" ✅
2. "任务已启动" ✅
3. Shimmer 加载动画
4. AI 开始思考和回复
```

### 场景2：创建任务但不立即执行

```
用户操作：
1. 填写任务内容: "帮我写代码"
2. 选择 Worker
3. 取消勾选"立即开始" ❌
4. 点击"创建"

前端流程：
1. POST /tasks (startNow=false)  → 创建任务（不启动）
2. ❌ 不发送消息

后端流程：
1. 创建任务记录（status="queue"）

用户看到：
1. "任务已创建" ✅
2. 任务列表中显示新任务（status="queue"）
3. 可以稍后点击任务，手动发送消息启动
```

### 场景3：追问现有任务

```
用户操作：
1. 打开已完成的任务
2. 输入追问内容: "继续优化"
3. 发送

前端流程：
1. POST /tasks/:id/messages { content: "继续优化" }

后端流程：
1. 收到消息请求，检查有 session_id
2. 调用 RunFollowUp(taskID, "继续优化", sessionID)
3. 使用 --resume 启动 cursor-agent
4. 发送 loading 消息
5. 发送追问内容给 cursor-agent
6. 接收输出并广播

用户看到：
1. 用户消息立即显示 ✅
2. Shimmer 加载动画
3. AI 继续回复
```

## 📝 API 对比

### 旧 API（已废弃）

```
POST /tasks                     - 创建任务（startNow 参数）
POST /tasks/:id/restart         - 重启任务
POST /tasks/:id/follow-up       - 追问
GET  /tasks/:id/session-id      - 获取会话ID
```

### 新 API（推荐）

```
POST /tasks                     - 创建任务（startNow 总是 false）
POST /tasks/:id/messages        - 统一消息接口（首次执行 + 追问）
POST /tasks/:id/stop            - 停止任务
GET  /tasks/:id/running         - 检查是否运行中
```

**向后兼容**：旧 API 保留，但不推荐使用。

## ✅ 优势总结

### 用户体验

- ✅ **一致性**：所有交互都是"发送消息"
- ✅ **灵活性**：可以先创建任务，稍后启动
- ✅ **直观性**：首次执行和追问没有区别

### 开发体验

- ✅ **代码简化**：单一入口，统一逻辑
- ✅ **易于维护**：不需要区分两种模式
- ✅ **扩展性好**：新功能只需修改一处

### 技术优势

- ✅ **解耦**：前端控制时机，后端处理逻辑
- ✅ **可测试**：统一API更容易测试
- ✅ **向后兼容**：旧API保留，平滑迁移

## 🧪 测试清单

### 测试1：创建任务并立即执行

- [ ] 创建任务，勾选"立即开始"
- [ ] 验证任务已创建（status="queue"）
- [ ] 验证任务已启动（收到 loading 消息）
- [ ] 验证 AI 开始回复（loading 自动消失）
- [ ] 验证会话可以继续追问

### 测试2：创建任务但不立即执行

- [ ] 创建任务，取消勾选"立即开始"
- [ ] 验证任务已创建（status="queue"）
- [ ] 验证任务未启动（没有执行记录）
- [ ] 手动点击任务，发送消息
- [ ] 验证任务启动并正常执行

### 测试3：追问现有任务

- [ ] 打开已完成的任务
- [ ] 发送追问消息
- [ ] 验证显示 loading 消息
- [ ] 验证 AI 回复（loading 消失）
- [ ] 验证会话 ID 保持一致

### 测试4：常驻进程追问

- [ ] 启用常驻进程配置
- [ ] 执行任务直到完成
- [ ] 发送追问（使用常驻进程）
- [ ] 验证响应速度更快
- [ ] 验证会话正确恢复

### 测试5：历史恢复

- [ ] 刷新页面
- [ ] 打开有历史的任务
- [ ] 验证历史消息正确加载
- [ ] 验证 loading 消息已被过滤
- [ ] 发送新消息可以继续会话

## 📂 修改的文件

### 后端

1. ✅ `backend/services/task_runner.go`
   - 添加 `RunWithContent` 方法
   - 修改 `execute` 方法支持自定义内容
   - 添加 loading 消息发送

2. ✅ `backend/internal/service/task/message_service.go`
   - 修改 `SendMessage` 使用 `RunWithContent`
   - 移除未使用的导入

### 前端

1. ✅ `src/lib/api.ts`
   - 添加 `sendTaskMessage` 方法

2. ✅ `src/pages/ProjectDetailPage.tsx`
   - 修改 `handleCreateTask` 逻辑
   - startNow 改为前端处理

3. ✅ `src/pages/WorkspaceDetailPage.tsx`
   - 修改 `handleCreateTask` 逻辑
   - startNow 改为前端处理

4. ✅ `src/components/workspace/ChatInterface.tsx`
   - 改进 loading UI（Shimmer 效果）
   - 自动移除 loading 消息

## 🎯 迁移指南

### 对于新开发

**推荐使用统一消息API**：

```typescript
// 创建任务（不启动）
const task = await api.createTask(projectId, {
  content: "任务描述",
  workerId: workerId,
  startNow: false  // 总是 false
})

// 启动任务（发送首条消息）
await api.sendTaskMessage(task.id, {
  content: "实际要执行的内容"
})

// 追问（发送后续消息）
await api.sendTaskMessage(task.id, {
  content: "追问内容"
})
```

### 对于现有代码

旧 API 保留，可以继续使用：

```typescript
// 旧方式（仍可用）
await api.createTask(projectId, { 
  content: "...", 
  startNow: true  // 后端会自动启动
})

await api.followUp(taskId, { 
  content: "..." 
})
```

**建议**: 逐步迁移到新 API。

## 🎨 用户体验改进

### 1. Shimmer Loading 效果

**修改前**：
```
加载中... ⏳
```

**修改后**：
```
┌────────────────────────────┐
│ 🤖  ▓▓▓▓▓▓▓▓▓▓▓░░░░        │  ← Shimmer 动画
│     ▓▓▓▓▓▓░░░░              │  ← Shimmer 动画
└────────────────────────────┘
```

### 2. 自动清理

**修改前**：
- Loading 消息一直显示，需要手动管理

**修改后**：
- 真实消息到来时自动移除 ✅
- 无需额外状态管理 ✅

### 3. 历史恢复

**修改前**：
- 历史中可能包含 loading 消息
- 用户看到已完成任务还显示"加载中"

**修改后**：
- 自动过滤历史中的 loading 消息 ✅
- 只显示真实内容 ✅

## 🔄 完整消息流程图

```
┌─────────────────┐
│  前端：创建任务   │
│  (startNow=false)│
└────────┬────────┘
         │
         ↓
  ┌──────────────┐
  │ 后端：创建记录 │
  │ (status=queue)│
  └──────┬───────┘
         │
         ↓ (如果用户选择"立即开始")
┌─────────────────────────┐
│ 前端：发送消息            │
│ POST /tasks/:id/messages│
│ { content: "任务内容" }  │
└────────┬───────────────┘
         │
         ↓
┌─────────────────────────┐
│ 后端：检查 session_id    │
├─────────────────────────┤
│ 无 session_id:          │
│   1. 启动新进程          │
│   2. 发送 loading 消息   │
│   3. 发送内容给进程      │
│                          │
│ 有 session_id:          │
│   1. 恢复会话            │
│   2. 发送 loading 消息   │
│   3. 发送内容给进程      │
└────────┬───────────────┘
         │
         ↓
┌─────────────────────────┐
│ 进程输出                 │
│ → thinking (delta)      │
│ → assistant             │
│ → tool_use              │
└────────┬───────────────┘
         │
         ↓
┌─────────────────────────┐
│ 前端：自动移除 loading   │
│ 显示真实内容             │
└─────────────────────────┘
```

## 📊 架构演进

| 版本 | 架构 | 问题 |
|------|------|------|
| **v1.0** | 单一执行API | 不支持追问 |
| **v2.0** | 执行 + 追问API | 两套逻辑，不统一 |
| **v3.0** | 统一消息API | ✅ 完美！ |

## 🎉 总结

### 实现的功能

✅ **统一消息接口**：首次执行和追问使用同一API  
✅ **前端控制时机**：创建任务和启动任务分离  
✅ **优雅加载状态**：Shimmer 动画 + 自动移除  
✅ **代码简化**：减少重复逻辑  
✅ **向后兼容**：旧API保留  

### 下一步

如果需要完全移除旧API，可以：
1. 删除 `restartTask` API
2. 删除 `followUp` API
3. 更新所有前端代码使用 `sendTaskMessage`

---

**重构完成时间**：2026-01-22  
**版本**：v3.0.0  
**影响范围**：任务执行核心流程
