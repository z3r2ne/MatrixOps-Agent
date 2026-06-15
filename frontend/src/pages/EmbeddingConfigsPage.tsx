import React, { useCallback, useEffect, useMemo, useState } from "react"
import { BrainCircuit, Pencil, Plus, RefreshCw, Trash2 } from "lucide-react"
import { toast } from "sonner"

import { useConfirmDialog } from "@/components/ConfirmDialogProvider"
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
import { Textarea } from "@/components/ui/textarea"
import {
  api,
  type EmbeddingConfig,
  type EmbeddingConfigCreate,
  type EmbeddingConfigUpdate,
} from "@/lib/api"

const DEFAULT_BASE_URL = "http://127.0.0.1:8081"

type DialogState =
  | { mode: "create"; config: null }
  | { mode: "edit"; config: EmbeddingConfig }
  | null

type FormState = {
  name: string
  type: "llama_cpp"
  baseUrl: string
  binaryPath: string
  modelPath: string
  dimension: string
  batchSize: string
  maxInputTokens: string
  enabled: boolean
  autoStart: boolean
}

const EMPTY_FORM: FormState = {
  name: "",
  type: "llama_cpp",
  baseUrl: DEFAULT_BASE_URL,
  binaryPath: "",
  modelPath: "",
  dimension: "",
  batchSize: "16",
  maxInputTokens: "512",
  enabled: false,
  autoStart: false,
}

function statusLabel(status?: string) {
  switch (status) {
    case "running":
      return "索引中"
    case "pending":
      return "排队中"
    case "failed":
      return "失败"
    case "ready":
      return "就绪"
    default:
      return "空闲"
  }
}

export function EmbeddingConfigsPage() {
  const { confirm } = useConfirmDialog()
  const [configs, setConfigs] = useState<EmbeddingConfig[]>([])
  const [loading, setLoading] = useState(true)
  const [pendingId, setPendingId] = useState<number | null>(null)
  const [dialogState, setDialogState] = useState<DialogState>(null)
  const [testDialogConfig, setTestDialogConfig] = useState<EmbeddingConfig | null>(null)
  const [form, setForm] = useState<FormState>(EMPTY_FORM)
  const [saving, setSaving] = useState(false)
  const [testing, setTesting] = useState(false)
  const [testSample, setTestSample] = useState("matrixops embedding test")
  const [testResult, setTestResult] = useState("")

  const enabledConfig = useMemo(() => configs.find((item) => item.enabled) ?? null, [configs])

  const loadConfigs = useCallback(async () => {
    try {
      setLoading(true)
      const data = await api.getEmbeddingConfigs()
      setConfigs(data)
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "加载 embedding 配置失败")
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
      return
    }
    if (dialogState.mode === "edit") {
      const config = dialogState.config
      setForm({
        name: config.name,
        type: "llama_cpp",
        baseUrl: config.baseUrl || DEFAULT_BASE_URL,
        binaryPath: config.binaryPath || "",
        modelPath: config.modelPath || "",
        dimension: config.dimension ? String(config.dimension) : "",
        batchSize: String(config.batchSize || 16),
        maxInputTokens: String(config.maxInputTokens || 512),
        enabled: config.enabled,
        autoStart: config.autoStart,
      })
      return
    }
    setForm(EMPTY_FORM)
  }, [dialogState])

  const buildPayload = (): EmbeddingConfigCreate | EmbeddingConfigUpdate => {
    const dimension = Number(form.dimension)
    const batchSize = Number(form.batchSize)
    const maxInputTokens = Number(form.maxInputTokens)
    return {
      name: form.name.trim(),
      type: "llama_cpp",
      baseUrl: form.baseUrl.trim() || DEFAULT_BASE_URL,
      binaryPath: form.binaryPath.trim(),
      modelPath: form.modelPath.trim(),
      dimension: Number.isFinite(dimension) && dimension > 0 ? dimension : undefined,
      batchSize: Number.isFinite(batchSize) && batchSize > 0 ? batchSize : undefined,
      maxInputTokens: Number.isFinite(maxInputTokens) && maxInputTokens > 0 ? maxInputTokens : undefined,
      enabled: form.enabled,
      autoStart: form.autoStart,
    }
  }

  const handleSave = async () => {
    if (!form.name.trim()) {
      toast.error("请填写配置名称")
      return
    }
    try {
      setSaving(true)
      const payload = buildPayload()
      if (dialogState?.mode === "create") {
        await api.createEmbeddingConfig(payload as EmbeddingConfigCreate)
        toast.success("Embedding 配置已创建")
      } else if (dialogState?.mode === "edit") {
        await api.updateEmbeddingConfig(dialogState.config.id, payload)
        toast.success("Embedding 配置已更新")
      }
      setDialogState(null)
      await loadConfigs()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "保存失败")
    } finally {
      setSaving(false)
    }
  }

  const handleDelete = async (config: EmbeddingConfig) => {
    const confirmed = await confirm({
      title: "删除 Embedding 配置",
      description: `确定删除 “${config.name}” 吗？`,
      confirmLabel: "删除",
      tone: "destructive",
    })
    if (!confirmed) return
    try {
      setPendingId(config.id)
      await api.deleteEmbeddingConfig(config.id)
      toast.success("Embedding 配置已删除")
      await loadConfigs()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "删除失败")
    } finally {
      setPendingId(null)
    }
  }

  const handleToggleEnabled = async (config: EmbeddingConfig, nextEnabled: boolean) => {
    try {
      setPendingId(config.id)
      await api.updateEmbeddingConfig(config.id, { enabled: nextEnabled })
      toast.success(nextEnabled ? "已启用 Embedding" : "已停用 Embedding")
      await loadConfigs()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "更新失败")
    } finally {
      setPendingId(null)
    }
  }

  const handleTest = async () => {
    if (!testDialogConfig) return
    try {
      setTesting(true)
      const result = await api.testEmbeddingConfig(testDialogConfig.id, { sample: testSample })
      setTestResult(`维度: ${result.dimension}\n示例: ${result.sample || testSample}`)
      toast.success("Embedding 测试成功")
    } catch (error) {
      setTestResult("")
      toast.error(error instanceof Error ? error.message : "测试失败")
    } finally {
      setTesting(false)
    }
  }

  return (
    <div className="space-y-4 p-4 md:p-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-xl font-semibold">Embedding 配置</h1>
          <p className="text-sm text-muted-foreground">配置本地 llama.cpp embedding 服务，用于记忆语义检索。</p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" onClick={() => void loadConfigs()} disabled={loading}>
            <RefreshCw className="mr-2 h-4 w-4" />
            刷新
          </Button>
          <Button onClick={() => setDialogState({ mode: "create", config: null })}>
            <Plus className="mr-2 h-4 w-4" />
            新建配置
          </Button>
        </div>
      </div>

      {enabledConfig ? (
        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="text-base">当前启用</CardTitle>
            <CardDescription>{enabledConfig.name} · {enabledConfig.baseUrl}</CardDescription>
          </CardHeader>
        </Card>
      ) : null}

      {loading ? (
        <div className="space-y-3">
          <Skeleton className="h-24 w-full" />
          <Skeleton className="h-24 w-full" />
        </div>
      ) : configs.length === 0 ? (
        <Card className="border-dashed">
          <CardHeader>
            <CardTitle className="text-base">暂无配置</CardTitle>
            <CardDescription>添加 llama.cpp embedding 服务后即可启用记忆检索。</CardDescription>
          </CardHeader>
        </Card>
      ) : (
        <div className="grid gap-3">
          {configs.map((config) => (
            <Card key={config.id}>
              <CardHeader className="flex flex-row items-start justify-between gap-4 space-y-0">
                <div className="space-y-2">
                  <div className="flex flex-wrap items-center gap-2">
                    <BrainCircuit className="h-4 w-4 text-primary" />
                    <CardTitle className="text-base">{config.name}</CardTitle>
                    {config.enabled ? <Badge>已启用</Badge> : <Badge variant="secondary">未启用</Badge>}
                    <Badge variant="outline">{statusLabel(config.status)}</Badge>
                  </div>
                  <CardDescription className="break-all">
                    {config.baseUrl}
                    {config.modelPath ? ` · ${config.modelPath}` : ""}
                  </CardDescription>
                  {config.lastError ? <p className="text-xs text-destructive">{config.lastError}</p> : null}
                </div>
                <div className="flex flex-wrap items-center gap-2">
                  <Switch
                    checked={config.enabled}
                    disabled={pendingId === config.id}
                    onCheckedChange={(checked) => void handleToggleEnabled(config, checked)}
                  />
                  <Button variant="outline" size="sm" onClick={() => setTestDialogConfig(config)}>
                    测试
                  </Button>
                  <Button variant="outline" size="sm" onClick={() => setDialogState({ mode: "edit", config })}>
                    <Pencil className="h-4 w-4" />
                  </Button>
                  <Button variant="outline" size="sm" onClick={() => void handleDelete(config)} disabled={pendingId === config.id}>
                    <Trash2 className="h-4 w-4" />
                  </Button>
                </div>
              </CardHeader>
            </Card>
          ))}
        </div>
      )}

      <Dialog open={dialogState !== null} onOpenChange={(open) => !open && setDialogState(null)}>
        <DialogContent className="max-w-xl">
          <DialogHeader>
            <DialogTitle>{dialogState?.mode === "edit" ? "编辑 Embedding 配置" : "新建 Embedding 配置"}</DialogTitle>
            <DialogDescription>连接本地 llama.cpp OpenAI-compatible `/v1/embeddings` 服务。</DialogDescription>
          </DialogHeader>
          <div className="grid gap-4 py-2">
            <div className="grid gap-2">
              <Label htmlFor="embedding-name">名称</Label>
              <Input id="embedding-name" value={form.name} onChange={(e) => setForm((prev) => ({ ...prev, name: e.target.value }))} />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="embedding-base-url">Base URL</Label>
              <Input id="embedding-base-url" value={form.baseUrl} onChange={(e) => setForm((prev) => ({ ...prev, baseUrl: e.target.value }))} />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="embedding-model-path">Model Path / Name</Label>
              <Input id="embedding-model-path" value={form.modelPath} onChange={(e) => setForm((prev) => ({ ...prev, modelPath: e.target.value }))} />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="embedding-binary-path">llama.cpp Binary Path</Label>
              <Input id="embedding-binary-path" value={form.binaryPath} onChange={(e) => setForm((prev) => ({ ...prev, binaryPath: e.target.value }))} />
            </div>
            <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
              <div className="grid gap-2">
                <Label htmlFor="embedding-dimension">Dimension</Label>
                <Input id="embedding-dimension" value={form.dimension} onChange={(e) => setForm((prev) => ({ ...prev, dimension: e.target.value }))} />
              </div>
              <div className="grid gap-2">
                <Label htmlFor="embedding-batch-size">Batch Size</Label>
                <Input id="embedding-batch-size" value={form.batchSize} onChange={(e) => setForm((prev) => ({ ...prev, batchSize: e.target.value }))} />
              </div>
              <div className="grid gap-2">
                <Label htmlFor="embedding-max-input">Max Input Tokens</Label>
                <Input id="embedding-max-input" value={form.maxInputTokens} onChange={(e) => setForm((prev) => ({ ...prev, maxInputTokens: e.target.value }))} />
              </div>
            </div>
            <div className="flex items-center justify-between">
              <Label htmlFor="embedding-enabled">启用</Label>
              <Switch id="embedding-enabled" checked={form.enabled} onCheckedChange={(checked) => setForm((prev) => ({ ...prev, enabled: checked }))} />
            </div>
            <div className="flex items-center justify-between">
              <Label htmlFor="embedding-auto-start">自动启动</Label>
              <Switch id="embedding-auto-start" checked={form.autoStart} onCheckedChange={(checked) => setForm((prev) => ({ ...prev, autoStart: checked }))} />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDialogState(null)}>取消</Button>
            <Button onClick={() => void handleSave()} disabled={saving}>{saving ? "保存中..." : "保存"}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={testDialogConfig !== null} onOpenChange={(open) => !open && setTestDialogConfig(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>测试 Embedding</DialogTitle>
            <DialogDescription>{testDialogConfig?.name}</DialogDescription>
          </DialogHeader>
          <div className="grid gap-3">
            <div className="grid gap-2">
              <Label htmlFor="embedding-test-sample">测试文本</Label>
              <Input id="embedding-test-sample" value={testSample} onChange={(e) => setTestSample(e.target.value)} />
            </div>
            {testResult ? <Textarea readOnly value={testResult} rows={4} /> : null}
          </div>
          <DialogFooter>
            <Button onClick={() => void handleTest()} disabled={testing}>{testing ? "测试中..." : "开始测试"}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
