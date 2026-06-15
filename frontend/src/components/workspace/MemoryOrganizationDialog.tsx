import React, { useState } from "react"
import { api, SessionMemoryCompactionPreview } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogTitle } from "@/components/ui/dialog"
import { Badge } from "@/components/ui/badge"
import { ScrollArea } from "@/components/ui/scroll-area"
import { toast } from "sonner"
import { Archive, BrainCircuit, Loader2, Wand2 } from "lucide-react"

interface MemoryOrganizationDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  sessionId?: string
  taskId?: number
  taskTitle?: string
}

function asFiniteMsgId(value: unknown): number | undefined {
  if (typeof value === "number" && Number.isFinite(value)) return value
  if (typeof value === "string" && value.trim() !== "") {
    const n = Number(value)
    if (Number.isFinite(n)) return n
  }
  return undefined
}

export function MemoryOrganizationDialog({
  open,
  onOpenChange,
  sessionId,
  taskId,
  taskTitle,
}: MemoryOrganizationDialogProps) {
  const [preview, setPreview] = useState<SessionMemoryCompactionPreview | null>(null)
  const [streaming, setStreaming] = useState(false)
  const [applying, setApplying] = useState(false)
  const [error, setError] = useState<string>("")

  const startOrganization = async () => {
    if (!sessionId) {
      toast.error("当前任务还没有会话，无法整理记忆")
      return
    }

    setPreview(null)
    setError("")
    setStreaming(true)

    try {
      let latestPreview: SessionMemoryCompactionPreview | null = null
      await api.streamPreviewSessionMemoryCompaction(sessionId, taskId, {
        onMeta: (meta) => {
          setPreview({
            message: "正在生成记忆压缩摘要",
            count: meta.count,
            scopePercent: meta.l2ScopePercent ?? meta.scopePercent,
            targetPercent: meta.targetPercent,
            l2ScopePercent: meta.l2ScopePercent ?? meta.scopePercent,
            beforeCount: meta.beforeCount,
            afterCount: meta.beforeCount,
            compressionRate: 0,
            beforePreview: meta.beforePreview,
            afterPreview: "",
            summary: "",
          })
        },
        onDelta: (summary) => {
          setPreview((current) => ({
            message: current?.message || "正在生成记忆压缩摘要",
            count: current?.count || 0,
            scopePercent: current?.l2ScopePercent || current?.scopePercent || 80,
            targetPercent: current?.targetPercent,
            l2ScopePercent: current?.l2ScopePercent || current?.scopePercent || 80,
            beforeCount: current?.beforeCount || 0,
            afterCount: current?.afterCount || current?.beforeCount || 0,
            compressionRate: current?.compressionRate || 0,
            beforePreview: current?.beforePreview || "",
            afterPreview: current?.afterPreview || "",
            summary,
          }))
        },
        onDone: (preview) => {
          latestPreview = preview
        },
        onError: (message) => {
          throw new Error(message)
        },
      })
      if (latestPreview) {
        setPreview(latestPreview)
      }
    } catch (streamError) {
      const message = streamError instanceof Error ? streamError.message : "自动记忆整理失败"
      setError(message)
      toast.error("自动记忆整理失败", { description: message })
    } finally {
      setStreaming(false)
    }
  }

  const confirmApply = async () => {
    if (!sessionId || !preview?.summary) return
    if (!preview.summary.trim()) {
      toast.error("当前没有可应用的压缩摘要")
      return
    }

    setApplying(true)
    try {
      const applied = await api.applySessionMemoryCompaction(sessionId, preview.summary)
      setPreview(applied)
      toast.success("记忆压缩已应用")
      onOpenChange(false)
    } catch (applyError) {
      const message = applyError instanceof Error ? applyError.message : "应用记忆整理失败"
      setError(message)
      toast.error("应用记忆整理失败", { description: message })
    } finally {
      setApplying(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="flex !left-0 !right-0 !bottom-0 !top-[var(--electron-window-chrome-top,0px)] h-[calc(100dvh-var(--electron-window-chrome-top,0px))] w-[100vw] max-w-[100vw] !translate-x-0 !translate-y-0 flex-col overflow-hidden rounded-none border-0 p-0">
        <div className="flex h-full min-h-0 flex-col">
          <div className="border-b px-6 py-4 pr-12">
            <div className="flex flex-wrap items-center gap-3">
              <DialogTitle className="text-base font-semibold">自动记忆整理</DialogTitle>
              {taskTitle && (
                <span className="max-w-[360px] truncate text-sm text-muted-foreground" title={taskTitle}>
                  {taskTitle}
                </span>
              )}
              <Badge variant="secondary">目标 ≤ {preview?.targetPercent ?? 60}%</Badge>
              <Badge variant="outline">L2 范围 {preview?.l2ScopePercent ?? preview?.scopePercent ?? 80}%</Badge>
              {streaming && <Badge variant="outline">正在总结</Badge>}
            </div>
          </div>

          <div className="grid min-h-0 flex-1 grid-cols-1 gap-4 px-6 py-4 lg:grid-cols-[minmax(0,1.15fr)_minmax(320px,0.85fr)]">
            <div className="min-h-0 rounded-md border">
              <div className="flex h-full min-h-0 flex-col">
                <div className="flex items-center justify-between border-b px-4 py-3">
                  <div>
                    <div className="text-sm font-medium">记忆压缩预览</div>
                    <div className="text-xs text-muted-foreground">点击“整理记忆”只生成摘要，点击“确认开始压缩”才会替换历史记忆</div>
                  </div>
                  <div className="flex items-center gap-2">
                    <Button type="button" size="sm" variant="outline" disabled={streaming || applying} onClick={startOrganization}>
                      {streaming ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : <Wand2 className="mr-2 h-4 w-4" />}
                      整理记忆
                    </Button>
                    <Button type="button" size="sm" disabled={streaming || applying || !preview?.summary} onClick={confirmApply}>
                      {applying ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : <Archive className="mr-2 h-4 w-4" />}
                      确认开始压缩
                    </Button>
                  </div>
                </div>
                <ScrollArea className="h-full">
                  <div className="space-y-4 p-4">
                    {error && (
                      <div className="rounded-md border border-destructive/30 bg-destructive/5 p-3 text-sm text-destructive">
                        {error}
                      </div>
                    )}
                    {!preview ? (
                      <div className="rounded-md border border-dashed bg-muted/20 p-4 text-sm text-muted-foreground">
                        点击“整理记忆”后，这里会展示即将压缩的历史记忆和生成的摘要。
                      </div>
                    ) : (
                      <>
                        <div className="flex flex-wrap items-center gap-2">
                          <Badge variant="outline">{preview.beforeCount} → {preview.afterCount}</Badge>
                          <Badge variant="outline">压缩率 {preview.compressionRate.toFixed(preview.compressionRate >= 10 ? 0 : 1)}%</Badge>
                          {(preview.levelsExecuted?.length ?? 0) > 0 && (
                            <Badge variant="secondary">
                              执行 {preview.levelsExecuted?.map((level) => `L${level}`).join(" → ")}
                            </Badge>
                          )}
                        </div>
                        <div>
                          <div className="mb-1 text-xs font-medium text-muted-foreground">压缩前</div>
                          <pre className="max-w-full whitespace-pre-wrap rounded bg-muted/30 p-3 text-xs break-words [overflow-wrap:anywhere]">
                            {preview.beforePreview}
                          </pre>
                        </div>
                        <div>
                          <div className="mb-1 text-xs font-medium text-muted-foreground">
                            摘要内容{streaming && preview.summary ? "（生成中）" : ""}
                          </div>
                          <pre className="max-w-full whitespace-pre-wrap rounded bg-muted/30 p-3 text-xs break-words [overflow-wrap:anywhere]">
                            {preview.summary || (streaming ? "正在生成摘要..." : "")}
                          </pre>
                        </div>
                        <div>
                          <div className="mb-1 text-xs font-medium text-muted-foreground">压缩后</div>
                          <pre className="max-w-full whitespace-pre-wrap rounded bg-muted/30 p-3 text-xs break-words [overflow-wrap:anywhere]">
                            {preview.afterPreview}
                          </pre>
                        </div>
                      </>
                    )}
                  </div>
                </ScrollArea>
              </div>
            </div>

            <div className="min-h-0 rounded-md border">
              <ScrollArea className="h-full">
                <div className="space-y-3 p-4">
                  <div className="flex items-center gap-2">
                    <BrainCircuit className="h-4 w-4 text-muted-foreground" />
                    <span className="text-sm font-medium">压缩计划</span>
                  </div>
                  {!preview ? (
                    <div className="rounded-md border border-dashed bg-muted/20 p-4 text-sm text-muted-foreground">
                      点击“整理记忆”后，这里会展示本次将压缩的范围、摘要和压缩效果。
                    </div>
                  ) : (
                    <div className="space-y-2 rounded-md border bg-muted/10 p-3">
                      <div className="flex flex-wrap items-center gap-2">
                        <Badge variant="secondary">memory_compaction</Badge>
                        <Badge variant="outline">目标 ≤ {preview.targetPercent ?? 60}%</Badge>
                        <Badge variant="outline">L2 范围 {preview.l2ScopePercent ?? preview.scopePercent}%</Badge>
                        {(preview.levelsExecuted?.length ?? 0) > 0 && (
                          <Badge variant="outline">
                            {preview.levelsExecuted?.map((level) => `L${level}`).join(" → ")}
                          </Badge>
                        )}
                        <Badge variant="outline">{preview.beforeCount} → {preview.afterCount}</Badge>
                      </div>
                      <div className="text-xs text-muted-foreground">
                        记忆压缩：对最旧的一段历史调用 LLM 生成摘要并替换原条目。第一步只生成预览，第二步才会写回。
                      </div>
                    </div>
                  )}
                </div>
              </ScrollArea>
            </div>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  )
}
