import type { WithParts } from "@/lib/api"

function normalizeWhitespace(value: string): string {
  return value.replace(/\r\n/g, "\n").replace(/\r/g, "\n").replace(/\n{3,}/g, "\n\n").trim()
}

function partToLine(part: WithParts["parts"][number]): string {
  if (!part) return ""
  if (part.type === "text" && part.text) {
    return normalizeWhitespace(part.text)
  }
  if (part.type === "tool" && part.tool) {
    const toolName = part.tool.tool || "tool"
    const state = part.tool.state
    const title = state?.title?.trim()
    const output = state?.output?.trim()
    const status = state?.status?.trim()
    if (output) return normalizeWhitespace(stripToolPrefix(output)).slice(0, 280)
    if (title) return `[${toolName}] ${title}`
    if (status) return `[${toolName}] ${status}`
    return `[${toolName}] 执行中…`
  }
  if (part.type === "error") {
    const message = part.error?.message || part.text
    return message ? `错误: ${message}` : "错误"
  }
  return ""
}

function stripToolPrefix(value: string): string {
  return value.replace(/^\[[^\]]+\]\s*/u, "").trim()
}

/** 从任务消息中提取适合在仿真显示器上展示的文本。 */
export function extractSimulationScreenOutput(messages: WithParts[] | undefined, fallback = "等待 Agent 输出…"): string {
  if (!messages?.length) return fallback

  const lines: string[] = []
  for (const message of messages.slice(-6)) {
    const role = message.info?.role === "user" ? "用户" : message.info?.worker || "assistant"
    for (const part of message.parts ?? []) {
      const line = partToLine(part)
      if (!line) continue
      lines.push(`${role}: ${line}`)
    }
  }

  const merged = lines.join("\n").trim()
  if (!merged) return fallback
  return merged.length > 1200 ? merged.slice(-1200) : merged
}

export function taskStatusLabel(status?: string): string {
  switch ((status || "").toLowerCase()) {
    case "running":
    case "active":
      return "执行中"
    case "done":
    case "completed":
    case "success":
      return "已完成"
    case "failed":
    case "error":
      return "失败"
    case "queue":
      return "排队中"
    case "cancelled":
    case "canceled":
      return "已取消"
    default:
      return status || "未知"
  }
}
