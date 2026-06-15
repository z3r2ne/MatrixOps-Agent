type PatchFileEntry = {
  action: "add" | "delete" | "update"
  path: string
  moveTo?: string
  lines: string[]
}

export type PatchUnifiedSection = {
  path: string
  unifiedDiff: string
}

const LANG_MAP: Record<string, string> = {
  ts: "typescript",
  tsx: "typescript",
  js: "javascript",
  jsx: "javascript",
  py: "python",
  go: "go",
  rs: "rust",
  java: "java",
  cpp: "cpp",
  c: "c",
  css: "css",
  html: "html",
  json: "json",
  md: "markdown",
  yaml: "yaml",
  yml: "yaml",
}

export function buildDiffViewData(filePath: string, unifiedDiff: string) {
  const ext = filePath.split(".").pop() || "txt"
  const lang = LANG_MAP[ext] || "plaintext"
  return {
    hunks: [unifiedDiff],
    oldFile: { fileName: filePath, fileLang: lang },
    newFile: { fileName: filePath, fileLang: lang },
  }
}

export function parsePatchToUnifiedSections(patchText: string): PatchUnifiedSection[] | null {
  const lines = normalizeLines(patchText)
  if (lines.length === 0 || lines[0] !== "*** Begin Patch" || lines[lines.length - 1] !== "*** End Patch") {
    return null
  }

  const files = parsePatchFiles(lines.slice(1, -1))
  if (files.length === 0) {
    return null
  }

  return files.map((file) => ({
    path: file.moveTo || file.path,
    unifiedDiff: entryToUnified(file),
  }))
}

function normalizeLines(text: string): string[] {
  const normalized = text.replace(/\r\n/g, "\n").replace(/\n+$/, "")
  if (!normalized) return []
  return normalized.split("\n")
}

function parsePatchFiles(lines: string[]): PatchFileEntry[] {
  const files: PatchFileEntry[] = []
  let current: PatchFileEntry | null = null

  const flush = () => {
    if (current) {
      files.push(current)
      current = null
    }
  }

  for (const line of lines) {
    if (line.startsWith("*** Add File: ")) {
      flush()
      current = {
        action: "add",
        path: line.slice("*** Add File: ".length).trim(),
        lines: [],
      }
      continue
    }
    if (line.startsWith("*** Delete File: ")) {
      flush()
      files.push({
        action: "delete",
        path: line.slice("*** Delete File: ".length).trim(),
        lines: [],
      })
      continue
    }
    if (line.startsWith("*** Update File: ")) {
      flush()
      current = {
        action: "update",
        path: line.slice("*** Update File: ".length).trim(),
        lines: [],
      }
      continue
    }
    if (line.startsWith("*** Move to: ")) {
      if (current) {
        current.moveTo = line.slice("*** Move to: ".length).trim()
      }
      continue
    }
    if (current) {
      current.lines.push(line)
    }
  }

  flush()
  return files
}

function entryToUnified(entry: PatchFileEntry): string {
  if (entry.action === "add") {
    const added = entry.lines.filter((line) => line.startsWith("+")).map((line) => line.slice(1))
    const body = added.map((line) => `+${line}`).join("\n")
    const header = added.length > 0 ? `@@ -0,0 +1,${added.length} @@` : "@@"
    return [`--- /dev/null`, `+++ b/${entry.path}`, header, body].filter(Boolean).join("\n")
  }

  if (entry.action === "delete") {
    return [
      `--- a/${entry.path}`,
      `+++ /dev/null`,
      "@@",
      `- ${entry.path}`,
    ].join("\n")
  }

  const newPath = entry.moveTo || entry.path
  const body = entry.lines.join("\n")
  return [`--- a/${entry.path}`, `+++ b/${newPath}`, body].filter(Boolean).join("\n")
}

export function resolvePatchDisplaySections(text: string): PatchUnifiedSection[] {
  const fromPatch = parsePatchToUnifiedSections(text)
  if (fromPatch?.length) {
    return fromPatch
  }

  const trimmed = text.trim()
  if (trimmed.includes("---") && trimmed.includes("+++")) {
    const path = extractPathFromUnified(trimmed) || "diff"
    return [{ path, unifiedDiff: trimmed }]
  }

  return [{ path: "patch", unifiedDiff: trimmed }]
}

function extractPathFromUnified(unifiedDiff: string): string | null {
  for (const line of unifiedDiff.split("\n")) {
    if (line.startsWith("+++ b/")) return line.slice("+++ b/".length).trim()
    if (line.startsWith("+++ ")) return line.slice("+++ ".length).trim()
  }
  return null
}
