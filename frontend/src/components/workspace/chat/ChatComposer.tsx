import React from "react"
import { Send, Square, Plus, X, Paperclip, Wrench, FileText, Loader2 } from "lucide-react"
import { Button } from "@/components/ui/button"
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip"
import { LexicalMentionInput } from "../LexicalMentionInput"
import { TaskMessageQueue } from "../TaskMessageQueue"
import { TaskPlanBar } from "../TaskPlanBar"
import { ContextUsageDial } from "../ContextUsageDial"
import type { LocalChatAttachment } from "./chat-utils"
import type { TaskMessageQueueItem, TaskPlanDocument } from "@/lib/api"
import type { MentionItem } from "../MarkdownMentionInput"

export interface ChatComposerProps {
  readOnly?: boolean
  isRunning?: boolean
  taskStatus?: string
  showSendWhileRunning: boolean
  hasComposerPayload: boolean
  input: string
  onInputChange: (value: string) => void
  onSend: () => void
  onStop: (e?: React.MouseEvent | React.FormEvent) => void
  placeholder?: string
  pendingAttachments: LocalChatAttachment[]
  onRemoveAttachment: (id: string) => void
  onAttachmentPickerChange: (e: React.ChangeEvent<HTMLInputElement>) => void
  onComposerPasteCapture: (e: React.ClipboardEvent) => void
  attachmentPickerRef: React.RefObject<HTMLInputElement | null>
  composerWorkerLabel?: string
  taskTitle?: string
  taskPlan?: TaskPlanDocument | null
  taskId?: number
  sessionId?: string
  sessionContextLoading?: boolean
  onOpenSessionInfoDialog: () => void
  onOpenProjectFilePromptsDialog: () => void
  onOpenContextDialog: () => void
  contextButtonTitle?: string
  contextUsagePercent?: number
  defaultMentionItems: MentionItem[]
  onMentionSearch: (query: string) => Promise<MentionItem[]>
  onFileSearch: (query: string) => Promise<MentionItem[]>
  onSpecialCommand: (command: string) => void
  pendingMention: MentionItem | null
  onInsertMentionComplete: () => void
  composerExtra?: React.ReactNode
  messageQueue?: TaskMessageQueueItem[]
  messageQueueAutoSend?: boolean
  onMessageQueueAutoSendChange?: (autoSend: boolean) => void
  onMessageQueueChange?: (queue: TaskMessageQueueItem[]) => void
  onSendNextQueueItem?: (itemId: string) => void
  showAlternateContent?: boolean
}

export const ChatComposer: React.FC<ChatComposerProps> = ({
  readOnly,
  isRunning,
  taskStatus,
  showSendWhileRunning,
  hasComposerPayload,
  input,
  onInputChange,
  onSend,
  onStop,
  placeholder,
  pendingAttachments,
  onRemoveAttachment,
  onAttachmentPickerChange,
  onComposerPasteCapture,
  attachmentPickerRef,
  composerWorkerLabel,
  taskTitle,
  taskPlan,
  taskId,
  sessionId,
  sessionContextLoading,
  onOpenSessionInfoDialog,
  onOpenProjectFilePromptsDialog,
  onOpenContextDialog,
  contextButtonTitle,
  contextUsagePercent,
  defaultMentionItems,
  onMentionSearch,
  onFileSearch,
  onSpecialCommand,
  pendingMention,
  onInsertMentionComplete,
  composerExtra,
  messageQueue,
  messageQueueAutoSend,
  onMessageQueueAutoSendChange,
  onMessageQueueChange,
  onSendNextQueueItem,
  showAlternateContent,
}) => {
  const composerNode = !readOnly ? (
    <div className="p-3 space-y-2">
      <form
        onSubmit={(e) => {
          e.preventDefault()
          if (showSendWhileRunning) {
            onSend()
          } else if (isRunning && onStop) {
            onStop(e)
          } else {
            onSend()
          }
        }}
        className="flex flex-col gap-2"
      >
        {taskId && taskPlan ? (
          <TaskPlanBar
            plan={taskPlan}
            taskTitle={taskTitle || composerWorkerLabel || "任务计划"}
            isRunning={isRunning || taskStatus === "running"}
          />
        ) : null}
        {composerWorkerLabel && taskId ? (
          <div className="flex flex-wrap items-center gap-2 rounded-md border border-border/60 bg-muted/40 px-2.5 py-1.5 text-xs text-muted-foreground">
            <span>
              当前 Worker：
              <span className="ml-1 font-medium text-foreground">{composerWorkerLabel}</span>
            </span>
            <Button
              type="button"
              size="sm"
              variant="ghost"
              className="h-6 px-2 text-[11px]"
              onClick={onOpenSessionInfoDialog}
              disabled={!sessionId || sessionContextLoading}
            >
              <Wrench className="mr-1 h-3 w-3" />
              技能/工具
            </Button>
            <Button
              type="button"
              size="sm"
              variant="ghost"
              className="h-6 px-2 text-[11px]"
              onClick={onOpenProjectFilePromptsDialog}
              disabled={!sessionId || sessionContextLoading}
            >
              <FileText className="mr-1 h-3 w-3" />
              关联提示词
            </Button>
            <Button
              type="button"
              size="sm"
              variant="ghost"
              className="h-6 px-2 text-[11px]"
              onClick={onOpenContextDialog}
              disabled={!sessionId || sessionContextLoading}
              title={contextButtonTitle}
            >
              <ContextUsageDial percent={contextUsagePercent} loading={sessionContextLoading} className="mr-1" />
              上下文
            </Button>
          </div>
        ) : null}
        <input
          ref={attachmentPickerRef}
          type="file"
          multiple
          accept="image/*,.pdf,.txt,.md,.json,.csv,.log"
          className="hidden"
          onChange={onAttachmentPickerChange}
        />
        {pendingAttachments.length > 0 && (
          <div className="flex flex-wrap gap-2 rounded-md border border-border/80 bg-muted/30 p-2">
            {pendingAttachments.map((a) => {
              const isImg = a.mime.startsWith("image/")
              return (
                <div
                  key={a.id}
                  className="group relative flex h-16 w-16 shrink-0 items-center justify-center overflow-hidden rounded-md border border-border bg-background"
                >
                  {isImg ? (
                    <img src={a.previewUrl} alt="" className="h-full w-full object-cover" />
                  ) : (
                    <Paperclip className="h-6 w-6 text-muted-foreground" aria-hidden />
                  )}
                  <button
                    type="button"
                    className="absolute inset-0 flex items-start justify-end bg-black/0 p-0.5 opacity-0 transition group-hover:bg-black/40 group-hover:opacity-100"
                    onClick={() => onRemoveAttachment(a.id)}
                    aria-label="移除附件"
                  >
                    <X className="h-4 w-4 text-white drop-shadow" />
                  </button>
                  <span className="pointer-events-none absolute bottom-0 left-0 right-0 truncate bg-background/90 px-0.5 text-[9px] text-muted-foreground" title={a.name}>
                    {a.name}
                  </span>
                </div>
              )
            })}
          </div>
        )}
        <div className="flex gap-2 items-end" onPasteCapture={onComposerPasteCapture}>
          <TooltipProvider>
            <Tooltip>
              <TooltipTrigger asChild>
                <Button
                  type="button"
                  variant="outline"
                  size="icon"
                  className="h-10 w-10 shrink-0"
                  disabled={readOnly}
                  onClick={() => attachmentPickerRef.current?.click()}
                >
                  <Plus className="h-4 w-4" />
                </Button>
              </TooltipTrigger>
              <TooltipContent side="top">添加图片或文件（支持粘贴截图）</TooltipContent>
            </Tooltip>
          </TooltipProvider>
          <LexicalMentionInput
            value={input}
            onChange={onInputChange}
            onSubmit={onSend}
            placeholder={placeholder}
            disabled={readOnly}
            mentionItems={defaultMentionItems}
            onMentionSearch={onMentionSearch}
            onFileSearch={onFileSearch}
            onSpecialCommand={onSpecialCommand}
            insertMentionRequest={pendingMention}
            onInsertMentionComplete={onInsertMentionComplete}
            maxHeight="120px"
          />
          {isRunning && !showSendWhileRunning ? (
            <Button
              type="button"
              size="sm"
              variant="destructive"
              className="h-10 w-10 shrink-0"
              onClick={onStop}
              aria-label="停止任务"
            >
              <Square className="h-4 w-4" />
            </Button>
          ) : (
            <Button
              type="submit"
              size="sm"
              className="h-10 w-10 shrink-0"
              disabled={!hasComposerPayload || readOnly}
              aria-label={showSendWhileRunning ? "发送到队列" : "发送"}
            >
              <Send className="h-4 w-4" />
            </Button>
          )}
        </div>
        {composerExtra ? <div className="pt-1">{composerExtra}</div> : null}
      </form>
    </div>
  ) : null

  return (
    <div className="shrink-0 min-w-0 border-t bg-background">
      {messageQueue != null && messageQueue.length > 0 && (
        <div className="px-3 pt-2">
          <TaskMessageQueue
            queue={messageQueue}
            autoSend={messageQueueAutoSend}
            onAutoSendChange={onMessageQueueAutoSendChange}
            isTaskRunning={taskStatus === "running" || isRunning}
            onQueueChange={onMessageQueueChange}
            onSendNextItem={onSendNextQueueItem}
            readOnly={readOnly}
          />
        </div>
      )}
      {!showAlternateContent ? composerNode : null}
    </div>
  )
}
