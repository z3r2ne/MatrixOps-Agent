import React, { useCallback, useEffect, useMemo, useState } from "react"
import { Eye, EyeOff, Pencil, Plus, RefreshCw, Search, Trash2 } from "lucide-react"
import { toast } from "sonner"

import { useConfirmDialog } from "@/components/ConfirmDialogProvider"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Combobox, type ComboboxOption } from "@/components/ui/combobox"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { InputGroup, InputGroupAddon, InputGroupButton, InputGroupInput } from "@/components/ui/input-group"
import { Label } from "@/components/ui/label"
import { Skeleton } from "@/components/ui/skeleton"
import { Switch } from "@/components/ui/switch"
import { Textarea } from "@/components/ui/textarea"
import { api, type SearchConfig, type SearchConfigCreate, type SearchConfigUpdate } from "@/lib/api"
import { cn } from "@/lib/utils"

const SEARCH_TYPE_OPTIONS: ComboboxOption[] = [
  { value: "kimi_search_api", label: "Kimi Search API", searchText: "kimi search api" },
]

const DEFAULT_BASE_URL = "https://agent-gw.kimi.com/coding"

type DialogState =
  | { mode: "create"; config: null }
  | { mode: "edit"; config: SearchConfig }
  | null

type FormState = {
  name: string
  type: string
  apiKey: string
  baseUrl: string
  enabled: boolean
}

const EMPTY_FORM: FormState = {
  name: "",
  type: "kimi_search_api",
  apiKey: "",
  baseUrl: DEFAULT_BASE_URL,
  enabled: false,
}

function getTypeLabel(type: string) {
  return SEARCH_TYPE_OPTIONS.find((item) => item.value === type)?.label ?? type
}

export function SearchConfigsPage() {
  const { confirm } = useConfirmDialog()
  const [configs, setConfigs] = useState<SearchConfig[]>([])
  const [loading, setLoading] = useState(true)
  const [pendingId, setPendingId] = useState<number | null>(null)
  const [dialogState, setDialogState] = useState<DialogState>(null)
  const [testDialogConfig, setTestDialogConfig] = useState<SearchConfig | null>(null)
  const [form, setForm] = useState<FormState>(EMPTY_FORM)
  const [showApiKey, setShowApiKey] = useState(false)
  const [saving, setSaving] = useState(false)
  const [testing, setTesting] = useState(false)
  const [testQuery, setTestQuery] = useState("matrixops")
  const [testResult, setTestResult] = useState("")

  const enabledConfig = useMemo(() => configs.find((item) => item.enabled) ?? null, [configs])
  const enabledCount = useMemo(() => configs.filter((item) => item.enabled).length, [configs])

  const loadConfigs = useCallback(async () => {
    try {
      setLoading(true)
      const data = await api.getSearchConfigs()
      setConfigs(data)
    } catch (error) {
      const message = error instanceof Error ? error.message : "加载搜索配置失败"
      toast.error(message)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void loadConfigs()
  }, [loadConfigs])

  useEffect(() => {
    if (!dialogState) {
      setForm(EMPTY_FORM)
      setShowApiKey(false)
      return
    }
    if (dialogState.mode === "edit") {
      const config = dialogState.config
      setForm({
        name: config.name,
        type: config.type,
        apiKey: config.apiKey,
        baseUrl: config.baseUrl || DEFAULT_BASE_URL,
        enabled: config.enabled,
      })
      setShowApiKey(false)
      return
    }
    setForm(EMPTY_FORM)
    setShowApiKey(false)
  }, [dialogState])

  useEffect(() => {
    if (!testDialogConfig) {
      setTestQuery("matrixops")
      setTestResult("")
      return
    }
    setTestQuery("matrixops")
    setTestResult("")
  }, [testDialogConfig])

  const handleSave = async () => {
    if (!form.name.trim()) {
      toast.error("请填写配置名称")
      return
    }
    if (!form.apiKey.trim()) {
      toast.error("请填写 API Key")
      return
    }

    const payload: SearchConfigCreate | SearchConfigUpdate = {
      name: form.name.trim(),
      type: form.type,
      apiKey: form.apiKey.trim(),
      baseUrl: form.baseUrl.trim() || DEFAULT_BASE_URL,
      enabled: form.enabled,
    }

    try {
      setSaving(true)
      if (dialogState?.mode === "create") {
        await api.createSearchConfig(payload as SearchConfigCreate)
        toast.success("搜索配置已创建")
      } else if (dialogState?.mode === "edit") {
        await api.updateSearchConfig(dialogState.config.id, payload)
        toast.success("搜索配置已更新")
      }
      setDialogState(null)
      await loadConfigs()
    } catch (error) {
      const message = error instanceof Error ? error.message : "保存失败"
      toast.error(message)
    } finally {
      setSaving(false)
    }
  }

  const handleDelete = async (config: SearchConfig) => {
    const confirmed = await confirm({
      title: "删除搜索配置",
      description: `确定删除 “${config.name}” 吗？`,
      confirmLabel: "删除",
      tone: "destructive",
    })
    if (!confirmed) return

    try {
      setPendingId(config.id)
      await api.deleteSearchConfig(config.id)
      toast.success("搜索配置已删除")
      if (dialogState?.mode === "edit" && dialogState.config.id === config.id) {
        setDialogState(null)
      }
      if (testDialogConfig?.id === config.id) {
        setTestDialogConfig(null)
      }
      await loadConfigs()
    } catch (error) {
      const message = error instanceof Error ? error.message : "删除失败"
      toast.error(message)
    } finally {
      setPendingId(null)
    }
  }

  const handleToggleEnabled = async (config: SearchConfig, nextEnabled: boolean) => {
    try {
      setPendingId(config.id)
      await api.updateSearchConfig(config.id, { enabled: nextEnabled })
      toast.success(nextEnabled ? "已启用搜索插件" : "已停用搜索插件")
      if (dialogState?.mode === "edit" && dialogState.config.id === config.id) {
        setForm((prev) => ({ ...prev, enabled: nextEnabled }))
      }
      await loadConfigs()
    } catch (error) {
      const message = error instanceof Error ? error.message : "更新失败"
      toast.error(message)
      await loadConfigs()
    } finally {
      setPendingId(null)
    }
  }

  const handleTest = async () => {
    if (!testDialogConfig) return
    if (!testQuery.trim()) {
      toast.error("请填写测试关键词")
      return
    }
    try {
      setTesting(true)
      const result = await api.testSearchConfig(testDialogConfig.id, {
        query: testQuery.trim(),
        limit: 3,
      })
      setTestResult(JSON.stringify(result, null, 2))
      toast.success("搜索测试成功")
    } catch (error) {
      const message = error instanceof Error ? error.message : "测试失败"
      toast.error(message)
    } finally {
      setTesting(false)
    }
  }

  return (
    <div className="flex-1 overflow-y-auto p-8">
      <div className="mx-auto max-w-6xl space-y-4">
        <div className="space-y-1">
          <h1 className="text-lg font-semibold">搜索配置</h1>
          <p className="text-sm text-muted-foreground">
            启用搜索插件后，AI 将自动获得 `web_search` 工具用于联网搜索。
            {enabledConfig ? ` 当前启用：${enabledConfig.name}` : " 当前没有启用的搜索插件。"}
          </p>
        </div>

        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="flex flex-wrap items-center gap-2">
            <Badge variant="outline">配置 {configs.length}</Badge>
            <Badge variant="outline">已启用 {enabledCount}</Badge>
          </div>
          <div className="flex items-center gap-2">
            <Button variant="outline" onClick={loadConfigs} disabled={loading}>
              <RefreshCw className={cn("mr-2 h-4 w-4", loading && "animate-spin")} />
              刷新
            </Button>
            <Button onClick={() => setDialogState({ mode: "create", config: null })}>
              <Plus className="mr-2 h-4 w-4" />
              新建配置
            </Button>
          </div>
        </div>

        {loading ? (
          <div className="space-y-4">
            {Array.from({ length: 3 }).map((_, index) => (
              <Skeleton key={index} className="h-36 rounded-xl" />
            ))}
          </div>
        ) : configs.length === 0 ? (
          <div className="flex h-[280px] flex-col items-center justify-center rounded-xl border border-dashed border-border/70 bg-muted/10 text-center">
            <div className="mb-4 rounded-xl bg-muted p-3">
              <Search className="h-6 w-6 text-muted-foreground" />
            </div>
            <h3 className="text-base font-medium">还没有搜索配置</h3>
            <p className="mt-1 max-w-md text-sm text-muted-foreground">
              添加 Kimi Search API 等搜索插件，启用后 Agent 可使用 `web_search` 联网搜索。
            </p>
            <Button className="mt-4" onClick={() => setDialogState({ mode: "create", config: null })}>
              <Plus className="mr-2 h-4 w-4" />
              新建配置
            </Button>
          </div>
        ) : (
          <div className="space-y-3">
            {configs.map((config) => {
              const pending = pendingId === config.id
              return (
                <Card key={config.id} className="border-border/60 bg-background/80">
                  <CardHeader className="space-y-2 pb-2">
                    <div className="flex flex-wrap items-start justify-between gap-3">
                      <div className="min-w-0">
                        <CardTitle className="flex flex-wrap items-center gap-2 text-base">
                          <span className="truncate">{config.name}</span>
                          <Badge variant="secondary">{getTypeLabel(config.type)}</Badge>
                          <Badge variant={config.enabled ? "default" : "outline"}>
                            {config.enabled ? "已启用" : "已停用"}
                          </Badge>
                        </CardTitle>
                        <CardDescription className="mt-1 break-all text-xs">
                          {config.baseUrl || DEFAULT_BASE_URL}
                        </CardDescription>
                      </div>
                      <div className="flex flex-wrap items-center gap-2">
                        <Switch
                          checked={config.enabled}
                          disabled={pending}
                          onCheckedChange={(checked) => handleToggleEnabled(config, checked)}
                        />
                        <Button
                          variant="outline"
                          size="sm"
                          className="h-8"
                          onClick={() => setTestDialogConfig(config)}
                        >
                          <Search className="mr-2 h-4 w-4" />
                          测试
                        </Button>
                        <Button
                          variant="outline"
                          size="sm"
                          className="h-8"
                          onClick={() => setDialogState({ mode: "edit", config })}
                        >
                          <Pencil className="mr-2 h-4 w-4" />
                          编辑
                        </Button>
                        <Button
                          variant="destructive"
                          size="sm"
                          className="h-8"
                          disabled={pending}
                          onClick={() => handleDelete(config)}
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

      <Dialog open={dialogState !== null} onOpenChange={(open) => !open && setDialogState(null)}>
        <DialogContent className="max-w-lg">
          <DialogHeader>
            <DialogTitle>{dialogState?.mode === "edit" ? "编辑搜索配置" : "新建搜索配置"}</DialogTitle>
            <DialogDescription>配置搜索插件后，Agent 可使用 `web_search` 进行联网搜索。同时只能启用一个插件。</DialogDescription>
          </DialogHeader>
          <div className="space-y-4">
            <div className="grid gap-4 sm:grid-cols-2">
              <div className="space-y-2 sm:col-span-1">
                <Label htmlFor="search-name">配置名称</Label>
                <Input
                  id="search-name"
                  value={form.name}
                  onChange={(event) => setForm((prev) => ({ ...prev, name: event.target.value }))}
                  placeholder="例如：Kimi 搜索"
                />
              </div>
              <div className="space-y-2 sm:col-span-1">
                <Label htmlFor="search-type">类型</Label>
                <Combobox
                  id="search-type"
                  items={SEARCH_TYPE_OPTIONS}
                  value={form.type}
                  onValueChange={(value) => setForm((prev) => ({ ...prev, type: value }))}
                  placeholder="选择搜索类型"
                  searchPlaceholder="搜索类型"
                  emptyText="未找到类型"
                />
              </div>
            </div>

            <div className="space-y-2">
              <Label htmlFor="search-base-url">Base URL</Label>
              <Input
                id="search-base-url"
                value={form.baseUrl}
                onChange={(event) => setForm((prev) => ({ ...prev, baseUrl: event.target.value }))}
                placeholder={DEFAULT_BASE_URL}
              />
              <p className="text-xs text-muted-foreground">
                默认 `{DEFAULT_BASE_URL}`，实际搜索端点为 `{DEFAULT_BASE_URL}/v1/search`
              </p>
            </div>

            <div className="space-y-2">
              <Label htmlFor="search-api-key">API Key</Label>
              <InputGroup>
                <InputGroupInput
                  id="search-api-key"
                  type={showApiKey ? "text" : "password"}
                  value={form.apiKey}
                  onChange={(event) => setForm((prev) => ({ ...prev, apiKey: event.target.value }))}
                  placeholder="输入 Kimi Search API Key"
                />
                <InputGroupAddon align="inline-end">
                  <InputGroupButton
                    variant="ghost"
                    size="icon-xs"
                    aria-label={showApiKey ? "隐藏 API Key" : "显示 API Key"}
                    onClick={() => setShowApiKey((prev) => !prev)}
                  >
                    {showApiKey ? <EyeOff /> : <Eye />}
                  </InputGroupButton>
                </InputGroupAddon>
              </InputGroup>
            </div>

            <div className="flex items-center justify-between rounded-lg border px-3 py-3">
              <div>
                <div className="text-sm font-medium">启用此搜索插件</div>
                <div className="text-xs text-muted-foreground">同时只能启用一个搜索插件</div>
              </div>
              <Switch
                checked={form.enabled}
                disabled={dialogState?.mode === "edit" && pendingId === dialogState.config.id}
                onCheckedChange={(checked) => {
                  setForm((prev) => ({ ...prev, enabled: checked }))
                  if (dialogState?.mode === "edit") {
                    void handleToggleEnabled(dialogState.config, checked)
                  }
                }}
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDialogState(null)}>
              取消
            </Button>
            <Button onClick={handleSave} disabled={saving}>
              {saving ? "保存中..." : "保存配置"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={testDialogConfig !== null} onOpenChange={(open) => !open && setTestDialogConfig(null)}>
        <DialogContent className="flex max-h-[80vh] max-w-2xl flex-col overflow-hidden">
          <DialogHeader className="shrink-0">
            <DialogTitle>测试搜索：{testDialogConfig?.name}</DialogTitle>
            <DialogDescription>使用当前保存的配置发起一次搜索请求，验证 API Key 与 Base URL 是否可用。</DialogDescription>
          </DialogHeader>
          <div className="min-h-0 flex-1 space-y-3 overflow-y-auto overscroll-contain pr-2">
            <div className="space-y-2">
              <Label htmlFor="search-test-query">测试关键词</Label>
              <Input
                id="search-test-query"
                value={testQuery}
                onChange={(event) => setTestQuery(event.target.value)}
                placeholder="输入测试关键词"
              />
            </div>
            {testResult && (
              <Textarea value={testResult} readOnly className="min-h-[220px] font-mono text-xs" />
            )}
          </div>
          <DialogFooter className="shrink-0">
            <Button variant="outline" onClick={() => setTestDialogConfig(null)}>
              关闭
            </Button>
            <Button onClick={handleTest} disabled={testing}>
              {testing ? "测试中..." : "测试连接"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
