/**
 * 工具调用一行摘要与补丁统计（供聊天区工具卡片使用）
 */

export type ToolVisualGroup =
  | "read"
  | "write"
  | "patch"
  | "edit"
  | "bash"
  | "search"
  | "files"
  | "tree"
  | "diff"
  | "other"

/** 与后端 tool.Name() / VerbosName 一致 */
export function toolDisplayLabel(name: string): string {
  const n = (name || "").trim()
  switch (n) {
    case "message":
      return "发送消息"
    case "remind":
      return "提醒"
    default:
      return n || "tool"
  }
}

export function classifyToolGroup(name: string): ToolVisualGroup {
  const n = (name || "").trim()
  if (n === "read" || n === "read_whole") return "read"
  if (n === "write") return "write"
  if (n === "patch") return "patch"
  if (n === "edit") return "edit"
  if (n === "bash") return "bash"
  if (n === "grep" || n === "rg") return "search"
  if (n === "glob" || n === "list") return "files"
  if (n === "tree") return "tree"
  if (n === "diff") return "diff"
  return "other"
}

function num(v: unknown): number | undefined {
  if (typeof v === "number" && Number.isFinite(v)) return v
  if (typeof v === "string" && v.trim() !== "") {
    const x = Number(v)
    if (Number.isFinite(x)) return x
  }
  return undefined
}

function str(v: unknown): string {
  return typeof v === "string" ? v : ""
}

function basename(p: string): string {
  const s = p.replace(/\\/g, "/")
  const i = s.lastIndexOf("/")
  return i >= 0 ? s.slice(i + 1) : s
}

/** V4A / apply_patch 风格：统计以 + / - 开头的变更行（跳过 diff 文件头） */
export function countPatchLineStats(patchText: string): { added: number; removed: number } {
  let added = 0
  let removed = 0
  for (const line of patchText.split("\n")) {
    if (line.startsWith("+++ ") || line.startsWith("--- ")) continue
    if (line.startsWith("@@")) continue
    if (line.startsWith("*** ")) continue
    if (line.startsWith("+")) added += 1
    else if (line.startsWith("-")) removed += 1
  }
  return { added, removed }
}

export function extractPatchFilePaths(patchText: string): string[] {
  const paths: string[] = []
  const re = /^\*\*\* (?:Add|Update|Delete) File:\s*(.+)$/gm
  let m: RegExpExecArray | null
  while ((m = re.exec(patchText)) !== null) {
    paths.push(m[1].trim())
  }
  return paths
}

function formatReadRange(input: Record<string, unknown>): string {
  const offset = num(input.offset) ?? 0
  const limit = num(input.limit)
  if (limit != null && limit > 0) {
    const startLine = offset + 1
    const endLine = offset + limit
    return `lines ${startLine}–${endLine}`
  }
  if (offset > 0) {
    return `from line ${offset + 1}`
  }
  return "full file"
}

export function buildCompactSummary(
  toolName: string,
  input: Record<string, unknown> | undefined,
  metadata: Record<string, unknown> | undefined,
): string {
  const name = (toolName || "").trim()
  const meta = metadata || {}
  const inp = input || {}

  switch (name) {
    case "read":
    case "read_whole": {
      const path = str(inp.path) || "—"
      const short = basename(path) !== path ? basename(path) : path
      if (name === "read_whole") {
        return `${short} · full file`
      }
      return `${short} · ${formatReadRange(inp)}`
    }
    case "write": {
      const path = str(inp.path) || "—"
      const content = str(inp.content)
      const lines = content ? content.split("\n").length : 0
      return `${basename(path)} · +${lines} lines`
    }
    case "patch": {
      const patch = str(inp.patch)
      if (!patch) return "patch"
      const { added, removed } = countPatchLineStats(patch)
      const files = extractPatchFilePaths(patch)
      const head =
        files.length === 0
          ? "patch"
          : files.length === 1
            ? basename(files[0])
            : `${files.length} files`
      const stats =
        added === 0 && removed === 0 ? "" : ` · +${added} −${removed}`
      return `${head}${stats}`
    }
    case "edit": {
      const path = str(inp.path) || "—"
      const oldT = str(inp.old)
      const newT = str(inp.new)
      const oldLines = oldT ? oldT.split("\n").length : 0
      const newLines = newT ? newT.split("\n").length : 0
      return `${basename(path)} · replace ${oldLines}→${newLines} lines`
    }
    case "bash": {
      const cmd = str(meta.command) || str(inp.command) || ""
      const one = cmd.replace(/\s+/g, " ").trim()
      if (!one) return ""
      return one.length > 72 ? `${one.slice(0, 69)}…` : one
    }
    case "grep":
    case "rg": {
      const pattern = str(inp.pattern) || "—"
      const path = str(inp.path) || "."
      return `${pattern.slice(0, 40)}${pattern.length > 40 ? "…" : ""} · ${basename(path) || path}`
    }
    case "glob": {
      const pattern = str(inp.pattern) || "—"
      const root = str(inp.root) || "."
      return `${pattern} · ${basename(root) || root}`
    }
    case "tree": {
      const path = str(inp.path) || "."
      const depth = num(inp.depth)
      return depth != null ? `${basename(path) || path} · depth ${depth}` : basename(path) || path
    }
    case "list": {
      const path = str(inp.path) || "."
      return basename(path) || path
    }
    case "diff": {
      const from = str(inp.from)
      const to = str(inp.to)
      const ft = str(inp.fromType)
      const tt = str(inp.toType)
      if (from && to) {
        const left = from.length > 24 ? `${from.slice(0, 12)}…` : from
        const right = to.length > 24 ? `${to.slice(0, 12)}…` : to
        return ft && tt ? `${left} → ${right}` : `${left} ↔ ${right}`
      }
      const a = str(inp.a) || str(inp.path_a) || ""
      const b = str(inp.b) || str(inp.path_b) || ""
      if (a && b) return `${basename(a)} ↔ ${basename(b)}`
      return "diff"
    }
    default:
      return ""
  }
}

/** 与聊天 / 取消逻辑共享 */
export function isToolRunningStatus(status?: string): boolean {
  return status === "running" || status === "pending" || status === "preparing" || status === "input-streaming"
}

export function stripToolOutputPrefix(output: string): string {
  const s = output || ""
  const prefix = "[Tool Output]: "
  if (s.startsWith(prefix)) return s.slice(prefix.length)
  return s
}
