import React, { useState, useEffect } from "react"
import { Button } from "@/components/ui/button"
import { Combobox, type ComboboxOption } from "@/components/ui/combobox"
import { Input } from "@/components/ui/input"
import { InputGroup, InputGroupAddon, InputGroupButton, InputGroupInput } from "@/components/ui/input-group"
import { Label } from "@/components/ui/label"
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter, DialogDescription } from "@/components/ui/dialog"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Switch } from "@/components/ui/switch"
import { Badge } from "@/components/ui/badge"
import { useConfirmDialog } from "@/components/ConfirmDialogProvider"
import { Plus, Trash2, Edit, Bot, Check, RefreshCw, Eye, EyeOff } from "lucide-react"
import { api, type LLMConfig, type LLMConfigCreate } from "@/lib/api"
import { toast } from "sonner"

const LLM_TYPE_OPTIONS: ComboboxOption[] = [
  { value: "openai", label: "OpenAI", searchText: "openai" },
  { value: "claude", label: "Claude (Anthropic)", searchText: "claude anthropic" },
  { value: "custom", label: "自定义", searchText: "custom 自定义" },
]

const LLM_API_TYPE_OPTIONS: ComboboxOption[] = [
  { value: "response", label: "Response", searchText: "response responses api" },
  { value: "chat", label: "Chat", searchText: "chat completions" },
]

export function LLMConfigSection() {
  const { confirm } = useConfirmDialog()
  const [configs, setConfigs] = useState<LLMConfig[]>([])
  const [defaultLLMConfigId, setDefaultLLMConfigId] = useState<number | null>(null)
  const [loading, setLoading] = useState(true)
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editingConfig, setEditingConfig] = useState<LLMConfig | null>(null)
  const [fetchingModels, setFetchingModels] = useState(false)
  const [showApiKey, setShowApiKey] = useState(false)
  
  // 表单状态
  const [formData, setFormData] = useState<LLMConfigCreate>({
    name: "",
    type: "openai",
    apiKey: "",
    model: "",
    baseUrl: "",
    apiType: "response",
    proxy: "",
    maxRetries: 5,
    nativeOpenAIToolCalls: false,
    isDefault: false,
  })

  // 加载配置列表
  const loadConfigs = async () => {
    try {
      const [data, defaultConfig] = await Promise.all([
        api.getLLMConfigs(),
        api.getDefaultLLMConfig().catch(() => null),
      ])
      setConfigs(data)
      setDefaultLLMConfigId(defaultConfig?.id ?? null)
    } catch (error: any) {
      toast.error(`加载失败: ${error.message}`)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    const fetchConfigs = () => loadConfigs()
    fetchConfigs()
  }, [])

  const parseModels = (value: string) =>
    value
      .split(",")
      .map((item) => item.trim())
      .filter(Boolean)

  const formatModelsPreview = (value: string) => {
    const models = parseModels(value)
    if (models.length === 0) return "未配置"
    if (models.length <= 3) return models.join(", ")
    return `${models.slice(0, 3).join(", ")} 等 ${models.length} 个`
  }

  const supportsAPITypeConfig = (type: LLMConfigCreate["type"]) => type === "openai" || type === "custom"

  const getAPITypeLabel = (value?: string) => value === "chat" ? "Chat" : "Response"

  // 打开新建/编辑对话框
  const handleOpenDialog = (config?: LLMConfig) => {
    setShowApiKey(false)
    if (config) {
      setEditingConfig(config)
      setFormData({
        name: config.name,
        type: config.type,
        apiKey: config.apiKey,
        model: config.model,
        baseUrl: config.baseUrl || "",
        apiType: config.apiType || "response",
        proxy: config.proxy || "",
        maxRetries: config.maxRetries && config.maxRetries > 0 ? config.maxRetries : 5,
        nativeOpenAIToolCalls: !!config.nativeOpenAIToolCalls,
        isDefault: defaultLLMConfigId === config.id,
      })
    } else {
      setEditingConfig(null)
      setFormData({
        name: "",
        type: "openai",
        apiKey: "",
        model: "",
        baseUrl: "",
        apiType: "response",
        proxy: "",
        maxRetries: 5,
        nativeOpenAIToolCalls: false,
        isDefault: false,
      })
    }
    setDialogOpen(true)
  }

  // 保存配置
  const handleSave = async () => {
    try {
      const payload = {
        name: formData.name,
        type: formData.type,
        apiKey: formData.apiKey,
        model: formData.model,
        baseUrl: formData.baseUrl,
        apiType: supportsAPITypeConfig(formData.type) ? (formData.apiType || "response") : "response",
        proxy: formData.proxy,
        maxRetries: formData.maxRetries && formData.maxRetries > 0 ? formData.maxRetries : 5,
        nativeOpenAIToolCalls: !!formData.nativeOpenAIToolCalls,
      }

      let savedConfig: LLMConfig
      if (editingConfig) {
        savedConfig = await api.updateLLMConfig(editingConfig.id, payload)
        toast.success("配置已更新")
      } else {
        savedConfig = await api.createLLMConfig(payload)
        toast.success("配置已创建")
      }
      if (formData.isDefault && savedConfig.id) {
        await api.setDefaultLLMConfig(savedConfig.id)
      }
      setDialogOpen(false)
      await loadConfigs()
    } catch (error: any) {
      toast.error(`保存失败: ${error.message}`)
    }
  }

  // 删除配置
  const handleDelete = async (id: number) => {
    const confirmed = await confirm({
      title: "删除 Provider 配置",
      description: "确定要删除这个配置吗？",
      confirmLabel: "删除",
      tone: "destructive",
    })
    if (!confirmed) return
    
    try {
      await api.deleteLLMConfig(id)
      toast.success("配置已删除")
      loadConfigs()
    } catch (error: any) {
      toast.error(`删除失败: ${error.message}`)
    }
  }

  // 设置为默认
  const handleSetDefault = async (config: LLMConfig) => {
    try {
      await api.setDefaultLLMConfig(config.id)
      toast.success("已设置为默认配置")
      loadConfigs()
    } catch (error: any) {
      toast.error(`设置失败: ${error.message}`)
    }
  }

  // 自动获取模型列表
  const handleFetchModels = async () => {
    if (!formData.apiKey.trim()) {
      toast.error("请先填写 API Key")
      return
    }
    if (formData.type === "custom" && !formData.baseUrl?.trim()) {
      toast.error("自定义模型请先填写 Base URL")
      return
    }

    setFetchingModels(true)
    try {
      const { models } = await api.previewLLMModels({
        type: formData.type,
        apiKey: formData.apiKey.trim(),
        baseUrl: formData.baseUrl?.trim() || "",
        proxy: formData.proxy?.trim() || "",
      })
      if (models && models.length > 0) {
        setFormData(prev => ({
          ...prev,
          model: models.join(",")
        }))
        toast.success(`成功获取 ${models.length} 个模型`)
      } else {
        toast.info("未获取到模型列表")
      }
    } catch (error: any) {
      toast.error(`获取模型列表失败: ${error.message}`)
    } finally {
      setFetchingModels(false)
    }
  }

  // 获取模型类型的显示名称
  const getTypeLabel = (type: string) => {
    const labels: Record<string, string> = {
      openai: "OpenAI",
      claude: "Claude",
      custom: "自定义",
    }
    return labels[type] || type
  }

  return (
    <Card>
      <CardHeader>
        <div className="flex flex-wrap items-center justify-between gap-4">
          <div className="min-w-0 flex-1">
            <CardTitle className="flex items-center gap-2">
              <Bot className="h-5 w-5" />
              Provider 配置
            </CardTitle>
            <CardDescription className="mt-2">
              配置用于 AI 功能的大语言模型，如自动生成提交消息等
            </CardDescription>
          </div>
          <Button className="shrink-0" onClick={() => handleOpenDialog()}>
            <Plus className="h-4 w-4 mr-2" />
            新建配置
          </Button>
        </div>
      </CardHeader>
      
      <CardContent>
        {loading ? (
          <div className="text-center py-8 text-sm text-muted-foreground">加载中...</div>
        ) : configs.length === 0 ? (
          <div className="text-center py-8 text-sm text-muted-foreground">
            暂无配置，点击"新建配置"添加
          </div>
        ) : (
          <div className="space-y-3">
            {configs.map((config) => {
              const isDefault = defaultLLMConfigId === config.id
              const maxRetries = config.maxRetries && config.maxRetries > 0 ? config.maxRetries : 5

              return (
                <div
                  key={config.id}
                  className="flex flex-col gap-3 border p-4 transition-colors hover:bg-accent/50 sm:flex-row sm:items-center sm:justify-between"
                >
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2 mb-1">
                      <h4 className="font-medium truncate">{config.name}</h4>
                      {isDefault && (
                        <Badge variant="default" className="h-5">
                          <Check className="h-3 w-3 mr-1" />
                          默认
                        </Badge>
                      )}
                      <Badge variant="outline" className="h-5">
                        {getTypeLabel(config.type)}
                      </Badge>
                      {supportsAPITypeConfig(config.type) && (
                        <Badge variant="outline" className="h-5">
                          {getAPITypeLabel(config.apiType)}
                        </Badge>
                      )}
                      {config.nativeOpenAIToolCalls && (
                        <Badge variant="secondary" className="h-5">
                          原生工具
                        </Badge>
                      )}
                    </div>
                    <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-muted-foreground">
                      <span
                        className="block min-w-0 max-w-[28rem] truncate"
                        title={config.model}
                      >
                        模型: {formatModelsPreview(config.model)}
                      </span>
                      <span className="shrink-0">失败重试: {maxRetries} 次</span>
                      {config.baseUrl && (
                        <span className="block min-w-0 max-w-[20rem] truncate" title={config.baseUrl}>
                          URL: {config.baseUrl}
                        </span>
                      )}
                      {config.proxy && (
                        <span className="block min-w-0 max-w-[16rem] truncate" title={config.proxy}>
                          代理: {config.proxy}
                        </span>
                      )}
                    </div>
                  </div>
                  
                  <div className="flex flex-wrap items-center gap-2 sm:ml-4 sm:justify-end">
                    {!isDefault && (
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => handleSetDefault(config)}
                        className="h-8"
                      >
                        设为默认
                      </Button>
                    )}
                    <Button
                      variant="ghost"
                      size="icon"
                      onClick={() => handleOpenDialog(config)}
                      className="h-8 w-8"
                    >
                      <Edit className="h-4 w-4" />
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon"
                      onClick={() => handleDelete(config.id)}
                      className="h-8 w-8 text-destructive hover:text-destructive"
                    >
                      <Trash2 className="h-4 w-4" />
                    </Button>
                  </div>
                </div>
              )
            })}
          </div>
        )}
      </CardContent>

      {/* 新建/编辑对话框：加宽、双列、主体可滚动 */}
      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className="flex h-auto max-h-[min(88vh,840px)] w-[calc(100vw-1.5rem)] max-w-3xl flex-col gap-0 overflow-hidden p-0 sm:max-w-3xl">
          <div className="shrink-0 border-b px-6 pb-4 pt-6">
            <DialogHeader className="space-y-1.5 text-left">
              <DialogTitle>{editingConfig ? "编辑配置" : "新建配置"}</DialogTitle>
              <DialogDescription>
                配置大语言模型 API 连接信息
              </DialogDescription>
            </DialogHeader>
          </div>

          <div className="min-h-0 flex-1 overflow-y-auto overscroll-contain px-6 py-4">
            <div className="grid gap-x-6 gap-y-4 sm:grid-cols-2">
              <div className="space-y-2 sm:col-span-1">
                <Label htmlFor="name">配置名称 *</Label>
                <Input
                  id="name"
                  placeholder="例如：GPT-4 配置"
                  value={formData.name}
                  onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                />
              </div>

              <div className="space-y-2 sm:col-span-1">
                <Label htmlFor="type">模型类型 *</Label>
                <Combobox
                  id="type"
                  items={LLM_TYPE_OPTIONS}
                  value={formData.type}
                  onValueChange={(value: any) => setFormData({ ...formData, type: value, model: "" })}
                  placeholder="选择模型类型"
                  searchPlaceholder="搜索模型类型"
                  emptyText="未找到模型类型"
                />
              </div>

              {supportsAPITypeConfig(formData.type) && (
                <div className="space-y-2 sm:col-span-1">
                  <Label htmlFor="apiType">接口类型 *</Label>
                  <Combobox
                    id="apiType"
                    items={LLM_API_TYPE_OPTIONS}
                    value={formData.apiType || "response"}
                    onValueChange={(value: any) => setFormData({ ...formData, apiType: value })}
                    placeholder="选择接口类型"
                    searchPlaceholder="搜索接口类型"
                    emptyText="未找到接口类型"
                  />
                  <p className="text-[10px] text-muted-foreground leading-snug">
                    `Chat` 使用 `/chat/completions`，`Response` 使用 `/responses`
                  </p>
                </div>
              )}

              <div className="space-y-2 sm:col-span-2">
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <Label htmlFor="model" className="shrink-0">
                    模型列表 *
                  </Label>
                  <Button
                    variant="ghost"
                    size="sm"
                    className="h-7 px-2 text-xs"
                    onClick={handleFetchModels}
                    disabled={fetchingModels}
                  >
                    {fetchingModels ? (
                      <RefreshCw className="h-3 w-3 mr-1 animate-spin" />
                    ) : (
                      <RefreshCw className="h-3 w-3 mr-1" />
                    )}
                    获取模型列表
                  </Button>
                </div>
                <Input
                  id="model"
                  placeholder="例如：gpt-4,gpt-3.5-turbo (多个模型用逗号分隔)"
                  value={formData.model}
                  onChange={(e) => setFormData({ ...formData, model: e.target.value })}
                />
                <p className="text-[10px] text-muted-foreground">
                  输入模型名称，多个请使用英文逗号分隔
                </p>
              </div>

              <div className="space-y-2 sm:col-span-2">
                <Label htmlFor="apiKey">API Key *</Label>
                <InputGroup>
                  <InputGroupInput
                    id="apiKey"
                    type={showApiKey ? "text" : "password"}
                    placeholder="输入您的 API Key"
                    value={formData.apiKey}
                    onChange={(e) => setFormData({ ...formData, apiKey: e.target.value })}
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

              <div className="space-y-2 sm:col-span-1">
                <Label htmlFor="baseUrl">Base URL</Label>
                <Input
                  id="baseUrl"
                  placeholder="https://api.openai.com/v1 (留空使用默认值)"
                  value={formData.baseUrl}
                  onChange={(e) => setFormData({ ...formData, baseUrl: e.target.value })}
                />
              </div>

              <div className="space-y-2 sm:col-span-1">
                <Label htmlFor="proxy">HTTP 代理（可选）</Label>
                <Input
                  id="proxy"
                  placeholder="http://127.0.0.1:7890"
                  value={formData.proxy ?? ""}
                  onChange={(e) => setFormData({ ...formData, proxy: e.target.value })}
                />
                <p className="text-[10px] text-muted-foreground leading-snug">
                  须含协议（http/https）。访问该 provider API 时使用，留空则直连。
                </p>
              </div>

              <div className="space-y-2 sm:col-span-1">
                <Label htmlFor="maxRetries">失败后重试次数</Label>
                <Input
                  id="maxRetries"
                  type="number"
                  min={1}
                  step={1}
                  value={formData.maxRetries ?? 5}
                  onChange={(e) =>
                    setFormData({
                      ...formData,
                      maxRetries: Math.max(1, Number(e.target.value) || 5),
                    })
                  }
                />
                <p className="text-[10px] text-muted-foreground leading-snug">
                  默认 5 次；限流、5xx 或网络抖动时自动重试。
                </p>
              </div>

              <div className="flex items-start justify-between gap-4 border p-3 sm:col-span-2">
                <div className="min-w-0 space-y-0.5 pr-2">
                  <Label htmlFor="isDefault" className="text-sm font-medium">
                    设为默认配置
                  </Label>
                  <p className="text-xs text-muted-foreground leading-relaxed">
                    默认配置将用于 AI 功能（如生成提交消息）
                  </p>
                </div>
                <Switch
                  id="isDefault"
                  className="mt-0.5 shrink-0"
                  checked={formData.isDefault}
                  onCheckedChange={(checked) => setFormData({ ...formData, isDefault: checked })}
                />
              </div>

              <div className="sm:col-span-2 border border-dashed p-3 text-xs text-muted-foreground">
                `System Prompt Placement` 已迁移到模型配置，请在“模型配置”中设置。
              </div>
            </div>
          </div>

          <DialogFooter className="shrink-0 gap-2 border-t bg-background px-6 py-4 sm:justify-end">
            <Button variant="outline" onClick={() => setDialogOpen(false)}>
              取消
            </Button>
            <Button
              onClick={handleSave}
              disabled={!formData.name || !formData.apiKey || !formData.model}
            >
              保存
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </Card>
  )
}
