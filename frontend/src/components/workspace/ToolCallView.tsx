import React, { useEffect, useRef, useState } from "react"
import { useAutoScroll } from "@/hooks/useAutoScroll"
import {
  Braces,
  FileCode,
  FolderOpen,
  Loader2,
  Search,
  Square,
  Terminal,
  ChevronDown,
  ChevronRight,
} from "lucide-react"
import { Button } from "@/components/ui/button"
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip"
import { cn } from "@/lib/utils"
import type { Part } from "@/lib/api"
import { ToolTerminal } from "./ToolTerminal"
import { PatchDiffViewer } from "./PatchDiffViewer"
import { SubtaskToolCard } from "./SubtaskToolCard"
import {
  buildCompactSummary,
  classifyToolGroup,
  isToolRunningStatus as toolIsRunning,
  stripToolOutputPrefix,
  toolDisplayLabel,
} from "./toolDisplayUtils"

function toolUsesTerminalMetadata(metadata?: Record<string, unknown>) {
  if (!metadata) return false
  return metadata.outputFormat === "terminal" || metadata.streamMode === "terminal" || metadata.tty === true
}

function getToolIcon(name: string) {
  const g = classifyToolGroup(name)
  if (g === "bash") return Terminal
  if (g === "search") return Search
  if (g === "files" || g === "tree") return FolderOpen
  if (g === "read" || g === "write" || g === "patch" || g === "edit" || g === "diff") return FileCode
  return Braces
}

function str(v: unknown): string {
  return typeof v === "string" ? v : ""
}

function isNoiseOutput(group: ReturnType<typeof classifyToolGroup>, plain: string): boolean {
  const t = plain.trim().toLowerCase()
  if (t === "ok") return true
  if (group === "patch" && t === "ok") return true
  if (group === "write" && t === "ok") return true
  return false
}

interface ToolCallViewProps {
  part: Part
  isForcedRunning?: boolean
  canCancel?: boolean
  onCancel?: () => void
  onOpenTask?: (taskId: number) => void
  defaultExpanded?: boolean
  defaultIsCollapsed?: boolean
}

function toNumber(value: unknown): number | null {
  if (typeof value === 'number' && Number.isFinite(value)) return value
  if (typeof value === 'string' && value.trim() !== '') {
    const parsed = Number(value)
    if (Number.isFinite(parsed)) return parsed
  }
  return null
}

export function ToolCallView({ part, isForcedRunning, canCancel = false, onCancel, onOpenTask, defaultExpanded = false, defaultIsCollapsed = false }: ToolCallViewProps) {
  const [expanded, setExpanded] = useState(defaultExpanded)
  const [isCollapsed, setIsCollapsed] = useState(defaultIsCollapsed)
  const tool = part.tool
  if (!tool) return null

  const { ref: detailRef, handleScroll } = useAutoScroll({
    deps: [tool.state.input, tool.state.output, tool.state.status],
    enabled: expanded,
  })

  useEffect(() => {
    setIsCollapsed(defaultIsCollapsed)
  }, [defaultIsCollapsed])

  useEffect(() => {
    setExpanded(defaultExpanded)
  }, [defaultExpanded])

  const input = (tool.state.input && typeof tool.state.input === "object"
    ? (tool.state.input as Record<string, unknown>)
    : undefined) as Record<string, unknown> | undefined
  const meta = (tool.state.metadata || {}) as Record<string, unknown>
  const inputPreview = typeof meta.inputPreview === "string" ? meta.inputPreview : ""
  const blockedReason = typeof meta.blockedReason === "string" ? meta.blockedReason : ""
  const blockedDetailReason = typeof meta.reason === "string" ? meta.reason : ""
  const subtaskTaskId = toNumber(meta.subtaskTaskId)
  const subtaskWorkerName =
    typeof meta.subtaskWorkerName === 'string'
      ? meta.subtaskWorkerName
      : typeof input?.worker === 'string'
        ? input.worker
        : ''
  const subtaskTaskName =
    typeof meta.subtaskTaskName === 'string'
      ? meta.subtaskTaskName
      : typeof input?.name === 'string'
        ? input.name
        : ''
  const subtaskContent =
    typeof meta.subtaskContent === 'string'
      ? meta.subtaskContent
      : typeof input?.content === 'string'
        ? input.content
        : ''
  const subtaskStatus = typeof meta.subtaskStatus === 'string' ? meta.subtaskStatus : tool.state.status
  const subtaskAnswer = typeof meta.subtaskAnswer === 'string' ? meta.subtaskAnswer : ''
  const subtaskPreviewMessages = meta.subtaskPreviewMessages
  const rawActionOutput =
    typeof part.metadata?.rawOutput === "string"
      ? part.metadata.rawOutput.trim()
      : ""

  if (tool.tool === 'run_worker_task') {
    return (
      <SubtaskToolCard
        taskId={subtaskTaskId}
        workerName={subtaskWorkerName}
        taskName={subtaskTaskName}
        content={subtaskContent}
        status={subtaskStatus}
        answer={subtaskAnswer}
        previewMessages={subtaskPreviewMessages}
        onOpen={onOpenTask}
      />
    )
  }
  const isPreparing = tool.state.status === "preparing"
  const isInputStreaming = tool.state.status === "input-streaming"
  const isRunning = isForcedRunning || tool.state.status === "running" || tool.state.status === "pending"
  const isCancelled = tool.state.status === "cancelled"
  const isBlocked = tool.state.status === "blocked" || Boolean(meta.blocked)
  const isRejected = tool.state.status === "rejected" || blockedReason === "user_rejected"
  const isTerminalOutput = toolUsesTerminalMetadata(meta as Record<string, any>)
  const rawArguments =
    str(tool.state.raw).trim() ||
    (input ? JSON.stringify(input, null, 2) : "")
  const inputContent = inputPreview || rawArguments
  const group = classifyToolGroup(tool.tool)
  const compact = buildCompactSummary(tool.tool, input, meta)
  const outPlain = tool.state.output ? stripToolOutputPrefix(tool.state.output) : ""
  const showOut =
    !!outPlain &&
    !isNoiseOutput(group, outPlain) &&
    !(group === "bash" && isTerminalOutput)

  const hasDetails =
    !!tool.state.error ||
    !!inputContent ||
    !!rawArguments ||
    !!rawActionOutput ||
    !!tool.state.output ||
    (group === "patch" && !!str(input?.patch)) ||
    (group === "write" && typeof input?.content === "string") ||
    (group === "edit" && input && (typeof input.old === "string" || typeof input.new === "string")) ||
    (group === "bash" && (isTerminalOutput || toolIsRunning(tool.state.status)))

  const hasReason = !!part.reason
  const ToolIcon = getToolIcon(tool.tool)

  useEffect(() => {
    if (["completed", "error", "blocked", "rejected", "cancelled"].includes(tool.state.status)) {
      // If we have a defaultExpanded preference, use it. Otherwise default to false for completed tools.
      setExpanded(defaultExpanded)
    }
  }, [tool.state.status, defaultExpanded])



  if (isCollapsed && !defaultIsCollapsed) {
    return (
      <div className="ml-8 mb-1 flex items-center gap-2 text-[10px] text-muted-foreground/60 italic">
        <div className="h-px flex-1 bg-border/30" />
        <button 
          className="hover:text-foreground hover:underline transition-colors"
          onClick={() => setIsCollapsed(false)}
        >
          已折叠工具调用 (点击展开)
        </button>
        <div className="h-px flex-1 bg-border/30" />
      </div>
    )
  }

  const statusColor =
    isInputStreaming || isRunning
      ? "text-blue-500"
      : isPreparing
        ? "text-slate-500"
        : isCancelled
          ? "text-amber-600"
          : isRejected
            ? "text-amber-600"
            : isBlocked
              ? "text-orange-600"
              : tool.state.status === "success" || tool.state.status === "completed"
                ? "text-emerald-500"
                : tool.state.status === "error" || tool.state.status === "failed"
                  ? "text-red-500"
                  : "text-muted-foreground"

  const statusBarClass = isInputStreaming || isRunning
    ? "border-l-blue-500"
    : isPreparing
      ? "border-l-slate-400"
      : isCancelled || isRejected
        ? "border-l-amber-500"
        : isBlocked
          ? "border-l-orange-500"
          : tool.state.status === "success" || tool.state.status === "completed"
            ? "border-l-emerald-500"
            : tool.state.status === "error" || tool.state.status === "failed"
              ? "border-l-red-500"
              : "border-l-muted-foreground/35"

  const detailToneClass =
    isRejected || isCancelled
      ? "bg-amber-50 border-amber-300 text-amber-900"
      : isBlocked
        ? "bg-orange-50 border-orange-300 text-orange-900"
        : tool.state.status === "error" || tool.state.status === "failed"
          ? "bg-red-50 border-red-300 text-red-900"
          : "bg-muted/50 border-blue-300"

  return (
    <div
      className={cn(
        "ml-8 mb-2 flex min-w-0 items-start gap-2 rounded-r-md border border-border/45 border-l-[3px] bg-muted/10 py-1.5 pl-2 pr-2 text-sm",
        statusBarClass,
      )}
    >
      <div className={cn("mt-0.5 flex h-5 w-5 shrink-0 items-center justify-center", statusColor)}>
        {isInputStreaming || isRunning ? (
          <Loader2 className="h-3.5 w-3.5 animate-spin" />
        ) : (
          <ToolIcon className="h-3.5 w-3.5" />
        )}
      </div>
      <div className="min-w-0 flex-1">
        <div
          className={cn(
            "flex min-w-0 items-center gap-1.5 text-muted-foreground",
            hasDetails && "cursor-pointer hover:text-foreground",
          )}
          onClick={hasDetails ? () => setExpanded(!expanded) : undefined}
        >
          {hasDetails && (expanded ? <ChevronDown className="h-3 w-3 shrink-0" /> : <ChevronRight className="h-3 w-3 shrink-0" />)}
          <span className="shrink-0 font-mono text-[11px] font-semibold tracking-tight text-foreground/90">
            {toolDisplayLabel(tool.tool)}
          </span>
          {canCancel && onCancel && toolIsRunning(tool.state.status) && (
            <TooltipProvider>
              <Tooltip>
                <TooltipTrigger asChild>
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon"
                    className="h-6 w-6 shrink-0 text-muted-foreground hover:text-amber-700"
                    onClick={(event) => {
                      event.stopPropagation()
                      onCancel()
                    }}
                  >
                    <Square className="h-3 w-3" />
                  </Button>
                </TooltipTrigger>
                <TooltipContent side="bottom">Cancel</TooltipContent>
              </Tooltip>
            </TooltipProvider>
          )}
          {compact ? (
            <span className="min-w-0 flex-1 truncate whitespace-nowrap font-mono text-[11px] text-muted-foreground/90">{compact}</span>
          ) : inputContent ? (
            <span className="min-w-0 flex-1 truncate whitespace-nowrap text-[11px] text-muted-foreground/70">
              {inputContent.replace(/\s+/g, " ").slice(0, 80)}
            </span>
          ) : null}
        </div>
        {hasReason && (
          <div className="mt-1 text-xs text-muted-foreground">
            <span className="font-semibold text-foreground/80">reason:</span> {part.reason}
          </div>
        )}
        {isRejected && blockedDetailReason && (
          <div className="mt-1 text-xs text-amber-700">
            <span className="font-semibold">rejection reason:</span> {blockedDetailReason}
          </div>
        )}
        {expanded && hasDetails && (
          <div
            ref={detailRef}
            onScroll={handleScroll}
            className="mt-1.5 max-h-[min(420px,70vh)] min-w-0 space-y-2 overflow-x-hidden overflow-y-auto pr-1 scrollbar-thin scrollbar-thumb-muted-foreground/20 scrollbar-track-transparent"
          >
            {rawArguments && (
              <div className="space-y-1">
                <div className="text-[10px] font-semibold uppercase tracking-wide text-muted-foreground">arguments</div>
                <pre className="max-h-52 overflow-auto rounded-md border border-border/80 bg-muted/30 p-2 font-mono text-[11px] leading-snug [overflow-wrap:anywhere]">
                  {rawArguments}
                </pre>
              </div>
            )}

            {rawActionOutput && rawActionOutput !== rawArguments && (
              <div className="space-y-1">
                <div className="text-[10px] font-semibold uppercase tracking-wide text-muted-foreground">raw action</div>
                <pre className="max-h-52 overflow-auto rounded-md border border-border/80 bg-muted/30 p-2 font-mono text-[11px] leading-snug [overflow-wrap:anywhere]">
                  {rawActionOutput}
                </pre>
              </div>
            )}

            {group === "bash" && isTerminalOutput && (
              <div className="space-y-1">
                <div className="text-[10px] font-semibold uppercase tracking-wide text-muted-foreground">terminal</div>
                <ToolTerminal content={tool.state.output || ""} isRunning={toolIsRunning(tool.state.status)} />
              </div>
            )}

            {group === "patch" && str(input?.patch) && (
              <div className="space-y-1">
                <div className="text-[10px] font-semibold uppercase tracking-wide text-muted-foreground">patch</div>
                <PatchDiffViewer text={str(input.patch)} />
              </div>
            )}

            {group === "write" && typeof input?.content === "string" && (
              <div className="space-y-1">
                <div className="text-[10px] font-semibold uppercase tracking-wide text-muted-foreground">content</div>
                <pre className="max-h-52 overflow-auto rounded-md border border-border/80 bg-muted/30 p-2 font-mono text-[11px] leading-snug [overflow-wrap:anywhere]">
                  {input.content}
                </pre>
              </div>
            )}

            {group === "edit" && input && (
              <div className="grid gap-2 md:grid-cols-2">
                <div>
                  <div className="mb-0.5 text-[10px] font-semibold text-rose-700/90">old</div>
                  <pre className="max-h-52 overflow-auto rounded-md border border-rose-200/80 bg-rose-50/50 p-2 font-mono text-[11px] leading-snug text-rose-950 [overflow-wrap:anywhere]">
                    {typeof input.old === "string" ? input.old : ""}
                  </pre>
                </div>
                <div>
                  <div className="mb-0.5 text-[10px] font-semibold text-emerald-700/90">new</div>
                  <pre className="max-h-52 overflow-auto rounded-md border border-emerald-200/80 bg-emerald-50/50 p-2 font-mono text-[11px] leading-snug text-emerald-950 [overflow-wrap:anywhere]">
                    {typeof input.new === "string" ? input.new : ""}
                  </pre>
                </div>
              </div>
            )}

            {showOut && (
              <div className="space-y-1">
                <div className="text-[10px] font-semibold uppercase tracking-wide text-muted-foreground">output</div>
                <div className={cn("min-w-0 max-w-full overflow-x-auto rounded-md border-l-2 px-2 py-1.5 font-mono text-[11px] leading-snug", detailToneClass)}>
                  <pre className="max-w-full whitespace-pre-wrap break-words [overflow-wrap:anywhere]">{outPlain}</pre>
                </div>
              </div>
            )}

            {group === "other" && inputContent && (
              <div className="min-w-0 max-w-full text-[11px] text-muted-foreground">
                <div className="font-semibold text-foreground/80">tool_input</div>
                <pre className="mt-0.5 max-w-full overflow-x-auto rounded bg-muted/30 p-2 whitespace-pre-wrap break-words [overflow-wrap:anywhere]">
                  {inputContent}
                </pre>
              </div>
            )}

            {tool.state.error && (
              <div className="min-w-0 max-w-full overflow-x-auto border-l-2 border-red-300 bg-red-50 px-2 py-1.5 font-mono text-[11px] text-red-800">
                <pre className="max-w-full whitespace-pre-wrap break-words [overflow-wrap:anywhere]">{tool.state.error}</pre>
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  )
}
