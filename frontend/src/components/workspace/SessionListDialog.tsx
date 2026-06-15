import React, { useEffect, useState } from "react"
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { ScrollArea } from "@/components/ui/scroll-area"
import { api, TaskExecution } from "@/lib/api"
import { cn } from "@/lib/utils"
import { 
  CheckCircle2, 
  XCircle, 
  Loader2, 
  Clock, 
  GitBranch,
  MessageSquare
} from "lucide-react"
import { formatDistanceToNow } from "date-fns"
import { zhCN } from "date-fns/locale"

interface SessionListDialogProps {
  taskId: number | null
  open: boolean
  onOpenChange: (open: boolean) => void
  onSelectSession: (execution: TaskExecution) => void
}

const statusConfig = {
  running: { icon: Loader2, color: "text-orange-500", bg: "bg-orange-50", label: "执行中" },
  success: { icon: CheckCircle2, color: "text-emerald-500", bg: "bg-emerald-50", label: "成功" },
  failed: { icon: XCircle, color: "text-red-500", bg: "bg-red-50", label: "失败" },
}

export function SessionListDialog({ 
  taskId, 
  open, 
  onOpenChange, 
  onSelectSession 
}: SessionListDialogProps) {
  const [executions, setExecutions] = useState<TaskExecution[]>([])
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    if (open && taskId) {
      loadExecutions()
    }
  }, [open, taskId])

  const loadExecutions = async () => {
    if (!taskId) return
    setLoading(true)
    try {
      const data = await api.getTaskExecutions(taskId)
      setExecutions(data)
    } catch (error) {
      console.error("Failed to load executions:", error)
    } finally {
      setLoading(false)
    }
  }

  const formatDuration = (ms: number | undefined): string => {
    if (!ms) return "-"
    if (ms < 1000) return `${ms}ms`
    if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`
    return `${Math.floor(ms / 60000)}m ${Math.floor((ms % 60000) / 1000)}s`
  }

  const formatTime = (dateStr: string): string => {
    try {
      return formatDistanceToNow(new Date(dateStr), { 
        addSuffix: true,
        locale: zhCN 
      })
    } catch {
      return dateStr
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <GitBranch className="h-5 w-5" />
            会话记录
          </DialogTitle>
        </DialogHeader>

        <ScrollArea className="max-h-[400px]">
          {loading ? (
            <div className="flex items-center justify-center py-8">
              <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
            </div>
          ) : executions.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-8 text-muted-foreground">
              <MessageSquare className="h-10 w-10 mb-2 opacity-50" />
              <p>暂无会话记录</p>
            </div>
          ) : (
            <div className="space-y-2">
              {executions.map((exec, index) => {
                const config = statusConfig[exec.status] || statusConfig.running
                const StatusIcon = config.icon

                return (
                  <div
                    key={exec.id}
                    onClick={() => {
                      onSelectSession(exec)
                      onOpenChange(false)
                    }}
                    className={cn(
                      "flex items-center gap-3 p-3 rounded-lg border cursor-pointer transition-colors",
                      "hover:bg-accent hover:border-accent-foreground/20"
                    )}
                  >
                    <div className={cn(
                      "flex h-8 w-8 shrink-0 items-center justify-center rounded-full",
                      config.bg
                    )}>
                      <StatusIcon className={cn(
                        "h-4 w-4", 
                        config.color,
                        exec.status === "running" && "animate-spin"
                      )} />
                    </div>

                    <div className="flex-1 min-w-0">
                      <div className="flex items-center gap-2">
                        <span className="font-medium text-sm">
                          会话 #{executions.length - index}
                        </span>
                        {exec.agentSessionId && (
                          <span className="text-xs px-1.5 py-0.5 rounded bg-gray-100 text-gray-600 font-mono truncate max-w-[100px]" title={exec.agentSessionId}>
                            {exec.agentSessionId.slice(0, 8)}...
                          </span>
                        )}
                      </div>
                      <div className="flex items-center gap-3 text-xs text-muted-foreground mt-1">
                        <span className="flex items-center gap-1">
                          <Clock className="h-3 w-3" />
                          {formatTime(exec.startedAt)}
                        </span>
                        {exec.duration && (
                          <span>耗时 {formatDuration(exec.duration)}</span>
                        )}
                        <span className={config.color}>{config.label}</span>
                      </div>
                    </div>
                  </div>
                )
              })}
            </div>
          )}
        </ScrollArea>
      </DialogContent>
    </Dialog>
  )
}
