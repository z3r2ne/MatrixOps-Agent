import React, { useState, useCallback } from "react"
import { List, Send, Trash2, MoreHorizontal, Pencil, X, Check, MessageSquarePlus, Bot } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Switch } from "@/components/ui/switch"
import { Label } from "@/components/ui/label"
import { cn } from "@/lib/utils"
import type { TaskMessageQueueItem } from "@/lib/api"

export interface TaskMessageQueueProps {
  queue: TaskMessageQueueItem[]
  /** 队列自动发送：开启时当前对话结束后自动依次执行队列消息 */
  autoSend?: boolean
  onAutoSendChange?: (autoSend: boolean) => void
  /** 任务是否正在执行（决定「补充」/「发送」文案与行为） */
  isTaskRunning?: boolean
  onQueueChange?: (queue: TaskMessageQueueItem[]) => void
  onSendNextItem?: (itemId: string) => void
  readOnly?: boolean
}

export const TaskMessageQueue: React.FC<TaskMessageQueueProps> = ({
  queue,
  autoSend = true,
  onAutoSendChange,
  isTaskRunning = false,
  onQueueChange,
  onSendNextItem,
  readOnly,
}) => {
  const [editingId, setEditingId] = useState<string | null>(null)
  const [editValue, setEditValue] = useState("")
  const [openMenuId, setOpenMenuId] = useState<string | null>(null)

  const isSupplementMode = isTaskRunning

  const handleDelete = useCallback(
    (itemId: string) => {
      const next = queue.filter((item) => item.id !== itemId)
      onQueueChange?.(next)
      setOpenMenuId(null)
    },
    [queue, onQueueChange],
  )

  const handleStartEdit = useCallback(
    (item: TaskMessageQueueItem) => {
      setEditingId(item.id)
      setEditValue(item.content)
      setOpenMenuId(null)
    },
    [],
  )

  const handleConfirmEdit = useCallback(() => {
    if (!editingId) return
    const next = queue.map((item) =>
      item.id === editingId ? { ...item, content: editValue } : item,
    )
    onQueueChange?.(next)
    setEditingId(null)
    setEditValue("")
  }, [editingId, editValue, queue, onQueueChange])

  const handleCancelEdit = useCallback(() => {
    setEditingId(null)
    setEditValue("")
  }, [])

  if (queue.length === 0) {
    return null
  }

  return (
    <div className="mb-2 rounded-lg border border-border/60 bg-muted/20">
      <div className="flex items-center justify-between gap-2 px-3 py-1.5">
        <span className="text-[11px] font-medium text-muted-foreground">
          待发送队列 ({queue.length})
        </span>
        <div className="flex items-center gap-2">
          <Label
            htmlFor="task-queue-auto-send"
            className={cn(
              "cursor-pointer text-[11px] font-normal",
              autoSend ? "text-foreground" : "text-muted-foreground",
            )}
          >
            自动发送
          </Label>
          <Switch
            id="task-queue-auto-send"
            checked={autoSend}
            onCheckedChange={(checked) => onAutoSendChange?.(checked)}
            disabled={readOnly || !onAutoSendChange}
            aria-label="队列自动发送"
          />
        </div>
      </div>
      <div className="divide-y divide-border/40">
        {queue.map((item) => {
          const isAppendMessage = item.type === "append"
          const isSupplementMessage = item.supplement === true || isAppendMessage
          const isSystemMessage = item.type === "system" || isAppendMessage
          return (
          <div
            key={item.id}
            className="group flex items-center gap-2 px-3 py-2 hover:bg-muted/40"
          >
            {isSystemMessage ? (
              <Bot className="h-3.5 w-3.5 shrink-0 text-muted-foreground/60" />
            ) : (
              <List className="h-3.5 w-3.5 shrink-0 text-muted-foreground/60" />
            )}
            {editingId === item.id ? (
              <div className="flex min-w-0 flex-1 items-center gap-2">
                <input
                  type="text"
                  value={editValue}
                  onChange={(e) => setEditValue(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter") handleConfirmEdit()
                    if (e.key === "Escape") handleCancelEdit()
                  }}
                  className="min-w-0 flex-1 rounded border border-border bg-background px-2 py-1 text-sm outline-none focus:border-primary/50"
                  autoFocus
                />
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  className="h-6 w-6 shrink-0"
                  onClick={handleConfirmEdit}
                >
                  <Check className="h-3.5 w-3.5" />
                </Button>
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  className="h-6 w-6 shrink-0"
                  onClick={handleCancelEdit}
                >
                  <X className="h-3.5 w-3.5" />
                </Button>
              </div>
            ) : (
              <>
                <div className="flex min-w-0 flex-1 items-center gap-2">
                  {isSystemMessage && (
                    <Badge variant="secondary" className="shrink-0 px-1.5 py-0 text-[10px]">
                      {isAppendMessage ? "追加" : isSupplementMessage ? "补充" : "系统"}
                    </Badge>
                  )}
                  <span className="min-w-0 flex-1 truncate text-sm text-foreground">
                    {item.content}
                  </span>
                </div>
                <div className="flex shrink-0 items-center gap-0.5 opacity-0 transition group-hover:opacity-100">
                  <Button
                    type="button"
                    variant="ghost"
                    size="sm"
                    className="h-6 gap-0.5 px-1.5 text-[11px] text-muted-foreground hover:text-foreground"
                    onClick={() => onSendNextItem?.(item.id)}
                    disabled={readOnly}
                    title={isSupplementMessage || isSupplementMode ? "在本轮对话循环中补充信息" : "立即发送该消息"}
                  >
                    {isSupplementMessage || isSupplementMode ? (
                      <MessageSquarePlus className="h-3 w-3" />
                    ) : (
                      <Send className="h-3 w-3" />
                    )}
                    {isSupplementMessage || isSupplementMode ? "补充" : "发送"}
                  </Button>
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon"
                    className="h-6 w-6 text-muted-foreground hover:text-destructive"
                    onClick={() => handleDelete(item.id)}
                    disabled={readOnly}
                    title="删除"
                  >
                    <Trash2 className="h-3.5 w-3.5" />
                  </Button>
                  <div className="relative">
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon"
                      className="h-6 w-6 text-muted-foreground hover:text-foreground"
                      onClick={() =>
                        setOpenMenuId(openMenuId === item.id ? null : item.id)
                      }
                    >
                      <MoreHorizontal className="h-3.5 w-3.5" />
                    </Button>
                    {openMenuId === item.id && (
                      <div className="absolute right-0 top-full z-50 mt-1 w-32 rounded-md border border-border bg-popover p-1 shadow-md">
                        {!isSystemMessage && (
                          <button
                            type="button"
                            className="flex w-full items-center gap-2 rounded-sm px-2 py-1.5 text-sm text-popover-foreground hover:bg-accent"
                            onClick={() => handleStartEdit(item)}
                          >
                            <Pencil className="h-3.5 w-3.5" />
                            编辑
                          </button>
                        )}
                        <button
                          type="button"
                          className="flex w-full items-center gap-2 rounded-sm px-2 py-1.5 text-sm text-destructive hover:bg-accent"
                          onClick={() => handleDelete(item.id)}
                        >
                          <Trash2 className="h-3.5 w-3.5" />
                          删除
                        </button>
                      </div>
                    )}
                  </div>
                </div>
              </>
            )}
          </div>
        )})}
      </div>
    </div>
  )
}
