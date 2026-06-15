import React, { useState } from "react"
import {
  Wrench, ChevronRight, ChevronDown, ChevronUp, Loader2, Square,
  Copy, Check, AlertCircle, Sparkles,
} from "lucide-react"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { cn } from "@/lib/utils"
import { toast } from "sonner"
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip"
import { type Part } from "@/lib/api"
import { ToolCallView } from "../ToolCallView"
import { isToolRunningStatus } from "../toolDisplayUtils"
import { ExpandableOverflowList } from "../ExpandableOverflowList"
import { MessageAttachmentBubble, isRenderableFilePart } from "../MessageAttachmentBubble"
import { ThinkingPart, ErrorPart } from "./ChatPartRenders"

export const ToolGroupRender: React.FC<{
  parts: Part[]
  readOnly: boolean
  onCancelTool?: (callID: string) => void
  onOpenTask?: (taskId: number) => void | Promise<void>
  defaultMode: "peek" | "full"
  isLastInMessage: boolean
  msgState?: string
}> = ({ parts, readOnly, onCancelTool, onOpenTask, defaultMode, isLastInMessage, msgState }) => {
  const toolCount = parts.filter((p) => p.type === "tool" || p.type === "tool-delta").length
  const thinkingCount = parts.filter((p) => p.type === "reasoning" || p.type === "reasoning-delta").length
  const errorCount = parts.filter((p) => p.type === "error").length

  // 从 metadata 统计编辑类工具的实际变更
  let editAdded = 0
  let editRemoved = 0
  let editFiles = 0
  for (const part of parts) {
    if (part.type !== "tool" && part.type !== "tool-delta") continue
    const tool = part.tool
    if (!tool) continue
    const meta = (tool.state.metadata || {}) as Record<string, unknown>
    const name = (tool.tool || "").trim()
    if (name === "patch" || name === "edit" || name === "write") {
      const added = typeof meta.linesAdded === "number" ? meta.linesAdded : 0
      const removed = typeof meta.linesRemoved === "number" ? meta.linesRemoved : 0
      const files = typeof meta.filesChanged === "number" ? meta.filesChanged : 0
      editAdded += added
      editRemoved += removed
      editFiles += files
    }
  }
  const hasEditStats = editFiles > 0 || editAdded > 0 || editRemoved > 0

  const collapsedSummary = (expand: () => void) => (
    <div className="ml-8 mb-2 flex items-center gap-2 text-[10px] text-muted-foreground/60 italic">
      <div className="h-px flex-1 bg-border/30" />
      <button
        type="button"
        className="flex items-center gap-1 transition hover:text-foreground hover:underline"
        onClick={expand}
      >
        <Wrench className="h-3 w-3" />
        <span>
          已折叠 {toolCount} 个工具调用
          {thinkingCount ? ` + ${thinkingCount} 段思考` : ""}
          {errorCount ? ` + ${errorCount} 个错误` : ""}
          {hasEditStats && (
            <span className="ml-1 text-emerald-600/80">
              · {editFiles} 文件
              {editAdded > 0 && ` +${editAdded}`}
              {editRemoved > 0 && ` −${editRemoved}`}
            </span>
          )}
          {" "} (点击展开)
        </span>
      </button>
      <div className="h-px flex-1 bg-border/30" />
    </div>
  )

  return (
    <ExpandableOverflowList
      defaultMode={defaultMode}
      collapsedSummary={collapsedSummary}
      collapseLabel="收起工具调用"
      peekHeightClassName="max-h-64"
    >
      {parts.map((part, idx) => {
        const isLastPart = idx === parts.length - 1
        if (part.type === "reasoning" || part.type === "reasoning-delta") {
          return (
            <ThinkingPart
              key={part.id}
              content={part.reasoning || ""}
              isStreaming={isLastInMessage && isLastPart && msgState === "reasoning"}
              hasContentBelow={!isLastPart}
            />
          )
        }
        if (part.type === "error") {
          return <ErrorPart key={part.id} part={part} />
        }
        return (
          <ToolCallView
            key={part.id}
            part={part}
            canCancel={!readOnly && !!onCancelTool}
            onCancel={part.tool?.callID ? () => onCancelTool?.(part.tool!.callID) : undefined}
            onOpenTask={onOpenTask ? (taskId) => void onOpenTask(taskId) : undefined}
            // Always collapsed by default: keep tool cards compact (no args/output)
            defaultExpanded={false}
            defaultIsCollapsed={false}
          />
        )
      })}
    </ExpandableOverflowList>
  )
}

export function isToolGroupPart(part: Part) {
  return part.type === "tool" || part.type === "tool-delta"
}

export function isThinkingGroupPart(part: Part) {
  return part.type === "reasoning" || part.type === "reasoning-delta"
}

export function isToolGroupErrorPart(part: Part | null | undefined) {
  return part?.type === "error"
}

export function isVisibleAssistantBoundaryPart(part: Part) {
  switch (part.type) {
    case "text":
    case "text-delta":
      return Boolean((part.text || "").trim())
    case "reasoning":
    case "reasoning-delta":
      return true
    case "error":
    case "compaction":
    case "memory-organization":
      return true
    case "finish-step":
      return part.reason === "user-cancelled"
    case "file":
      return isRenderableFilePart(part)
    default:
      return false
  }
}

export function findNextGroupingTarget(parts: Part[], startIndex: number): Part | null {
  for (let index = startIndex; index < parts.length; index += 1) {
    const candidate = parts[index]
    if (isToolGroupPart(candidate) || isVisibleAssistantBoundaryPart(candidate)) {
      return candidate
    }
  }
  return null
}

