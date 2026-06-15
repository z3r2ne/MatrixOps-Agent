import React from "react"
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { ScrollArea } from "@/components/ui/scroll-area"
import { TaskExecution } from "@/lib/api"
import { cn } from "@/lib/utils"
import { 
  CheckCircle2, 
  XCircle, 
  Loader2, 
  Clock, 
  Terminal,
  Copy,
  FolderOpen
} from "lucide-react"
import { Button } from "@/components/ui/button"
import { toast } from "sonner"

interface SessionInfoDialogProps {
  execution: TaskExecution | null
  open: boolean
  onOpenChange: (open: boolean) => void
  onViewProcessLogs?: () => void
}

const statusConfig = {
  running: { icon: Loader2, color: "text-orange-500", label: "执行中" },
  success: { icon: CheckCircle2, color: "text-emerald-500", label: "成功" },
  failed: { icon: XCircle, color: "text-red-500", label: "失败" },
}

export function SessionInfoDialog({ 
  execution, 
  open, 
  onOpenChange,
  onViewProcessLogs
}: SessionInfoDialogProps) {
  if (!execution) return null

  const config = statusConfig[execution.status] || statusConfig.running
  const StatusIcon = config.icon

  const copyToClipboard = (text: string, label: string) => {
    navigator.clipboard.writeText(text)
    toast.success(`${label} 已复制`)
  }

  const formatTime = (dateStr: string): string => {
    try {
      return new Date(dateStr).toLocaleString("zh-CN")
    } catch {
      return dateStr
    }
  }

  const formatDuration = (ms: number | undefined): string => {
    if (!ms) return "-"
    if (ms < 1000) return `${ms}ms`
    if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`
    return `${Math.floor(ms / 60000)}m ${Math.floor((ms % 60000) / 1000)}s`
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <Terminal className="h-5 w-5" />
            会话信息
          </DialogTitle>
        </DialogHeader>

        <ScrollArea className="max-h-[500px]">
          <div className="space-y-4">
            {/* 基本信息 */}
            <div className="space-y-3">
              <InfoRow label="会话 ID" value={`#${execution.id}`} />
              <InfoRow label="任务 ID" value={`#${execution.taskId}`} />
              <InfoRow 
                label="状态" 
                value={
                  <span className={cn("flex items-center gap-1", config.color)}>
                    <StatusIcon className={cn("h-4 w-4", execution.status === "running" && "animate-spin")} />
                    {config.label}
                  </span>
                } 
              />
              <InfoRow label="Worker" value={execution.workerName || "-"} />
            </div>

            {/* 时间信息 */}
            <div className="border-t pt-3 space-y-3">
              <InfoRow 
                label="开始时间" 
                value={formatTime(execution.startedAt)} 
                icon={<Clock className="h-3.5 w-3.5" />}
              />
              {execution.finishedAt && (
                <InfoRow 
                  label="结束时间" 
                  value={formatTime(execution.finishedAt)} 
                  icon={<Clock className="h-3.5 w-3.5" />}
                />
              )}
              {execution.duration && (
                <InfoRow label="耗时" value={formatDuration(execution.duration)} />
              )}
            </div>

            {/* Agent Session ID */}
            {execution.agentSessionId && (
              <div className="border-t pt-3">
                <div className="flex items-center justify-between mb-1">
                  <span className="text-xs text-muted-foreground">Agent Session ID</span>
                  <Button 
                    variant="ghost" 
                    size="sm" 
                    className="h-6 text-xs"
                    onClick={() => copyToClipboard(execution.agentSessionId!, "Session ID")}
                  >
                    <Copy className="h-3 w-3 mr-1" />
                    复制
                  </Button>
                </div>
                <code className="block text-xs bg-muted p-2 rounded font-mono break-all">
                  {execution.agentSessionId}
                </code>
              </div>
            )}

            {/* 工作目录 */}
            {execution.workDir && (
              <div className="border-t pt-3">
                <div className="flex items-center justify-between mb-1">
                  <span className="text-xs text-muted-foreground flex items-center gap-1">
                    <FolderOpen className="h-3.5 w-3.5" />
                    工作目录
                  </span>
                  <Button 
                    variant="ghost" 
                    size="sm" 
                    className="h-6 text-xs"
                    onClick={() => copyToClipboard(execution.workDir!, "工作目录")}
                  >
                    <Copy className="h-3 w-3 mr-1" />
                    复制
                  </Button>
                </div>
                <code className="block text-xs bg-muted p-2 rounded font-mono break-all">
                  {execution.workDir}
                </code>
              </div>
            )}

            {/* 启动命令 */}
            {execution.command && (
              <div className="border-t pt-3">
                <div className="flex items-center justify-between mb-1">
                  <span className="text-xs text-muted-foreground flex items-center gap-1">
                    <Terminal className="h-3.5 w-3.5" />
                    启动命令
                  </span>
                  <Button 
                    variant="ghost" 
                    size="sm" 
                    className="h-6 text-xs"
                    onClick={() => copyToClipboard(execution.command!, "启动命令")}
                  >
                    <Copy className="h-3 w-3 mr-1" />
                    复制
                  </Button>
                </div>
                <code className="block text-xs bg-muted p-2 rounded font-mono break-all">
                  {execution.command}
                </code>
              </div>
            )}

            {/* 错误信息 */}
            {execution.errorMsg && (
              <div className="border-t pt-3">
                <span className="text-xs text-red-500 mb-1 block">错误信息</span>
                <code className="block text-xs bg-red-50 text-red-700 p-2 rounded font-mono break-all">
                  {execution.errorMsg}
                </code>
              </div>
            )}

            {/* 进程日志按钮 */}
            {onViewProcessLogs && (
              <div className="border-t pt-3">
                <Button 
                  variant="outline" 
                  className="w-full"
                  onClick={() => {
                    onOpenChange(false)
                    onViewProcessLogs()
                  }}
                >
                  <Terminal className="h-4 w-4 mr-2" />
                  查看进程日志
                </Button>
              </div>
            )}
          </div>
        </ScrollArea>
      </DialogContent>
    </Dialog>
  )
}

function InfoRow({ 
  label, 
  value, 
  icon 
}: { 
  label: string
  value: React.ReactNode
  icon?: React.ReactNode 
}) {
  return (
    <div className="flex items-center justify-between">
      <span className="text-xs text-muted-foreground flex items-center gap-1">
        {icon}
        {label}
      </span>
      <span className="text-sm font-medium">{value}</span>
    </div>
  )
}
