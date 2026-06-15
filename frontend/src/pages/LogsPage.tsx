import React, { useState, useEffect, useCallback } from "react"
import { 
  Terminal, 
  RefreshCw, 
  Trash2, 
  CheckCircle, 
  XCircle, 
  Clock, 
  ChevronDown, 
  ChevronRight,
  Filter,
  BarChart3,
  Copy
} from "lucide-react"
import { api, CommandLog, CommandLogStats } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { ScrollArea } from "@/components/ui/scroll-area"
import { Skeleton } from "@/components/ui/skeleton"
import { Combobox, type ComboboxOption } from "@/components/ui/combobox"
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription } from "@/components/ui/dialog"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { useConfirmDialog } from "@/components/ConfirmDialogProvider"
import { toast } from "sonner"
import { cn } from "@/lib/utils"

const sourceLabels: Record<string, string> = {
  task_runner: "任务执行",
  task_runner_followup: "追问执行",
  git_handler: "Git 操作",
  memory_compaction: "记忆压缩",
  llm_action_error: "模型动作异常",
  llm_api_call: "大模型 API 调用",
}

const statusConfig = {
  running: { icon: Clock, color: "text-blue-600", bg: "bg-blue-50", label: "运行中" },
  success: { icon: CheckCircle, color: "text-emerald-600", bg: "bg-emerald-50", label: "成功" },
  failed: { icon: XCircle, color: "text-red-600", bg: "bg-red-50", label: "失败" },
}

const SOURCE_FILTER_OPTIONS: ComboboxOption[] = [
  { value: "all", label: "所有来源", searchText: "all 所有来源" },
  ...Object.entries(sourceLabels).map(([value, label]) => ({
    value,
    label,
    searchText: `${value} ${label}`,
  })),
]

const STATUS_FILTER_OPTIONS: ComboboxOption[] = [
  { value: "all", label: "所有状态", searchText: "all 所有状态" },
  ...Object.entries(statusConfig).map(([value, config]) => ({
    value,
    label: config.label,
    searchText: `${value} ${config.label}`,
  })),
]

export function LogsPage() {
  const { confirm } = useConfirmDialog()
  const [logs, setLogs] = useState<CommandLog[]>([])
  const [stats, setStats] = useState<CommandLogStats | null>(null)
  const [isLoading, setIsLoading] = useState(true)
  const [total, setTotal] = useState(0)
  const [offset, setOffset] = useState(0)
  const [selectedLog, setSelectedLog] = useState<CommandLog | null>(null)
  
  // Filters
  const [sourceFilter, setSourceFilter] = useState<string>("all")
  const [statusFilter, setStatusFilter] = useState<string>("all")
  
  const limit = 50

  const loadLogs = useCallback(async () => {
    setIsLoading(true)
    try {
      const query: any = { limit, offset }
      if (sourceFilter !== "all") query.source = sourceFilter
      if (statusFilter !== "all") query.status = statusFilter
      
      const [logsResult, statsResult] = await Promise.all([
        api.getCommandLogs(query),
        api.getCommandLogStats()
      ])
      
      setLogs(logsResult.logs || [])
      setTotal(logsResult.total)
      setStats(statsResult)
    } catch (error) {
      console.error("Failed to load logs:", error)
      toast.error("加载日志失败")
    } finally {
      setIsLoading(false)
    }
  }, [offset, sourceFilter, statusFilter])

  useEffect(() => {
    loadLogs()
  }, [loadLogs])

  const handleClearLogs = async () => {
    const confirmed = await confirm({
      title: "清理旧日志",
      description: "确定要清理 7 天前的日志吗？",
      confirmLabel: "清理",
      tone: "destructive",
    })
    if (!confirmed) return
    try {
      const result = await api.clearCommandLogs(7)
      toast.success(`已删除 ${result.deleted} 条日志`)
      loadLogs()
    } catch (error) {
      toast.error("清理失败")
    }
  }

  const formatDuration = (ms: number) => {
    if (ms < 1000) return `${ms}ms`
    if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`
    return `${(ms / 60000).toFixed(1)}m`
  }

  const formatDate = (dateStr: string) => {
    const date = new Date(dateStr)
    return date.toLocaleString("zh-CN", {
      month: "2-digit",
      day: "2-digit",
      hour: "2-digit",
      minute: "2-digit",
      second: "2-digit"
    })
  }

  const parseArgs = (argsJson: string): string[] => {
    try {
      return JSON.parse(argsJson)
    } catch {
      return []
    }
  }

  const getDisplayFields = (log: CommandLog) => {
    if (log.fields && log.fields.length > 0) {
      return log.fields.filter((field) => field.value)
    }
    return [
      log.stdinData ? { key: "stdin", label: "输入", value: log.stdinData, tone: "default" } : null,
      log.stdout ? { key: "stdout", label: "输出", value: log.stdout, tone: "default" } : null,
      log.stderr ? { key: "stderr", label: "错误输出", value: log.stderr, tone: "error" } : null,
      log.error ? { key: "error", label: "错误信息", value: log.error, tone: "error" } : null,
    ].filter(Boolean) as Array<{ key: string; label: string; value: string; tone?: string }>
  }

  /** 原始请求/JSON 类字段：格式化展示（解析失败则原样） */
  const formatJsonForDisplay = (raw: string): string => {
    const trimmed = raw.trim()
    if (!trimmed) return raw
    try {
      return JSON.stringify(JSON.parse(trimmed), null, 2)
    } catch {
      return raw
    }
  }

  const copyToClipboard = async (text: string, label: string) => {
    try {
      await navigator.clipboard.writeText(text)
      toast.success(`${label}已复制`)
    } catch (error) {
      console.error(`Failed to copy ${label}:`, error)
      toast.error(`复制${label}失败`)
    }
  }

  /** 命令详情「命令」页：标签 + 复制（复制内容可与展示略有不同，便于粘贴使用） */
  const CommandDetailField = ({
    label,
    copyText,
    children,
  }: {
    label: string
    copyText: string
    children: React.ReactNode
  }) => (
    <div>
      <div className="flex items-center justify-between gap-2">
        <span className="text-sm font-medium text-muted-foreground">{label}</span>
        <Button
          type="button"
          variant="ghost"
          size="icon"
          className="h-8 w-8 shrink-0 text-muted-foreground hover:text-foreground"
          title={`复制${label}`}
          onClick={() => void copyToClipboard(copyText, label)}
        >
          <Copy className="h-4 w-4" />
          <span className="sr-only">复制{label}</span>
        </Button>
      </div>
      <div className="mt-1">{children}</div>
    </div>
  )

  return (
    <div className="flex-1 p-8 overflow-y-auto">
      <div className="max-w-6xl mx-auto space-y-6">
        <div className="flex items-center justify-end">
          <div className="flex items-center gap-2">
            <Button variant="outline" size="sm" onClick={loadLogs} disabled={isLoading}>
              <RefreshCw className={cn("h-4 w-4 mr-2", isLoading && "animate-spin")} />
              刷新
            </Button>
            <Button variant="outline" size="sm" onClick={handleClearLogs}>
              <Trash2 className="h-4 w-4 mr-2" />
              清理旧日志
            </Button>
          </div>
        </div>

        {/* Stats Cards */}
        {stats && (
          <div className="grid gap-4 md:grid-cols-4">
            <Card>
              <CardHeader className="pb-2">
                <CardDescription>总命令数</CardDescription>
                <CardTitle className="text-2xl">{stats.total}</CardTitle>
              </CardHeader>
            </Card>
            <Card>
              <CardHeader className="pb-2">
                <CardDescription className="flex items-center gap-1">
                  <CheckCircle className="h-3 w-3 text-emerald-600" />
                  成功
                </CardDescription>
                <CardTitle className="text-2xl text-emerald-600">
                  {stats.byStatus.success || 0}
                </CardTitle>
              </CardHeader>
            </Card>
            <Card>
              <CardHeader className="pb-2">
                <CardDescription className="flex items-center gap-1">
                  <XCircle className="h-3 w-3 text-red-600" />
                  失败
                </CardDescription>
                <CardTitle className="text-2xl text-red-600">
                  {stats.byStatus.failed || 0}
                </CardTitle>
              </CardHeader>
            </Card>
            <Card>
              <CardHeader className="pb-2">
                <CardDescription className="flex items-center gap-1">
                  <Clock className="h-3 w-3 text-blue-600" />
                  运行中
                </CardDescription>
                <CardTitle className="text-2xl text-blue-600">
                  {stats.byStatus.running || 0}
                </CardTitle>
              </CardHeader>
            </Card>
          </div>
        )}

        {/* Filters */}
        <div className="flex items-center gap-4">
          <div className="flex items-center gap-2">
            <Filter className="h-4 w-4 text-muted-foreground" />
            <span className="text-sm text-muted-foreground">筛选：</span>
          </div>
          <Combobox
            id="logs-source-filter"
            items={SOURCE_FILTER_OPTIONS}
            value={sourceFilter}
            onValueChange={setSourceFilter}
            placeholder="来源"
            searchPlaceholder="搜索来源"
            emptyText="未找到来源"
            className="w-[180px]"
          />
          <Combobox
            id="logs-status-filter"
            items={STATUS_FILTER_OPTIONS}
            value={statusFilter}
            onValueChange={setStatusFilter}
            placeholder="状态"
            searchPlaceholder="搜索状态"
            emptyText="未找到状态"
            className="w-[160px]"
          />
          <span className="text-sm text-muted-foreground ml-auto">
            共 {total} 条记录
          </span>
        </div>

        {/* Logs List */}
        <Card>
          <CardContent className="p-0">
            {isLoading ? (
              <div className="space-y-2 p-4">
                {[1, 2, 3, 4, 5].map(i => (
                  <Skeleton key={i} className="h-16" />
                ))}
              </div>
            ) : logs.length === 0 ? (
              <div className="p-8 text-center text-muted-foreground">
                暂无日志记录
              </div>
            ) : (
              <ScrollArea className="h-[600px]">
                <div className="divide-y">
                  {logs.map((log) => {
                    const config = statusConfig[log.status] || statusConfig.running
                    const StatusIcon = config.icon
                    const args = parseArgs(log.args)
                    
                    return (
                      <div
                        key={log.id}
                        className="p-4 hover:bg-muted/50 cursor-pointer transition-colors"
                        onClick={() => setSelectedLog(log)}
                      >
                        <div className="flex items-start justify-between gap-4">
                          <div className="flex items-start gap-3 min-w-0 flex-1">
                            <div className={cn("p-1.5 rounded", config.bg)}>
                              <StatusIcon className={cn("h-4 w-4", config.color)} />
                            </div>
                            <div className="space-y-1 min-w-0 flex-1">
                              <div className="flex items-center gap-2 flex-wrap">
                                <code className="text-sm font-mono font-medium">
                                  {log.command}
                                </code>
                                {args.length > 0 && (
                                  <code className="text-xs text-muted-foreground font-mono truncate max-w-[400px]">
                                    {args.join(" ")}
                                  </code>
                                )}
                              </div>
                              <div className="flex items-center gap-3 text-xs text-muted-foreground">
                                <Badge variant="outline" className="text-xs">
                                  {sourceLabels[log.source] || log.source}
                                </Badge>
                                {log.sourceName && (
                                  <span className="truncate max-w-[200px]">{log.sourceName}</span>
                                )}
                                <span>{formatDate(log.createdAt)}</span>
                                {log.duration > 0 && (
                                  <span>耗时: {formatDuration(log.duration)}</span>
                                )}
                                {log.exitCode !== null && log.exitCode !== undefined && (
                                  <span className={log.exitCode !== 0 ? "text-red-600" : ""}>
                                    退出码: {log.exitCode}
                                  </span>
                                )}
                              </div>
                            </div>
                          </div>
                          <ChevronRight className="h-4 w-4 text-muted-foreground shrink-0" />
                        </div>
                      </div>
                    )
                  })}
                </div>
              </ScrollArea>
            )}
          </CardContent>
        </Card>

        {/* Pagination */}
        {total > limit && (
          <div className="flex items-center justify-center gap-2">
            <Button
              variant="outline"
              size="sm"
              onClick={() => setOffset(Math.max(0, offset - limit))}
              disabled={offset === 0}
            >
              上一页
            </Button>
            <span className="text-sm text-muted-foreground">
              {offset + 1} - {Math.min(offset + limit, total)} / {total}
            </span>
            <Button
              variant="outline"
              size="sm"
              onClick={() => setOffset(offset + limit)}
              disabled={offset + limit >= total}
            >
              下一页
            </Button>
          </div>
        )}
      </div>

      {/* Log Detail Dialog */}
      <Dialog open={selectedLog !== null} onOpenChange={(open) => !open && setSelectedLog(null)}>
        <DialogContent className="max-w-4xl max-h-[80vh] overflow-hidden flex flex-col">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <Terminal className="h-5 w-5" />
              命令详情
            </DialogTitle>
            <DialogDescription>
              {selectedLog && formatDate(selectedLog.createdAt)}
            </DialogDescription>
          </DialogHeader>
          
          {selectedLog && (
            <Tabs defaultValue="command" className="flex-1 overflow-hidden flex flex-col">
              <TabsList>
                <TabsTrigger value="command">命令</TabsTrigger>
                {getDisplayFields(selectedLog).map((field) => (
                  <TabsTrigger key={field.key} value={field.key}>{field.label}</TabsTrigger>
                ))}
              </TabsList>
              
              <TabsContent value="command" className="flex-1 overflow-auto mt-4">
                <div className="space-y-4">
                  <CommandDetailField
                    label="来源"
                    copyText={[
                      sourceLabels[selectedLog.source] || selectedLog.source,
                      selectedLog.source,
                      selectedLog.sourceName,
                    ]
                      .filter((s): s is string => Boolean(s && String(s).trim()))
                      .join(" | ")}
                  >
                    <div>
                      <Badge>{sourceLabels[selectedLog.source] || selectedLog.source}</Badge>
                      {selectedLog.sourceName && (
                        <span className="ml-2 text-sm">{selectedLog.sourceName}</span>
                      )}
                    </div>
                  </CommandDetailField>
                  <CommandDetailField
                    label="命令"
                    copyText={
                      [selectedLog.command, ...parseArgs(selectedLog.args)]
                        .filter(Boolean)
                        .join(" ")
                        .trim()
                    }
                  >
                    <pre className="p-3 bg-muted rounded-md font-mono text-sm overflow-x-auto whitespace-pre-wrap break-all">
                      {selectedLog.command} {parseArgs(selectedLog.args).join(" ")}
                    </pre>
                  </CommandDetailField>
                  <CommandDetailField
                    label="工作目录"
                    copyText={(selectedLog.workDir || "").trim() || "-"}
                  >
                    <pre className="p-3 bg-muted rounded-md font-mono text-sm whitespace-pre-wrap break-all">
                      {selectedLog.workDir || "-"}
                    </pre>
                  </CommandDetailField>
                  <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
                    <CommandDetailField
                      label="状态"
                      copyText={statusConfig[selectedLog.status]?.label || selectedLog.status}
                    >
                      <div>
                        <Badge variant={selectedLog.status === "success" ? "default" : selectedLog.status === "failed" ? "destructive" : "secondary"}>
                          {statusConfig[selectedLog.status]?.label || selectedLog.status}
                        </Badge>
                      </div>
                    </CommandDetailField>
                    <CommandDetailField
                      label="退出码"
                      copyText={
                        selectedLog.exitCode !== null && selectedLog.exitCode !== undefined
                          ? String(selectedLog.exitCode)
                          : "-"
                      }
                    >
                      <div className="font-mono">{selectedLog.exitCode ?? "-"}</div>
                    </CommandDetailField>
                    <CommandDetailField
                      label="耗时"
                      copyText={
                        selectedLog.duration > 0
                          ? `${formatDuration(selectedLog.duration)} (${selectedLog.duration}ms)`
                          : "-"
                      }
                    >
                      <div>
                        {selectedLog.duration > 0 ? formatDuration(selectedLog.duration) : "-"}
                      </div>
                    </CommandDetailField>
                  </div>
                </div>
              </TabsContent>
              {getDisplayFields(selectedLog).map((field) => {
                const isRawRequestTab = field.key === "raw_request"
                const displayText =
                  isRawRequestTab && field.value ? formatJsonForDisplay(field.value) : field.value || "(无内容)"
                return (
                <TabsContent key={field.key} value={field.key} className="flex-1 overflow-auto mt-4">
                  <div className="mb-3 flex items-center justify-between gap-2">
                    <span className="text-sm font-medium text-muted-foreground">{field.label}</span>
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => copyToClipboard(field.value, field.label)}
                      disabled={!field.value}
                    >
                      <Copy className="mr-2 h-4 w-4" />
                      复制
                    </Button>
                  </div>
                  <ScrollArea className="h-[min(60vh,520px)]">
                    <pre className={cn(
                      "p-4 rounded-md font-mono text-sm whitespace-pre-wrap break-words",
                      field.tone === "error" ? "bg-red-50 text-red-700" : "bg-muted"
                    )}>
                      {displayText}
                    </pre>
                  </ScrollArea>
                </TabsContent>
                )
              })}
            </Tabs>
          )}
        </DialogContent>
      </Dialog>
    </div>
  )
}
