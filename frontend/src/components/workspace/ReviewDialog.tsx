import React, { useEffect, useMemo, useState } from 'react'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Combobox, type ComboboxOption } from '@/components/ui/combobox'
import { Label } from '@/components/ui/label'
import { ScrollArea } from '@/components/ui/scroll-area'
import { api, type BranchInfo } from '@/lib/api'
import { buildBranchOptions } from '@/lib/branches'
import { GitBranch, Camera, ArrowRight, Loader2 } from 'lucide-react'
import { toast } from 'sonner'

interface ReviewDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  onConfirm: (command: string) => void
  projectId?: number
  taskId?: number
}

type SnapshotOption = ComboboxOption & {
  timestamp: number
}

function formatSnapshotTime(timestamp: number) {
  const date = new Date(timestamp * 1000)
  if (Number.isNaN(date.getTime())) {
    return '未知时间'
  }

  return new Intl.DateTimeFormat('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(date)
}

function buildSnapshotOptions(data: Awaited<ReturnType<typeof api.getTaskGitTimeline>>): SnapshotOption[] {
  const seen = new Set<string>()

  return data.items
    .filter((item) => item.kind === 'snapshot')
    .sort((left, right) => right.timestamp - left.timestamp)
    .flatMap((item) => {
      const snapshotValue = item.snapshot.snapshot?.trim()
      if (!snapshotValue || seen.has(snapshotValue)) {
        return []
      }

      seen.add(snapshotValue)

      const description = item.snapshot.description?.trim() || ''
      const formattedTime = formatSnapshotTime(item.timestamp)
      const shortHash = snapshotValue.slice(0, 7)
      const label = description || `检查点 ${formattedTime}`
      const detail = description
        ? `${formattedTime} · ${shortHash}`
        : `快照哈希 ${shortHash}`

      return [{
        value: snapshotValue,
        label,
        description: detail,
        searchText: `${label} ${detail} ${snapshotValue}`,
        timestamp: item.timestamp,
      }]
    })
}

export const ReviewDialog: React.FC<ReviewDialogProps> = ({
  open,
  onOpenChange,
  onConfirm,
  projectId,
  taskId,
}) => {
  const [sourceType, setSourceType] = useState<'branch' | 'snapshot'>('branch')
  const [sourceValue, setSourceValue] = useState<string>('')
  const [targetType, setTargetType] = useState<'branch' | 'snapshot'>('branch')
  const [targetValue, setTargetValue] = useState<string>('')
  const [branches, setBranches] = useState<BranchInfo[]>([])
  const [loadingBranches, setLoadingBranches] = useState(false)
  const [snapshotOptions, setSnapshotOptions] = useState<SnapshotOption[]>([])
  const [loadingSnapshots, setLoadingSnapshots] = useState(false)
  const [defaultSourceBranch, setDefaultSourceBranch] = useState('')
  const [defaultTargetBranch, setDefaultTargetBranch] = useState('')

  const branchOptions = useMemo(
    () => buildBranchOptions(branches),
    [branches]
  )
  const typeOptions = useMemo<ComboboxOption[]>(
    () => [
      {
        value: 'branch',
        label: '分支',
        description: 'Git 分支',
        searchText: 'branch 分支 git',
      },
      {
        value: 'snapshot',
        label: '快照',
        description: taskId ? '任务检查点' : '当前无任务可用',
        searchText: 'snapshot 快照 检查点',
        disabled: !taskId,
      },
    ],
    [taskId]
  )

  useEffect(() => {
    if (!open) {
      return
    }

    setSourceType('branch')
    setTargetType('branch')

    if (!projectId) {
      setBranches([])
      setDefaultSourceBranch('')
      setDefaultTargetBranch('')
      setSourceValue('')
      setTargetValue('')
    }

    if (!taskId) {
      setSnapshotOptions([])
    }

    let cancelled = false

    const loadData = async () => {
      try {
        if (projectId) {
          setLoadingBranches(true)
        }
        if (taskId) {
          setLoadingSnapshots(true)
        }

        const [branchResult, snapshotResult] = await Promise.allSettled([
          projectId
            ? Promise.all([
                api.getBranches(projectId),
                api.getDefaultBranch(projectId),
                api.getCurrentBranch(projectId),
              ])
            : Promise.resolve(null),
          taskId
            ? api.getTaskGitTimeline(taskId)
            : Promise.resolve(null),
        ])

        if (cancelled) {
          return
        }

        if (branchResult.status === 'fulfilled' && branchResult.value) {
          const [branchList, defaultBranchResult, currentBranchResult] = branchResult.value
          setBranches(branchList)
          setDefaultSourceBranch(defaultBranchResult.branch || '')
          setDefaultTargetBranch(currentBranchResult.branch || '')
          setSourceValue(defaultBranchResult.branch || '')
          setTargetValue(currentBranchResult.branch || '')
        } else if (projectId) {
          const error = branchResult.status === 'rejected' ? branchResult.reason : null
          console.error('Failed to load review branches:', error)
          toast.error('加载审查分支失败', {
            description: (error as { message?: string } | undefined)?.message || '未知错误',
          })
          setBranches([])
          setDefaultSourceBranch('')
          setDefaultTargetBranch('')
          setSourceValue('')
          setTargetValue('')
        }

        if (snapshotResult.status === 'fulfilled' && snapshotResult.value) {
          setSnapshotOptions(buildSnapshotOptions(snapshotResult.value))
        } else if (taskId) {
          const error = snapshotResult.status === 'rejected' ? snapshotResult.reason : null
          console.error('Failed to load review snapshots:', error)
          toast.error('加载检查点失败', {
            description: (error as { message?: string } | undefined)?.message || '未知错误',
          })
          setSnapshotOptions([])
        }
      } finally {
        if (!cancelled) {
          setLoadingBranches(false)
          setLoadingSnapshots(false)
        }
      }
    }

    void loadData()

    return () => {
      cancelled = true
    }
  }, [open, projectId, taskId])

  useEffect(() => {
    if (sourceType === 'snapshot') {
      setSourceValue((current) => (snapshotOptions.some((option) => option.value === current) ? current : ''))
      return
    }

    if (!sourceValue && defaultSourceBranch && branchOptions.some((option) => option.value === defaultSourceBranch)) {
      setSourceValue(defaultSourceBranch)
      return
    }

    if (!branchOptions.some((option) => option.value === sourceValue)) {
      setSourceValue('')
    }
  }, [branchOptions, defaultSourceBranch, snapshotOptions, sourceType, sourceValue])

  useEffect(() => {
    if (targetType === 'snapshot') {
      setTargetValue((current) => (snapshotOptions.some((option) => option.value === current) ? current : ''))
      return
    }

    if (!targetValue && defaultTargetBranch && branchOptions.some((option) => option.value === defaultTargetBranch)) {
      setTargetValue(defaultTargetBranch)
      return
    }

    if (!branchOptions.some((option) => option.value === targetValue)) {
      setTargetValue('')
    }
  }, [branchOptions, defaultTargetBranch, snapshotOptions, targetType, targetValue])

  const sourceOptions = sourceType === 'branch' ? branchOptions : snapshotOptions
  const targetOptions = targetType === 'branch' ? branchOptions : snapshotOptions
  const sourceDisplayLabel = sourceOptions.find((option) => option.value === sourceValue)?.label || sourceValue
  const targetDisplayLabel = targetOptions.find((option) => option.value === targetValue)?.label || targetValue

  const handleConfirm = () => {
    if (!sourceValue || !targetValue) {
      return
    }

    if (sourceType !== targetType) {
      toast.error('当前仅支持同类型对比', {
        description: '请统一选择分支或统一选择检查点。',
      })
      return
    }

    const fromLabel = `${sourceDisplayLabel}`
    const toLabel = `${targetDisplayLabel}`
    const display = `review ${fromLabel}-${toLabel}`
    const command = `[${display}](review://default?fromType=${sourceType}&from=${encodeURIComponent(sourceValue)}&toType=${targetType}&to=${encodeURIComponent(targetValue)})`
    
    onConfirm(command)
    onOpenChange(false)
    
  }

  const getSourceOptions = () => {
    return sourceOptions
  }

  const getTargetOptions = () => {
    return targetOptions
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-4xl max-h-[80vh] overflow-hidden">
        <DialogHeader>
          <DialogTitle>代码审查对比</DialogTitle>
        </DialogHeader>

        <div className="space-y-6 max-h-[60vh] overflow-y-auto pr-1">
          {/* 源选择 */}
          <div className="space-y-4">
            <div className="flex items-center gap-4">
              <div className="flex-1 space-y-2">
                <Label>源类型</Label>
                <Combobox
                  id="review-source-type"
                  items={typeOptions}
                  value={sourceType}
                  onValueChange={(value) => setSourceType(value as 'branch' | 'snapshot')}
                  placeholder="选择源类型"
                  searchPlaceholder="搜索源类型"
                  emptyText="未找到源类型"
                  renderItem={(item) => (
                    <div className="flex items-center gap-2">
                      {item.value === 'branch' ? <GitBranch className="h-4 w-4" /> : <Camera className="h-4 w-4" />}
                      <span>{item.label}</span>
                    </div>
                  )}
                />
              </div>

              <div className="flex-[2] space-y-2">
                <Label>选择{sourceType === 'branch' ? '分支' : '快照'}</Label>
                {sourceType === 'branch' ? (
                  <Combobox
                    items={getSourceOptions()}
                    value={sourceValue}
                    onValueChange={setSourceValue}
                    placeholder="选择主分支"
                    searchPlaceholder={loadingBranches ? '加载分支中...' : '搜索本地或远程分支...'}
                    emptyText={loadingBranches ? '正在加载分支...' : '未找到匹配分支'}
                    disabled={loadingBranches || !projectId}
                    renderItem={(item) => (
                      <div className="flex min-w-0 flex-1 items-center justify-between gap-2">
                        <span className="min-w-0 flex-1 truncate font-mono">{item.label}</span>
                        <span className="flex shrink-0 items-center gap-1">
                          {'isRemote' in item && item.isRemote ? (
                            <span className="rounded border border-blue-200 bg-blue-50 px-1.5 py-0.5 text-[10px] text-blue-700">远程</span>
                          ) : (
                            <span className="rounded border border-muted bg-muted/60 px-1.5 py-0.5 text-[10px] text-muted-foreground">本地</span>
                          )}
                          {'isCurrent' in item && item.isCurrent ? (
                            <span className="rounded border border-emerald-200 bg-emerald-50 px-1.5 py-0.5 text-[10px] text-emerald-700">当前</span>
                          ) : null}
                        </span>
                      </div>
                    )}
                  />
                ) : (
                  <Combobox
                    items={getSourceOptions()}
                    value={sourceValue}
                    onValueChange={setSourceValue}
                    placeholder="选择快照"
                    searchPlaceholder={loadingSnapshots ? '加载检查点中...' : '搜索检查点...'}
                    emptyText={loadingSnapshots ? '正在加载检查点...' : taskId ? '当前任务暂无检查点' : '当前不在任务会话中'}
                    disabled={loadingSnapshots || !taskId}
                    renderItem={(item) => (
                      <div className="min-w-0">
                        <div className="truncate">{item.label}</div>
                        {item.description && (
                          <div className="truncate text-xs text-muted-foreground">{item.description}</div>
                        )}
                      </div>
                    )}
                  />
                )}
              </div>
            </div>
          </div>

          {/* VS 图标 */}
          <div className="flex justify-center">
            <div className="flex items-center gap-2 text-muted-foreground">
              <ArrowRight className="h-5 w-5" />
              <span className="text-sm font-medium">对比</span>
              <ArrowRight className="h-5 w-5" />
            </div>
          </div>

          {/* 目标选择 */}
          <div className="space-y-4">
            <div className="flex items-center gap-4">
              <div className="flex-1 space-y-2">
                <Label>目标类型</Label>
                <Combobox
                  id="review-target-type"
                  items={typeOptions}
                  value={targetType}
                  onValueChange={(value) => setTargetType(value as 'branch' | 'snapshot')}
                  placeholder="选择目标类型"
                  searchPlaceholder="搜索目标类型"
                  emptyText="未找到目标类型"
                  renderItem={(item) => (
                    <div className="flex items-center gap-2">
                      {item.value === 'branch' ? <GitBranch className="h-4 w-4" /> : <Camera className="h-4 w-4" />}
                      <span>{item.label}</span>
                    </div>
                  )}
                />
              </div>

              <div className="flex-[2] space-y-2">
                <Label>选择{targetType === 'branch' ? '分支' : '快照'}</Label>
                {targetType === 'branch' ? (
                  <Combobox
                    items={getTargetOptions()}
                    value={targetValue}
                    onValueChange={setTargetValue}
                    placeholder="选择当前分支"
                    searchPlaceholder={loadingBranches ? '加载分支中...' : '搜索本地或远程分支...'}
                    emptyText={loadingBranches ? '正在加载分支...' : '未找到匹配分支'}
                    disabled={loadingBranches || !projectId}
                    renderItem={(item) => (
                      <div className="flex min-w-0 flex-1 items-center justify-between gap-2">
                        <span className="min-w-0 flex-1 truncate font-mono">{item.label}</span>
                        <span className="flex shrink-0 items-center gap-1">
                          {'isRemote' in item && item.isRemote ? (
                            <span className="rounded border border-blue-200 bg-blue-50 px-1.5 py-0.5 text-[10px] text-blue-700">远程</span>
                          ) : (
                            <span className="rounded border border-muted bg-muted/60 px-1.5 py-0.5 text-[10px] text-muted-foreground">本地</span>
                          )}
                          {'isCurrent' in item && item.isCurrent ? (
                            <span className="rounded border border-emerald-200 bg-emerald-50 px-1.5 py-0.5 text-[10px] text-emerald-700">当前</span>
                          ) : null}
                        </span>
                      </div>
                    )}
                  />
                ) : (
                  <Combobox
                    items={getTargetOptions()}
                    value={targetValue}
                    onValueChange={setTargetValue}
                    placeholder="选择快照"
                    searchPlaceholder={loadingSnapshots ? '加载检查点中...' : '搜索检查点...'}
                    emptyText={loadingSnapshots ? '正在加载检查点...' : taskId ? '当前任务暂无检查点' : '当前不在任务会话中'}
                    disabled={loadingSnapshots || !taskId}
                    renderItem={(item) => (
                      <div className="min-w-0">
                        <div className="truncate">{item.label}</div>
                        {item.description && (
                          <div className="truncate text-xs text-muted-foreground">{item.description}</div>
                        )}
                      </div>
                    )}
                  />
                )}
              </div>
            </div>
          </div>

          {/* 预览差异 */}
          {sourceValue && targetValue && (
            <div className="border rounded-md">
              <div className="px-4 py-2 bg-muted/50 border-b">
                <h4 className="text-sm font-medium">差异预览</h4>
              </div>
              <ScrollArea className="h-[200px]">
                <div className="p-4 space-y-2">
                  <div className="text-sm text-muted-foreground">
                    <p>将对比以下内容：</p>
                    <div className="mt-2 p-3 bg-muted/30 rounded font-mono text-xs space-y-1">
                      <div className="flex items-center gap-2">
                        {sourceType === 'branch' ? (
                          <GitBranch className="h-3 w-3" />
                        ) : (
                          <Camera className="h-3 w-3" />
                        )}
                        <span className="text-primary">{sourceDisplayLabel}</span>
                      </div>
                      <div className="text-muted-foreground">vs</div>
                      <div className="flex items-center gap-2">
                        {targetType === 'branch' ? (
                          <GitBranch className="h-3 w-3" />
                        ) : (
                          <Camera className="h-3 w-3" />
                        )}
                        <span className="text-primary">{targetDisplayLabel}</span>
                      </div>
                    </div>
                  </div>
                  <div className="text-xs text-muted-foreground mt-4">
                    提示：点击"开始审查"后，将生成审查命令并开始对比分析。
                  </div>
                </div>
              </ScrollArea>
            </div>
          )}

          {loadingBranches && sourceType === 'branch' && targetType === 'branch' && (
            <div className="flex items-center gap-2 rounded-md border border-dashed border-border/70 px-3 py-2 text-xs text-muted-foreground">
              <Loader2 className="h-3.5 w-3.5 animate-spin" />
              正在加载默认主分支和当前分支...
            </div>
          )}
          {loadingSnapshots && (sourceType === 'snapshot' || targetType === 'snapshot') && (
            <div className="flex items-center gap-2 rounded-md border border-dashed border-border/70 px-3 py-2 text-xs text-muted-foreground">
              <Loader2 className="h-3.5 w-3.5 animate-spin" />
              正在加载当前任务的检查点...
            </div>
          )}
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            取消
          </Button>
          <Button 
            onClick={handleConfirm} 
            disabled={!sourceValue || !targetValue}
          >
            开始审查
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
