import React from "react"
import { Brain, CircleCheck, CircleDashed, Flame, MoreVertical, Trash2, XCircle, Terminal, GitBranch, Info, FolderOpen, FileCode, GripVertical, BarChart3, MessageSquare, MessageCircle } from "lucide-react"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { cn } from "@/lib/utils"
import { Task } from "@/lib/api"
import { motion } from "framer-motion"

export interface TaskCardProps {
  task: Task
  isSelected: boolean
  onClick: () => void
  onDelete: (e: React.MouseEvent) => void
  onStatusChange: (status: Task["status"]) => void
  onRestart?: () => void
  onViewLogs?: () => void
  onViewSessions?: () => void
  onViewInfo?: () => void
  onViewAnalytics?: () => void
  onViewDiff?: () => void
  onManageMemory?: () => void
  onOrganizeMemory?: () => void
  onManageProjectPrompt?: () => void
  onOpenInEditor?: () => void
  onRename?: () => void
  hasResidentProcess?: boolean
  sessionTitle?: string
  /** 已绑定 iLink 微信账号时的展示文案（如微信号） */
  wechatBindingLabel?: string
  showProjectName?: boolean
  /** 显示左侧拖动手柄（配合 framer-motion Reorder + dragControls） */
  showReorderHandle?: boolean
  onReorderPointerDown?: (e: React.PointerEvent) => void
}

const statusConfig = {
  queue: { icon: CircleDashed, color: "text-muted-foreground", bg: "bg-muted", label: "排队中" },
  running: { icon: Flame, color: "text-orange-600", bg: "bg-orange-50", label: "执行中" },
  done: { icon: CircleCheck, color: "text-emerald-600", bg: "bg-emerald-50", label: "已完成" },
  cancelled: { icon: XCircle, color: "text-amber-700", bg: "bg-amber-50", label: "已取消" },
  failed: { icon: XCircle, color: "text-red-600", bg: "bg-red-50", label: "失败" },
}

export function TaskCard({ task, isSelected, onClick, onDelete, onStatusChange, onRestart, onViewLogs, onViewSessions, onViewInfo, onViewAnalytics, onViewDiff, onManageMemory, onOrganizeMemory, onManageProjectPrompt, onOpenInEditor, onRename, hasResidentProcess, sessionTitle, wechatBindingLabel, showProjectName = true, showReorderHandle, onReorderPointerDown }: TaskCardProps) {
  const config = statusConfig[task.status] || statusConfig.queue
  const Icon = config.icon
  const title = sessionTitle || task.name || (task.content.length > 80 ? task.content.slice(0, 80) + "..." : task.content)

  // 与 Reorder.Item 并用时子层不要用 y 等 transform 入场，避免与父级拖拽 translate 叠加重叠错位
  const sortable = Boolean(showReorderHandle && onReorderPointerDown)
  return (
    <motion.div
      initial={sortable ? false : { opacity: 0, y: 10 }}
      animate={sortable ? { opacity: 1 } : { opacity: 1, y: 0 }}
      exit={{ opacity: 0, scale: 0.95 }}
      className={cn(
        "w-full min-w-0 max-w-full",
        sortable ? "mb-0" : "mb-2"
      )}
    >
      <div
        onClick={onClick}
        className={cn(
          "group relative flex w-full min-w-0 max-w-full cursor-pointer flex-col gap-1.5 overflow-hidden border p-2 transition-all",
          isSelected 
            ? "border-foreground bg-accent" 
            : "border-border bg-card hover:bg-accent/50",
          task.status === "running" && "border-l-2 border-l-orange-500"
        )}
      >
        {/* 项目名称 - 显眼展示 */}
        {showProjectName && task.projectName && (
          <div className="flex w-full min-w-0 max-w-full items-center gap-1.5 overflow-hidden border-b border-border/50 pb-1 pr-20">
            <FolderOpen className="h-3.5 w-3.5 text-blue-600" />
            <span className="w-20 shrink-0 truncate text-xs font-semibold text-blue-600 sm:w-24">
              {task.projectName}
            </span>
          </div>
        )}

        <div className="flex min-w-0 max-w-full items-start gap-2">
          {showReorderHandle && onReorderPointerDown && (
            <div
              role="button"
              tabIndex={0}
              onPointerDown={(e) => {
                e.stopPropagation()
                e.preventDefault()
                onReorderPointerDown(e)
              }}
              onClick={(e) => e.stopPropagation()}
              onKeyDown={(e) => {
                if (e.key === "Enter" || e.key === " ") e.preventDefault()
              }}
              className="mt-0.5 shrink-0 cursor-grab touch-none text-muted-foreground hover:text-foreground active:cursor-grabbing"
              title="拖动排序"
            >
              <GripVertical className="h-4 w-4" />
            </div>
          )}
          <div className="flex w-0 min-w-0 max-w-full flex-1 items-start gap-2.5 overflow-hidden">
            <div className={cn(
              "mt-0.5 flex h-5 w-5 shrink-0 items-center justify-center", 
              config.bg,
              task.status === "running" && "animate-pulse"
            )}>
              <Icon className={cn("h-3.5 w-3.5", config.color)} />
            </div>
            <div className="w-0 min-w-0 flex-1 space-y-1 overflow-hidden">
              <h4 className="truncate text-sm font-semibold leading-tight text-foreground">
                {title}
              </h4>
              {(hasResidentProcess || wechatBindingLabel) && (
                <div className="flex min-w-0 max-w-full flex-wrap gap-1 overflow-hidden text-[11px]">
                  {hasResidentProcess && (
                    <span
                      className="flex min-w-0 max-w-full items-center gap-1 overflow-hidden rounded bg-green-50 px-1.5 py-0.5 font-medium text-green-700"
                      title="常驻进程活跃"
                    >
                      <Terminal className="h-3 w-3 shrink-0" />
                      <span className="truncate">进程活跃</span>
                    </span>
                  )}
                  {wechatBindingLabel ? (
                    <span
                      className="flex min-w-0 max-w-full items-center gap-1 overflow-hidden rounded bg-sky-50 px-1.5 py-0.5 font-medium text-sky-700"
                      title={`已绑定 iLink 微信：${wechatBindingLabel}`}
                    >
                      <MessageCircle className="h-3 w-3 shrink-0" />
                      <span className="truncate">iLink · {wechatBindingLabel}</span>
                    </span>
                  ) : null}
                </div>
              )}
            </div>
          </div>
          
          <div className="absolute right-0.5 top-0.5 h-7 w-[112px]">
            {task.branch && (
              <div className="absolute inset-0 flex items-center justify-end pointer-events-none transition-opacity group-hover:opacity-0">
                <span className="flex max-w-[112px] items-center gap-1 overflow-hidden rounded bg-blue-50 px-1.5 py-0.5 text-[11px] text-blue-700">
                  <GitBranch className="h-3 w-3 shrink-0" />
                  <span className="truncate">{task.branch}</span>
                </span>
              </div>
            )}
            <DropdownMenu>
              <DropdownMenuTrigger asChild onClick={(e) => e.stopPropagation()}>
                <Button
                  variant="ghost"
                  size="icon"
                  className={cn(
                    "absolute right-0 top-0 h-7 w-7 transition-opacity",
                    task.branch
                      ? "pointer-events-none opacity-0 group-hover:pointer-events-auto group-hover:opacity-100"
                      : "opacity-0 group-hover:opacity-100"
                  )}
                >
                  <MoreVertical className="h-4 w-4" />
                </Button>
              </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              {onViewDiff && (
                <DropdownMenuItem onClick={onViewDiff}>
                  <FileCode className="mr-2 h-4 w-4" /> 查看 Diff
                </DropdownMenuItem>
              )}
              {onOpenInEditor && (
                <DropdownMenuItem onClick={onOpenInEditor}>
                  <FolderOpen className="mr-2 h-4 w-4" /> 打开编辑器
                </DropdownMenuItem>
              )}
              {onManageMemory && (
                <DropdownMenuItem onClick={onManageMemory}>
                  <Brain className="mr-2 h-4 w-4" /> 记忆管理
                </DropdownMenuItem>
              )}
              {onOrganizeMemory && (
                <DropdownMenuItem onClick={onOrganizeMemory}>
                  <Brain className="mr-2 h-4 w-4" /> 自动记忆整理
                </DropdownMenuItem>
              )}
              {onManageProjectPrompt && (
                <DropdownMenuItem onClick={onManageProjectPrompt}>
                  <MessageSquare className="mr-2 h-4 w-4" /> 修改项目提示词
                </DropdownMenuItem>
              )}
              {(onViewDiff || onOpenInEditor || onManageMemory || onOrganizeMemory || onManageProjectPrompt) && <DropdownMenuSeparator />}
              {onViewInfo && (
                <DropdownMenuItem onClick={onViewInfo}>
                  <Info className="mr-2 h-4 w-4" /> 查看详情
                </DropdownMenuItem>
              )}
              {onViewAnalytics && (
                <DropdownMenuItem onClick={onViewAnalytics}>
                  <BarChart3 className="mr-2 h-4 w-4" /> 任务统计
                </DropdownMenuItem>
              )}
              {onRename && (
                <DropdownMenuItem onClick={onRename}>
                  <Info className="mr-2 h-4 w-4" /> 重命名
                </DropdownMenuItem>
              )}
              {/* {canRestart && onRestart && (
                <DropdownMenuItem onClick={onRestart}>
                  <RotateCcw className="mr-2 h-4 w-4" /> 重新执行
                </DropdownMenuItem>
              )} */}
              {onViewLogs && (
                <DropdownMenuItem onClick={onViewLogs}>
                  <Terminal className="mr-2 h-4 w-4" /> 进程日志
                </DropdownMenuItem>
              )}
              {/* {onViewSessions && (
                <DropdownMenuItem onClick={onViewSessions}>
                  <GitBranch className="mr-2 h-4 w-4" /> 会话记录
                </DropdownMenuItem>
              )} */}
              {/* {(onViewInfo || canRestart || onViewLogs || onViewSessions) && <DropdownMenuSeparator />} */}
              {/* <DropdownMenuItem onClick={() => onStatusChange("queue")}>设为排队</DropdownMenuItem>
              <DropdownMenuItem onClick={() => onStatusChange("running")}>设为执行中</DropdownMenuItem>
              <DropdownMenuItem onClick={() => onStatusChange("done")}>设为完成</DropdownMenuItem>
              <DropdownMenuItem onClick={() => onStatusChange("failed")}>设为失败</DropdownMenuItem> */}
              <DropdownMenuSeparator />
              <DropdownMenuItem className="text-destructive focus:text-destructive" onClick={onDelete}>
                <Trash2 className="mr-2 h-4 w-4" /> 删除任务
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
          </div>
        </div>
      </div>
    </motion.div>
  )
}
