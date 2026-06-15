import type { UserInputFileSource } from "@/hooks/useGlobalWebSocket"
import type { Part, WithParts } from "@/lib/api"

/** 单条附件最大体积（与 WS / 浏览器性能平衡） */
export const CHAT_ATTACHMENT_MAX_BYTES = 4 * 1024 * 1024
export const USER_MESSAGE_COLLAPSE_MAX_HEIGHT_PX = 220

export type LocalChatAttachment = {
  id: string
  file: File
  previewUrl: string
  mime: string
  name: string
  source: UserInputFileSource
}

export function filePreviewURL(file: File): string {
  return URL.createObjectURL(file)
}

export type ProjectFilePromptItem = {
  path: string
  prompt: string
}

export function basename(path: string) {
  const cleaned = String(path || "").trim()
  if (!cleaned) return ""
  const parts = cleaned.split("/").filter(Boolean)
  return parts.length ? parts[parts.length - 1] : cleaned
}


export const RUNNING_TOOLS_MESSAGE_LOOKBACK = 48
/** 超过此条数使用虚拟列表，降低 DOM 节点数 */
export const VIRTUAL_MESSAGE_LIST_THRESHOLD = 50
/** 距底部小于此像素视为「在底部」，新消息时自动跟随滚动 */
export const CHAT_STICKY_BOTTOM_THRESHOLD_PX = 80
export function isScrollContainerNearBottom(el: HTMLElement): boolean {
  const { scrollTop, scrollHeight, clientHeight } = el
  return scrollHeight - scrollTop - clientHeight <= CHAT_STICKY_BOTTOM_THRESHOLD_PX
}

export function formatMessageTime(timestamp?: number) {
  if (!timestamp) return ""
  try {
    return new Date(timestamp).toLocaleTimeString("zh-CN", {
      hour: "2-digit",
      minute: "2-digit",
      second: "2-digit",
    })
  } catch {
    return String(timestamp)
  }
}

export function partCreatedAt(part: Part, fallback: number) {
  return (
    part.time?.created ||
    part.time?.start ||
    part.tool?.state?.time?.created ||
    part.tool?.state?.time?.start ||
    fallback
  )
}

export function sortMessageParts(parts: Part[], fallback: number) {
  return [...parts].sort((left, right) => {
    const leftCreated = partCreatedAt(left, fallback)
    const rightCreated = partCreatedAt(right, fallback)
    if (leftCreated !== rightCreated) {
      return leftCreated - rightCreated
    }
    return left.id.localeCompare(right.id)
  })
}

export function isSystemChatMessage(msg: WithParts): boolean {
  return msg.info.messageKind === "system"
}

export function isUserAuthoredMessage(msg: WithParts): boolean {
  return msg.info.role === "user" && !isSystemChatMessage(msg)
}

export function systemMessageOriginLabel(origin?: string): string {
  switch (origin) {
    case "reminder":
      return "提醒"
    case "stall_watchdog":
      return "停滞检测"
    case "tool_repeat_watchdog":
      return "重复工具"
    case "silent_tool_watchdog":
      return "阶段性总结"
    case "empty_stream_retry":
      return "重试"
    default:
      return "系统"
  }
}


export function mergeConsecutiveAssistantMessages(messages: WithParts[]): WithParts[] {
  if (messages.length === 0) {
    return messages
  }
  const merged: WithParts[] = []
  for (const message of messages) {
    const previous = merged[merged.length - 1]
    if (
      previous &&
      previous.info.role === "assistant" &&
      message.info.role === "assistant"
    ) {
      previous.parts = [...previous.parts, ...message.parts]
      if (!previous.info.completed && message.info.completed) {
        previous.info.completed = message.info.completed
      }
      if (!previous.info.finish && message.info.finish) {
        previous.info.finish = message.info.finish
      }
      if (message.info.footerStatus) {
        previous.info.footerStatus = message.info.footerStatus
      }
      previous.info.state = message.info.state || previous.info.state
      previous.info.error = message.info.error || previous.info.error
      previous.info.snapshot = message.info.snapshot || previous.info.snapshot
      previous.info.time = message.info.time || previous.info.time
      continue
    }
    merged.push({
      info: { ...message.info },
      parts: [...message.parts],
    })
  }
  return merged
}

export function formatCompactByteCount(value: number) {
  if (value >= 1024 * 1024) {
    return `${(value / (1024 * 1024)).toFixed(value >= 10 * 1024 * 1024 ? 0 : 1)}MB`
  }
  if (value >= 1024) {
    return `${(value / 1024).toFixed(value >= 10 * 1024 ? 0 : 1)}KB`
  }
  return `${value}B`
}

export function sanitizeExportFilename(input: string) {
  return input
    .trim()
    .replace(/[\\/:*?"<>|]+/g, "-")
    .replace(/\s+/g, "-")
    .replace(/-+/g, "-")
    .replace(/^-|-$/g, "")
}

export function buildSessionTransferFilename(title: string, ext: "zip" | "json" = "zip") {
  const safeTitle = sanitizeExportFilename(title || "session")
  const timestamp = new Date().toISOString().replace(/[:.]/g, "-")
  return `${safeTitle || "session"}-memory-chat-${timestamp}.${ext}`
}

