import React, { useState, useEffect } from "react"
import { X, Copy, Check, Clock, Terminal, AlertCircle, CheckCircle, Loader2, ChevronRight, Folder } from "lucide-react"
import { Button } from "@/components/ui/button"
import { ScrollArea } from "@/components/ui/scroll-area"
import { cn } from "@/lib/utils"
import { api, TaskExecution } from "@/lib/api"
import { toast } from "sonner"
import { getElectronWindowChromeCSSVars } from "@/lib/electron"

interface ExecutionLogsDialogProps {
  taskId: number
  isOpen: boolean
  onClose: () => void
}

export function ExecutionLogsDialog({ taskId, isOpen, onClose }: ExecutionLogsDialogProps) {
  const [executions, setExecutions] = useState<TaskExecution[]>([])
  const [selectedExec, setSelectedExec] = useState<TaskExecution | null>(null)
  const [isLoading, setIsLoading] = useState(false)
  const [copied, setCopied] = useState(false)

  useEffect(() => {
    if (isOpen && taskId) {
      loadExecutions()
    }
  }, [isOpen, taskId])

  const loadExecutions = async () => {
    setIsLoading(true)
    try {
      const data = await api.getTaskExecutions(taskId)
      setExecutions(data)
      setSelectedExec(null)
    } catch (error) {
      toast.error("加载执行记录失败")
    } finally {
      setIsLoading(false)
    }
  }

  const handleCopy = async () => {
    if (!selectedExec) return
    
    const text = `命令: ${selectedExec.command}\n工作目录: ${selectedExec.workDir}\n状态: ${selectedExec.status}\n开始时间: ${new Date(selectedExec.startedAt).toLocaleString()}\n${selectedExec.finishedAt ? `结束时间: ${new Date(selectedExec.finishedAt).toLocaleString()}\n` : ''}耗时: ${formatDuration(selectedExec.duration)}\n\n--- 输出 ---\n${selectedExec.output || '(无输出)'}\n${selectedExec.errorMsg ? `\n--- 错误 ---\n${selectedExec.errorMsg}` : ''}`
    
    await navigator.clipboard.writeText(text)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
    toast.success("已复制到剪贴板")
  }

  const formatDuration = (ms: number) => {
    if (ms < 1000) return `${ms}ms`
    if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`
    return `${Math.floor(ms / 60000)}m ${Math.floor((ms % 60000) / 1000)}s`
  }

  const getStatusIcon = (status: string) => {
    switch (status) {
      case "running":
        return <Loader2 className="h-4 w-4 animate-spin text-blue-500" />
      case "success":
        return <CheckCircle className="h-4 w-4 text-emerald-500" />
      case "failed":
        return <AlertCircle className="h-4 w-4 text-red-500" />
      default:
        return <Clock className="h-4 w-4 text-muted-foreground" />
    }
  }

  const getStatusLabel = (status: string) => {
    switch (status) {
      case "running": return "执行中"
      case "success": return "成功"
      case "failed": return "失败"
      default: return status
    }
  }

  if (!isOpen) return null

  return (
    <div
      className="fixed inset-x-0 bottom-0 top-[var(--electron-window-chrome-top,0px)] z-50 flex items-center justify-center bg-black/50 p-4"
      style={getElectronWindowChromeCSSVars()}
    >
      <div className="bg-background border w-full max-w-4xl h-[80vh] flex flex-col">
        {/* Header */}
        <div className="flex items-center justify-between border-b px-4 py-3">
          <div className="flex items-center gap-2">
            <Terminal className="h-5 w-5 text-muted-foreground" />
            <h2 className="font-semibold">进程日志</h2>
            <span className="text-xs text-muted-foreground">任务 #{taskId}</span>
          </div>
          <Button variant="ghost" size="icon" onClick={onClose}>
            <X className="h-4 w-4" />
          </Button>
        </div>

        {/* Content */}
        <div className="flex-1 flex overflow-hidden">
          {/* Execution List */}
          <div className="w-72 border-r flex flex-col">
            <div className="px-3 py-2 border-b text-xs font-medium text-muted-foreground">
              执行记录 ({executions.length})
            </div>
            <ScrollArea className="flex-1">
              {isLoading ? (
                <div className="flex items-center justify-center py-10">
                  <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
                </div>
              ) : executions.length === 0 ? (
                <div className="text-center text-xs text-muted-foreground py-10">
                  暂无执行记录
                </div>
              ) : (
                <div className="p-2 space-y-1">
                  {executions.map((exec) => (
                    <button
                      key={exec.id}
                      onClick={() => setSelectedExec(exec)}
                      className={cn(
                        "w-full text-left p-2 transition-colors",
                        selectedExec?.id === exec.id
                          ? "bg-accent"
                          : "hover:bg-muted/50"
                      )}
                    >
                      <div className="flex items-center gap-2">
                        {getStatusIcon(exec.status)}
                        <div className="flex-1 min-w-0">
                          <div className="text-xs font-medium truncate">
                            {new Date(exec.startedAt).toLocaleString()}
                          </div>
                          <div className="text-[10px] text-muted-foreground">
                            {getStatusLabel(exec.status)} · {formatDuration(exec.duration)}
                          </div>
                        </div>
                        <ChevronRight className="h-4 w-4 text-muted-foreground shrink-0" />
                      </div>
                    </button>
                  ))}
                </div>
              )}
            </ScrollArea>
          </div>

          {/* Execution Detail */}
          <div className="flex-1 flex flex-col min-w-0">
            {selectedExec ? (
              <>
                {/* Detail Header */}
                <div className="border-b px-4 py-2 flex items-center justify-between">
                  <div className="flex items-center gap-3 text-xs">
                    <div className="flex items-center gap-1">
                      {getStatusIcon(selectedExec.status)}
                      <span>{getStatusLabel(selectedExec.status)}</span>
                    </div>
                    <span className="text-muted-foreground">·</span>
                    <span className="text-muted-foreground">{formatDuration(selectedExec.duration)}</span>
                    <span className="text-muted-foreground">·</span>
                    <span className="text-muted-foreground">{selectedExec.workerName || "未知 Worker"}</span>
                  </div>
                  <Button variant="outline" size="sm" className="h-7 text-xs" onClick={handleCopy}>
                    {copied ? (
                      <>
                        <Check className="h-3 w-3 mr-1" /> 已复制
                      </>
                    ) : (
                      <>
                        <Copy className="h-3 w-3 mr-1" /> 复制全部
                      </>
                    )}
                  </Button>
                </div>

                {/* Meta Info */}
                <div className="border-b px-4 py-2 space-y-1 text-xs bg-muted/30">
                  <div className="flex items-center gap-2">
                    <Terminal className="h-3 w-3 text-muted-foreground shrink-0" />
                    <code className="flex-1 truncate text-muted-foreground">{selectedExec.command}</code>
                  </div>
                  <div className="flex items-center gap-2">
                    <Folder className="h-3 w-3 text-muted-foreground shrink-0" />
                    <code className="flex-1 truncate text-muted-foreground">{selectedExec.workDir}</code>
                  </div>
                </div>

                {/* Output */}
                <ScrollArea className="flex-1">
                  <div className="p-4">
                    {selectedExec.output ? (
                      <pre className="text-xs font-mono whitespace-pre-wrap break-all text-foreground">
                        {selectedExec.output}
                      </pre>
                    ) : (
                      <div className="text-xs text-muted-foreground text-center py-10">
                        (无输出)
                      </div>
                    )}
                    {selectedExec.errorMsg && (
                      <div className="mt-4 p-3 bg-red-50 border border-red-200 text-red-900">
                        <div className="text-xs font-medium mb-1">错误信息</div>
                        <pre className="text-xs font-mono whitespace-pre-wrap break-all">
                          {selectedExec.errorMsg}
                        </pre>
                      </div>
                    )}
                  </div>
                </ScrollArea>
              </>
            ) : (
              <div className="flex-1 flex items-center justify-center text-muted-foreground text-sm">
                选择左侧的执行记录查看详情
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}
