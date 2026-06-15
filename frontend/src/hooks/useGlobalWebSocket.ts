import { useEffect, useRef, useState, useCallback, useMemo } from "react"
import { WithParts, api, TaskMessageQueueItem, TaskPlanDocument } from "@/lib/api"
import { normalizeTaskQueuePayload } from "@/lib/taskQueue"
import { normalizeTaskPlanPayload } from "@/lib/taskPlan"

function buildMessagesV2Index(messages: WithParts[]): Map<string, number> {
  const map = new Map<string, number>()
  for (let i = 0; i < messages.length; i++) {
    const id = messages[i]?.info?.id
    if (typeof id === "string" && id.length > 0) {
      map.set(id, i)
    }
  }
  return map
}

// WebSocket 动作类型
type WSAction = "subscribe" | "unsubscribe" | "send_message" | "restart" | "stop" | "cancel_tool" | "wait_user_input"

// WebSocket 消息类型
type WSMessageType = "message_v2" | "task_status" | "error" | "subscribed" | "retry" | "is_working" | "is_not_working" | "session_title" | "wait_user_input" | "task_queue" | "task_plan" | "ilink_session_expired"

export type ILinkSessionExpiredPayload = {
  accountId: number
  botId: string
  ilinkUserId: string
}

/** 随 send_message 一并发送的用户消息 part */
export type UserInputFileSource = "paste" | "picker" | "drop"

export type UserMessagePart =
  | { type: "text"; text: string }
  | {
      type: "file"
      path: string
      mime?: string
      filename?: string
      inputSource?: UserInputFileSource
      url?: string
    }

// 发送到服务器的消息
interface WSOutgoingMessage {
  action: WSAction
  taskId: number
  content?: string
  parts?: UserMessagePart[]
  data?: any
}

// 从服务器接收的消息
interface WSIncomingMessage {
  type: WSMessageType
  taskId?: number
  data?: WithParts | string | Record<string, any>
  status?: string
  error?: string
  sessionId?: string
}

// 任务消息状态
interface TaskState {
  messagesV2: WithParts[] // V2 消息列表
  /** info.id -> messagesV2 下标，合并流式 message_v2 时 O(1) 定位 */
  messagesV2IndexById: Map<string, number>
  isSubscribed: boolean
  hasMoreHistory?: boolean
  nextBeforeMessageId?: string
  isHistoryLoading?: boolean
  sessionId?: string
  isStreaming?: boolean
  shouldRetry?: boolean  // 是否应该显示重试按钮
  isWorking?: boolean    // 是否正在工作（显示 shimmer）
  messageQueue: TaskMessageQueueItem[]
  messageQueueAutoSend: boolean
  taskPlan: TaskPlanDocument | null
}

// 创建默认任务状态
const createDefaultTaskState = (): TaskState => ({
  messagesV2: [],
  messagesV2IndexById: new Map(),
  isSubscribed: false,
  hasMoreHistory: false,
  nextBeforeMessageId: undefined,
  isHistoryLoading: false,
  shouldRetry: false,
  isStreaming: true,
  isWorking: false,
  messageQueue: [],
  messageQueueAutoSend: true,
  taskPlan: null,
})

function applyTaskQueueToState(
  state: TaskState,
  payload: { queue: TaskMessageQueueItem[]; autoSend: boolean },
): TaskState {
  return {
    ...state,
    messageQueue: payload.queue,
    messageQueueAutoSend: payload.autoSend,
  }
}

const TASK_LOGS_V2_PAGE_SIZE = 100
const MAX_IN_MEMORY_MESSAGES_PER_TASK = 5000
const MAX_PENDING_WS_MESSAGES = 500

function trimMessagesForMemory(messages: WithParts[]) {
  if (messages.length <= MAX_IN_MEMORY_MESSAGES_PER_TASK) {
    return {
      messages,
      trimmed: false,
    }
  }
  return {
    messages: messages.slice(-MAX_IN_MEMORY_MESSAGES_PER_TASK),
    trimmed: true,
  }
}

function mergeHistoryPages(existing: WithParts[], incoming: WithParts[]) {
  if (existing.length === 0) return incoming
  if (incoming.length === 0) return existing

  const seen = new Set(existing.map((item) => item.info.id))
  const prepended = incoming.filter((item) => !seen.has(item.info.id))
  const result = prepended.length > 0 ? [...prepended, ...existing] : existing
  return result
}

/** 用 API 结果刷新已有消息，并补齐更早的历史；不会删除 API 暂未返回的消息 */
function syncHistoryFromAPI(existing: WithParts[], incoming: WithParts[]): WithParts[] {
  if (incoming.length === 0) return existing
  if (existing.length === 0) return incoming

  const incomingById = new Map(incoming.map((item) => [item.info.id, item]))
  const existingIds = new Set(existing.map((item) => item.info.id))
  const updated = existing.map((item) => incomingById.get(item.info.id) ?? item)
  const prepended = incoming.filter((item) => !existingIds.has(item.info.id))
  const result = prepended.length > 0 ? [...prepended, ...updated] : updated
  return result
}

/** 将单条 message_v2 合并进任务状态（与批处理 / 即时更新共用） */
function applyMessageV2ToTaskState(state: TaskState, v2Msg: WithParts): TaskState {
  const prev = state.messagesV2
  let indexMap = state.messagesV2IndexById ?? buildMessagesV2Index(prev)
  const msgId = v2Msg.info?.id

  if (typeof msgId !== "string" || msgId.length === 0) {
    const appended = [...prev, v2Msg]
    const { messages: newMessages, trimmed } = trimMessagesForMemory(appended)
    return {
      ...state,
      messagesV2: newMessages,
      messagesV2IndexById: buildMessagesV2Index(newMessages),
      isSubscribed: true,
      hasMoreHistory: trimmed || state.hasMoreHistory,
      nextBeforeMessageId: trimmed
        ? newMessages[0]?.info?.id
        : state.nextBeforeMessageId,
    }
  }

  let idx = indexMap.get(msgId)
  if (idx !== undefined && prev[idx]?.info?.id !== msgId) {
    indexMap = buildMessagesV2Index(prev)
    idx = indexMap.get(msgId)
  }

  if (idx !== undefined) {
    const newMessages = [...prev]
    newMessages[idx] = v2Msg
    return {
      ...state,
      messagesV2: newMessages,
      messagesV2IndexById: indexMap,
      isSubscribed: true,
    }
  }

  const appended = [...prev, v2Msg]
  const { messages: newMessages, trimmed } = trimMessagesForMemory(appended)
  const newIndexMap = buildMessagesV2Index(newMessages)
  return {
    ...state,
    messagesV2: newMessages,
    messagesV2IndexById: newIndexMap,
    isSubscribed: true,
    hasMoreHistory: trimmed || state.hasMoreHistory,
    nextBeforeMessageId: trimmed
      ? newMessages[0]?.info?.id
      : state.nextBeforeMessageId,
  }
}

type MessageV2FlushSchedule =
  | { type: "raf"; id: number }
  | { type: "timeout"; id: ReturnType<typeof setTimeout> }

function mergePendingMessageV2IntoMap(
  prev: Map<number, TaskState>,
  batch: Map<number, WithParts[]>
): Map<number, TaskState> {
  let next: Map<number, TaskState> | null = null
  for (const [tid, msgs] of batch) {
    if (!msgs.length) continue
    const baseMap = next ?? prev
    let st = baseMap.get(tid) || createDefaultTaskState()
    for (const v2Msg of msgs) {
      st = applyMessageV2ToTaskState(st, v2Msg)
    }
    if (!next) next = new Map(prev)
    next.set(tid, st)
  }
  return next ?? prev
}

export interface UseGlobalWebSocketOptions {
  onTaskMessageV2?: (taskId: number, message: WithParts) => void
  onTaskStatus?: (taskId: number, status: string, sessionId?: string, workDir?: string) => void
  onError?: (taskId: number | undefined, error: string) => void
  onRetry?: (taskId: number) => void
  onIsStreaming?: (taskId: number, isStreaming: boolean) => void
  onSessionTitle?: (taskId: number, title: string) => void
  onWaitUserInput?: (taskId: number, payload: Record<string, any>) => void
  onTaskQueue?: (taskId: number, payload: { queue: TaskMessageQueueItem[]; autoSend: boolean }) => void
  onTaskPlan?: (taskId: number, plan: TaskPlanDocument | null) => void
  onILinkSessionExpired?: (payload: ILinkSessionExpiredPayload) => void
}

// 全局 WebSocket 单例
let globalWsInstance: WebSocket | null = null
let globalWsConnecting = false
let globalWsListeners: Set<(msg: WSIncomingMessage) => void> = new Set()
let reconnectTimer: NodeJS.Timeout | null = null
let pendingMessages: WSIncomingMessage[] = [] // 缓存未分发的消息，防止监听器尚未注册时丢失
let isDrainingPending = false // 防止在回放过程中乱序

function getGlobalWs(): Promise<WebSocket> {
  return new Promise((resolve, reject) => {
    if (globalWsInstance?.readyState === WebSocket.OPEN) {
      resolve(globalWsInstance)
      return
    }

    if (globalWsConnecting) {
      // 等待连接完成
      const checkInterval = setInterval(() => {
        if (globalWsInstance?.readyState === WebSocket.OPEN) {
          clearInterval(checkInterval)
          resolve(globalWsInstance)
        }
      }, 100)
      setTimeout(() => {
        clearInterval(checkInterval)
        reject(new Error("WebSocket connection timeout"))
      }, 5000)
      return
    }

    globalWsConnecting = true

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const host = window.location.host
    const wsUrl = `${protocol}//${host}/api/ws`

    
    const ws = new WebSocket(wsUrl)
    ws.onopen = () => {
      globalWsInstance = ws
      globalWsConnecting = false
      resolve(ws)
    }

    ws.onmessage = (event) => {
      try {
        const msg: WSIncomingMessage = JSON.parse(event.data)
        if (globalWsListeners.size === 0 || isDrainingPending) {
          pendingMessages.push(msg)
          if (pendingMessages.length > MAX_PENDING_WS_MESSAGES) {
            pendingMessages = pendingMessages.slice(-MAX_PENDING_WS_MESSAGES)
          }
        } else {
          globalWsListeners.forEach(listener => listener(msg))
        }
      } catch (e) {
        console.error('[GlobalWS] Failed to parse message:', e)
      }
    }

    ws.onclose = (event) => {
            globalWsInstance = null
      globalWsConnecting = false

      // 自动重连
      if (event.code !== 1000 && !reconnectTimer) {
        reconnectTimer = setTimeout(() => {
          reconnectTimer = null
                    getGlobalWs().catch(console.error)
        }, 3000)
      }
    }

    ws.onerror = (error) => {
      console.error('[GlobalWS] Error:', error)
      globalWsConnecting = false
      reject(error)
    }
  })
}

function sendWsMessage(msg: WSOutgoingMessage) {
  getGlobalWs().then(ws => {
    ws.send(JSON.stringify(msg))
  }).catch(error => {
    console.error('[GlobalWS] Failed to send message:', error)
  })
}

export function useGlobalWebSocket(options: UseGlobalWebSocketOptions = {}) {
  const [isConnected, setIsConnected] = useState(false)
  const [taskStates, setTaskStates] = useState<Map<number, TaskState>>(new Map())
  const optionsRef = useRef(options)

  useEffect(() => {
    optionsRef.current = options
  }, [options])

  const messageV2BatchRef = useRef<Map<number, WithParts[]>>(new Map())
  const messageV2ScheduleRef = useRef<MessageV2FlushSchedule | null>(null)
  /** 递增后使进行中的 HTTP 队列拉取失效，避免慢响应覆盖 WS / 乐观更新 */
  const taskQueueLoadGenRef = useRef(new Map<number, number>())

  const invalidateTaskQueueLoads = useCallback((taskId: number) => {
    const next = (taskQueueLoadGenRef.current.get(taskId) ?? 0) + 1
    taskQueueLoadGenRef.current.set(taskId, next)
  }, [])

  const commitTaskQueueState = useCallback((
    taskId: number,
    payload: { queue: TaskMessageQueueItem[]; autoSend: boolean },
    options?: { invalidateLoads?: boolean },
  ) => {
    if (options?.invalidateLoads !== false) {
      invalidateTaskQueueLoads(taskId)
    }
    setTaskStates((prev) => {
      const state = prev.get(taskId) || createDefaultTaskState()
      const next = new Map(prev)
      next.set(taskId, applyTaskQueueToState(state, payload))
      return next
    })
    optionsRef.current.onTaskQueue?.(taskId, payload)
  }, [invalidateTaskQueueLoads])

  const cancelScheduledMessageV2Flush = useCallback(() => {
    const s = messageV2ScheduleRef.current
    if (!s) return
    if (s.type === "raf") cancelAnimationFrame(s.id)
    else clearTimeout(s.id)
    messageV2ScheduleRef.current = null
  }, [])

  const flushPendingMessageV2 = useCallback(() => {
    const batch = messageV2BatchRef.current
    if (batch.size === 0) return

    const snapshot = new Map<number, WithParts[]>()
    for (const [tid, arr] of batch) {
      if (arr.length) snapshot.set(tid, arr.slice())
    }
    batch.clear()
    if (snapshot.size === 0) return

    setTaskStates(prev => {
      const merged = mergePendingMessageV2IntoMap(prev, snapshot)
      return merged === prev ? prev : merged
    })

    const cb = optionsRef.current.onTaskMessageV2
    if (cb) {
      for (const [tid, msgs] of snapshot) {
        for (const m of msgs) cb(tid, m)
      }
    }
  }, [])

  const scheduleFlushMessageV2 = useCallback(() => {
    if (messageV2ScheduleRef.current != null) return
    const run = () => {
      messageV2ScheduleRef.current = null
      flushPendingMessageV2()
    }
    // 后台标签页 rAF 大幅节流，改用 macrotask 避免消息积压过久
    if (typeof document !== "undefined" && document.hidden) {
      messageV2ScheduleRef.current = { type: "timeout", id: setTimeout(run, 0) }
    } else {
      messageV2ScheduleRef.current = { type: "raf", id: requestAnimationFrame(run) }
    }
  }, [flushPendingMessageV2])

  const loadTaskQueueForTask = useCallback(async (taskId: number) => {
    const loadGen = taskQueueLoadGenRef.current.get(taskId) ?? 0
    try {
      const data = await api.getTaskQueue(taskId)
      if ((taskQueueLoadGenRef.current.get(taskId) ?? 0) !== loadGen) {
        return
      }
      const payload = normalizeTaskQueuePayload(data)
      commitTaskQueueState(taskId, payload, { invalidateLoads: false })
    } catch (error) {
      console.error("[GlobalWS] 加载任务队列失败:", error)
    }
  }, [commitTaskQueueState])

  const commitTaskPlanState = useCallback((taskId: number, plan: TaskPlanDocument | null) => {
    setTaskStates((prev) => {
      const oldState = prev.get(taskId) || createDefaultTaskState()
      const newMap = new Map(prev)
      newMap.set(taskId, {
        ...oldState,
        taskPlan: plan,
        isSubscribed: true,
      })
      return newMap
    })
    optionsRef.current.onTaskPlan?.(taskId, plan)
  }, [])

  const loadTaskPlanForTask = useCallback(async (taskId: number) => {
    try {
      const data = await api.getTaskPlan(taskId)
      commitTaskPlanState(taskId, normalizeTaskPlanPayload(data.plan))
    } catch (error) {
      console.error("[GlobalWS] 加载任务计划失败:", error)
    }
  }, [commitTaskPlanState])

  const loadHistoryForTask = useCallback(async (taskId: number, opts: { force?: boolean; beforeMessageId?: string } = {}) => {
    flushPendingMessageV2()
    const { force = false, beforeMessageId } = opts
    
    try {
      setTaskStates(prev => {
        const prevState = prev.get(taskId) || createDefaultTaskState()
        const silent = force && prevState.messagesV2.length > 0
        if (silent) return prev
        const newMap = new Map(prev)
        newMap.set(taskId, { ...prevState, isHistoryLoading: true })
        return newMap
      })

      const historyV2 = await api.getTaskLogsV2(taskId, {
        limit: TASK_LOGS_V2_PAGE_SIZE,
        beforeMessageId,
      }).catch(() => ({ items: [], hasMore: false as boolean, nextBeforeMessageId: undefined as string | undefined }))
      setTaskStates(prev => {
        const prevState = prev.get(taskId) || createDefaultTaskState()

        const list = Array.isArray(historyV2.items) ? historyV2.items : []
        const mergedMessages = beforeMessageId
          ? mergeHistoryPages(prevState.messagesV2, list)
          : force
            ? syncHistoryFromAPI(prevState.messagesV2, list)
            : prevState.messagesV2.length === 0
              ? list
              : mergeHistoryPages(prevState.messagesV2, list)
        // 加载历史时不截断，让用户可以看到完整的已加载历史
        const newMap = new Map(prev)
        newMap.set(taskId, {
          ...prevState,
          messagesV2: mergedMessages,
          messagesV2IndexById: buildMessagesV2Index(mergedMessages),
          isSubscribed: true,
          shouldRetry: false,
          hasMoreHistory: historyV2.hasMore,
          nextBeforeMessageId: historyV2.nextBeforeMessageId,
          isHistoryLoading: false,
        })
        return newMap
      })
    } catch (e) {
      console.error('[GlobalWS] Failed to load history:', e)
      setTaskStates(prev => {
        const prevState = prev.get(taskId)
        if (!prevState) return prev
        const newMap = new Map(prev)
        newMap.set(taskId, { ...prevState, isHistoryLoading: false })
        return newMap
      })
    }
  }, [flushPendingMessageV2])

  // 消息处理器
  // 辅助函数：高效更新任务状态
  const updateTaskState = useCallback((
    taskId: number,
    updater: (oldState: TaskState) => Partial<TaskState>
  ) => {
    setTaskStates(prev => {
      const oldState = prev.get(taskId) || createDefaultTaskState()
      const updates = updater(oldState)
      const shouldMarkSubscribed = !oldState.isSubscribed

      // 如果没有任何变化且已订阅，返回原 Map（避免不必要的更新）
      if (Object.keys(updates).length === 0 && !shouldMarkSubscribed) {
        return prev
      }

      const newState = { ...oldState, ...updates, isSubscribed: true }
      const newMap = new Map(prev)
      newMap.set(taskId, newState)
      return newMap
    })
  }, [])

  const handleMessage = useCallback((msg: WSIncomingMessage) => {
    if (msg.type !== "message_v2") {
      flushPendingMessageV2()
    }
    const taskId = msg.taskId
    switch (msg.type) {
      case "message_v2":
        if (taskId && msg.data && typeof msg.data === "object" && msg.data !== null && "info" in msg.data) {
          const v2Msg = msg.data as WithParts
          let q = messageV2BatchRef.current.get(taskId)
          if (!q) {
            q = []
            messageV2BatchRef.current.set(taskId, q)
          }
          q.push(v2Msg)
          scheduleFlushMessageV2()
        }
        break

      case "task_status":
        if (taskId) {
          updateTaskState(taskId, () => 
            msg.sessionId ? { sessionId: msg.sessionId } : {}
          )
          const workDir =
            msg.data && typeof msg.data === "object" && msg.data !== null && "workDir" in msg.data
              ? String((msg.data as Record<string, unknown>).workDir || "").trim()
              : undefined
          optionsRef.current.onTaskStatus?.(
            taskId,
            msg.status || "",
            msg.sessionId,
            workDir || undefined,
          )
          
          // 任务结束后清空缓存，改为使用历史记录恢复
          if (msg.status === "done" || msg.status === "failed" || msg.status === "cancelled") {
            loadHistoryForTask(taskId, { force: true })
          }
        }
        break

      case "subscribed":
        if (taskId) {
          updateTaskState(taskId, () => ({})) // 只标记为已订阅
        }
        break

      case "retry":
        if (taskId) {
          updateTaskState(taskId, () => ({ shouldRetry: true }))
          optionsRef.current.onRetry?.(taskId)
        }
        break

      case "is_working":
        if (taskId) {
          updateTaskState(taskId, () => ({ isWorking: true }))
        }
        break

      case "is_not_working":
        if (taskId) {
          updateTaskState(taskId, () => ({ isWorking: false }))
        }
        break

      case "error":
        console.error('[GlobalWS] Error:', msg.error)
        optionsRef.current.onError?.(taskId, msg.error || "Unknown error")
        break

      case "session_title":
        if (taskId && msg.data && typeof msg.data === 'string') {
          optionsRef.current.onSessionTitle?.(taskId, msg.data)
        }
        break

      case "wait_user_input":
        if (taskId && msg.data && typeof msg.data === 'object') {
          optionsRef.current.onWaitUserInput?.(taskId, msg.data as Record<string, any>)
        }
        break

      case "task_queue": {
        const normalizedTaskId = Number(taskId)
        if (!Number.isFinite(normalizedTaskId) || normalizedTaskId <= 0) break
        if (msg.data == null) break
        const payload = normalizeTaskQueuePayload(msg.data)
        commitTaskQueueState(normalizedTaskId, payload)
        break
      }

      case "task_plan": {
        const normalizedTaskId = Number(taskId)
        if (!Number.isFinite(normalizedTaskId) || normalizedTaskId <= 0) break
        commitTaskPlanState(normalizedTaskId, normalizeTaskPlanPayload(msg.data))
        break
      }

      case "ilink_session_expired": {
        if (!msg.data || typeof msg.data !== "object") break
        const raw = msg.data as Record<string, unknown>
        const accountId = Number(raw.accountId)
        const botId = typeof raw.botId === "string" ? raw.botId : ""
        const ilinkUserId = typeof raw.ilinkUserId === "string" ? raw.ilinkUserId : ""
        if (!Number.isFinite(accountId) || accountId <= 0 || !botId) break
        optionsRef.current.onILinkSessionExpired?.({ accountId, botId, ilinkUserId })
        break
      }
    }
  }, [updateTaskState, loadHistoryForTask, flushPendingMessageV2, scheduleFlushMessageV2, commitTaskQueueState, commitTaskPlanState])

  // 注册消息监听器
  useEffect(() => {
    globalWsListeners.add(handleMessage)
    // 将可能在监听器注册前收到的消息立即分发，避免遗漏（如 shimmer-loading）
    if (pendingMessages.length > 0) {
      isDrainingPending = true
      try {
        while (pendingMessages.length > 0) {
          const buffered = [...pendingMessages]
          pendingMessages = []
          buffered.forEach(msg => handleMessage(msg))
        }
      } finally {
        isDrainingPending = false
      }
    }

    // 尝试连接
    getGlobalWs()
      .then(() => setIsConnected(true))
      .catch(() => setIsConnected(false))

    // 定期检查连接状态
    const checkInterval = setInterval(() => {
      setIsConnected(globalWsInstance?.readyState === WebSocket.OPEN)
    }, 1000)

    const onVisibilityChange = () => {
      if (typeof document === "undefined" || document.hidden) return
      cancelScheduledMessageV2Flush()
      flushPendingMessageV2()
    }
    if (typeof document !== "undefined") {
      document.addEventListener("visibilitychange", onVisibilityChange)
    }

    return () => {
      globalWsListeners.delete(handleMessage)
      clearInterval(checkInterval)
      if (typeof document !== "undefined") {
        document.removeEventListener("visibilitychange", onVisibilityChange)
      }
      cancelScheduledMessageV2Flush()
      flushPendingMessageV2()
    }
  }, [handleMessage, flushPendingMessageV2, cancelScheduledMessageV2Flush])

  // 订阅任务
  const subscribe = useCallback((taskId: number) => {
    flushPendingMessageV2()
    let historyForce = false

    setTaskStates(prev => {
      const existingState = prev.get(taskId) || createDefaultTaskState()
      const hasMessages = existingState.messagesV2.length > 0
      historyForce = hasMessages

      const newMap = new Map(prev)
      newMap.set(taskId, {
        ...existingState,
        isSubscribed: true,
        messagesV2IndexById:
          existingState.messagesV2IndexById ?? buildMessagesV2Index(existingState.messagesV2),
        // 无缓存时立即进入加载态，避免短暂显示「暂无消息」
        isHistoryLoading: hasMessages ? existingState.isHistoryLoading : true,
      })
      return newMap
    })

    sendWsMessage({ action: "subscribe", taskId })

    setTimeout(() => {
      void loadTaskQueueForTask(taskId)
      void loadTaskPlanForTask(taskId)
      void loadHistoryForTask(taskId, { force: historyForce })
    }, 0)
  }, [loadHistoryForTask, loadTaskQueueForTask, loadTaskPlanForTask, flushPendingMessageV2])

  // 取消订阅任务（保留消息缓存，切换回来时可立即展示并后台同步）
  const unsubscribe = useCallback((taskId: number) => {
    flushPendingMessageV2()
    sendWsMessage({ action: "unsubscribe", taskId })
    setTaskStates(prev => {
      const state = prev.get(taskId)
      if (!state) return prev

      const newMap = new Map(prev)
      newMap.set(taskId, {
        ...state,
        isSubscribed: false,
      })
      return newMap
    })
  }, [flushPendingMessageV2])

  // 发送消息到任务
  const sendMessage = useCallback((taskId: number, content: string, parts?: UserMessagePart[]) => {
    sendWsMessage({
      action: "send_message",
      taskId,
      content,
      ...(parts && parts.length > 0 ? { parts } : {}),
    })
  }, [])

  // 重启任务
  const restart = useCallback((taskId: number) => {
        sendWsMessage({ action: "restart", taskId })
  }, [])

  // 停止任务
  const stop = useCallback((taskId: number) => {
    sendWsMessage({ action: "stop", taskId })
  }, [])

  const cancelTool = useCallback((taskId: number, callID: string) => {
    sendWsMessage({
      action: "cancel_tool",
      taskId,
      data: { callID }
    })
  }, [])

  const waitUserInput = useCallback((taskId: number, id: string, result: Record<string, any>) => {
    sendWsMessage({
      action: "wait_user_input",
      taskId,
      content: JSON.stringify({ id, result })
    })
  }, [])

  // 获取任务会话 ID
  const getSessionId = useCallback((taskId: number): string | undefined => {
    return taskStates.get(taskId)?.sessionId
  }, [taskStates])

  // 获取是否应该显示重试
  const getShouldRetry = useCallback((taskId: number): boolean => {
    return taskStates.get(taskId)?.shouldRetry || false
  }, [taskStates])

  // 清除重试标记
  const clearRetry = useCallback((taskId: number) => {
    flushPendingMessageV2()
    setTaskStates(prev => {
      const state = prev.get(taskId)
      if (!state) return prev
      
      const newMap = new Map(prev)
      newMap.set(taskId, { ...state, shouldRetry: false })
      return newMap
    })
  }, [flushPendingMessageV2])

  // 清除任务消息
  const clearMessages = useCallback((taskId: number) => {
    flushPendingMessageV2()
    setTaskStates(prev => {
      const state = prev.get(taskId)
      if (!state) return prev
      
      const newMap = new Map(prev)
      newMap.set(taskId, {
        ...state,
        messagesV2: [],
        messagesV2IndexById: new Map(),
        hasMoreHistory: false,
        nextBeforeMessageId: undefined,
        isHistoryLoading: false,
      })
      return newMap
    })
  }, [flushPendingMessageV2])

  const reloadHistory = useCallback((taskId: number) => {
    return loadHistoryForTask(taskId, { force: true })
  }, [loadHistoryForTask])

  const loadMoreHistory = useCallback((taskId: number) => {
    const state = taskStates.get(taskId)
    if (!state?.hasMoreHistory || !state.nextBeforeMessageId || state.isHistoryLoading) {
      return Promise.resolve()
    }
    return loadHistoryForTask(taskId, {
      force: true,
      beforeMessageId: state.nextBeforeMessageId,
    })
  }, [loadHistoryForTask, taskStates])

  const setTaskQueueState = useCallback((taskId: number, payload: { queue: TaskMessageQueueItem[]; autoSend: boolean }) => {
    commitTaskQueueState(taskId, payload)
  }, [commitTaskQueueState])

  return {
    isConnected,
    subscribe,
    unsubscribe,
    sendMessage,
    restart,
    stop,
    cancelTool,
    waitUserInput,
    getSessionId,
    getShouldRetry,
    clearRetry,
    clearMessages,
    reloadHistory,
    loadMoreHistory,
    loadTaskQueueForTask,
    loadTaskPlanForTask,
    setTaskQueueState,
    taskStates
  }
}

// 简化的 Hook，用于单个任务（内部仅调用一次 useGlobalWebSocket，避免重复注册 WS 监听器）
export function useTaskMessages(
  taskId: number | null,
  globalOptions: UseGlobalWebSocketOptions = {}
) {
  const { 
    isConnected, 
    subscribe, 
    unsubscribe, 
    sendMessage,
    waitUserInput,
    stop,
    cancelTool,
    restart,
    clearRetry,
    reloadHistory,
    loadMoreHistory,
    loadTaskQueueForTask,
    loadTaskPlanForTask,
    setTaskQueueState,
    taskStates 
  } = useGlobalWebSocket(globalOptions)

  // 订阅/取消订阅
  useEffect(() => {
    if (taskId) {
      subscribe(taskId)
      return () => unsubscribe(taskId)
    }
  }, [taskId, subscribe, unsubscribe])
  
  // 直接提取当前任务的具体字段作为依赖，而不是整个 state 对象
  const currentTaskState = taskId ? taskStates.get(taskId) : undefined
  const messagesV2Array = currentTaskState?.messagesV2
  const shouldRetryValue = currentTaskState?.shouldRetry
  const isWorkingValue = currentTaskState?.isWorking
  const hasMoreHistoryValue = currentTaskState?.hasMoreHistory
  const isHistoryLoadingValue = currentTaskState?.isHistoryLoading

  const messagesV2 = useMemo(() => {
    return messagesV2Array || []
  }, [messagesV2Array])
  
  const shouldRetry = useMemo(() => {
    return shouldRetryValue || false
  }, [shouldRetryValue])
  
  const isWorking = useMemo(() => {
    return isWorkingValue || false
  }, [isWorkingValue])

  const hasMoreHistory = useMemo(() => {
    return hasMoreHistoryValue || false
  }, [hasMoreHistoryValue])

  const isHistoryLoading = useMemo(() => {
    return isHistoryLoadingValue || false
  }, [isHistoryLoadingValue])

  const messageQueue = useMemo(() => {
    return currentTaskState?.messageQueue ?? []
  }, [currentTaskState?.messageQueue])

  const messageQueueAutoSend = useMemo(() => {
    return currentTaskState?.messageQueueAutoSend !== false
  }, [currentTaskState?.messageQueueAutoSend])

  const taskPlan = useMemo(() => {
    return currentTaskState?.taskPlan ?? null
  }, [currentTaskState?.taskPlan])

  const send = useCallback((content: string, parts?: UserMessagePart[]) => {
    if (taskId) {
      sendMessage(taskId, content, parts)
    }
  }, [taskId, sendMessage])

  const stopTask = useCallback(() => {
    if (taskId) {
      stop(taskId)
    }
  }, [taskId, stop])

  const cancelTaskTool = useCallback((callID: string) => {
    if (taskId) {
      cancelTool(taskId, callID)
    }
  }, [taskId, cancelTool])

  const restartTask = useCallback(() => {
    if (taskId) {
      restart(taskId)
    }
  }, [taskId, restart])

  const dismissRetry = useCallback(() => {
    if (taskId) {
      clearRetry(taskId)
    }
  }, [taskId, clearRetry])

  const reloadTaskHistory = useCallback(() => {
    if (!taskId) return Promise.resolve()
    return reloadHistory(taskId)
  }, [taskId, reloadHistory])

  const loadMoreTaskHistory = useCallback(() => {
    if (!taskId) return Promise.resolve()
    return loadMoreHistory(taskId)
  }, [taskId, loadMoreHistory])
  
  return {
    messagesV2,
    isConnected,
    sendMessage: send,
    stopTask,
    cancelTaskTool,
    restartTask,
    shouldRetry,
    dismissRetry,
    isWorking,
    reloadHistory: reloadTaskHistory,
    loadMoreHistory: loadMoreTaskHistory,
    hasMoreHistory,
    isHistoryLoading,
    messageQueue,
    messageQueueAutoSend,
    taskPlan,
    loadTaskQueueForTask,
    loadTaskPlanForTask,
    setTaskQueueState,
    waitUserInput,
  }
}

const ARCHIVED_SESSION_LOGS_PAGE_SIZE = 100

export function useArchivedSessionHistory(sessionId: string | null | undefined) {
  const [messages, setMessages] = useState<WithParts[]>([])
  const [hasMoreHistory, setHasMoreHistory] = useState(false)
  const [nextBeforeMessageId, setNextBeforeMessageId] = useState<string | undefined>()
  const [isHistoryLoading, setIsHistoryLoading] = useState(false)

  const loadPage = useCallback(async (targetSessionId: string, beforeMessageId?: string) => {
    setIsHistoryLoading(true)
    try {
      const history = await api.getSessionLogsV2(targetSessionId, {
        limit: ARCHIVED_SESSION_LOGS_PAGE_SIZE,
        beforeMessageId,
      })
      const items = Array.isArray(history.items) ? history.items : []
      setMessages((prev) => (beforeMessageId ? mergeHistoryPages(prev, items) : items))
      setHasMoreHistory(history.hasMore)
      setNextBeforeMessageId(history.nextBeforeMessageId)
    } catch (error) {
      console.error("[ArchivedSession] Failed to load history:", error)
      if (!beforeMessageId) {
        setMessages([])
        setHasMoreHistory(false)
        setNextBeforeMessageId(undefined)
      }
    } finally {
      setIsHistoryLoading(false)
    }
  }, [])

  useEffect(() => {
    if (!sessionId) {
      setMessages([])
      setHasMoreHistory(false)
      setNextBeforeMessageId(undefined)
      setIsHistoryLoading(false)
      return
    }

    void loadPage(sessionId)
  }, [sessionId, loadPage])

  const loadMoreHistory = useCallback(() => {
    if (!sessionId || !hasMoreHistory || !nextBeforeMessageId || isHistoryLoading) {
      return Promise.resolve()
    }
    return loadPage(sessionId, nextBeforeMessageId)
  }, [sessionId, hasMoreHistory, nextBeforeMessageId, isHistoryLoading, loadPage])

  return {
    messages,
    hasMoreHistory,
    isHistoryLoading,
    loadMoreHistory,
  }
}
