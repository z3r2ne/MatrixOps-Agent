import type { TaskMessageQueueItem } from "@/lib/api"

export type TaskQueuePayload = {
  queue: TaskMessageQueueItem[]
  autoSend: boolean
}

/** 统一解析 HTTP / WebSocket 的队列载荷（兼容旧版纯数组广播） */
export function normalizeTaskQueuePayload(data: unknown): TaskQueuePayload {
  if (Array.isArray(data)) {
    return { queue: data as TaskMessageQueueItem[], autoSend: true }
  }
  if (!data || typeof data !== "object") {
    return { queue: [], autoSend: true }
  }
  const record = data as Record<string, unknown>
  const rawQueue = record.queue ?? record.messageQueue ?? record.items
  const queue = Array.isArray(rawQueue) ? (rawQueue as TaskMessageQueueItem[]) : []
  const autoSend =
    record.autoSend !== false &&
    record.messageQueueAutoSend !== false
  return { queue, autoSend }
}
