import React, { useCallback, useEffect, useMemo, useState } from "react"
import { useNavigate } from "react-router-dom"
import { ArrowLeft, Plus, RefreshCw, Pencil, Trash2, GitBranch, AlertCircle } from "lucide-react"

import { api, SkillSource, SkillSourceCreate, SkillSourceUpdate } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Switch } from "@/components/ui/switch"
import { Badge } from "@/components/ui/badge"
import { Skeleton } from "@/components/ui/skeleton"
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { useConfirmDialog } from "@/components/ConfirmDialogProvider"
import { cn } from "@/lib/utils"
import { toast } from "sonner"

import { useElectronSettingsPanel } from "@/components/electron/electron-main-contexts"

type DialogState =
  | { mode: "create"; source: null }
  | { mode: "edit"; source: SkillSource }
  | null

type SourceKind = "git" | "local"

function inferSourceKind(address: string): SourceKind {
  const value = address.trim()
  if (!value) return "git"
  if (
    value.startsWith("~") ||
    value.startsWith("./") ||
    value.startsWith("../") ||
    value.startsWith("/") ||
    value.toLowerCase().startsWith("file://") ||
    /^[a-zA-Z]:[\\/]/.test(value)
  ) {
    return "local"
  }
  return "git"
}

export function SkillSourcesPage() {
  const navigate = useNavigate()
  const settingsPanel = useElectronSettingsPanel()
  const { confirm } = useConfirmDialog()
  const [sources, setSources] = useState<SkillSource[]>([])
  const [loading, setLoading] = useState(true)
  const [pendingId, setPendingId] = useState<number | null>(null)
  const [dialogState, setDialogState] = useState<DialogState>(null)
  const [name, setName] = useState("")
  const [repoUrl, setRepoUrl] = useState("")
  const [skillsPath, setSkillsPath] = useState("skills")
  const [sourceKind, setSourceKind] = useState<SourceKind>("git")
  const [enabled, setEnabled] = useState(true)

  const loadSources = useCallback(async () => {
    try {
      setLoading(true)
      const data = await api.getSkillSources()
      setSources(data)
    } catch (error) {
      console.error("Failed to load skill sources:", error)
      toast.error("加载技能源失败")
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    loadSources()
  }, [loadSources])

  useEffect(() => {
    if (!dialogState) return
    if (dialogState.mode === "edit") {
      setName(dialogState.source.name)
      setRepoUrl(dialogState.source.repoUrl)
      setSkillsPath(dialogState.source.skillsPath)
      setSourceKind(inferSourceKind(dialogState.source.repoUrl))
      setEnabled(dialogState.source.enabled)
      return
    }
    setName("")
    setRepoUrl("")
    setSkillsPath("skills")
    setSourceKind("git")
    setEnabled(true)
  }, [dialogState])

  const enabledCount = useMemo(() => sources.filter((source) => source.enabled).length, [sources])

  const handleSourceKindChange = useCallback((nextKind: SourceKind) => {
    setSourceKind(nextKind)
    setSkillsPath((current) => {
      const trimmed = current.trim()
      if (nextKind === "local") {
        return !trimmed || trimmed === "skills" ? "." : current
      }
      return !trimmed || trimmed === "." ? "skills" : current
    })
  }, [])

  const handleSubmit = useCallback(async () => {
    const payload: SkillSourceCreate | SkillSourceUpdate = {
      name: name.trim(),
      repoUrl: repoUrl.trim(),
      skillsPath: skillsPath.trim(),
      enabled,
    }
    if (!name.trim() || !repoUrl.trim() || !skillsPath.trim()) {
      toast.error("请填写完整的源信息")
      return
    }
    try {
      if (dialogState?.mode === "edit") {
        await api.updateSkillSource(dialogState.source.id, payload)
        toast.success("技能源已更新")
      } else {
        await api.createSkillSource(payload as SkillSourceCreate)
        toast.success("技能源已创建并同步")
      }
      setDialogState(null)
      await loadSources()
    } catch (error: any) {
      toast.error(error?.message || "保存技能源失败")
      await loadSources()
    }
  }, [dialogState, enabled, loadSources, name, repoUrl, skillsPath])

  const handleSync = useCallback(async (source: SkillSource) => {
    try {
      setPendingId(source.id)
      await api.syncSkillSource(source.id)
      toast.success(`已同步 ${source.name}`)
      await loadSources()
    } catch (error: any) {
      toast.error(error?.message || "同步技能源失败")
      await loadSources()
    } finally {
      setPendingId(null)
    }
  }, [loadSources])

  const handleDelete = useCallback(async (source: SkillSource) => {
    const confirmed = await confirm({
      title: "删除技能源",
      description: `确定删除技能源 “${source.name}” 吗？`,
      confirmLabel: "删除",
      tone: "destructive",
    })
    if (!confirmed) return
    try {
      setPendingId(source.id)
      await api.deleteSkillSource(source.id)
      toast.success("技能源已删除")
      await loadSources()
    } catch (error: any) {
      toast.error(error?.message || "删除技能源失败")
    } finally {
      setPendingId(null)
    }
  }, [confirm, loadSources])

  const handleToggleEnabled = useCallback(async (source: SkillSource, nextEnabled: boolean) => {
    try {
      setPendingId(source.id)
      await api.updateSkillSource(source.id, { enabled: nextEnabled })
      toast.success(nextEnabled ? "已启用技能源" : "已停用技能源")
      await loadSources()
    } catch (error: any) {
      toast.error(error?.message || "更新技能源失败")
      await loadSources()
    } finally {
      setPendingId(null)
    }
  }, [loadSources])

  return (
    <div className="flex-1 overflow-y-auto p-8">
      <div className="mx-auto max-w-6xl space-y-4">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="flex items-center gap-3">
            <Button
              variant="outline"
              size="icon"
              onClick={() => {
                if (settingsPanel) {
                  settingsPanel.setPanel("skills")
                } else {
                  navigate("/skills")
                }
              }}
            >
              <ArrowLeft className="h-4 w-4" />
            </Button>
            <div className="flex flex-wrap items-center gap-2">
              <Badge variant="outline">源 {sources.length}</Badge>
              <Badge variant="outline">已启用 {enabledCount}</Badge>
              <Badge variant="outline">技能 {sources.reduce((sum, source) => sum + source.skillCount, 0)}</Badge>
            </div>
          </div>
          <div className="flex items-center gap-2">
            <Button variant="outline" onClick={loadSources} disabled={loading}>
              <RefreshCw className={cn("mr-2 h-4 w-4", loading && "animate-spin")} />
              刷新
            </Button>
            <Button onClick={() => setDialogState({ mode: "create", source: null })}>
              <Plus className="mr-2 h-4 w-4" />
              新建源
            </Button>
          </div>
        </div>

        {loading ? (
          <div className="space-y-4">
            {Array.from({ length: 4 }).map((_, idx) => (
              <Skeleton key={idx} className="h-36 rounded-xl" />
            ))}
          </div>
        ) : sources.length === 0 ? (
          <div className="flex h-[280px] flex-col items-center justify-center rounded-xl border border-dashed border-border/70 bg-muted/10 text-center">
            <div className="mb-4 rounded-xl bg-muted p-3">
              <GitBranch className="h-6 w-6 text-muted-foreground" />
            </div>
            <h3 className="text-base font-medium">还没有技能源</h3>
            <p className="mt-1 text-sm text-muted-foreground">先添加一个 Git 仓库或本地目录源并同步技能。</p>
          </div>
        ) : (
          <div className="space-y-3">
            {sources.map((source) => {
              const pending = pendingId === source.id
              const kind = inferSourceKind(source.repoUrl)
              return (
                <Card key={source.id} className="border-border/60 bg-background/80">
                  <CardHeader className="space-y-2 pb-2">
                    <div className="flex flex-wrap items-start justify-between gap-3">
                      <div className="min-w-0">
                        <CardTitle className="flex items-center gap-2 text-base">
                          <span className="truncate">{source.name}</span>
                          <Badge variant="secondary">{kind === "local" ? "本地目录" : "Git 仓库"}</Badge>
                          <Badge variant={source.enabled ? "default" : "outline"}>
                            {source.enabled ? "已启用" : "已停用"}
                          </Badge>
                        </CardTitle>
                        <CardDescription className="mt-1 break-all text-xs">{source.repoUrl}</CardDescription>
                      </div>
                      <div className="flex items-center gap-2">
                        <Switch
                          checked={source.enabled}
                          disabled={pending}
                          onCheckedChange={(checked) => handleToggleEnabled(source, checked)}
                        />
                        <Button variant="outline" size="sm" className="h-8" disabled={pending} onClick={() => handleSync(source)}>
                          <RefreshCw className={cn("mr-2 h-4 w-4", pending && "animate-spin")} />
                          同步
                        </Button>
                        <Button variant="outline" size="sm" className="h-8" onClick={() => setDialogState({ mode: "edit", source })}>
                          <Pencil className="mr-2 h-4 w-4" />
                          编辑
                        </Button>
                        <Button variant="destructive" size="sm" className="h-8" disabled={pending} onClick={() => handleDelete(source)}>
                          <Trash2 className="mr-2 h-4 w-4" />
                          删除
                        </Button>
                      </div>
                    </div>
                  </CardHeader>
                  <CardContent className="space-y-3 pt-0">
                    <div className="grid gap-2 md:grid-cols-4">
                      <div className="rounded-lg bg-muted/50 px-2.5 py-2">
                        <div className="text-[11px] font-medium text-foreground/80">扫描路径</div>
                        <div className="mt-1 text-xs text-muted-foreground">{source.skillsPath}</div>
                      </div>
                      <div className="rounded-lg bg-muted/50 px-2.5 py-2">
                        <div className="text-[11px] font-medium text-foreground/80">技能数量</div>
                        <div className="mt-1 text-xs text-muted-foreground">{source.skillCount}</div>
                      </div>
                      <div className="rounded-lg bg-muted/50 px-2.5 py-2">
                        <div className="text-[11px] font-medium text-foreground/80">最近同步</div>
                        <div className="mt-1 text-xs text-muted-foreground">
                          {source.lastSyncAt ? new Date(source.lastSyncAt).toLocaleString("zh-CN") : "尚未同步"}
                        </div>
                      </div>
                      <div className="rounded-lg bg-muted/50 px-2.5 py-2">
                        <div className="text-[11px] font-medium text-foreground/80">状态</div>
                        <div className="mt-1 text-xs text-muted-foreground">{source.enabled ? "启用中" : "已停用"}</div>
                      </div>
                    </div>
                    <div className="rounded-lg border border-border/60 bg-background/70 px-2.5 py-2 text-[11px] text-muted-foreground">
                      <div className="font-medium text-foreground/80">本地缓存目录</div>
                      <div className="mt-1 break-all font-mono">{source.localPath || "尚未生成"}</div>
                    </div>
                    {source.lastSyncError && (
                      <div className="md:col-span-3 flex items-start gap-2 rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-xs text-red-800">
                        <AlertCircle className="mt-0.5 h-4 w-4 shrink-0" />
                        <pre className="whitespace-pre-wrap break-words font-mono">{source.lastSyncError}</pre>
                      </div>
                    )}
                  </CardContent>
                </Card>
              )
            })}
          </div>
        )}

        <Dialog open={!!dialogState} onOpenChange={(open) => !open && setDialogState(null)}>
          <DialogContent className="sm:max-w-lg">
            <DialogHeader>
              <DialogTitle>{dialogState?.mode === "edit" ? "编辑技能源" : "新建技能源"}</DialogTitle>
              <DialogDescription>
                填写 Git 仓库或本地目录地址。保存时会自动同步并扫描技能。
              </DialogDescription>
            </DialogHeader>
            <div className="space-y-4">
              <div className="grid gap-2">
                <Label>源类型</Label>
                <Tabs value={sourceKind} onValueChange={(value) => handleSourceKindChange(value as SourceKind)}>
                  <TabsList className="grid w-full grid-cols-2">
                    <TabsTrigger value="git">Git 仓库</TabsTrigger>
                    <TabsTrigger value="local">本地目录</TabsTrigger>
                  </TabsList>
                </Tabs>
              </div>
              <div className="grid gap-2">
                <Label htmlFor="skill-source-name">名称</Label>
                <Input id="skill-source-name" value={name} onChange={(e) => setName(e.target.value)} placeholder="Anthropic Skills" />
              </div>
              <div className="grid gap-2">
                <Label htmlFor="skill-source-repo">{sourceKind === "local" ? "本地地址" : "仓库地址"}</Label>
                <Input
                  id="skill-source-repo"
                  value={repoUrl}
                  onChange={(e) => setRepoUrl(e.target.value)}
                  placeholder={sourceKind === "local" ? "~/.codex/skills" : "https://github.com/anthropics/skills"}
                />
                <p className="text-xs text-muted-foreground">
                  {sourceKind === "local" ? "支持 ~、绝对路径、相对路径和 file:// 地址。" : "支持常见 Git HTTPS / SSH 地址。"}
                </p>
              </div>
              <div className="grid gap-2">
                <Label htmlFor="skill-source-path">技能路径</Label>
                <Input
                  id="skill-source-path"
                  value={skillsPath}
                  onChange={(e) => setSkillsPath(e.target.value)}
                  placeholder={sourceKind === "local" ? "." : "skills"}
                />
                <p className="text-xs text-muted-foreground">
                  {sourceKind === "local" ? "本地目录通常直接填 .；如果技能在子目录中，再填写相对子路径。" : "填写仓库内存放技能的目录，默认是 skills。"}
                </p>
              </div>
              <div className="flex items-center justify-between rounded-lg border border-border/60 px-3 py-2">
                <div>
                  <div className="text-sm font-medium">启用源</div>
                  <div className="text-xs text-muted-foreground">启用后会参与技能展示与后续同步管理。</div>
                </div>
                <Switch checked={enabled} onCheckedChange={setEnabled} />
              </div>
            </div>
            <DialogFooter>
              <Button variant="outline" onClick={() => setDialogState(null)}>取消</Button>
              <Button onClick={handleSubmit}>{dialogState?.mode === "edit" ? "保存并同步" : "创建并同步"}</Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </div>
    </div>
  )
}
