import React from "react"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from "@/components/ui/dialog"
import { Task } from "@/lib/api"
import { Bot, CalendarClock, GitBranch, FolderOpen, Hash, Activity, Clock } from "lucide-react"
import { Badge } from "@/components/ui/badge"
import { ScrollArea } from "@/components/ui/scroll-area"

interface TaskInfoDialogProps {
  task: Task | null
  open: boolean
  onOpenChange: (open: boolean) => void
}

const statusLabels: Record<string, { label: string; color: string }> = {
  queue: { label: "排队中", color: "bg-muted text-muted-foreground" },
  active: { label: "执行中", color: "bg-orange-100 text-orange-700" },
  running: { label: "执行中", color: "bg-orange-100 text-orange-700" },
  done: { label: "已完成", color: "bg-emerald-100 text-emerald-700" },
  cancelled: { label: "已取消", color: "bg-amber-100 text-amber-800" },
  failed: { label: "失败", color: "bg-red-100 text-red-700" },
}

export function TaskInfoDialog({ task, open, onOpenChange }: TaskInfoDialogProps) {
  if (!task) return null

  const statusConfig = statusLabels[task.status] || statusLabels.queue

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <Hash className="h-4 w-4" />
            任务详情 #{task.id}
          </DialogTitle>
          <DialogDescription>查看任务的详细信息</DialogDescription>
        </DialogHeader>

        <ScrollArea className="max-h-[60vh]">
          <div className="space-y-4 py-4">
            {/* 任务名称 */}
            {task.name && (
              <div className="flex items-start gap-2">
                <Hash className="h-4 w-4 text-muted-foreground mt-0.5" />
                <div className="flex-1">
                  <span className="text-sm font-medium text-muted-foreground">任务名称：</span>
                  <p className="text-sm mt-1 font-semibold text-foreground">{task.name}</p>
                </div>
              </div>
            )}

            {/* 状态 */}
            <div className="flex items-center gap-2">
              <Activity className="h-4 w-4 text-muted-foreground" />
              <span className="text-sm font-medium text-muted-foreground">状态：</span>
              <Badge className={statusConfig.color}>{statusConfig.label}</Badge>
            </div>

            {/* 项目名称 */}
            {task.projectName && (
              <div className="flex items-start gap-2">
                <FolderOpen className="h-4 w-4 text-muted-foreground mt-0.5" />
                <div className="flex-1">
                  <span className="text-sm font-medium text-muted-foreground">项目：</span>
                  <p className="text-sm mt-1 font-semibold text-foreground">{task.projectName}</p>
                </div>
              </div>
            )}

            {/* 任务内容 */}
            <div className="flex items-start gap-2">
              <Hash className="h-4 w-4 text-muted-foreground mt-0.5" />
              <div className="flex-1">
                <span className="text-sm font-medium text-muted-foreground">任务内容：</span>
                <p className="text-sm mt-1 text-foreground whitespace-pre-wrap break-words bg-muted p-3 rounded-md">
                  {task.content}
                </p>
              </div>
            </div>

            {/* Worker */}
            {task.workerName && (
              <div className="flex items-start gap-2">
                <Bot className="h-4 w-4 text-muted-foreground mt-0.5" />
                <div className="flex-1">
                  <span className="text-sm font-medium text-muted-foreground">Worker：</span>
                  <p className="text-sm mt-1 text-foreground font-mono">{task.workerName}</p>
                </div>
              </div>
            )}

            {/* 分支 */}
            {task.branch && (
              <div className="flex items-start gap-2">
                <GitBranch className="h-4 w-4 text-muted-foreground mt-0.5" />
                <div className="flex-1">
                  <span className="text-sm font-medium text-muted-foreground">分支：</span>
                  <p className="text-sm mt-1 text-foreground font-mono">{task.branch}</p>
                </div>
              </div>
            )}

            {/* 工作目录 */}
            {task.workDir && (
              <div className="flex items-start gap-2">
                <FolderOpen className="h-4 w-4 text-muted-foreground mt-0.5" />
                <div className="flex-1">
                  <span className="text-sm font-medium text-muted-foreground">工作目录：</span>
                  <p className="text-sm mt-1 text-foreground font-mono text-xs break-all bg-muted p-2 rounded">
                    {task.workDir}
                  </p>
                </div>
              </div>
            )}

            {/* 创建时间 */}
            <div className="flex items-start gap-2">
              <CalendarClock className="h-4 w-4 text-muted-foreground mt-0.5" />
              <div className="flex-1">
                <span className="text-sm font-medium text-muted-foreground">创建时间：</span>
                <p className="text-sm mt-1 text-foreground">
                  {new Date(task.createdAt).toLocaleString('zh-CN', {
                    year: 'numeric',
                    month: '2-digit',
                    day: '2-digit',
                    hour: '2-digit',
                    minute: '2-digit',
                    second: '2-digit'
                  })}
                </p>
              </div>
            </div>

            {/* 更新时间 */}
            {task.updatedAt && task.updatedAt !== task.createdAt && (
              <div className="flex items-start gap-2">
                <Clock className="h-4 w-4 text-muted-foreground mt-0.5" />
                <div className="flex-1">
                  <span className="text-sm font-medium text-muted-foreground">更新时间：</span>
                  <p className="text-sm mt-1 text-foreground">
                    {new Date(task.updatedAt).toLocaleString('zh-CN', {
                      year: 'numeric',
                      month: '2-digit',
                      day: '2-digit',
                      hour: '2-digit',
                      minute: '2-digit',
                      second: '2-digit'
                    })}
                  </p>
                </div>
              </div>
            )}

            {/* 项目 ID */}
            <div className="flex items-start gap-2 opacity-60">
              <Hash className="h-4 w-4 text-muted-foreground mt-0.5" />
              <div className="flex-1">
                <span className="text-sm font-medium text-muted-foreground">项目 ID：</span>
                <p className="text-sm mt-1 text-foreground font-mono">{task.projectId}</p>
              </div>
            </div>
          </div>
        </ScrollArea>
      </DialogContent>
    </Dialog>
  )
}
