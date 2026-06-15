import React, { useCallback, useEffect, useMemo, useState } from "react"
import {
  ArrowUpCircle,
  BookOpen,
  Database,
  Loader2,
  Pencil,
  Plus,
  RefreshCw,
  Trash2,
  type LucideIcon,
} from "lucide-react"
import { toast } from "sonner"

import { useConfirmDialog } from "@/components/ConfirmDialogProvider"
import { MemoryLibrarySearchIndexStatusBadge } from "@/components/MemoryLibrarySearchIndexStatusBadge"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Skeleton } from "@/components/ui/skeleton"
import { Switch } from "@/components/ui/switch"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { Textarea } from "@/components/ui/textarea"
import { api, type MemoryLibrary, type MemoryLibrarySearchIndexStatus } from "@/lib/api"
import { cn } from "@/lib/utils"
import { MarkdownRenderer } from "@/components/workspace/MarkdownRenderer"

type Variant = "memory" | "rag"

type DialogState =
  | { mode: "create" }
  | { mode: "edit"; library: MemoryLibrary }
  | null

type FormState = {
  name: string
  content: string
}

const EMPTY_FORM: FormState = { name: "", content: "" }

const VARIANT_META: Record<
  Variant,
  {
    icon: LucideIcon
    title: string
    description: string
    emptyTitle: string
    emptyDescription: string
    searchPlaceholder: string
    createLabel: string
    entityLabel: string
    namePlaceholder: string
    deleteExtra?: string
  }
> = {
  memory: {
    icon: BookOpen,
    title: "记忆库",
    description: "为项目提供可关联的背景说明，任务启动时会注入介绍内容。",
    emptyTitle: "还没有记忆库",
    emptyDescription: "创建记忆库并填写介绍，然后在项目设置中关联使用。",
    searchPlaceholder: "搜索名称或介绍…",
    createLabel: "新建记忆库",
    entityLabel: "记忆库",
    namePlaceholder: "例如：支付系统背景",
    deleteExtra: "关联该记忆库的项目会自动解除关联。",
  },
  rag: {
    icon: Database,
    title: "RAG 知识库",
    description: "用于语义检索的知识库，支持 embedding 索引；创建任务时可选择使用。",
    emptyTitle: "还没有 RAG 知识库",
    emptyDescription: "创建后可建立检索索引，Agent 可通过 memory_search 检索相关内容。",
    searchPlaceholder: "搜索 RAG 知识库名称或内容…",
    createLabel: "新建 RAG",
    entityLabel: "RAG 知识库",
    namePlaceholder: "例如：API 设计规范",
  },
}

function formatLibraryTime(value: string) {
  if (!value) return ""
  return new Date(value).toLocaleString("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  })
}

function formatLibraryBytes(bytes: number) {
  if (bytes >= 1024 * 1024) {
    return `${(bytes / (1024 * 1024)).toFixed(bytes >= 10 * 1024 * 1024 ? 0 : 1)} MB`
  }
  if (bytes >= 1024) {
    return `${(bytes / 1024).toFixed(bytes >= 10 * 1024 ? 0 : 1)} KB`
  }
  return `${bytes} B`
}

function LibraryUsageStats({
  status,
  variant,
}: {
  status?: MemoryLibrarySearchIndexStatus
  variant: Variant
}) {
  if (!status) {
    return (
      <p className="mt-2 text-[11px] text-muted-foreground">
        <Loader2 className="mr-1 inline h-3 w-3 animate-spin" />
        加载占用信息…
      </p>
    )
  }

  const vectorCount = status.vectorCount ?? status.documentCount ?? 0
  const contentBytes = status.contentBytes ?? 0
  const totalBytes = status.totalBytes ?? contentBytes
  const contentLabel = variant === "memory" ? "介绍" : "文本"

  return (
    <p className="mt-2 flex flex-wrap gap-x-3 gap-y-1 text-[11px] text-muted-foreground">
      <span>向量 {vectorCount.toLocaleString()} 条</span>
      <span>
        {contentLabel} {formatLibraryBytes(contentBytes)}
      </span>
      {variant === "rag" && (status.indexContentBytes ?? 0) > 0 ? (
        <span>索引文本 {formatLibraryBytes(status.indexContentBytes ?? 0)}</span>
      ) : null}
      {(status.vectorBytes ?? 0) > 0 ? (
        <span>向量数据 {formatLibraryBytes(status.vectorBytes ?? 0)}</span>
      ) : null}
      <span>共 {formatLibraryBytes(totalBytes)}</span>
    </p>
  )
}

function contentPreview(content: string, emptyLabel = "暂无内容") {
  const normalized = content.replace(/\s+/g, " ").trim()
  return normalized ? normalized.slice(0, 120) : emptyLabel
}

function LibraryEditFields({
  variant,
  form,
  meta,
  onFormChange,
}: {
  variant: Variant
  form: FormState
  meta: (typeof VARIANT_META)[Variant]
  onFormChange: (next: FormState) => void
}) {
  if (variant === "memory") {
    return (
      <>
        <div className="space-y-2">
          <Label htmlFor="library-name">名称</Label>
          <Input
            id="library-name"
            placeholder={meta.namePlaceholder}
            value={form.name}
            onChange={(event) => onFormChange({ ...form, name: event.target.value })}
          />
        </div>
        <div className="space-y-2">
          <Label htmlFor="library-intro">介绍</Label>
          <Textarea
            id="library-intro"
            value={form.content}
            onChange={(event) => onFormChange({ ...form, content: event.target.value })}
            placeholder="简要说明该记忆库的用途、适用场景或关键背景…"
            rows={5}
            className="resize-y text-sm leading-6"
          />
          <p className="text-xs text-muted-foreground">关联到项目后，介绍会在任务启动时注入为上下文。</p>
        </div>
      </>
    )
  }

  return (
    <>
      <div className="space-y-2">
        <Label htmlFor="library-name">名称</Label>
        <Input
          id="library-name"
          placeholder={meta.namePlaceholder}
          value={form.name}
          onChange={(event) => onFormChange({ ...form, name: event.target.value })}
        />
      </div>
      <Tabs defaultValue="edit" className="w-full">
        <TabsList>
          <TabsTrigger value="edit">编辑</TabsTrigger>
          <TabsTrigger value="preview">预览</TabsTrigger>
        </TabsList>
        <TabsContent value="edit" className="mt-4 space-y-2">
          <Label htmlFor="library-content">内容</Label>
          <Textarea
            id="library-content"
            value={form.content}
            onChange={(event) => onFormChange({ ...form, content: event.target.value })}
            placeholder="写入可被检索的知识条目、文档片段或说明…"
            rows={16}
            className="min-h-[280px] resize-y font-mono text-sm leading-6"
          />
        </TabsContent>
        <TabsContent value="preview" className="mt-4">
          <div className="min-h-[280px] rounded-lg border border-border/70 bg-muted/10 p-4">
            {form.content.trim() ? (
              <MarkdownRenderer content={form.content} />
            ) : (
              <div className="text-sm text-muted-foreground">暂无内容。</div>
            )}
          </div>
        </TabsContent>
      </Tabs>
    </>
  )
}

export function MarkdownLibrariesManager({ variant }: { variant: Variant }) {
  const meta = VARIANT_META[variant]
  const PageIcon = meta.icon
  const { confirm } = useConfirmDialog()

  const [libraries, setLibraries] = useState<MemoryLibrary[]>([])
  const [loading, setLoading] = useState(true)
  const [pendingId, setPendingId] = useState<number | null>(null)
  const [searchQuery, setSearchQuery] = useState("")
  const [includeTemporary, setIncludeTemporary] = useState(false)
  const [dialogState, setDialogState] = useState<DialogState>(null)
  const [form, setForm] = useState<FormState>(EMPTY_FORM)
  const [saving, setSaving] = useState(false)
  const [promoting, setPromoting] = useState(false)
  const [rebuildingLibraryId, setRebuildingLibraryId] = useState<number | null>(null)
  const [searchIndexStatusMap, setSearchIndexStatusMap] = useState<Record<number, MemoryLibrarySearchIndexStatus>>({})

  const editingLibrary = dialogState?.mode === "edit" ? dialogState.library : null

  const filteredLibraries = useMemo(() => {
    const needle = searchQuery.trim().toLowerCase()
    if (!needle) return libraries
    return libraries.filter(
      (library) =>
        library.name.toLowerCase().includes(needle) ||
        library.content.toLowerCase().includes(needle),
    )
  }, [libraries, searchQuery])

  const loadLibraries = useCallback(async () => {
    setLoading(true)
    try {
      const data =
        variant === "rag"
          ? await api.getRagLibraries({ includeTemporary })
          : await api.getMemoryLibraries({ isRag: false })
      setLibraries(data)
    } catch (error) {
      console.error("Failed to load libraries", error)
      setLibraries([])
      toast.error(`加载${meta.entityLabel}失败`)
    } finally {
      setLoading(false)
    }
  }, [includeTemporary, meta.entityLabel, variant])

  useEffect(() => {
    void loadLibraries()
  }, [loadLibraries])

  useEffect(() => {
    if (libraries.length === 0) {
      setSearchIndexStatusMap({})
      return
    }
    let cancelled = false
    const loadStatuses = async () => {
      const entries = await Promise.all(
        libraries.map(async (library) => {
          try {
            const status = await api.getMemoryLibrarySearchIndexStatus(library.id)
            return [library.id, status] as const
          } catch {
            return [
              library.id,
              {
                memoryLibraryId: library.id,
                documentCount: 0,
                vectorCount: 0,
                vectorDimension: 0,
                contentBytes: 0,
                indexContentBytes: 0,
                vectorBytes: 0,
                totalBytes: 0,
                status: "idle",
                progress: 0,
                hasIndex: false,
              } satisfies MemoryLibrarySearchIndexStatus,
            ] as const
          }
        }),
      )
      if (!cancelled) {
        setSearchIndexStatusMap(Object.fromEntries(entries))
      }
    }
    void loadStatuses()
    return () => {
      cancelled = true
    }
  }, [libraries])

  const editingIndexStatus =
    editingLibrary != null ? searchIndexStatusMap[editingLibrary.id] : undefined

  useEffect(() => {
    if (!dialogState) {
      setForm(EMPTY_FORM)
      return
    }
    if (dialogState.mode === "edit") {
      setForm({ name: dialogState.library.name, content: dialogState.library.content })
      return
    }
    setForm(EMPTY_FORM)
  }, [dialogState])

  const handleSave = async () => {
    const name = form.name.trim()
    if (!name) {
      toast.error(`请填写${meta.entityLabel}名称`)
      return
    }
    setSaving(true)
    try {
      if (dialogState?.mode === "edit") {
        await api.updateMemoryLibrary(dialogState.library.id, {
          name,
          content: form.content,
        })
        toast.success(`${meta.entityLabel}已更新`)
      } else {
        if (variant === "rag") {
          await api.createRagLibrary({ name, content: form.content })
        } else {
          await api.createMemoryLibrary({ name, content: form.content, isRag: false })
        }
        toast.success(`${meta.entityLabel}已创建`)
      }
      setDialogState(null)
      await loadLibraries()
    } catch (error: unknown) {
      toast.error(error instanceof Error ? error.message : `保存${meta.entityLabel}失败`)
    } finally {
      setSaving(false)
    }
  }

  const handleDelete = async (library: MemoryLibrary) => {
    const confirmed = await confirm({
      title: `删除${meta.entityLabel}`,
      description: [`确定删除「${library.name}」吗？`, meta.deleteExtra].filter(Boolean).join("\n\n"),
      confirmLabel: "删除",
      tone: "destructive",
    })
    if (!confirmed) return

    try {
      setPendingId(library.id)
      await api.deleteMemoryLibrary(library.id)
      toast.success(`${meta.entityLabel}已删除`)
      if (dialogState?.mode === "edit" && dialogState.library.id === library.id) {
        setDialogState(null)
      }
      await loadLibraries()
    } catch (error: unknown) {
      toast.error(error instanceof Error ? error.message : `删除${meta.entityLabel}失败`)
    } finally {
      setPendingId(null)
    }
  }

  const handlePromoteTemporary = async (library: MemoryLibrary) => {
    const confirmed = await confirm({
      title: "转为长期 RAG",
      description: `确定将「${library.name}」转为长期 RAG 知识库吗？\n\n转正后可在创建任务时选择。`,
      confirmLabel: "转为长期",
    })
    if (!confirmed) return

    setPromoting(true)
    try {
      const updated = await api.promoteMemoryLibrary(library.id, { name: library.name })
      toast.success("已转为长期 RAG 知识库")
      if (dialogState?.mode === "edit" && dialogState.library.id === library.id) {
        setDialogState({ mode: "edit", library: updated })
      }
      await loadLibraries()
    } catch (error: unknown) {
      toast.error(error instanceof Error ? error.message : "转正失败")
    } finally {
      setPromoting(false)
    }
  }

  const handleRebuildLibrarySearchIndex = async (libraryId: number) => {
    try {
      setRebuildingLibraryId(libraryId)
      await api.rebuildMemoryLibrarySearchIndex(libraryId)
      toast.success("已提交检索索引重建任务")
      const status = await api.getMemoryLibrarySearchIndexStatus(libraryId)
      setSearchIndexStatusMap((prev) => ({ ...prev, [libraryId]: status }))
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "提交检索索引任务失败")
    } finally {
      setRebuildingLibraryId(null)
    }
  }

  return (
    <div className="flex-1 overflow-x-hidden overflow-y-auto p-8">
      <div className="mx-auto max-w-4xl space-y-5">
        <div className="space-y-1">
          <h1 className="text-lg font-semibold">{meta.title}</h1>
          <p className="text-sm text-muted-foreground">{meta.description}</p>
        </div>

        <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <Input
            placeholder={meta.searchPlaceholder}
            value={searchQuery}
            onChange={(event) => setSearchQuery(event.target.value)}
            className="sm:max-w-sm"
          />
          <div className="flex flex-wrap items-center gap-2">
            {variant === "rag" ? (
              <div className="flex items-center gap-2 rounded-lg border border-border/60 bg-muted/20 px-3 py-1.5">
                <Label htmlFor="show-temporary-rag" className="text-xs text-muted-foreground">
                  显示临时
                </Label>
                <Switch
                  id="show-temporary-rag"
                  checked={includeTemporary}
                  onCheckedChange={setIncludeTemporary}
                  disabled={loading}
                />
              </div>
            ) : null}
            <Badge variant="outline">{filteredLibraries.length} 项</Badge>
            <Button variant="outline" onClick={() => void loadLibraries()} disabled={loading}>
              <RefreshCw className={cn("mr-2 h-4 w-4", loading && "animate-spin")} />
              刷新
            </Button>
            <Button onClick={() => setDialogState({ mode: "create" })}>
              <Plus className="mr-2 h-4 w-4" />
              {meta.createLabel}
            </Button>
          </div>
        </div>

        {loading ? (
          <div className="space-y-3">
            {Array.from({ length: 4 }).map((_, index) => (
              <Skeleton key={index} className="h-28 rounded-xl" />
            ))}
          </div>
        ) : filteredLibraries.length === 0 ? (
          <div className="flex min-h-[280px] flex-col items-center justify-center rounded-xl border border-dashed border-border/70 bg-muted/10 px-6 text-center">
            <div className="mb-4 rounded-xl bg-muted p-3">
              <PageIcon className="h-6 w-6 text-muted-foreground" />
            </div>
            <h3 className="text-base font-medium">
              {searchQuery ? `没有匹配的${meta.entityLabel}` : meta.emptyTitle}
            </h3>
            <p className="mt-1 max-w-md text-sm text-muted-foreground">
              {searchQuery ? "试试其他关键词。" : meta.emptyDescription}
            </p>
            {!searchQuery ? (
              <Button className="mt-4" onClick={() => setDialogState({ mode: "create" })}>
                <Plus className="mr-2 h-4 w-4" />
                {meta.createLabel}
              </Button>
            ) : null}
          </div>
        ) : (
          <div className="space-y-3">
            {filteredLibraries.map((library) => {
              const pending = pendingId === library.id
              const indexStatus = searchIndexStatusMap[library.id]
              return (
                <Card key={library.id} className="border-border/60 bg-background/80">
                  <CardHeader className="space-y-3 pb-3">
                    <div className="flex flex-wrap items-start justify-between gap-3">
                      <div className="min-w-0 flex-1">
                        <CardTitle className="flex flex-wrap items-center gap-2 text-base">
                          <span className="truncate">{library.name}</span>
                          {library.isTemporary ? (
                            <Badge variant="secondary" className="text-[10px]">
                              临时
                            </Badge>
                          ) : null}
                          {variant === "rag" && indexStatus ? (
                            <MemoryLibrarySearchIndexStatusBadge
                              status={indexStatus.status}
                              progress={indexStatus.progress}
                            />
                          ) : null}
                        </CardTitle>
                        <CardDescription className="mt-2 line-clamp-2 text-sm leading-6">
                          {contentPreview(library.content, variant === "memory" ? "暂无介绍" : "暂无内容")}
                        </CardDescription>
                        <LibraryUsageStats status={indexStatus} variant={variant} />
                        <p className="mt-1 text-[11px] text-muted-foreground">
                          更新于 {formatLibraryTime(library.updatedAt)}
                        </p>
                      </div>
                      <div className="flex flex-wrap items-center gap-2">
                        {variant === "rag" ? (
                          <>
                            <Button
                              variant="outline"
                              size="sm"
                              className="h-8"
                              disabled={rebuildingLibraryId === library.id}
                              onClick={() => void handleRebuildLibrarySearchIndex(library.id)}
                            >
                              {rebuildingLibraryId === library.id ? (
                                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                              ) : null}
                              重建索引
                            </Button>
                            {library.isTemporary ? (
                              <Button
                                variant="outline"
                                size="sm"
                                className="h-8"
                                disabled={promoting || pending}
                                onClick={() => void handlePromoteTemporary(library)}
                              >
                                <ArrowUpCircle className="mr-2 h-4 w-4" />
                                转为长期
                              </Button>
                            ) : null}
                          </>
                        ) : null}
                        <Button
                          variant="outline"
                          size="sm"
                          className="h-8"
                          disabled={pending}
                          onClick={() => setDialogState({ mode: "edit", library })}
                        >
                          <Pencil className="mr-2 h-4 w-4" />
                          编辑
                        </Button>
                        <Button
                          variant="destructive"
                          size="sm"
                          className="h-8"
                          disabled={pending || saving}
                          onClick={() => void handleDelete(library)}
                        >
                          <Trash2 className="mr-2 h-4 w-4" />
                          删除
                        </Button>
                      </div>
                    </div>
                  </CardHeader>
                </Card>
              )
            })}
          </div>
        )}
      </div>

      <Dialog open={dialogState !== null} onOpenChange={(open) => !open && !saving && setDialogState(null)}>
        <DialogContent
          className={cn(
            "flex max-h-[min(90dvh,820px)] flex-col gap-0 overflow-hidden p-0",
            variant === "memory" ? "max-w-lg" : "max-w-2xl",
          )}
        >
          <DialogHeader className="border-b px-6 py-4">
            <DialogTitle>
              {dialogState?.mode === "edit" ? `编辑${meta.entityLabel}` : meta.createLabel}
            </DialogTitle>
            <DialogDescription>{meta.description}</DialogDescription>
          </DialogHeader>

          <div className="min-h-0 flex-1 overflow-y-auto px-6 py-4">
            <div className="space-y-4">
              {editingLibrary?.isTemporary ? (
                <div className="flex flex-wrap items-center justify-between gap-2 rounded-lg border border-border/60 bg-muted/20 px-3 py-2">
                  <p className="text-xs text-muted-foreground">当前为临时 RAG，转正后可被任务创建时选择。</p>
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    disabled={promoting || saving}
                    onClick={() => void handlePromoteTemporary(editingLibrary)}
                  >
                    <ArrowUpCircle className="mr-2 h-4 w-4" />
                    转为长期
                  </Button>
                </div>
              ) : null}

              <LibraryEditFields
                variant={variant}
                form={form}
                meta={meta}
                onFormChange={setForm}
              />

              {dialogState?.mode === "edit" ? (
                <div className="rounded-lg border border-border/60 bg-muted/10 px-3 py-2">
                  <p className="text-xs font-medium text-foreground">占用统计</p>
                  <LibraryUsageStats status={editingIndexStatus} variant={variant} />
                </div>
              ) : null}
            </div>
          </div>

          <DialogFooter className="border-t px-6 py-4">
            <Button variant="outline" onClick={() => setDialogState(null)} disabled={saving}>
              取消
            </Button>
            <Button onClick={() => void handleSave()} disabled={saving}>
              {saving ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : null}
              保存
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
