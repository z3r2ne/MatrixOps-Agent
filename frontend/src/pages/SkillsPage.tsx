import React, { useCallback, useEffect, useMemo, useState } from "react"
import { useNavigate } from "react-router-dom"
import { RefreshCw, Settings2, Sparkles, Download, Trash2, Search, Plug } from "lucide-react"

import { api, SkillCard, SkillSource } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Badge } from "@/components/ui/badge"
import { Switch } from "@/components/ui/switch"
import { Label } from "@/components/ui/label"
import { Skeleton } from "@/components/ui/skeleton"
import { cn } from "@/lib/utils"
import { toast } from "sonner"

import { useElectronSettingsPanel } from "@/components/electron/electron-main-contexts"

export function SkillsPage() {
  const navigate = useNavigate()
  const settingsPanel = useElectronSettingsPanel()
  const [skills, setSkills] = useState<SkillCard[]>([])
  const [sources, setSources] = useState<SkillSource[]>([])
  const [loading, setLoading] = useState(true)
  const [installedOnly, setInstalledOnly] = useState(false)
  const [query, setQuery] = useState("")
  const [pendingSkillId, setPendingSkillId] = useState<string | null>(null)

  const loadData = useCallback(async () => {
    try {
      setLoading(true)
      const [sourceData, skillData] = await Promise.all([
        api.getSkillSources(),
        api.getSkills(installedOnly),
      ])
      setSources(sourceData)
      setSkills(skillData)
    } catch (error) {
      console.error("Failed to load skills:", error)
      toast.error("加载技能广场失败")
    } finally {
      setLoading(false)
    }
  }, [installedOnly])

  useEffect(() => {
    loadData()
  }, [loadData])

  const filteredSkills = useMemo(() => {
    const needle = query.trim().toLowerCase()
    if (!needle) return skills
    return skills.filter((skill) =>
      skill.name.toLowerCase().includes(needle) ||
      skill.description.toLowerCase().includes(needle) ||
      skill.sourceName.toLowerCase().includes(needle),
    )
  }, [query, skills])

  const installedCount = useMemo(() => skills.filter((skill) => skill.installed).length, [skills])
  const enabledSourceCount = useMemo(() => sources.filter((source) => source.enabled).length, [sources])

  const handleInstallToggle = useCallback(async (skill: SkillCard) => {
    try {
      setPendingSkillId(skill.id)
      if (skill.installed) {
        await api.uninstallSkill(skill.sourceId, skill.relativePath)
        toast.success(`已卸载 ${skill.name}`)
      } else {
        await api.installSkill(skill.sourceId, skill.relativePath)
        toast.success(`已安装 ${skill.name}`)
      }
      await loadData()
    } catch (error: any) {
      toast.error(error?.message || `${skill.installed ? "卸载" : "安装"}技能失败`)
    } finally {
      setPendingSkillId(null)
    }
  }, [loadData])

  return (
    <div className="flex-1 overflow-y-auto p-8">
      <div className="mx-auto max-w-7xl space-y-4">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="flex flex-wrap items-center gap-2">
            <Badge variant="outline">源 {sources.length}</Badge>
            <Badge variant="outline">已启用 {enabledSourceCount}</Badge>
            <Badge variant="outline">已安装 {installedCount}</Badge>
          </div>
          <div className="flex items-center gap-2">
            <Button variant="outline" onClick={loadData} disabled={loading}>
              <RefreshCw className={cn("mr-2 h-4 w-4", loading && "animate-spin")} />
              刷新
            </Button>
            <Button
              variant="outline"
              onClick={() => {
                if (settingsPanel) {
                  settingsPanel.setPanel("mcp")
                } else {
                  navigate("/mcp")
                }
              }}
            >
              <Plug className="mr-2 h-4 w-4" />
              MCP
            </Button>
            <Button
              onClick={() => {
                if (settingsPanel) {
                  settingsPanel.setPanel("skills-sources")
                } else {
                  navigate("/skills/sources")
                }
              }}
            >
              <Settings2 className="mr-2 h-4 w-4" />
              源管理
            </Button>
          </div>
        </div>

        <div className="flex flex-col gap-3 rounded-xl border border-border/60 bg-background/80 p-3 md:flex-row md:items-center md:justify-between">
          <div className="relative flex-1">
            <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
            <Input
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="搜索技能名称、描述或来源..."
              className="pl-10"
            />
          </div>
          <div className="flex items-center gap-2">
            <Switch id="installed-only" checked={installedOnly} onCheckedChange={setInstalledOnly} />
            <Label htmlFor="installed-only" className="text-sm text-muted-foreground">仅显示已安装</Label>
          </div>
        </div>

        {loading ? (
          <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
            {Array.from({ length: 6 }).map((_, idx) => (
              <Skeleton key={idx} className="h-44 rounded-xl" />
            ))}
          </div>
        ) : filteredSkills.length === 0 ? (
          <div className="flex h-[320px] flex-col items-center justify-center rounded-xl border border-dashed border-border/70 bg-muted/10 text-center">
            <div className="mb-4 rounded-xl bg-muted p-3">
              <Sparkles className="h-6 w-6 text-muted-foreground" />
            </div>
            <h3 className="text-base font-medium">暂无可展示的技能</h3>
            <p className="mt-1 text-sm text-muted-foreground">
              {sources.length === 0 ? "先添加一个技能源并同步。" : "试试调整搜索条件或检查技能源配置。"}
            </p>
          </div>
        ) : (
          <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
            {filteredSkills.map((skill) => {
              const pending = pendingSkillId === skill.id
              return (
                <Card
                  key={skill.id}
                  className={cn(
                    "border-border/60 bg-background/80 transition-colors",
                    skill.installed && "border-emerald-200 bg-emerald-50/30",
                    !skill.sourceEnabled && "opacity-75",
                  )}
                >
                  <CardHeader className="space-y-2 pb-2">
                    <div className="flex items-start justify-between gap-3">
                      <div className="min-w-0">
                        <CardTitle className="truncate text-sm">{skill.name}</CardTitle>
                        <CardDescription className="mt-1 flex flex-wrap items-center gap-1.5 text-[11px]">
                          <span className="truncate">{skill.sourceName}</span>
                          {!skill.sourceEnabled && <Badge variant="outline">源已禁用</Badge>}
                        </CardDescription>
                      </div>
                      {skill.installed ? (
                        <Badge className="bg-emerald-100 text-emerald-700 hover:bg-emerald-100">已安装</Badge>
                      ) : (
                        <Badge variant="outline">未安装</Badge>
                      )}
                    </div>
                  </CardHeader>
                  <CardContent className="space-y-3 pt-0">
                    <p className="line-clamp-3 text-xs leading-5 text-muted-foreground">
                      {skill.description || "暂无描述"}
                    </p>
                    <div className="space-y-1 rounded-lg bg-muted/50 px-2.5 py-2 text-[11px] text-muted-foreground">
                      <div className="font-medium text-foreground/80">相对路径</div>
                      <div className="break-all font-mono leading-4">{skill.relativePath}</div>
                    </div>
                    <Button
                      className="h-8 w-full"
                      variant={skill.installed ? "outline" : "default"}
                      size="sm"
                      disabled={pending}
                      onClick={() => handleInstallToggle(skill)}
                    >
                      {skill.installed ? (
                        <>
                          <Trash2 className="mr-2 h-4 w-4" />
                          卸载
                        </>
                      ) : (
                        <>
                          <Download className="mr-2 h-4 w-4" />
                          安装
                        </>
                      )}
                    </Button>
                  </CardContent>
                </Card>
              )
            })}
          </div>
        )}
      </div>
    </div>
  )
}
