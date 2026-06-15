import React, { useMemo } from "react"
import { useAutoScroll } from "@/hooks/useAutoScroll"
import {
  AlertCircle,
  Bot,
  Braces,
  ChevronRight,
  FileCode,
  FolderOpen,
  Loader2,
  Search,
  Terminal,
} from "lucide-react"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"
import {
  buildCompactSummary,
  classifyToolGroup,
  isToolRunningStatus,
  stripToolOutputPrefix,
  toolDisplayLabel,
} from "./toolDisplayUtils"

type SubtaskPreviewPart =
  | {
      id?: string
      type: "text"
      text?: string
    }
  | {
      id?: string
      type: "tool"
      toolName?: string
      status?: string
      title?: string
      input?: string
      output?: string
    }
  | {
      id?: string
      type: "error"
      message?: string
    }

type SubtaskPreviewMessage = {
  id?: string
  role?: string
  workerName?: string
  parts?: SubtaskPreviewPart[]
}

function normalizePreviewText(value: string): string {
  return value
    .replace(/\r\n/g, "\n")
    .replace(/\r/g, "\n")
    .replace(/\n{3,}/g, "\n\n")
    .trim()
}

function getStatusTone(status: string) {
  const normalized = status.trim().toLowerCase()

  if (normalized === "failed" || normalized === "error") {
    return {
      label: "失败",
      cardClassName: "border-l-red-500",
      badgeClassName: "border-red-200 bg-red-50 text-red-700",
      iconClassName: "bg-red-50 text-red-700",
    }
  }

  if (normalized === "cancelled" || normalized === "canceled") {
    return {
      label: "已取消",
      cardClassName: "border-l-amber-500",
      badgeClassName: "border-amber-200 bg-amber-50 text-amber-800",
      iconClassName: "bg-amber-50 text-amber-800",
    }
  }

  if (normalized === "done" || normalized === "success" || normalized === "completed") {
    return {
      label: "已完成",
      cardClassName: "border-l-emerald-500",
      badgeClassName: "border-emerald-200 bg-emerald-50 text-emerald-700",
      iconClassName: "bg-emerald-50 text-emerald-700",
    }
  }

  return {
    label: "执行中",
    cardClassName: "border-l-blue-500",
    badgeClassName: "border-blue-200 bg-blue-50 text-blue-700",
    iconClassName: "bg-blue-50 text-blue-700",
  }
}

function getToolIcon(name: string) {
  const group = classifyToolGroup(name)
  if (group === "bash") return Terminal
  if (group === "search") return Search
  if (group === "files" || group === "tree") return FolderOpen
  if (group === "read" || group === "write" || group === "patch" || group === "edit" || group === "diff") return FileCode
  return Braces
}

function normalizePreviewMessages(value: unknown): SubtaskPreviewMessage[] {
  if (!Array.isArray(value)) return []
  return value
    .map((item) => {
      if (!item || typeof item !== "object") return null
      const raw = item as Record<string, unknown>
      const parts = Array.isArray(raw.parts)
        ? raw.parts.filter((part): part is SubtaskPreviewPart => !!part && typeof part === "object" && typeof (part as { type?: unknown }).type === "string") as SubtaskPreviewPart[]
        : []
      return {
        id: typeof raw.id === "string" ? raw.id : undefined,
        role: typeof raw.role === "string" ? raw.role : undefined,
        workerName: typeof raw.workerName === "string" ? raw.workerName : undefined,
        parts,
      } satisfies SubtaskPreviewMessage
    })
    .filter((item): item is SubtaskPreviewMessage => !!item && Array.isArray(item.parts) && item.parts.length > 0)
}

function LogRow({
  icon,
  tone,
  label,
  content,
  spinning = false,
}: {
  icon: React.ElementType
  tone?: "default" | "blue" | "emerald" | "red"
  label: string
  content: string
  spinning?: boolean
}) {
  const Icon = icon
  return (
    <div className="flex h-6 min-w-0 items-center gap-2 rounded border border-border/40 bg-muted/15 px-2 text-[11px] leading-none">
      <Icon
        className={cn(
          "h-3.5 w-3.5 shrink-0",
          spinning && "animate-spin",
          tone === "blue" ? "text-blue-600" :
            tone === "emerald" ? "text-emerald-600" :
              tone === "red" ? "text-red-600" :
                "text-muted-foreground",
        )}
      />
      <span className="shrink-0 font-mono font-semibold text-foreground/85">{label}</span>
      <span className="min-w-0 flex-1 truncate text-muted-foreground" title={content}>
        {content}
      </span>
    </div>
  )
}

function MiniToolLogRow({ part }: { part: Extract<SubtaskPreviewPart, { type: "tool" }> }) {
  const toolName = part.toolName || "tool"
  const isRunning = isToolRunningStatus(part.status)
  const isFailed = part.status === "failed" || part.status === "error"
  const isCancelled = part.status === "cancelled" || part.status === "canceled"
  const isDone = part.status === "done" || part.status === "success" || part.status === "completed"
  const ToolIcon = getToolIcon(toolName)
  const compact = buildCompactSummary(toolName, undefined, {})
  const inputPreview = normalizePreviewText(part.input || "")
  const output = normalizePreviewText(stripToolOutputPrefix(part.output || ""))
  const content = normalizePreviewText(inputPreview || part.title || compact || output || part.status || "")
  const tone = isFailed ? "red" : isCancelled ? "amber" : isDone ? "emerald" : isRunning ? "blue" : "default"

  return (
    <LogRow
      icon={isRunning ? Loader2 : ToolIcon}
      tone={tone}
      label={toolDisplayLabel(toolName)}
      content={content || "工具调用"}
      spinning={isRunning}
    />
  )
}

type StreamItem =
  | { key: string; kind: "assistant"; content: string }
  | { key: string; kind: "tool"; part: Extract<SubtaskPreviewPart, { type: "tool" }> }
  | { key: string; kind: "error"; content: string }

function buildStreamItems(messages: SubtaskPreviewMessage[]): StreamItem[] {
  const items: StreamItem[] = []

  for (const message of messages) {
    for (const part of message.parts || []) {
      if (part.type === "text" && part.text?.trim()) {
        items.push({
          key: `${message.id || "message"}:${part.id || "part"}:text`,
          kind: "assistant",
          content: part.text,
        })
      } else if (part.type === "tool") {
        items.push({
          key: `${message.id || "message"}:${part.id || "part"}:tool`,
          kind: "tool",
          part,
        })
      } else if (part.type === "error" && part.message?.trim()) {
        items.push({
          key: `${message.id || "message"}:${part.id || "part"}:error`,
          kind: "error",
          content: part.message,
        })
      }
    }
  }

  return items
}

export interface SubtaskToolCardProps {
  taskId?: number | null
  workerName: string
  taskName: string
  content: string
  status: string
  answer: string
  previewMessages?: unknown
  onOpen?: (taskId: number) => void
}

export function SubtaskToolCard({
  taskId,
  workerName,
  taskName,
  content,
  status,
  answer,
  previewMessages,
  onOpen,
}: SubtaskToolCardProps) {
  const tone = getStatusTone(status)
  const hasTaskId = typeof taskId === "number" && Number.isFinite(taskId) && taskId > 0
  const title = taskName || content || (hasTaskId ? `子任务 #${taskId}` : "正在创建子任务…")
  const fallbackPreview = useMemo(() => normalizePreviewText(answer || content), [answer, content])
  const normalizedPreviewMessages = useMemo(() => normalizePreviewMessages(previewMessages), [previewMessages])
  const streamItems = useMemo(() => buildStreamItems(normalizedPreviewMessages), [normalizedPreviewMessages])
  const { ref: streamContainerRef, handleScroll } = useAutoScroll({
    deps: [streamItems],
    enabled: isToolRunningStatus(status),
  })



  return (
    <div
      className={cn(
        "group ml-8 mb-2 min-w-0 rounded-r-md border border-border/45 border-l-[3px] bg-muted/10 px-2 py-1.5 text-sm transition-colors",
        tone.cardClassName,
        hasTaskId && onOpen && "focus-within:ring-2 focus-within:ring-primary/40 hover:bg-accent/25",
      )}
    >
      <div className="flex min-w-0 items-start gap-2">
        <div className={cn("mt-0.5 flex h-5 w-5 shrink-0 items-center justify-center rounded-sm", tone.iconClassName)}>
          {isToolRunningStatus(status) ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <ChevronRight className="h-3.5 w-3.5" />}
        </div>

        <div className="min-w-0 flex-1">
          <div className="flex min-w-0 flex-wrap items-center gap-1.5">
            <span className="font-mono text-[11px] font-semibold tracking-tight text-foreground/90">
              子 Worker · {workerName || "worker"}
            </span>
            <Badge variant="outline" className={cn("rounded-full px-1.5 py-0 text-[10px] font-medium", tone.badgeClassName)}>
              {tone.label}
            </Badge>
            {hasTaskId ? (
              <span className="rounded-full border border-border bg-background/80 px-2 py-0.5 text-[10px] text-muted-foreground">
                #{taskId}
              </span>
            ) : (
              <span className="rounded-full border border-border bg-background/80 px-2 py-0.5 text-[10px] text-muted-foreground">
                创建中
              </span>
            )}
            {isToolRunningStatus(status) && (
              <span className="inline-flex items-center gap-1 text-[10px] text-muted-foreground">
                <Loader2 className="h-3 w-3 animate-spin" />
                实时更新
              </span>
            )}
          </div>

          <div className="mt-1 truncate text-sm text-foreground">{title}</div>

          <div
            ref={streamContainerRef}
            onScroll={handleScroll}
            className="mt-2 h-[8.25rem] overflow-y-auto rounded border border-border/50 bg-background/70 px-2.5 py-2"
            onClick={(event) => event.stopPropagation()}
          >
            <div className="space-y-2">
              {streamItems.length > 0 ? (
                streamItems.map((item) => {
                  if (item.kind === "assistant") {
                    return <LogRow key={item.key} icon={Bot} label="AI" content={normalizePreviewText(item.content)} />
                  }
                  if (item.kind === "tool") {
                    return <MiniToolLogRow key={item.key} part={item.part} />
                  }
                  return <LogRow key={item.key} icon={AlertCircle} tone="red" label="ERROR" content={normalizePreviewText(item.content)} />
                })
              ) : (
                <div className="rounded border border-dashed border-border/60 bg-muted/20 px-2.5 py-2 text-xs text-muted-foreground">
                  {fallbackPreview || "正在等待子任务输出，这里的内容会通过父任务流实时推送。"}
                </div>
              )}
            </div>
          </div>

          <div className="mt-1.5 flex items-center justify-between gap-2 text-[11px] text-muted-foreground">
            <span className="truncate">
              {streamItems.length > 0
                ? "通过父任务流实时展示子任务消息"
                : isToolRunningStatus(status)
                  ? "等待子任务开始输出"
                  : "暂无子任务输出"}
            </span>
            {hasTaskId && onOpen ? (
              <Button
                type="button"
                variant="ghost"
                size="sm"
                className="h-6 shrink-0 gap-1 px-1.5 text-[11px] text-foreground/80"
                onClick={(event) => {
                  event.stopPropagation()
                  onOpen(taskId)
                }}
              >
                打开会话
                <ChevronRight className="h-3.5 w-3.5" />
              </Button>
            ) : null}
          </div>
        </div>
      </div>
    </div>
  )
}
