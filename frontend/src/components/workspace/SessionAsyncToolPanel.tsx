import React, { useEffect, useMemo, useState } from "react"
import { Timer, Wrench, XCircle } from "lucide-react"

import { type SessionCriticalInfo } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { ScrollArea } from "@/components/ui/scroll-area"
import { cn } from "@/lib/utils"
import { toast } from "sonner"

interface SessionAsyncToolPanelProps {
  sessionId?: string
  criticalInfo?: SessionCriticalInfo | null
  onCancelTask?: (callId: string) => void
  className?: string
}

function formatDuration(ms: number): string {
  const totalSeconds = Math.max(0, Math.floor(ms / 1000))
  const minutes = Math.floor(totalSeconds / 60)
  const seconds = totalSeconds % 60
  if (minutes > 0) {
    return `${minutes}分${seconds}秒`
  }
  return `${seconds}秒`
}

function summarizeParams(params?: Record<string, unknown>): string {
  if (!params || Object.keys(params).length === 0) {
    return "{}"
  }
  try {
    const text = JSON.stringify(params)
    return text.length > 120 ? `${text.slice(0, 120)}…` : text
  } catch {
    return "{}"
  }
}

export function SessionAsyncToolPanel({
  sessionId,
  criticalInfo,
  onCancelTask,
  className,
}: SessionAsyncToolPanelProps) {
  const [now, setNow] = useState(() => Date.now())

  const runningTasks = useMemo(() => {
    const items = criticalInfo?.items ?? []
    return items.filter(
      (item) => item.type === "async_tool" && item.asyncTask?.status === "running",
    )
  }, [criticalInfo])

  useEffect(() => {
    if (runningTasks.length === 0) {
      return
    }
    const timer = window.setInterval(() => setNow(Date.now()), 1000)
    return () => window.clearInterval(timer)
  }, [runningTasks.length])

  return (
    <div className={cn("flex min-h-0 flex-col border-t bg-background", className)}>
      <div className="border-b px-3 py-2.5">
        <div className="flex items-center gap-1.5 text-sm font-medium">
          <Wrench className="h-4 w-4 shrink-0 text-primary" />
          <span>异步工具</span>
          {runningTasks.length > 0 ? (
            <span className="text-xs font-normal text-muted-foreground">({runningTasks.length})</span>
          ) : null}
        </div>
        <p className="mt-1 text-[11px] leading-4 text-muted-foreground">
          后台执行中的工具任务，可手动结束
        </p>
      </div>

      <ScrollArea className="max-h-56">
        <div className="space-y-2 p-3">
          {!sessionId ? (
            <div className="rounded-md border border-dashed px-3 py-6 text-center text-xs text-muted-foreground">
              选择任务后可查看异步工具
            </div>
          ) : runningTasks.length === 0 ? (
            <div className="rounded-md border border-dashed px-3 py-6 text-center text-xs text-muted-foreground">
              当前没有进行中的异步工具
            </div>
          ) : (
            runningTasks.map((item) => {
              const task = item.asyncTask
              if (!task) return null
              const elapsed = formatDuration(now - task.startedAt)
              return (
                <div key={item.id} className="rounded-md border bg-muted/20 p-2.5">
                  <div className="flex items-start justify-between gap-2">
                    <div className="min-w-0 flex-1">
                      <div className="truncate text-sm font-medium">{task.toolName}</div>
                      <div className="mt-1 break-all text-[11px] leading-4 text-muted-foreground">
                        {summarizeParams(task.params)}
                      </div>
                      <div className="mt-1.5 flex items-center gap-1 text-[11px] text-muted-foreground">
                        <Timer className="h-3 w-3" />
                        已运行 {elapsed}
                      </div>
                    </div>
                    <Button
                      type="button"
                      size="sm"
                      variant="outline"
                      className="h-7 shrink-0 gap-1 px-2 text-xs"
                      onClick={() => {
                        if (!task.callId) {
                          toast.error("缺少 call_id，无法结束任务")
                          return
                        }
                        onCancelTask?.(task.callId)
                        toast.message("已请求结束异步工具")
                      }}
                    >
                      <XCircle className="h-3.5 w-3.5" />
                      结束
                    </Button>
                  </div>
                </div>
              )
            })
          )}
        </div>
      </ScrollArea>
    </div>
  )
}
