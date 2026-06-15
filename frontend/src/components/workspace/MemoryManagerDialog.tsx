import React, { useCallback, useEffect, useMemo, useState } from "react"
import {
  api,
  SessionMemoryEntry,
  SessionMemoryEntryInput,
  SessionMemoryResponse,
} from "@/lib/api"
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from "@/components/ui/alert-dialog"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { DropdownMenu, DropdownMenuContent, DropdownMenuGroup, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger } from "@/components/ui/dropdown-menu"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { ScrollArea } from "@/components/ui/scroll-area"
import { Skeleton } from "@/components/ui/skeleton"
import { Slider } from "@/components/ui/slider"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { Textarea } from "@/components/ui/textarea"
import { cn } from "@/lib/utils"
import { toast } from "sonner"
import { Archive, Clock3, Copy, Eye, History, MoreHorizontal, Pencil, Plus, Trash2 } from "lucide-react"

interface MemoryManagerDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  sessionId?: string
  taskTitle?: string
  isRunning?: boolean
}

type MemoryEditorState =
  | { mode: "create"; value: SessionMemoryEntryInput }
  | { mode: "edit"; id: number; value: SessionMemoryEntryInput }

type DetailState = { item: SessionMemoryEntry }

const DEFAULT_MEMORY_ENTRY_INPUT: SessionMemoryEntryInput = {
  entryKind: "manual",
  role: "system",
  content: "",
  rawOutput: "",
  callToolInfo: "",
  toolCallID: "",
  toolName: "",
  toolStatus: "",
  toolReason: "",
  toolRequestRawJSON: "",
  toolInputJSON: "",
  toolOutput: "",
  toolError: "",
  toolTitle: "",
  toolMetadataJSON: "",
  tokenCount: 0,
  synthetic: true,
  sequence: 0,
}

function formatTimestamp(timestamp?: number) {
  if (!timestamp) return "-"
  try {
    return new Date(timestamp).toLocaleString("zh-CN")
  } catch {
    return String(timestamp)
  }
}

function getMemoryEntryPreview(entry: SessionMemoryEntry) {
  return (
    entry.toolOutput?.trim() ||
    entry.toolError?.trim() ||
    entry.rawOutput?.trim() ||
    entry.content?.trim() ||
    entry.toolRequestRawJSON?.trim() ||
    entry.toolInputJSON?.trim() ||
    entry.callToolInfo?.trim() ||
    "这条历史记忆没有可展示的内容"
  )
}

function getCompressionLevelBadge(level?: number) {
  const normalized = typeof level === "number" && level >= 0 ? level : 0
  const label = `L${normalized}`
  const variant = normalized === 0 ? "outline" : normalized === 1 ? "secondary" : normalized === 2 ? "default" : "destructive"
  return { label, variant } as const
}

function getMemoryEntryRenderedContent(entry: SessionMemoryEntry) {
  return (
    entry.toolOutput?.trim() ||
    entry.toolError?.trim() ||
    entry.rawOutput?.trim() ||
    entry.content?.trim() ||
    entry.toolRequestRawJSON?.trim() ||
    entry.toolInputJSON?.trim() ||
    entry.callToolInfo?.trim() ||
    ""
  )
}

function getUtf8ByteSize(value: string) {
  if (!value) return 0
  return new TextEncoder().encode(value).length
}

function formatMemoryBytes(bytes: number) {
  if (bytes >= 1024 * 1024) {
    return `${(bytes / (1024 * 1024)).toFixed(bytes >= 10 * 1024 * 1024 ? 0 : 1)} MB`
  }
  if (bytes >= 1024) {
    return `${(bytes / 1024).toFixed(bytes >= 10 * 1024 ? 0 : 1)} KB`
  }
  return `${bytes} B`
}

function getMemoryEntrySize(entry: SessionMemoryEntry) {
  return getUtf8ByteSize(getMemoryEntryRenderedContent(entry))
}

function getMemorySizeToneClass(size: number, maxSize: number) {
  if (maxSize <= 0) return ""
  const ratio = size / maxSize
  if (ratio >= 0.85) return "border-amber-500/40 bg-amber-500/10 text-amber-700"
  if (ratio >= 0.55) return "border-sky-500/30 bg-sky-500/10 text-sky-700"
  return ""
}

function countAssistantResponseGroups(entries: SessionMemoryEntry[]) {
  let count = 0
  let inAssistantGroup = false
  for (const entry of entries) {
    const role = (entry.role || "").trim().toLowerCase()
    if (role === "assistant") {
      if (!inAssistantGroup) {
        count += 1
        inAssistantGroup = true
      }
      continue
    }
    inAssistantGroup = false
  }
  return count
}

function clampRatio(value: number) {
  if (Number.isNaN(value)) return 0
  return Math.min(1, Math.max(0, value))
}

function selectionCount(total: number, ratio: number) {
  return Math.max(0, Math.min(total, Math.round(total * ratio)))
}

function isMemoryChecked(selectedCount: number, totalCount: number) {
  if (totalCount === 0) return false
  return selectedCount === totalCount ? true : selectedCount > 0 ? "indeterminate" : false
}

function EmptyState({ title, description }: { title: string; description: string }) {
  return (
    <div className="flex h-full min-h-[240px] items-center justify-center rounded-lg border border-dashed bg-muted/15 p-10 text-center">
      <div className="max-w-md space-y-2">
        <p className="text-base font-medium">{title}</p>
        <p className="text-sm text-muted-foreground">{description}</p>
      </div>
    </div>
  )
}

export function MemoryManagerDialog({
  open,
  onOpenChange,
  sessionId,
  taskTitle,
  isRunning = false,
}: MemoryManagerDialogProps) {
  const [data, setData] = useState<SessionMemoryResponse | null>(null)
  const [loading, setLoading] = useState(false)
  const [selectedMemoryIds, setSelectedMemoryIds] = useState<Set<number>>(new Set())
  const [selectionRatio, setSelectionRatio] = useState(0)
  const [selectionRatioInput, setSelectionRatioInput] = useState("0.00")
  const [editorState, setEditorState] = useState<MemoryEditorState | null>(null)
  const [detailState, setDetailState] = useState<DetailState | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<number[] | null>(null)

  const loadSessionMemory = useCallback(async () => {
    if (!sessionId) return
    setLoading(true)
    try {
      const result = await api.getSessionMemory(sessionId)
      setData(result)
    } catch (error) {
      console.error("Failed to load session memory:", error)
      toast.error("加载历史记忆失败", {
        description: error instanceof Error ? error.message : "未知错误",
      })
    } finally {
      setLoading(false)
    }
  }, [sessionId])

  useEffect(() => {
    if (!open || !sessionId) {
      setData(null)
      setLoading(false)
      setSelectedMemoryIds(new Set())
      setEditorState(null)
      setDetailState(null)
      setDeleteTarget(null)
      return
    }

    setSelectedMemoryIds(new Set())
    void loadSessionMemory()
  }, [open, sessionId, loadSessionMemory])

  useEffect(() => {
    if (!open || !sessionId || !isRunning) return
    const intervalId = window.setInterval(() => {
      void loadSessionMemory()
    }, 1500)
    return () => {
      window.clearInterval(intervalId)
    }
  }, [open, sessionId, isRunning, loadSessionMemory])

  const memoryEntries = data?.memoryEntries ?? []

  const maxMemoryEntrySize = useMemo(
    () => memoryEntries.reduce((max, entry) => Math.max(max, getMemoryEntrySize(entry)), 0),
    [memoryEntries]
  )

  const summary = useMemo(() => ({
    memoryCount: memoryEntries.length,
    memoryBytes: memoryEntries.reduce((total, entry) => total + getUtf8ByteSize(getMemoryEntryRenderedContent(entry)), 0),
    assistantResponseGroupCount: countAssistantResponseGroups(memoryEntries),
  }), [memoryEntries])

  const applyRatioSelection = useCallback((ratio: number) => {
    const normalized = clampRatio(ratio)
    const count = selectionCount(memoryEntries.length, normalized)
    setSelectedMemoryIds(new Set(memoryEntries.slice(0, count).map(entry => entry.id)))
  }, [memoryEntries])

  const handleRatioSliderChange = (value: number[]) => {
    const normalized = clampRatio((value[0] ?? 0) / 100)
    setSelectionRatio(normalized)
    setSelectionRatioInput(normalized.toFixed(2))
    applyRatioSelection(normalized)
  }

  const handleRatioInputChange = (raw: string) => {
    setSelectionRatioInput(raw)
    const parsed = Number(raw)
    if (Number.isNaN(parsed)) return
    const normalized = clampRatio(parsed)
    setSelectionRatio(normalized)
    applyRatioSelection(normalized)
  }

  const handleToggleAll = (checked: boolean) => {
    setSelectedMemoryIds(checked ? new Set(memoryEntries.map(entry => entry.id)) : new Set())
  }

  const handleToggleMemoryRow = (entryId: number, checked: boolean) => {
    setSelectedMemoryIds(prev => {
      const next = new Set(prev)
      if (checked) {
        next.add(entryId)
      } else {
        next.delete(entryId)
      }
      return next
    })
  }

  const handleCopyMemoryEntry = async (entry: SessionMemoryEntry) => {
    await navigator.clipboard.writeText(getMemoryEntryPreview(entry))
    toast.success("历史记忆已复制")
  }

  const submitEditor = async () => {
    if (!sessionId || !editorState) return
    try {
      if (editorState.mode === "create") {
        await api.createSessionMemoryEntry(sessionId, editorState.value)
        toast.success("历史记忆已创建")
      } else {
        await api.updateSessionMemoryEntry(sessionId, editorState.id, editorState.value)
        toast.success("历史记忆已更新")
      }
      setEditorState(null)
      await loadSessionMemory()
    } catch (error) {
      console.error("Failed to submit memory editor:", error)
      toast.error("保存失败", {
        description: error instanceof Error ? error.message : "未知错误",
      })
    }
  }

  const confirmDelete = async () => {
    if (!sessionId || !deleteTarget) return
    try {
      await Promise.all(deleteTarget.map(id => api.deleteSessionMemoryEntry(sessionId, id)))
      setSelectedMemoryIds(prev => {
        const next = new Set(prev)
        deleteTarget.forEach(id => next.delete(id))
        return next
      })
      setDeleteTarget(null)
      toast.success("历史记忆已删除")
      await loadSessionMemory()
    } catch (error) {
      console.error("Failed to delete memory rows:", error)
      toast.error("删除失败", {
        description: error instanceof Error ? error.message : "未知错误",
      })
    }
  }

  const handleCompressSelected = async () => {
    if (!sessionId || selectedMemoryIds.size < 3) {
      toast.error("至少选择 3 条历史记忆才能压缩")
      return
    }

    try {
      await api.compressSessionMemoryEntries(sessionId, Array.from(selectedMemoryIds))
      setSelectedMemoryIds(new Set())
      toast.success("已完成历史记忆压缩")
      await loadSessionMemory()
    } catch (error) {
      console.error("Failed to compress selected memory entries:", error)
      toast.error("压缩失败", {
        description: error instanceof Error ? error.message : "未知错误",
      })
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="flex !left-0 !right-0 !bottom-0 !top-[var(--electron-window-chrome-top,0px)] h-[calc(100dvh-var(--electron-window-chrome-top,0px))] w-[100vw] max-w-[100vw] !translate-x-0 !translate-y-0 flex-col overflow-hidden rounded-none border-0 p-0">
        <div className="flex h-full min-h-0 flex-col">
          <div className="border-b px-6 py-4 pr-12">
            <div className="flex items-center gap-3">
              <h2 className="text-base font-semibold">记忆管理</h2>
              {taskTitle && (
                <span className="max-w-[360px] truncate text-sm text-muted-foreground" title={taskTitle}>
                  {taskTitle}
                </span>
              )}
              <Badge variant="secondary">历史记忆 {summary.memoryCount}</Badge>
              <Badge variant="outline">总大小 {formatMemoryBytes(summary.memoryBytes)}</Badge>
              <Badge variant="outline">AI 响应组 {summary.assistantResponseGroupCount}</Badge>
              {isRunning && <Badge variant="outline">运行中自动刷新</Badge>}
            </div>
          </div>

          <div className="flex min-h-0 flex-1 flex-col px-6 py-4">
            {!sessionId ? (
              <EmptyState
                title="当前任务还没有会话"
                description="等任务开始执行并生成会话后，这里才会出现可查看、可维护的历史记忆。"
              />
            ) : loading ? (
              <div className="flex h-full flex-col gap-2">
                <Skeleton className="h-9 w-52" />
                <Skeleton className="h-10 w-full" />
                <Skeleton className="h-10 w-full" />
                <Skeleton className="h-10 w-full" />
              </div>
            ) : (
              <>
                <div className="flex flex-wrap items-center justify-between gap-3">
                  <div className="inline-flex items-center gap-2 rounded-md border bg-muted/20 px-3 py-2 text-sm">
                    <History className="size-4" />
                    历史记忆
                  </div>

                  <div className="flex flex-wrap items-center gap-2">
                    <Button
                      type="button"
                      variant="outline"
                      onClick={() => setEditorState({ mode: "create", value: { ...DEFAULT_MEMORY_ENTRY_INPUT } })}
                    >
                      <Plus data-icon="inline-start" />
                      新增
                    </Button>
                    <Button
                      type="button"
                      variant="outline"
                      disabled={selectedMemoryIds.size === 0}
                      onClick={() => setDeleteTarget(Array.from(selectedMemoryIds))}
                    >
                      <Trash2 data-icon="inline-start" />
                      删除已选
                    </Button>
                    <Button
                      type="button"
                      variant="outline"
                      disabled={selectedMemoryIds.size < 3}
                      onClick={handleCompressSelected}
                    >
                      <Archive data-icon="inline-start" />
                      压缩已选
                    </Button>
                  </div>
                </div>

                <div className="mt-4 flex flex-wrap items-center gap-3 rounded-md border bg-muted/20 px-4 py-3">
                  <span className="text-sm font-medium">比例多选</span>
                  <div className="flex min-w-[220px] flex-1 items-center gap-3">
                    <Slider
                      value={[selectionRatio * 100]}
                      min={0}
                      max={100}
                      step={1}
                      onValueChange={handleRatioSliderChange}
                    />
                    <Input
                      value={selectionRatioInput}
                      onChange={(event) => handleRatioInputChange(event.target.value)}
                      className="w-24"
                      inputMode="decimal"
                    />
                  </div>
                  <Badge variant="outline">
                    已选 {selectedMemoryIds.size} / {memoryEntries.length}
                  </Badge>
                </div>

                <div className="mt-4 min-h-0 flex-1">
                  <ScrollArea className="h-full rounded-md border">
                    <Table>
                      <TableHeader>
                        <TableRow>
                          <TableHead className="w-10">
                            <Checkbox
                              checked={isMemoryChecked(selectedMemoryIds.size, memoryEntries.length)}
                              onCheckedChange={(checked) => handleToggleAll(checked === true)}
                              aria-label="选择全部历史记忆"
                            />
                          </TableHead>
                          <TableHead className="w-24">角色</TableHead>
                          <TableHead>内容预览</TableHead>
                          <TableHead className="w-24">大小(bytes)</TableHead>
                          <TableHead className="w-16">级别</TableHead>
                          <TableHead className="w-28">类型</TableHead>
                          <TableHead className="w-28">序号</TableHead>
                          <TableHead className="w-44">时间</TableHead>
                          <TableHead className="w-12 text-right">操作</TableHead>
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {memoryEntries.length === 0 ? (
                          <TableRow>
                            <TableCell colSpan={9} className="h-24 text-center text-muted-foreground">
                              暂无历史记忆
                            </TableCell>
                          </TableRow>
                        ) : (
                          memoryEntries.map((entry, index) => (
                            <TableRow key={entry.id} data-state={selectedMemoryIds.has(entry.id) ? "selected" : undefined}>
                              <TableCell>
                                <Checkbox
                                  checked={selectedMemoryIds.has(entry.id)}
                                  onCheckedChange={(checked) => handleToggleMemoryRow(entry.id, checked === true)}
                                  aria-label={`选择历史记忆 ${entry.id}`}
                                />
                              </TableCell>
                              <TableCell>
                                <Badge variant="outline" className="w-16 justify-center truncate">
                                  {entry.role || "unknown"}
                                </Badge>
                              </TableCell>
                              <TableCell className="max-w-0">
                                <span className="block truncate" title={getMemoryEntryPreview(entry)}>
                                  {getMemoryEntryPreview(entry)}
                                </span>
                              </TableCell>
                              <TableCell>
                                <Badge
                                  variant="outline"
                                  className={cn(
                                    "w-16 justify-center font-mono text-[11px]",
                                    getMemorySizeToneClass(getMemoryEntrySize(entry), maxMemoryEntrySize)
                                  )}
                                >
                                  {formatMemoryBytes(getMemoryEntrySize(entry))}
                                </Badge>
                              </TableCell>
                              <TableCell>
                                {(() => {
                                  const badge = getCompressionLevelBadge(entry.compressionLevel)
                                  return (
                                    <Badge variant={badge.variant} className="w-10 justify-center font-mono text-[11px]">
                                      {badge.label}
                                    </Badge>
                                  )
                                })()}
                              </TableCell>
                              <TableCell className="text-xs text-muted-foreground">{entry.entryKind}</TableCell>
                              <TableCell className="text-xs text-muted-foreground" title={`内部 sequence: ${entry.sequence}`}>
                                #{index + 1}
                              </TableCell>
                              <TableCell className="text-xs text-muted-foreground">{formatTimestamp(entry.created)}</TableCell>
                              <TableCell className="text-right">
                                <MemoryRowActions
                                  onCopy={() => handleCopyMemoryEntry(entry)}
                                  onDetail={() => setDetailState({ item: entry })}
                                  onEdit={() => setEditorState({
                                    mode: "edit",
                                    id: entry.id,
                                    value: {
                                      entryKind: entry.entryKind,
                                      role: entry.role,
                                      content: entry.content || "",
                                      rawOutput: entry.rawOutput || "",
                                      callToolInfo: entry.callToolInfo || "",
                                      toolCallID: entry.toolCallID || "",
                                      toolName: entry.toolName || "",
                                      toolStatus: entry.toolStatus || "",
                                      toolReason: entry.toolReason || "",
                                      toolRequestRawJSON: entry.toolRequestRawJSON || "",
                                      toolInputJSON: entry.toolInputJSON || "",
                                      toolOutput: entry.toolOutput || "",
                                      toolError: entry.toolError || "",
                                      toolTitle: entry.toolTitle || "",
                                      toolMetadataJSON: entry.toolMetadataJSON || "",
                                      tokenCount: entry.tokenCount,
                                      synthetic: entry.synthetic,
                                      sequence: entry.sequence,
                                    },
                                  })}
                                  onDelete={() => setDeleteTarget([entry.id])}
                                />
                              </TableCell>
                            </TableRow>
                          ))
                        )}
                      </TableBody>
                    </Table>
                  </ScrollArea>
                </div>
              </>
            )}
          </div>
        </div>

        <MemoryEntryEditorDialog
          state={editorState}
          onOpenChange={(nextOpen) => !nextOpen && setEditorState(null)}
          onSubmit={submitEditor}
          onChange={setEditorState}
        />

        <DetailDialog state={detailState} onOpenChange={(nextOpen) => !nextOpen && setDetailState(null)} />

        <AlertDialog open={deleteTarget !== null} onOpenChange={(nextOpen) => !nextOpen && setDeleteTarget(null)}>
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>确认删除？</AlertDialogTitle>
              <AlertDialogDescription>
                {deleteTarget ? `将删除 ${deleteTarget.length} 条历史记忆。` : "确认后将删除选中的条目。"}
              </AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel>取消</AlertDialogCancel>
              <AlertDialogAction onClick={confirmDelete}>删除</AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
      </DialogContent>
    </Dialog>
  )
}

function MemoryRowActions({
  onCopy,
  onDetail,
  onEdit,
  onDelete,
}: {
  onCopy: () => void
  onDetail: () => void
  onEdit: () => void
  onDelete: () => void
}) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button type="button" variant="ghost" size="icon" className="size-7">
          <MoreHorizontal />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuGroup>
          <DropdownMenuItem onClick={onCopy}>
            <Copy className="mr-2" />
            复制
          </DropdownMenuItem>
          <DropdownMenuItem onClick={onDetail}>
            <Eye className="mr-2" />
            详情
          </DropdownMenuItem>
          <DropdownMenuItem onClick={onEdit}>
            <Pencil className="mr-2" />
            编辑
          </DropdownMenuItem>
        </DropdownMenuGroup>
        <DropdownMenuSeparator />
        <DropdownMenuItem className="text-destructive focus:text-destructive" onClick={onDelete}>
          <Trash2 className="mr-2" />
          删除
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

function MemoryEntryEditorDialog({
  state,
  onOpenChange,
  onSubmit,
  onChange,
}: {
  state: MemoryEditorState | null
  onOpenChange: (open: boolean) => void
  onSubmit: () => void
  onChange: (next: MemoryEditorState | null) => void
}) {
  const open = state !== null
  if (!state) {
    return (
      <Dialog open={open} onOpenChange={onOpenChange}>
        <DialogContent className="max-w-2xl" srOnlyTitle="历史记忆" />
      </Dialog>
    )
  }

  const updateValue = (patch: Partial<SessionMemoryEntryInput>) => {
    onChange({ ...state, value: { ...state.value, ...patch } })
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl">
        <DialogHeader>
          <DialogTitle>{state.mode === "edit" ? "编辑历史记忆" : "新增历史记忆"}</DialogTitle>
          <DialogDescription>修改后会直接影响当前会话的历史记忆列表。</DialogDescription>
        </DialogHeader>

        <div className="grid gap-4">
          <div className="grid grid-cols-2 gap-4">
            <div className="flex flex-col gap-2">
              <Label htmlFor="memory-role">角色</Label>
              <Input id="memory-role" value={state.value.role} onChange={(event) => updateValue({ role: event.target.value })} />
            </div>
            <div className="flex flex-col gap-2">
              <Label htmlFor="memory-kind">类型</Label>
              <Input id="memory-kind" value={state.value.entryKind} onChange={(event) => updateValue({ entryKind: event.target.value })} />
            </div>
          </div>

          <EditorField label="内容" value={state.value.content ?? ""} onChange={(value) => updateValue({ content: value })} />
          <EditorField label="原始输出" value={state.value.rawOutput ?? ""} onChange={(value) => updateValue({ rawOutput: value })} />
          <EditorField label="工具信息" value={state.value.callToolInfo ?? ""} onChange={(value) => updateValue({ callToolInfo: value })} />
          <EditorField label="工具请求 JSON" value={state.value.toolRequestRawJSON ?? ""} onChange={(value) => updateValue({ toolRequestRawJSON: value })} />
          <EditorField label="工具输入 JSON" value={state.value.toolInputJSON ?? ""} onChange={(value) => updateValue({ toolInputJSON: value })} />
          <EditorField label="工具输出" value={state.value.toolOutput ?? ""} onChange={(value) => updateValue({ toolOutput: value })} />
          <EditorField label="工具错误" value={state.value.toolError ?? ""} onChange={(value) => updateValue({ toolError: value })} />

          <div className="grid grid-cols-2 gap-4">
            <div className="flex flex-col gap-2">
              <Label htmlFor="memory-token-count">Token 数</Label>
              <Input
                id="memory-token-count"
                type="number"
                value={state.value.tokenCount ?? 0}
                onChange={(event) => updateValue({ tokenCount: Number(event.target.value) || 0 })}
              />
            </div>
            <div className="flex flex-col gap-2">
              <Label htmlFor="memory-sequence">序号</Label>
              <Input
                id="memory-sequence"
                type="number"
                value={state.value.sequence ?? 0}
                onChange={(event) => updateValue({ sequence: Number(event.target.value) || 0 })}
              />
            </div>
          </div>
        </div>

        <div className="flex justify-end gap-2">
          <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>取消</Button>
          <Button type="button" onClick={onSubmit}>保存</Button>
        </div>
      </DialogContent>
    </Dialog>
  )
}

function EditorField({
  label,
  value,
  onChange,
}: {
  label: string
  value: string
  onChange: (value: string) => void
}) {
  return (
    <div className="flex flex-col gap-2">
      <Label>{label}</Label>
      <Textarea value={value} onChange={(event) => onChange(event.target.value)} rows={4} />
    </div>
  )
}

function DetailDialog({
  state,
  onOpenChange,
}: {
  state: DetailState | null
  onOpenChange: (open: boolean) => void
}) {
  return (
    <Dialog open={state !== null} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-3xl">
        <DialogHeader>
          <DialogTitle>历史记忆详情</DialogTitle>
          <DialogDescription>
            <span className="inline-flex items-center gap-2">
              <Clock3 className="size-4" />
              {state ? formatTimestamp(state.item.created) : "-"}
            </span>
          </DialogDescription>
        </DialogHeader>

        {!state ? null : (
          <div className="space-y-4">
            <div className="flex flex-wrap items-center gap-2">
              <Badge variant="outline">{state.item.role || "unknown"}</Badge>
              <Badge variant="secondary">{state.item.entryKind || "unknown"}</Badge>
              <Badge variant="outline">{getCompressionLevelBadge(state.item.compressionLevel).label}</Badge>
              <Badge variant="outline">#{state.item.id}</Badge>
            </div>
            <pre className="max-h-[60vh] overflow-auto whitespace-pre-wrap rounded-md border bg-muted/15 p-4 text-sm leading-6">
              {getMemoryEntryRenderedContent(state.item) || "这条历史记忆没有可展示的内容。"}
            </pre>
          </div>
        )}
      </DialogContent>
    </Dialog>
  )
}
