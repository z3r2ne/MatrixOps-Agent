import React, { useState, useMemo, useRef, useEffect } from "react"
import { Briefcase, Filter, List, LayoutGrid, ChevronDown, ChevronRight, Edit, Trash2, RotateCcw, Upload, Download, Plus, MoreHorizontal, CheckCheck, Settings2 } from "lucide-react"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Combobox, type ComboboxOption } from "@/components/ui/combobox"
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger } from "@/components/ui/dropdown-menu"
import { Worker } from "@/lib/api"
import { cn } from "@/lib/utils"

interface WorkersSectionProps {
  workers: Worker[]
  onCreateWorker: () => void
  onEditWorker: (worker: Worker) => void
  onDeleteWorker: (worker: Worker) => void
  onRestoreDefaults: () => void
  onBulkDelete: (ids: number[]) => Promise<void>
  onBulkExport: (ids: number[]) => Promise<void>
  onBulkConfigure: (ids: number[]) => void
  onImportWorkers: (file: File) => Promise<void>
}

// 职业配置
const OCCUPATIONS = {
  analyst: { label: "分析师", color: "bg-cyan-500/10 text-cyan-700 border-cyan-200 dark:bg-cyan-500/20 dark:text-cyan-400 dark:border-cyan-800" },
  coder: { label: "研发工程师", color: "bg-blue-500/10 text-blue-700 border-blue-200 dark:bg-blue-500/20 dark:text-blue-400 dark:border-blue-800" },
  reviewer: { label: "验收师", color: "bg-purple-500/10 text-purple-700 border-purple-200 dark:bg-purple-500/20 dark:text-purple-400 dark:border-purple-800" },
  orchestrator: { label: "指挥师", color: "bg-orange-500/10 text-orange-700 border-orange-200 dark:bg-orange-500/20 dark:text-orange-400 dark:border-orange-800" },
  planner: { label: "规划师", color: "bg-green-500/10 text-green-700 border-green-200 dark:bg-green-500/20 dark:text-green-400 dark:border-green-800" },
} as const

type ViewMode = "list" | "occupation"

export function WorkersSection({
  workers,
  onCreateWorker,
  onEditWorker,
  onDeleteWorker,
  onRestoreDefaults,
  onBulkDelete,
  onBulkExport,
  onBulkConfigure,
  onImportWorkers,
}: WorkersSectionProps) {
  const [viewMode, setViewMode] = useState<ViewMode>("list")
  const [selectedOccupation, setSelectedOccupation] = useState<string>("all")
  const [expandedOccupations, setExpandedOccupations] = useState<Set<string>>(new Set(Object.keys(OCCUPATIONS)))
  const [selectedIds, setSelectedIds] = useState<Set<number>>(new Set<number>())
  const fileInputRef = useRef<HTMLInputElement>(null)
  const selectedCount = selectedIds.size

  // 获取职业配置
  const getOccupationConfig = (occupation: string) => {
    return OCCUPATIONS[occupation as keyof typeof OCCUPATIONS] || {
      label: occupation || "未分类",
      color: "bg-gray-500/10 text-gray-700 border-gray-200 dark:bg-gray-500/20 dark:text-gray-400 dark:border-gray-800"
    }
  }

  // 过滤后的 workers
  const filteredWorkers = useMemo(() => {
    if (selectedOccupation === "all") {
      return workers
    }
    return workers.filter(w => w.occupation === selectedOccupation)
  }, [workers, selectedOccupation])

  // 按职业分组的 workers
  const workersByOccupation = useMemo(() => {
    const groups: Record<string, Worker[]> = {}
    
    // 初始化所有职业组
    Object.keys(OCCUPATIONS).forEach(key => {
      groups[key] = []
    })
    
    // 添加未分类组
    groups["uncategorized"] = []
    
    // 分组
    workers.forEach(worker => {
      const occupation = worker.occupation || "uncategorized"
      if (!groups[occupation]) {
        groups[occupation] = []
      }
      groups[occupation].push(worker)
    })
    
    return groups
  }, [workers])

  // 获取所有职业及其数量
  const occupationStats = useMemo(() => {
    const stats: Array<{ key: string; label: string; count: number; color: string }> = []
    
    ;(Object.entries(workersByOccupation) as Array<[string, Worker[]]>).forEach(([key, occupationWorkers]) => {
      if (occupationWorkers.length > 0) {
        const config = key === "uncategorized" 
          ? { label: "未分类", color: "bg-gray-500/10 text-gray-700 border-gray-200 dark:bg-gray-500/20 dark:text-gray-400 dark:border-gray-800" }
          : getOccupationConfig(key)
        
        stats.push({
          key,
          label: config.label,
          count: occupationWorkers.length,
          color: config.color
        })
      }
    })
    
    return stats
  }, [workersByOccupation])

  const occupationFilterOptions = useMemo<ComboboxOption[]>(() => {
    const allOption: ComboboxOption = {
      value: "all",
      label: "全部职业",
      description: `${workers.length} 个 Worker`,
      searchText: `all 全部职业 ${workers.length}`,
    }
    const statOptions = occupationStats.map((stat) => ({
      value: stat.key,
      label: stat.label,
      description: `${stat.count} 个 Worker`,
      searchText: `${stat.key} ${stat.label} ${stat.count}`,
    }))
    return [allOption, ...statOptions]
  }, [occupationStats, workers.length])

  const toggleOccupationExpanded = (occupation: string) => {
    const newExpanded = new Set(expandedOccupations)
    if (newExpanded.has(occupation)) {
      newExpanded.delete(occupation)
    } else {
      newExpanded.add(occupation)
    }
    setExpandedOccupations(newExpanded)
  }

  const visibleWorkers = useMemo(() => {
    return viewMode === "list" ? filteredWorkers : workers
  }, [viewMode, filteredWorkers, workers])

  const isAllSelected = useMemo(() => {
    return visibleWorkers.length > 0 && visibleWorkers.every(worker => selectedIds.has(worker.id))
  }, [visibleWorkers, selectedIds])

  useEffect(() => {
    if (selectedIds.size === 0) return
    const ids = new Set(workers.map(worker => worker.id))
    setSelectedIds(prev => {
      const next = new Set<number>()
      prev.forEach(id => {
        if (ids.has(id)) {
          next.add(id)
        }
      })
      return next
    })
  }, [workers])

  const toggleSelectAll = () => {
    if (isAllSelected) {
      setSelectedIds(new Set<number>())
      return
    }
    setSelectedIds(new Set<number>(visibleWorkers.map(worker => worker.id)))
  }

  const toggleWorkerSelected = (workerId: number) => {
    setSelectedIds(prev => {
      const next = new Set<number>(prev)
      if (next.has(workerId)) {
        next.delete(workerId)
      } else {
        next.add(workerId)
      }
      return next
    })
  }

  const handleBulkDelete = async () => {
    const ids = Array.from(selectedIds.values(), (id) => id as number)
    if (ids.length === 0) return
    await onBulkDelete(ids)
    setSelectedIds(new Set<number>())
  }

  const handleBulkExport = async () => {
    const ids = Array.from(selectedIds.values(), (id) => id as number)
    if (ids.length === 0) return
    await onBulkExport(ids)
  }

  const handleImportClick = () => {
    fileInputRef.current?.click()
  }

  const handleImportChange = async (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0]
    if (!file) return
    await onImportWorkers(file)
    event.target.value = ""
  }

  // Worker 卡片组件
  const WorkerCard = ({ worker }: { worker: Worker }) => {
    const occupationConfig = getOccupationConfig(worker.occupation)
    const isSelected = selectedIds.has(worker.id)
    
    return (
      <div className={cn(
        "group flex h-full flex-col border border-border/60 bg-card p-3 transition-all hover:border-border hover:shadow-sm",
        isSelected && "border-primary/60 bg-primary/[0.03] ring-1 ring-primary/20"
      )}
      role="button"
      tabIndex={0}
      aria-pressed={isSelected}
      onClick={() => toggleWorkerSelected(worker.id)}
      onKeyDown={(event) => {
        if (event.key === "Enter" || event.key === " ") {
          event.preventDefault()
          toggleWorkerSelected(worker.id)
        }
      }}>
        <div className="flex min-w-0 flex-1 flex-col">
          <div className="flex min-w-0 items-start justify-between gap-2">
            <div className="min-w-0 flex-1">
              <h3 className="min-h-[2.75rem] text-[15px] font-semibold leading-6 text-foreground">
                <span className="line-clamp-2 break-all">{worker.name}</span>
              </h3>
              <div className="mt-1 flex flex-wrap items-center gap-2">
                <Badge className={cn("border text-xs", occupationConfig.color)}>
                  {occupationConfig.label}
                </Badge>
                {worker.name === "compaction" ? (
                  <Badge variant="secondary" className="text-xs">记忆压缩</Badge>
                ) : null}
              </div>
            </div>
            <div className="flex shrink-0 gap-1 opacity-100 transition-opacity sm:opacity-0 sm:group-hover:opacity-100 sm:group-focus-within:opacity-100">
              <Button
                size="sm"
                variant="ghost"
                className="h-7 w-7 p-0"
                onClick={(event) => {
                  event.stopPropagation()
                  onEditWorker(worker)
                }}
              >
                <Edit className="h-3.5 w-3.5" />
              </Button>
              <Button
                size="sm"
                variant="ghost"
                className="h-7 w-7 p-0 text-destructive hover:bg-destructive/10 hover:text-destructive"
                onClick={(event) => {
                  event.stopPropagation()
                  onDeleteWorker(worker)
                }}
              >
                <Trash2 className="h-3.5 w-3.5" />
              </Button>
            </div>
          </div>

          <p className="mt-2 min-h-[4rem] text-xs leading-5 text-muted-foreground">
              <span className="line-clamp-3">
                {worker.description?.trim() || "未填写 Worker 描述。"}
              </span>
          </p>

          <div className="mt-auto space-y-1.5 border border-border/50 bg-muted/20 p-2.5">
            <div className="grid grid-cols-[72px_minmax(0,1fr)] items-start gap-2 text-xs">
              <span className="text-muted-foreground">Provider</span>
              <span className="truncate font-mono text-foreground">{worker.provider || "未设置"}</span>
            </div>
            <div className="grid grid-cols-[72px_minmax(0,1fr)] items-start gap-2 text-xs">
              <span className="text-muted-foreground">Model</span>
              <span className="line-clamp-2 break-all text-foreground">{worker.model || "未设置"}</span>
            </div>
            <div className="grid grid-cols-[72px_minmax(0,1fr)] items-start gap-2 text-xs">
              <span className="text-muted-foreground">Config</span>
              <span className="truncate font-mono text-foreground">{worker.modelSettingsName || "default_model_config"}</span>
            </div>
          </div>
        </div>
      </div>
    )
  }

  return (
    <div className="space-y-4">
      {/* 工具栏 */}
      <Card>
        <CardContent className="pt-6">
          <div className="flex flex-col gap-3 xl:flex-row xl:items-center xl:justify-between">
            <div className="flex min-w-0 items-center gap-3">
              <div className="border border-border/60 bg-background p-2">
                <Filter className="h-4 w-4 text-muted-foreground" />
              </div>
              <Combobox
                id="workers-occupation-filter"
                items={occupationFilterOptions}
                value={selectedOccupation}
                onValueChange={setSelectedOccupation}
                placeholder="筛选职业"
                searchPlaceholder="搜索职业"
                emptyText="未找到职业筛选项"
                className="w-full sm:max-w-[260px]"
                renderItem={(item) => {
                  const stat = occupationStats.find((entry) => entry.key === item.value)
                  return (
                    <div className="flex items-center gap-2">
                      {stat ? <div className={cn("h-2 w-2", stat.color.split(' ')[0])} /> : null}
                      <span>{item.label}</span>
                      {item.description ? <Badge variant="outline" className="ml-1">{item.description.replace(" 个 Worker", "")}</Badge> : null}
                    </div>
                  )
                }}
              />
            </div>

            <div className="flex flex-wrap items-center gap-2 xl:justify-end">
              <div className="flex flex-wrap items-center gap-1 border border-border/60 bg-muted/20 p-1">
                <Button
                  size="sm"
                  variant={viewMode === "list" ? "secondary" : "ghost"}
                  className="h-8 px-3"
                  onClick={() => setViewMode("list")}
                >
                  <List className="h-3.5 w-3.5 mr-1" />
                  Worker 列表
                </Button>
                <Button
                  size="sm"
                  variant={viewMode === "occupation" ? "secondary" : "ghost"}
                  className="h-8 px-3"
                  onClick={() => setViewMode("occupation")}
                >
                  <LayoutGrid className="h-3.5 w-3.5 mr-1" />
                  职业列表
                </Button>
              </div>

              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button size="sm" variant="outline" className="h-8 px-3">
                    <MoreHorizontal className="h-3.5 w-3.5 mr-1.5" />
                    操作
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end" className="w-48">
                  <DropdownMenuItem onClick={onCreateWorker}>
                    <Plus className="mr-2 h-4 w-4" />
                    新建 Worker
                  </DropdownMenuItem>
                  <DropdownMenuSeparator />
                  <DropdownMenuItem onClick={toggleSelectAll}>
                    <CheckCheck className="mr-2 h-4 w-4" />
                    {isAllSelected ? "取消全选" : "全选当前列表"}
                  </DropdownMenuItem>
                  <DropdownMenuItem
                    disabled={selectedCount === 0}
                    onClick={() => setSelectedIds(new Set<number>())}
                  >
                    <Trash2 className="mr-2 h-4 w-4" />
                    清空选择
                  </DropdownMenuItem>
                  <DropdownMenuItem onClick={() => onBulkConfigure(Array.from(selectedIds))}>
                    <Settings2 className="mr-2 h-4 w-4" />
                    批量配置
                  </DropdownMenuItem>
                  <DropdownMenuSeparator />
                  <DropdownMenuItem onClick={handleImportClick}>
                    <Upload className="mr-2 h-4 w-4" />
                    导入 Worker
                  </DropdownMenuItem>
                  <DropdownMenuItem
                    disabled={selectedCount === 0}
                    onClick={handleBulkExport}
                  >
                    <Download className="mr-2 h-4 w-4" />
                    导出已选
                  </DropdownMenuItem>
                  <DropdownMenuSeparator />
                  <DropdownMenuItem onClick={onRestoreDefaults}>
                    <RotateCcw className="mr-2 h-4 w-4" />
                    恢复默认
                  </DropdownMenuItem>
                  <DropdownMenuItem
                    className="text-destructive focus:text-destructive"
                    disabled={selectedCount === 0}
                    onClick={handleBulkDelete}
                  >
                    <Trash2 className="mr-2 h-4 w-4" />
                    删除已选 ({selectedCount})
                  </DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
            </div>
          </div>
          <input
            ref={fileInputRef}
            type="file"
            accept=".zip"
            className="hidden"
            onChange={handleImportChange}
          />
        </CardContent>
      </Card>

      {/* Worker 列表视图 */}
      {viewMode === "list" && (
        <Card>
          <CardContent className="pt-6">
            {filteredWorkers.length === 0 ? (
              <div className="border border-dashed border-border/70 p-8 text-center">
                <Briefcase className="h-12 w-12 mx-auto mb-3 text-muted-foreground/50" />
                <p className="text-sm text-muted-foreground">
                  {selectedOccupation === "all" ? "暂无 Worker" : "该职业暂无 Worker"}
                </p>
              </div>
            ) : (
              <div className="grid auto-rows-fr grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-3">
                {filteredWorkers.map(worker => (
                  <WorkerCard key={worker.id} worker={worker} />
                ))}
              </div>
            )}
          </CardContent>
        </Card>
      )}

      {/* 职业列表视图 */}
      {viewMode === "occupation" && (
        <div className="space-y-3">
          {/* 恢复按钮卡片 */}
          <Card>
            <CardHeader className="py-4">
              <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                <div className="text-base font-semibold text-foreground">职业分组</div>
                <div className="flex flex-wrap items-center gap-2">
                  <Badge variant="outline" className="px-3 py-1 font-normal">{occupationStats.length} 个职业分组</Badge>
                </div>
              </div>
            </CardHeader>
          </Card>

          {occupationStats.length === 0 ? (
            <Card>
              <CardContent className="pt-6">
                <div className="border border-dashed border-border/70 p-8 text-center">
                  <Briefcase className="h-12 w-12 mx-auto mb-3 text-muted-foreground/50" />
                  <p className="text-sm text-muted-foreground">暂无 Worker</p>
                </div>
              </CardContent>
            </Card>
          ) : (
            occupationStats.map(stat => {
              const isExpanded = expandedOccupations.has(stat.key)
              const occupationWorkers = workersByOccupation[stat.key] || []
              
              return (
                <Card key={stat.key} className="overflow-hidden">
                  <CardHeader 
                    className="cursor-pointer hover:bg-accent/50 transition-colors"
                    onClick={() => toggleOccupationExpanded(stat.key)}
                  >
                    <div className="flex items-center justify-between">
                      <div className="flex items-center gap-3">
                        {isExpanded ? (
                          <ChevronDown className="h-4 w-4 text-muted-foreground" />
                        ) : (
                          <ChevronRight className="h-4 w-4 text-muted-foreground" />
                        )}
                        <Briefcase className="h-4 w-4 text-primary" />
                        <CardTitle className="text-base">{stat.label}</CardTitle>
                        <Badge className={cn("text-xs border", stat.color)}>
                          {stat.count} 个
                        </Badge>
                      </div>
                    </div>
                  </CardHeader>
                  
                  {isExpanded && (
                    <CardContent className="pt-0">
                      <div className="grid auto-rows-fr grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-3">
                        {occupationWorkers.map(worker => (
                          <WorkerCard key={worker.id} worker={worker} />
                        ))}
                      </div>
                    </CardContent>
                  )}
                </Card>
              )
            })
          )}
        </div>
      )}
    </div>
  )
}
