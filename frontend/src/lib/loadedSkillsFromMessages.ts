import type { Part, ToolPart, WithParts } from "@/lib/api"

export interface LoadedSkillFromChat {
  name: string
  callID?: string
}

function asRecord(value: unknown): Record<string, unknown> | undefined {
  if (value && typeof value === "object" && !Array.isArray(value)) {
    return value as Record<string, unknown>
  }
  return undefined
}

function skillNameFromLoadSkillInput(input: Record<string, unknown> | undefined): string {
  if (!input) return ""
  for (const key of ["name", "skill", "skill_name", "skillName"]) {
    const value = input[key]
    if (typeof value === "string" && value.trim()) {
      return value.trim()
    }
  }
  return ""
}

function collectLoadSkillNamesFromCallToolInput(input: Record<string, unknown> | undefined): string[] {
  if (!input) return []

  const names: string[] = []
  const directToolName = typeof input.tool_name === "string" ? input.tool_name.trim() : ""
  if (directToolName === "load_skill") {
    const toolInput = asRecord(input.tool_input)
    const name = skillNameFromLoadSkillInput(toolInput)
    if (name) names.push(name)
  }

  const batch = input.tool_calls
  if (Array.isArray(batch)) {
    for (const entry of batch) {
      const item = asRecord(entry)
      if (!item) continue
      const toolName = typeof item.tool_name === "string" ? item.tool_name.trim() : ""
      if (toolName !== "load_skill") continue
      const name = skillNameFromLoadSkillInput(asRecord(item.tool_input))
      if (name) names.push(name)
    }
  }

  return names
}

function parseLoadSkillNamesFromRawOutput(raw: string): string[] {
  const trimmed = raw.trim()
  if (!trimmed) return []
  try {
    const envelope = JSON.parse(trimmed) as Record<string, unknown>
    const action = typeof envelope.call_tool === "string" ? envelope.call_tool.trim() : ""
    if (action === "load_skill") {
      const name = skillNameFromLoadSkillInput(asRecord(envelope.params))
      return name ? [name] : []
    }
    if (action === "call_tool") {
      return collectLoadSkillNamesFromCallToolInput(asRecord(envelope.params))
    }
  } catch {
    return []
  }
  return []
}

function resolveLoadSkillNames(tool: ToolPart, part: Part): string[] {
  const toolName = (tool.tool || "").trim()
  const input = asRecord(tool.state.input)

  if (toolName === "load_skill") {
    const name = skillNameFromLoadSkillInput(input)
    return name ? [name] : []
  }

  if (toolName === "call_tool") {
    const fromInput = collectLoadSkillNamesFromCallToolInput(input)
    if (fromInput.length) return fromInput
  }

  const rawOutput = typeof part.metadata?.rawOutput === "string" ? part.metadata.rawOutput : ""
  return parseLoadSkillNamesFromRawOutput(rawOutput)
}

function isSuccessfulLoadSkillCall(tool: ToolPart): boolean {
  const status = (tool.state.status || "").trim().toLowerCase()
  if (status === "error" || status === "cancelled") return false
  if (tool.state.error && String(tool.state.error).trim()) return false
  if (status !== "completed") return false

  const output = (tool.state.output || "").trim()
  if (output && !output.includes("[Skill:") && output.startsWith("[Tool Output]: call tool failed")) {
    return false
  }
  return true
}

function isToolPart(part: Part): part is Part & { tool: ToolPart } {
  return (part.type === "tool" || part.type === "tool-delta") && Boolean(part.tool)
}

/**
 * 从会话消息历史中扫描已成功执行的 load_skill 工具调用，并从调用参数解析技能名。
 */
export function loadedSkillsFromMessages(messages: WithParts[]): LoadedSkillFromChat[] {
  const latestByCall = new Map<string, { tool: ToolPart; part: Part }>()

  for (const message of messages) {
    for (const part of message.parts) {
      if (!isToolPart(part)) continue
      const callID = part.tool.callID?.trim() || part.id
      latestByCall.set(callID, { tool: part.tool, part })
    }
  }

  const seen = new Set<string>()
  const result: LoadedSkillFromChat[] = []

  for (const { tool, part } of latestByCall.values()) {
    if (!isSuccessfulLoadSkillCall(tool)) continue
    const names = resolveLoadSkillNames(tool, part)
    for (const name of names) {
      const key = name.toLowerCase()
      if (seen.has(key)) continue
      seen.add(key)
      result.push({
        name,
        callID: tool.callID || undefined,
      })
    }
  }

  return result
}
