# 任务状态实时更新功能

## 📋 功能说明

实现了通过 WebSocket 实时更新任务状态，用户无需手动刷新即可看到任务状态变化。

## 🔧 技术实现

### 后端改动

#### 1. 状态广播位置（backend/services/task_runner.go）

在所有任务状态变化点添加了 `GetGlobalWSHub().BroadcastTaskStatus()` 调用：

- ✅ 任务开始执行：`task.Status = "active"`
- ✅ 任务执行成功：`task.Status = "done"`
- ✅ 任务执行失败：`task.Status = "failed"` （10 个失败场景）
  - Worker 不存在
  - Worker 配置错误
  - 工作目录解析失败
  - 命令构建失败
  - 启动执行失败
  - 命令执行超时
  - 命令执行错误
  - Git 状态检查失败

#### 2. 广播消息格式

```go
GetGlobalWSHub().BroadcastTaskStatus(uint(taskID), "active|done|failed", "")
```

WebSocket 消息类型：
```json
{
  "type": "task_status",
  "taskId": 123,
  "status": "active|done|failed",
  "sessionId": ""
}
```

### 前端改动

#### 1. 导出接口（src/hooks/useGlobalWebSocket.ts）

```typescript
export interface UseGlobalWebSocketOptions {
  onTaskMessage?: (taskId: number, message: TaskLogMessage) => void
  onTaskStatus?: (taskId: number, status: string, sessionId?: string) => void  // ← 状态回调
  onError?: (taskId: number | undefined, error: string) => void
}
```

#### 2. 状态接收处理（src/pages/WorkspaceDetailPage.tsx）

```typescript
const { sendMessage: wsSendMessage } = useGlobalWebSocket({
  onTaskStatus: useCallback((taskId: number, status: string, sessionId?: string) => {
    console.log('[WorkspaceDetail] 收到任务状态更新:', taskId, status, sessionId)
    
    // 更新本地任务状态
    setTasks(prev => prev.map(task => 
      task.id === taskId 
        ? { ...task, status: status as Task['status'] }
        : task
    ))
  }, [])
})
```

## 🎯 用户体验改进

### 改进前
- ❌ 任务状态变化需要手动点击刷新按钮
- ❌ 无法实时知道任务何时完成
- ❌ UI 显示延迟，体验不连贯

### 改进后
- ✅ 任务状态实时更新，无需手动刷新
- ✅ 任务卡片动画效果自动触发（active 状态脉冲动画）
- ✅ UI 状态与后端完全同步
- ✅ 更好的用户体验

## 📊 状态流转

```
用户创建任务 → status: "queue"
    ↓
WebSocket 发送消息 → 后端接收
    ↓
后端开始执行 → status: "active" → 广播 task_status
    ↓                                    ↓
    |                          前端接收并更新 tasks
    |                          TaskCard 自动重新渲染
    |                          显示橙色边框 + 脉冲动画
    ↓
执行完成/失败
    ↓
status: "done"/"failed" → 广播 task_status
    ↓
前端接收并更新 → TaskCard 显示完成/失败状态
```

## 🔍 调试说明

### 后端日志
无额外日志（使用现有的任务执行日志）

### 前端日志
在浏览器控制台查看：
```
[WorkspaceDetail] 收到任务状态更新: 123 active undefined
[WorkspaceDetail] 收到任务状态更新: 123 done undefined
```

## ⚙️ 相关文件

### 后端
- `backend/services/task_runner.go` - 添加状态广播调用
- `backend/services/global_ws_hub.go` - WebSocket 广播方法

### 前端
- `src/hooks/useGlobalWebSocket.ts` - 导出接口，处理状态消息
- `src/pages/WorkspaceDetailPage.tsx` - 实现状态更新回调
- `src/components/workspace/TaskCard.tsx` - 根据状态显示 UI（已有）

## 📝 注意事项

1. **追问不改变任务状态**：追问只创建新的 execution 记录，不会改变 task.status
2. **常驻进程**：使用常驻进程的追问场景已正确处理（执行成功但不改变任务状态）
3. **兼容性**：保留了手动刷新功能，作为备用方案

## ✅ 测试清单

- [ ] 创建任务立即执行，观察状态从 queue → active → done/failed
- [ ] 多个任务同时执行，状态独立更新
- [ ] 任务执行失败，状态正确显示为 failed
- [ ] 刷新页面后重新订阅 WebSocket，状态同步正确
- [ ] 网络断开重连后，状态恢复正常
