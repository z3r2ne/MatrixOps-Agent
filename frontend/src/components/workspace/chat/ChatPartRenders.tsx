import React, { useState, useRef, useEffect, useLayoutEffect, useCallback, useMemo } from "react"
import {
  AlertCircle, ChevronDown, ChevronUp, ChevronRight, FileText, Braces, Archive,
  Brain, Loader2, Copy, Check, Trash2, Sparkles, Wrench, RefreshCw,
  FolderOpen, Send, Bot, ArrowDown, ArrowLeft, FileCode, Square,
  Plus, X, Paperclip, Download, Upload,
} from "lucide-react"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { ScrollArea } from "@/components/ui/scroll-area"
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { AlertDialog, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from "@/components/ui/alert-dialog"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { cn } from "@/lib/utils"
import { toast } from "sonner"
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip"
import {
  api, type Part, type Memory, type MessageTokens, type SessionPromptResponse,
  type SkillCard, type WithParts,
} from "@/lib/api"
import { MarkdownRenderer } from "../MarkdownRenderer"
import { ToolCallView } from "../ToolCallView"
import { MessageAttachmentBubble, isRenderableFilePart } from "../MessageAttachmentBubble"
import { ContextUsageDial } from "../ContextUsageDial"
import { ExpandableOverflowList } from "../ExpandableOverflowList"
import { TaskMessageQueue } from "../TaskMessageQueue"
import { useAutoScroll } from "@/hooks/useAutoScroll"
import { formatMessageTime, systemMessageOriginLabel, formatCompactByteCount, USER_MESSAGE_COLLAPSE_MAX_HEIGHT_PX } from "./chat-utils"

export const UserFileBubble: React.FC<{ part: Part }> = ({ part }) => (
  <MessageAttachmentBubble part={part} variant="user" />
)


export const SystemMessagePart: React.FC<{ content: string; origin?: string; timestamp?: number }> = ({
  content,
  origin,
  timestamp,
}) => {
  const timeLabel = formatMessageTime(timestamp)
  return (
    <div className="flex w-full max-w-[85%] flex-col gap-1">
      <div className="flex items-center gap-2 text-[11px] text-muted-foreground">
        <Bot className="h-3.5 w-3.5 shrink-0 opacity-70" />
        <Badge variant="secondary" className="px-1.5 py-0 text-[10px]">
          {systemMessageOriginLabel(origin)}
        </Badge>
        {timeLabel ? <span>{timeLabel}</span> : null}
      </div>
      <div className="rounded-lg border border-border/70 bg-muted/40 px-3 py-2 text-sm text-foreground/90 shadow-sm">
        <div className="whitespace-pre-wrap break-words [overflow-wrap:anywhere]">{content}</div>
      </div>
    </div>
  )
}


export const AssistantFileAttachment: React.FC<{ part: Part }> = ({ part }) => (
  <div className="mb-2 flex w-full flex-col">
    <div className="flex w-full items-start gap-2">
      <div className="flex h-6 w-6 shrink-0 items-center justify-center rounded-sm bg-muted mt-0.5">
        <Bot className="h-4 w-4 text-muted-foreground" />
      </div>
      <div className="min-w-0 max-w-[74%]">
        <MessageAttachmentBubble part={part} variant="assistant" />
      </div>
    </div>
  </div>
)

// User Message

export const UserMessagePart: React.FC<{ content: string; timestamp?: number; canRetry?: boolean; onRetry?: () => void }> = ({ content, timestamp, canRetry = false, onRetry }) => {
  const [copied, setCopied] = useState(false)
  const [expanded, setExpanded] = useState(false)
  const [isOverflowing, setIsOverflowing] = useState(false)
  const contentRef = useRef<HTMLDivElement>(null)
  const timeLabel = formatMessageTime(timestamp)
  
  const handleCopy = () => {
    navigator.clipboard.writeText(content)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  useLayoutEffect(() => {
    if (expanded) {
      setIsOverflowing(true)
      return
    }
    const node = contentRef.current
    if (!node) return
    setIsOverflowing(node.scrollHeight > USER_MESSAGE_COLLAPSE_MAX_HEIGHT_PX + 1)
  }, [content, expanded])

  return (
    <div className="flex w-full max-w-full flex-col items-end">
      <div className="group/bubble flex max-w-full flex-col items-end gap-1">
        <div className="relative w-fit max-w-full overflow-visible rounded-md bg-primary px-2 py-1 text-sm text-primary-foreground shadow-sm">
          <div
            ref={contentRef}
            className={cn(
              "whitespace-pre-wrap break-words [overflow-wrap:anywhere]",
              !expanded && isOverflowing && "max-h-[220px] overflow-hidden",
            )}
          >
            {content}
          </div>
          {!expanded && isOverflowing && (
            <button
              type="button"
              className="absolute inset-x-0 bottom-0 flex h-16 items-end justify-center bg-gradient-to-t from-primary via-primary/95 to-transparent pb-1.5 text-primary-foreground/90 transition hover:text-primary-foreground"
              onClick={() => setExpanded(true)}
            >
              <span className="rounded-full border border-primary-foreground/20 bg-primary/90 px-2 py-0.5 text-[10px]">
                展开全文
              </span>
            </button>
          )}
          {expanded && isOverflowing && (
            <div className="mt-1 flex justify-center">
              <button
                type="button"
                className="rounded-full border border-primary-foreground/20 bg-primary/90 px-2 py-0.5 text-[10px] text-primary-foreground/90 transition hover:text-primary-foreground"
                onClick={() => setExpanded(false)}
              >
                收起全文
              </button>
            </div>
          )}
          <TooltipProvider>
            <div
              className={cn(
                "absolute bottom-1 right-1 z-10 rounded-md border border-primary-foreground/15 bg-primary/92 p-0.5 shadow-sm backdrop-blur-sm transition-all",
                "pointer-events-none translate-y-1 opacity-0 group-hover/bubble:pointer-events-auto group-hover/bubble:translate-y-0 group-hover/bubble:opacity-100 group-focus-within/bubble:pointer-events-auto group-focus-within/bubble:translate-y-0 group-focus-within/bubble:opacity-100"
              )}
            >
              <Tooltip>
                <TooltipTrigger asChild>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="h-6 w-6 rounded-sm text-primary-foreground/80 hover:bg-primary-foreground/10 hover:text-primary-foreground"
                    onClick={handleCopy}
                  >
                    {copied ? <Check className="h-3 w-3 text-emerald-300" /> : <Copy className="h-3 w-3" />}
                  </Button>
                </TooltipTrigger>
                <TooltipContent>复制</TooltipContent>
              </Tooltip>
              {canRetry && onRetry && (
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button
                      variant="ghost"
                      size="icon"
                      className="h-6 w-6 rounded-sm text-primary-foreground/80 hover:bg-primary-foreground/10 hover:text-primary-foreground"
                      onClick={onRetry}
                    >
                      <RefreshCw className="h-3 w-3" />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>重试这条消息</TooltipContent>
                </Tooltip>
              )}
            </div>
          </TooltipProvider>
        </div>
        {timeLabel && (
          <div
            className={cn(
              "text-[11px] text-muted-foreground transition-all",
              "pointer-events-none translate-y-1 opacity-0 group-hover/bubble:translate-y-0 group-hover/bubble:opacity-100 group-focus-within/bubble:translate-y-0 group-focus-within/bubble:opacity-100"
            )}
          >
            {timeLabel}
          </div>
        )}
      </div>
    </div>
  )
}

// Assistant Text Message

export const AssistantMessagePart: React.FC<{
  content: string
  timestamp?: number
  canViewPrompt?: boolean
  onViewMemory?: () => void
}> = ({ content, timestamp, canViewPrompt = false, onViewMemory }) => {
  const [copied, setCopied] = useState(false)
  const timeLabel = formatMessageTime(timestamp)
  
  const handleCopy = () => {
    navigator.clipboard.writeText(content)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  if (!content.trim()) return null

  return (
    <div className="mb-2 flex w-full flex-col">
      <div className="flex w-full items-start gap-2">
        <div className="flex h-6 w-6 shrink-0 items-center justify-center bg-muted mt-0.5 rounded-sm">
          <Bot className="h-4 w-4 text-muted-foreground" />
        </div>
        <div className="group/bubble flex max-w-[74%] flex-col gap-1">
          <div className="relative w-fit max-w-full overflow-visible rounded-md bg-muted/50 px-2 py-1 text-sm leading-snug shadow-sm">
            <MarkdownRenderer content={content} />
            <TooltipProvider>
              <div
                className={cn(
                  "absolute bottom-1 right-1 z-10 flex items-center gap-0.5 rounded-md border border-border/70 bg-background/94 p-0.5 shadow-sm backdrop-blur-sm transition-all",
                  "pointer-events-none translate-y-1 opacity-0 group-hover/bubble:pointer-events-auto group-hover/bubble:translate-y-0 group-hover/bubble:opacity-100 group-focus-within/bubble:pointer-events-auto group-focus-within/bubble:translate-y-0 group-focus-within/bubble:opacity-100"
                )}
              >
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button 
                      variant="ghost" 
                      size="icon" 
                      className="h-6 w-6 rounded-sm text-muted-foreground hover:bg-muted hover:text-foreground" 
                      onClick={handleCopy}
                    >
                      {copied ? <Check className="h-3 w-3 text-emerald-500" /> : <Copy className="h-3 w-3" />}
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>复制</TooltipContent>
                </Tooltip>

                {canViewPrompt && onViewMemory && (
                  <Tooltip>
                    <TooltipTrigger asChild>
                      <Button 
                        variant="ghost" 
                        size="icon" 
                        className="h-6 w-6 rounded-sm text-muted-foreground hover:bg-muted hover:text-foreground" 
                        onClick={onViewMemory}
                      >
                        <FileText className="h-3 w-3" />
                      </Button>
                    </TooltipTrigger>
                    <TooltipContent>查看提示词</TooltipContent>
                  </Tooltip>
                )}
              </div>
            </TooltipProvider>
          </div>
          {timeLabel && (
            <div
              className={cn(
                "pl-1 text-[11px] text-muted-foreground transition-all",
                "pointer-events-none translate-y-1 opacity-0 group-hover/bubble:translate-y-0 group-hover/bubble:opacity-100 group-focus-within/bubble:translate-y-0 group-focus-within/bubble:opacity-100"
              )}
            >
              {timeLabel}
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

// Thinking Part

export const ThinkingPart: React.FC<{ content: string, isStreaming: boolean, hasContentBelow: boolean }> = ({ content, isStreaming, hasContentBelow }) => {
  const [expanded, setExpanded] = useState(isStreaming || !hasContentBelow)
  const { ref: contentRef, handleScroll } = useAutoScroll({
    deps: [content],
    enabled: expanded && isStreaming,
  })

  // 当 isStreaming 或 hasContentBelow 变化时更新展开状态
  useEffect(() => {
    setExpanded(isStreaming || !hasContentBelow)
  }, [isStreaming, hasContentBelow])

  return (
    <div className="flex items-start gap-2 text-muted-foreground text-sm ml-8 mb-2">
      <div className="flex h-5 w-5 shrink-0 items-center justify-center mt-0.5">
        {isStreaming ? (
          <Loader2 className="h-3.5 w-3.5 animate-spin" />
        ) : (
          <Brain className="h-3.5 w-3.5" />
        )}
      </div>
      <div className="flex-1 min-w-0">
        <div
          className="flex items-center gap-1 cursor-pointer hover:text-foreground select-none"
          onClick={() => setExpanded(!expanded)}
        >
          {expanded ? <ChevronDown className="h-3 w-3" /> : <ChevronRight className="h-3 w-3" />}
          <span className="font-medium text-xs">
            {isStreaming ? "正在思考..." : "思考过程"}
          </span>
        </div>
        {expanded && (
          <div
            ref={contentRef}
            onScroll={handleScroll}
            className={cn(
              "mt-1.5 pl-4 text-xs text-muted-foreground/80 whitespace-pre-wrap border-l-2 border-muted leading-relaxed",
              "max-h-48 overflow-y-auto scrollbar-thin scrollbar-thumb-muted-foreground/20 scrollbar-track-transparent"
            )}
          >
            {content}
          </div>
        )}
      </div>
    </div>
  )
}

function getCompactionNumber(metadata: Record<string, any>, key: string) {
  const value = metadata[key]
  if (typeof value === "number") return value
  if (typeof value === "string") {
    const parsed = Number(value)
    return Number.isFinite(parsed) ? parsed : 0
  }
  return 0
}

function resolveCompactionPartStatus(part: Part, metadata: Record<string, any>, status: string) {
  if (status !== "running") {
    return status
  }
  if (part.time?.end && part.time.end > 0) {
    return "completed"
  }
  const summaryStreaming = metadata.summaryStreaming === true
  const beforeCount = getCompactionNumber(metadata, "beforeCount")
  const afterCount = getCompactionNumber(metadata, "afterCount")
  const beforeBytes = getCompactionNumber(metadata, "beforeBytes")
  const afterBytes = getCompactionNumber(metadata, "afterBytes")
  if (!summaryStreaming && beforeCount > 0 && beforeCount === afterCount && beforeBytes === afterBytes) {
    return "completed"
  }
  return status
}

function getCompactionTypeLabel(kind: string, strategy: string) {
  if (kind === "file") {
    return "文件压缩"
  }
  if (strategy === "prompt_builder") {
    return "记忆整理"
  }
  if (strategy === "size") {
    return "消息内容压缩"
  }
  return "消息压缩"
}

function getCompactionContentLabel(kind: string, strategy: string) {
  if (kind === "file") {
    return "压缩内容（文件）"
  }
  if (strategy === "prompt_builder") {
    return "压缩前记忆"
  }
  if (strategy === "size") {
    return "压缩内容（消息内容）"
  }
  return "压缩内容（消息）"
}


export const CompactionPart: React.FC<{ part: Part; onLayoutChange?: () => void }> = ({ part, onLayoutChange }) => {
  const [expanded, setExpanded] = useState(false)
  const metadata = part.metadata ?? {}
  const kind = typeof metadata.kind === "string" ? metadata.kind : "memory"
  const strategy = typeof metadata.strategy === "string" ? metadata.strategy : ""
  const summary = typeof metadata.summary === "string" ? metadata.summary.trim() : ""
  const summaryStreaming = metadata.summaryStreaming === true
  const inputPreview = typeof metadata.inputPreview === "string" ? metadata.inputPreview.trim() : ""
  const resultPreview = typeof metadata.resultPreview === "string" ? metadata.resultPreview.trim() : ""
  const afterContent = resultPreview || summary
  const rawStatus = typeof metadata.status === "string" ? metadata.status : "completed"
  const status = resolveCompactionPartStatus(part, metadata, rawStatus)
  const isStreamingSummary = status === "running" && (summaryStreaming || Boolean(summary))
  const error = typeof metadata.error === "string" ? metadata.error.trim() : ""
  const batchIndex = getCompactionNumber(metadata, "batchIndex")
  const batchTotal = getCompactionNumber(metadata, "batchTotal")
  const compressedCount = getCompactionNumber(metadata, "compressedCount")
  const beforeCount = getCompactionNumber(metadata, "beforeCount")
  const afterCount = getCompactionNumber(metadata, "afterCount")
  const beforeBytes = getCompactionNumber(metadata, "beforeBytes")
  const afterBytes = getCompactionNumber(metadata, "afterBytes")
  const scope = typeof metadata.scope === "string" ? metadata.scope : ""
  const hasDetails = Boolean(inputPreview || afterContent || error)

  useEffect(() => {
    if (isStreamingSummary && summary) {
      setExpanded(true)
    }
  }, [isStreamingSummary, summary])

  const typeLabel = getCompactionTypeLabel(kind, strategy)
  const scopeLabel = scope === "assistant_response_group" ? "AI 响应组" : "消息记忆"
  const contentLabel = getCompactionContentLabel(kind, strategy)
  const statusBadge = status === "error"
    ? { label: "失败", className: "border-red-200 bg-red-50 text-red-700" }
    : status === "running"
      ? { label: "整理中", className: "border-blue-200 bg-blue-50 text-blue-700" }
      : { label: "已完成", className: "border-emerald-200 bg-emerald-50 text-emerald-700" }

  useEffect(() => {
    if (!onLayoutChange) return
    const frame = requestAnimationFrame(() => onLayoutChange())
    return () => cancelAnimationFrame(frame)
  }, [afterContent, error, expanded, inputPreview, onLayoutChange, status])

  return (
    <div className="ml-8 mb-2 flex min-w-0 items-start gap-2 text-sm">
      <div className={cn(
        'mt-0.5 flex h-6 w-6 shrink-0 items-center justify-center rounded-sm',
        status === 'error'
          ? 'bg-red-100 text-red-700'
          : status === 'running'
            ? 'bg-blue-100 text-blue-700'
            : 'bg-amber-100 text-amber-700',
      )}>
        {status === 'running' ? <Loader2 className="h-4 w-4 animate-spin" /> : <Archive className="h-4 w-4" />}
      </div>
      <div className={cn(
        'min-w-0 flex-1 overflow-hidden rounded-md border px-3 py-2',
        status === 'error'
          ? 'border-red-200 bg-red-50/70'
          : status === 'running'
            ? 'border-blue-200 bg-blue-50/70'
            : 'border-amber-200 bg-amber-50/70',
      )}>
        <div
          className={cn("flex min-w-0 items-start gap-2", hasDetails && "cursor-pointer")}
          onClick={hasDetails ? () => setExpanded((value) => !value) : undefined}
        >
          <div className="min-w-0 flex-1">
            <div className="flex min-w-0 flex-wrap items-center gap-1.5">
              <span className="text-sm font-medium text-amber-900">{part.description || "记忆压缩"}</span>
              <Badge variant="outline" className="h-5 rounded-full border-amber-300 bg-amber-100 px-2 text-[10px] font-medium text-amber-800">
                {typeLabel}
              </Badge>
              <Badge variant="outline" className="h-5 rounded-full border-amber-300 bg-amber-100 px-2 text-[10px] font-medium text-amber-800">
                {scopeLabel}
              </Badge>
              <Badge variant="outline" className={cn("h-5 rounded-full px-2 text-[10px] font-medium", statusBadge.className)}>
                {statusBadge.label}
              </Badge>
              {batchTotal > 1 && (
                <Badge variant="outline" className="h-5 rounded-full border-amber-300 bg-amber-100 px-2 text-[10px] font-medium text-amber-800">
                  第 {batchIndex}/{batchTotal} 组
                </Badge>
              )}
            </div>
            <div className="mt-2 flex flex-wrap items-center gap-2 text-xs text-amber-900/80">
              {compressedCount > 0 && (
                <span className="rounded-full bg-amber-100 px-2 py-0.5">压缩 {compressedCount} 条</span>
              )}
              {(beforeCount > 0 || afterCount > 0) && (
                <span className="rounded-full bg-amber-100 px-2 py-0.5">条数 {beforeCount} → {afterCount}</span>
              )}
              {(beforeBytes > 0 || afterBytes > 0) && (
                <span className="rounded-full bg-amber-100 px-2 py-0.5">大小 {formatCompactByteCount(beforeBytes)} → {formatCompactByteCount(afterBytes)}</span>
              )}
            </div>
            {isStreamingSummary && summary && (
              <div className="mt-2 rounded-md border border-blue-200/80 bg-background/70 px-3 py-2">
                <div className="mb-1 text-xs font-medium uppercase tracking-wide text-blue-700">摘要生成中</div>
                <div className="max-h-48 overflow-y-auto text-sm leading-relaxed text-foreground/90">
                  <MarkdownRenderer content={summary} />
                </div>
              </div>
            )}
            {!expanded && hasDetails && !isStreamingSummary && (
              <div className="mt-2 text-xs text-amber-800/80">
                点击查看压缩详情
              </div>
            )}
            {expanded && error && (
              <div className="mt-2 rounded-md border border-red-200 bg-background/70 px-3 py-2 text-sm leading-relaxed text-red-700">
                <div className="mb-1 text-xs font-medium uppercase tracking-wide text-red-600">错误信息</div>
                <pre className="max-w-full whitespace-pre-wrap text-xs leading-relaxed break-words [overflow-wrap:anywhere]">
                  {error}
                </pre>
              </div>
            )}
          </div>
          {hasDetails && (
            <div className="mt-0.5 shrink-0 text-amber-700">
              {expanded ? <ChevronDown className="h-4 w-4" /> : <ChevronRight className="h-4 w-4" />}
            </div>
          )}
        </div>
        {expanded && (inputPreview || afterContent) && (
          <div className="mt-2 grid h-[360px] min-h-0 gap-2 lg:grid-cols-2">
            <div className="flex min-w-0 flex-col overflow-hidden rounded-md border border-amber-200 bg-background/60">
              <div className="shrink-0 border-b border-amber-200 px-3 py-2 text-xs font-medium uppercase tracking-wide text-amber-700">
                {contentLabel}
              </div>
              <ScrollArea className="min-h-0 flex-1">
                <pre className="whitespace-pre-wrap px-3 py-2 text-xs leading-relaxed text-muted-foreground break-words [overflow-wrap:anywhere]">
                  {inputPreview || "无"}
                </pre>
              </ScrollArea>
            </div>
            <div className="flex min-w-0 flex-col overflow-hidden rounded-md border border-amber-200 bg-background/60">
              <div className="shrink-0 border-b border-amber-200 px-3 py-2 text-xs font-medium uppercase tracking-wide text-amber-700">
                压缩后记忆
              </div>
              <ScrollArea className="min-h-0 flex-1">
                {resultPreview ? (
                  <pre className="whitespace-pre-wrap px-3 py-2 text-xs leading-relaxed text-muted-foreground break-words [overflow-wrap:anywhere]">
                    {resultPreview}
                  </pre>
                ) : (
                  <div className="px-3 py-2 text-xs leading-relaxed text-muted-foreground">
                    <MarkdownRenderer content={afterContent || "无"} />
                  </div>
                )}
              </ScrollArea>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}


export const MemoryOrganizationPartRender: React.FC<{ part: Part }> = ({ part }) => {
  const [expanded, setExpanded] = useState(false)
  const metadata = part.metadata ?? {}
  const status = typeof metadata.status === "string" ? metadata.status : "preparing"
  const operationType = typeof metadata.organizationType === "string" ? metadata.organizationType : "unknown"
  const selectedPreview = typeof metadata.selectedPreview === "string" ? metadata.selectedPreview.trim() : ""
  const resultPreview = typeof metadata.resultPreview === "string" ? metadata.resultPreview.trim() : ""
  const summary = typeof metadata.summary === "string" ? metadata.summary.trim() : ""
  const error = typeof metadata.error === "string" ? metadata.error.trim() : ""
  const operationIndex = getCompactionNumber(metadata, "operationIndex")
  const selectedBytes = getCompactionNumber(metadata, "selectedBytes")
  const resultBytes = getCompactionNumber(metadata, "resultBytes")
  const selectedCount = getCompactionNumber(metadata, "selectedCount")
  const resultCount = getCompactionNumber(metadata, "resultCount")
  const compressionRatio = getCompactionNumber(metadata, "compressionRatio")
  const startMsgId = getCompactionNumber(metadata, "startMsgId")
  const endMsgId = getCompactionNumber(metadata, "endMsgId")
  const msgIdRangeLabel = startMsgId > 0 && endMsgId > 0 ? `MsgID ${startMsgId}–${endMsgId}` : ""

  const statusBadge = status === "error"
    ? { label: "失败", className: "border-red-200 bg-red-50 text-red-700" }
    : status === "completed"
      ? { label: "已完成", className: "border-emerald-200 bg-emerald-50 text-emerald-700" }
      : status === "running"
        ? { label: "整理中", className: "border-blue-200 bg-blue-50 text-blue-700" }
        : status === "input-streaming"
          ? { label: "解析中", className: "border-blue-200 bg-blue-50 text-blue-700" }
          : { label: "准备中", className: "border-slate-200 bg-slate-50 text-slate-700" }

  const typeLabel = operationType === "summary" ? "总结压缩" : operationType === "delete" ? "删除记忆" : "记忆整理"
  const ratioLabel = `${compressionRatio.toFixed(compressionRatio >= 10 ? 0 : 1)}%`

  return (
    <div className="ml-8 mb-3 flex min-w-0 items-start gap-3">
      <div className={cn(
        "mt-0.5 flex h-7 w-7 shrink-0 items-center justify-center rounded-md border",
        status === "error" ? "border-red-200 bg-red-50 text-red-600" :
        status === "completed" ? "border-emerald-200 bg-emerald-50 text-emerald-700" :
        "border-sky-200 bg-sky-50 text-sky-700"
      )}>
        {status === "running" || status === "input-streaming" ? (
          <Loader2 className="h-4 w-4 animate-spin" />
        ) : (
          <Archive className="h-4 w-4" />
        )}
      </div>
      <div className="min-w-0 flex-1 overflow-hidden rounded-lg border border-slate-200 bg-gradient-to-br from-slate-50 to-white">
        <div
          className="flex min-w-0 cursor-pointer flex-wrap items-center gap-2 border-b border-slate-200/80 px-4 py-3 select-none hover:bg-slate-50/80"
          onClick={() => setExpanded((v) => !v)}
          role="button"
          tabIndex={0}
          onKeyDown={(e) => {
            if (e.key === "Enter" || e.key === " ") {
              e.preventDefault()
              setExpanded((v) => !v)
            }
          }}
        >
          <div className="min-w-0 flex-1">
            <div className="flex min-w-0 flex-wrap items-center gap-2">
              {expanded ? (
                <ChevronDown className="h-4 w-4 shrink-0 text-slate-500" />
              ) : (
                <ChevronRight className="h-4 w-4 shrink-0 text-slate-500" />
              )}
              <span className="text-sm font-semibold text-slate-900">记忆整理操作</span>
              {operationIndex > 0 && (
                <Badge variant="outline" className="rounded-full border-slate-300 bg-white text-[10px] font-medium text-slate-700">
                  #{operationIndex}
                </Badge>
              )}
              <Badge variant="outline" className="rounded-full border-slate-300 bg-white text-[10px] font-medium text-slate-700">
                {typeLabel}
              </Badge>
              <Badge variant="outline" className={cn("rounded-full text-[10px] font-medium", statusBadge.className)}>
                {statusBadge.label}
              </Badge>
            </div>
            <div className="mt-2 flex min-w-0 flex-wrap items-center gap-2 text-[11px] text-slate-600">
              {msgIdRangeLabel !== "" && (
                <span className="rounded-full bg-slate-100 px-2 py-0.5">{msgIdRangeLabel}</span>
              )}
              {selectedCount > 0 && (
                <span className="rounded-full bg-slate-100 px-2 py-0.5">整理前 {selectedCount} 条</span>
              )}
              {resultCount > 0 && (
                <span className="rounded-full bg-slate-100 px-2 py-0.5">整理后 {resultCount} 条</span>
              )}
              {(selectedBytes > 0 || resultBytes > 0) && (
                <span className="rounded-full bg-slate-100 px-2 py-0.5">
                  大小 {formatCompactByteCount(selectedBytes)} → {formatCompactByteCount(resultBytes)}
                </span>
              )}
              {selectedBytes > 0 && (
                <span className="rounded-full bg-emerald-100 px-2 py-0.5 font-medium text-emerald-700">
                  压缩率 {ratioLabel}
                </span>
              )}
            </div>
            {!expanded && error && (
              <div className="mt-2 truncate text-xs text-red-600" title={error}>
                {error}
              </div>
            )}
          </div>
        </div>

        {expanded && (
        <div className="grid min-w-0 grid-cols-1 gap-3 p-4 lg:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]">
          <div className="min-w-0 rounded-md border border-slate-200 bg-white/80">
            <div className="border-b border-slate-200 bg-slate-50 px-3 py-2 text-xs font-semibold tracking-wide text-slate-700">
              整理前记忆
            </div>
            <div className="max-h-60 overflow-auto p-3">
              {selectedPreview ? (
                <pre className="max-w-full whitespace-pre-wrap text-xs leading-relaxed text-slate-700 break-words [overflow-wrap:anywhere]">{selectedPreview}</pre>
              ) : (
                <div className="text-xs text-muted-foreground">暂无整理前内容</div>
              )}
            </div>
          </div>

          <div className="min-w-0 rounded-md border border-slate-200 bg-white/80">
            <div className="border-b border-slate-200 bg-emerald-50 px-3 py-2 text-xs font-semibold tracking-wide text-emerald-700">
              整理后记忆
            </div>
            <div className="max-h-60 overflow-auto p-3">
              {resultPreview ? (
                <pre className="max-w-full whitespace-pre-wrap text-xs leading-relaxed text-slate-700 break-words [overflow-wrap:anywhere]">{resultPreview}</pre>
              ) : (
                <div className="text-xs text-muted-foreground">暂无整理后内容</div>
              )}
            </div>
          </div>
        </div>
        )}

        {expanded && (summary || part.text || error) && (
          <div className="border-t border-slate-200 px-4 py-3">
            {summary && operationType === "summary" && (
              <div className="mb-3">
                <div className="mb-1 text-xs font-semibold tracking-wide text-slate-700">摘要内容</div>
                <div className="rounded-md border border-slate-200 bg-white/80 px-3 py-2">
                  <MarkdownRenderer content={summary} />
                </div>
              </div>
            )}
            {part.text && (
              <div className={cn(
                "rounded-md border px-3 py-2 text-xs leading-relaxed",
                status === "error" ? "border-red-200 bg-red-50 text-red-800" : "border-slate-200 bg-slate-50 text-slate-700"
              )}>
                {part.text}
              </div>
            )}
            {!part.text && error && (
              <div className="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-xs leading-relaxed text-red-800">
                {error}
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  )
}

// Error Part

export const ErrorPart: React.FC<{ part: Part }> = ({ part }) => {
  const content = part.error?.message || part.text || "Unknown error"
  return (
    <div className="ml-8 space-y-2 mb-2">
      <div className="flex items-start gap-2">
        <div className="flex h-5 w-5 shrink-0 items-center justify-center text-red-500 mt-0.5">
          <AlertCircle className="h-3.5 w-3.5" />
        </div>
        <div className="flex-1 min-w-0 overflow-x-auto rounded border border-red-200 bg-red-50 px-3 py-2 font-mono text-xs text-red-800">
          <pre className="max-w-full whitespace-pre-wrap break-words [overflow-wrap:anywhere]">{content}</pre>
        </div>
      </div>
    </div>
  )
}


export const CancelledConversationDivider: React.FC<{ part: Part }> = ({ part }) => {
  const label = (part.text || "").trim() || "已取消对话"
  return (
    <div className="ml-8 mb-3 mt-2 flex items-center gap-3 text-[11px] text-muted-foreground">
      <div className="h-px flex-1 bg-border/60" />
      <span className="shrink-0 rounded-full border border-border/70 bg-background/80 px-2.5 py-0.5 tracking-wide">
        {label}
      </span>
      <div className="h-px flex-1 bg-border/60" />
    </div>
  )
}

