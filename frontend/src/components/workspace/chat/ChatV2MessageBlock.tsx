import React from "react"
import { Bot, Loader2 } from "lucide-react"
import { Badge } from "@/components/ui/badge"
import { cn } from "@/lib/utils"
import { type WithParts, type Part } from "@/lib/api"
import { MessageAttachmentBubble, isRenderableFilePart } from "../MessageAttachmentBubble"
import {
  UserMessagePart,
  UserFileBubble,
  SystemMessagePart,
  AssistantFileAttachment,
  AssistantMessagePart,
  ThinkingPart,
  CompactionPart,
  MemoryOrganizationPartRender,
  ErrorPart,
  CancelledConversationDivider,
} from "./ChatPartRenders"
import {
  ToolGroupRender,
  isToolGroupPart,
  isThinkingGroupPart,
  isToolGroupErrorPart,
  isVisibleAssistantBoundaryPart,
  findNextGroupingTarget,
} from "./ChatToolGroupRender"
import { isUserAuthoredMessage, isSystemChatMessage } from "./chat-utils"

export const ChatV2MessageBlock: React.FC<{
  msg: WithParts
  showAssistantHeader?: boolean
  isLastRetryableUserMessage: boolean
  isHighlighted: boolean
  readOnly: boolean
  onCancelTool?: (callID: string) => void
  onOpenTask?: (taskId: number) => void | Promise<void>
  onRetryUserMessage?: (messageId: string) => void | Promise<void>
  onViewMemory: (messageId: string) => void
  canViewPrompt: boolean
  onMessageLayoutChange?: () => void
  isLastMessage?: boolean
}> = ({ msg, showAssistantHeader = true, isLastRetryableUserMessage, isHighlighted, readOnly, onCancelTool, onOpenTask, onRetryUserMessage, onViewMemory, canViewPrompt, onMessageLayoutChange, isLastMessage }) => (
  <div
    id={`msg-${msg.info.id}`}
    className={cn(
      "group -mx-2 min-w-0 rounded-lg p-2 transition-colors duration-500",
      isHighlighted ? "bg-primary/10 ring-1 ring-primary/20" : ""
    )}
  >
    {isUserAuthoredMessage(msg) ? (
      <div className="flex justify-end mb-4">
        <div className="flex max-w-[74%] flex-col items-end gap-2">
          {msg.parts.map((p) => {
            if (p.type === "text" && (p.text || "").trim()) {
              return (
                <UserMessagePart
                  key={p.id}
                  content={p.text || ""}
                  timestamp={p.time?.created || msg.info.time.created}
                  canRetry={!readOnly && isLastRetryableUserMessage && !!onRetryUserMessage}
                  onRetry={!readOnly && isLastRetryableUserMessage && onRetryUserMessage ? () => void onRetryUserMessage(msg.info.id) : undefined}
                />
              )
            }
            if (p.type === "file") {
              return <UserFileBubble key={p.id} part={p} />
            }
            return null
          })}
        </div>
      </div>
    ) : isSystemChatMessage(msg) ? (
      <div className="mb-4 flex justify-start pl-6">
        <div className="flex max-w-[85%] flex-col gap-2">
          {msg.parts.map((p) => {
            if (p.type === "text" && (p.text || "").trim()) {
              return (
                <SystemMessagePart
                  key={p.id}
                  content={p.text || ""}
                  origin={msg.info.messageOrigin}
                  timestamp={p.time?.created || msg.info.time.created}
                />
              )
            }
            if (p.type === "file") {
              return (
                <div key={p.id} className="max-w-full">
                  <MessageAttachmentBubble part={p} variant="assistant" />
                </div>
              )
            }
            return null
          })}
        </div>
      </div>
    ) : (
      <div className="flex flex-col">
        {msg.info.role === "assistant" && showAssistantHeader && (
          <div className="flex items-center gap-2 ml-8 mb-1 text-xs text-muted-foreground">
            <span className="font-medium text-foreground/80">
              {msg.info.name || msg.info.agent || msg.info.worker || "AI"}
            </span>
            {msg.info.occupation && (
              <Badge variant="secondary" className="px-1.5 py-0 text-[10px]">
                {msg.info.occupation}
              </Badge>
            )}
          </div>
        )}
        {(() => {
          const orderedParts = msg.parts
          const renderedParts: React.ReactNode[] = []
          let toolGroup: Part[] = []

          const flushTools = (isLastInMessage: boolean) => {
            if (toolGroup.length === 0) return

            const group = [...toolGroup]
            const groupId = group[0].id

            const isLastGroupInMessage = isLastInMessage
            const lastBoundaryPart = group[group.length - 1]
            const hasSubsequentTextInMessage = orderedParts.slice(orderedParts.indexOf(lastBoundaryPart) + 1).some(p => 
              p.type === "text" || p.type === "text-delta" || 
              p.type === "reasoning" || p.type === "reasoning-delta"
            )
            const isFollowedByOtherMessage = !isLastMessage
            const defaultToolGroupMode: "peek" | "full" =
              hasSubsequentTextInMessage || (isLastGroupInMessage && isFollowedByOtherMessage)
                ? "full"
                : "peek"

            renderedParts.push(
              <ToolGroupRender
                key={`group-${groupId}`}
                parts={group}
                readOnly={readOnly}
                onCancelTool={onCancelTool}
                onOpenTask={onOpenTask}
                defaultMode={defaultToolGroupMode}
                isLastInMessage={isLastGroupInMessage && !isFollowedByOtherMessage}
                msgState={msg.info.state}
              />
            )
            toolGroup = []
          }

          orderedParts.forEach((part, idx) => {
            if (isToolGroupPart(part)) {
              toolGroup.push(part)
              return
            }

            if (isThinkingGroupPart(part)) {
              const nextTarget = findNextGroupingTarget(orderedParts, idx + 1)
              if (nextTarget && (isToolGroupPart(nextTarget) || isToolGroupErrorPart(nextTarget))) {
                toolGroup.push(part)
                return
              }
              if (toolGroup.length > 0) {
                if (nextTarget && (nextTarget.type === "text" || nextTarget.type === "text-delta")) {
                  // 前面有工具组且后面跟着文本，不把 thinking 放入 toolGroup
                } else {
                  toolGroup.push(part)
                  return
                }
              }
            }

            if (toolGroup.length > 0 && isToolGroupErrorPart(part)) {
              toolGroup.push(part)
              return
            }

            if (!isVisibleAssistantBoundaryPart(part)) {
              return
            }

            flushTools(false)
            const isLastPart = idx === orderedParts.length - 1
            // 判断当前 reasoning 之后是否还有需要显示的内容（text 或 reasoning）
            const hasContentBelow = orderedParts.slice(idx + 1).some(p =>
              p.type === "text" || p.type === "text-delta" ||
              p.type === "reasoning" || p.type === "reasoning-delta" ||
              isRenderableFilePart(p)
            )
            
            switch (part.type) {
              case "text":
              case "text-delta":
                renderedParts.push(
                  <AssistantMessagePart
                    key={part.id}
                    content={part.text || ""}
                    timestamp={part.time?.created || msg.info.time.created}
                    canViewPrompt={canViewPrompt}
                    onViewMemory={() => onViewMemory(msg.info.id)}
                  />
                )
                break
              case "file":
                renderedParts.push(<AssistantFileAttachment key={part.id} part={part} />)
                break
              case "reasoning":
              case "reasoning-delta":
                renderedParts.push(
                  <ThinkingPart
                    key={part.id}
                    content={part.reasoning || ""}
                    isStreaming={isLastPart && msg.info.state === "reasoning"}
                    hasContentBelow={hasContentBelow}
                  />
                )
                break
              case "error":
                renderedParts.push(<ErrorPart key={part.id} part={part} />)
                break
              case "compaction":
                renderedParts.push(<CompactionPart key={part.id} part={part} onLayoutChange={onMessageLayoutChange} />)
                break
              case "memory-organization":
                renderedParts.push(<MemoryOrganizationPartRender key={part.id} part={part} />)
                break
              case "finish-step":
                if (part.reason === "user-cancelled") {
                  renderedParts.push(<CancelledConversationDivider key={part.id} part={part} />)
                }
                break
            }
          })
          flushTools(true)
          return renderedParts
        })()}
        {msg.info.role === "assistant" && msg.info.state === "loading" && (
          <div className="flex w-full items-start gap-2 mb-4">
            <div className="flex h-6 w-6 shrink-0 items-center justify-center bg-muted mt-0.5 rounded-sm">
              <Bot className="h-4 w-4 text-muted-foreground" />
            </div>
            <div className="flex-1 min-w-0 px-3 py-2 bg-muted/50 rounded-md">
              <div className="space-y-2 animate-pulse">
                <div className="h-3 bg-muted-foreground/20 rounded w-3/4"></div>
                <div className="h-3 bg-muted-foreground/20 rounded w-1/2"></div>
              </div>
            </div>
          </div>
        )}
        {msg.info.role === "assistant" &&
          msg.info.footerStatus &&
          (msg.info.footerStatus.loading || (msg.info.footerStatus.text && msg.info.footerStatus.text.trim() !== "")) && (
            <div className="ml-8 mt-1 flex min-h-5 max-w-full items-center gap-2 text-xs text-muted-foreground">
              {msg.info.footerStatus.loading && (
                <Loader2 className="h-3.5 w-3.5 shrink-0 animate-spin text-muted-foreground" aria-hidden />
              )}
              <span className="min-w-0 truncate">
                {(msg.info.footerStatus.text && msg.info.footerStatus.text.trim()) ||
                  (msg.info.footerStatus.loading ? "处理中…" : "")}
              </span>
            </div>
          )}
      </div>
    )}
  </div>
)

// Main Interface
