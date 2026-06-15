import React, { useEffect, useMemo, useState } from "react"
import { Loader2, RefreshCw, Play } from "lucide-react"

import { api, type LLMConfig } from "@/lib/api"
import { useNotification } from "@/components/NotificationProvider"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Combobox, type ComboboxOption } from "@/components/ui/combobox"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Textarea } from "@/components/ui/textarea"
import { cn } from "@/lib/utils"

const DEFAULT_TEMPERATURE = "0.7"
const DEFAULT_MAX_TOKENS = "512"

export function DebugPage() {
  const { notify } = useNotification()
  const [configs, setConfigs] = useState<LLMConfig[]>([])
  const [loadingConfigs, setLoadingConfigs] = useState(false)
  const [selectedConfigId, setSelectedConfigId] = useState("")
  const [model, setModel] = useState("")
  const [temperature, setTemperature] = useState(DEFAULT_TEMPERATURE)
  const [maxTokens, setMaxTokens] = useState(DEFAULT_MAX_TOKENS)
  const [input, setInput] = useState("")
  const [response, setResponse] = useState("")
  const [error, setError] = useState<string | null>(null)
  const [isRunning, setIsRunning] = useState(false)

  const loadConfigs = async () => {
    setLoadingConfigs(true)
    try {
      const [configData, defaultConfig] = await Promise.all([
        api.getLLMConfigs(),
        api.getDefaultLLMConfig().catch(() => null),
      ])
      setConfigs(configData)
      if (configData.length > 0) {
        const defaultId = defaultConfig?.id ?? configData[0].id
        setSelectedConfigId(String(defaultId))
      } else {
        setSelectedConfigId("")
      }
    } catch (err) {
      notify({
        type: "error",
        title: "加载 LLM 配置失败",
        description: err instanceof Error ? err.message : "未知错误",
      })
    } finally {
      setLoadingConfigs(false)
    }
  }

  useEffect(() => {
    loadConfigs()
  }, [])

  const selectedConfig = useMemo(
    () => configs.find((config) => String(config.id) === selectedConfigId),
    [configs, selectedConfigId]
  )
  const configOptions = useMemo<ComboboxOption[]>(
    () =>
      configs.map((config) => ({
        value: String(config.id),
        label: config.name,
        description: config.type,
        searchText: `${config.name} ${config.type} ${config.model || ""}`,
      })),
    [configs]
  )

  const availableModels = useMemo(() => {
    if (!selectedConfig?.model) return []
    return selectedConfig.model
      .split(",")
      .map((item) => item.trim())
      .filter(Boolean)
  }, [selectedConfig])
  const modelOptions = useMemo<ComboboxOption[]>(
    () =>
      availableModels.map((item) => ({
        value: item,
        label: item,
        searchText: item,
      })),
    [availableModels]
  )

  useEffect(() => {
    if (!selectedConfig) {
      setModel("")
      return
    }
    const nextModel = availableModels[0] ?? selectedConfig.model
    setModel(nextModel)
  }, [selectedConfigId, selectedConfig, availableModels])

  const handleRun = async (event?: React.FormEvent) => {
    event?.preventDefault()
    setError(null)

    if (!input.trim()) {
      setError("请输入调试内容")
      return
    }

    if (!selectedConfigId) {
      setError("请先选择 Provider 配置")
      return
    }

    const temperatureValue = Number(temperature)
    const maxTokensValue = Number(maxTokens)

    setIsRunning(true)
    setResponse("")

    try {
      const result = await api.debugLLM({
        input: input.trim(),
        configId: Number(selectedConfigId),
        model: model.trim(),
        temperature: Number.isFinite(temperatureValue) ? temperatureValue : undefined,
        maxTokens: Number.isFinite(maxTokensValue) ? maxTokensValue : undefined,
      })
      setResponse(result.text)
    } catch (err) {
      const message = err instanceof Error ? err.message : "调用失败"
      setError(message)
      notify({ type: "error", title: "调试调用失败", description: message })
    } finally {
      setIsRunning(false)
    }
  }

  return (
    <div className="flex-1 p-8 overflow-y-auto">
      <div className="max-w-7xl mx-auto space-y-6">
        <div className="flex items-center justify-end gap-4">
          <Button
            variant="outline"
            size="sm"
            onClick={loadConfigs}
            disabled={loadingConfigs}
          >
            {loadingConfigs ? (
              <Loader2 className="h-4 w-4 mr-2 animate-spin" />
            ) : (
              <RefreshCw className="h-4 w-4 mr-2" />
            )}
            刷新配置
          </Button>
        </div>

        <div className="grid gap-6 lg:grid-cols-[320px_minmax(0,1fr)]">
          <Card className="h-fit">
            <CardHeader>
              <CardTitle className="text-lg">调试参数</CardTitle>
              <CardDescription>选择 Provider、模型和推理参数。</CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="space-y-2">
                <Label>Provider</Label>
                <Combobox
                  id="debug-provider"
                  items={configOptions}
                  value={selectedConfigId}
                  onValueChange={(value) => setSelectedConfigId(value)}
                  placeholder={configs.length === 0 ? "暂无配置" : "选择配置"}
                  searchPlaceholder="搜索 Provider 配置"
                  emptyText="未找到配置"
                  disabled={configs.length === 0}
                />
                {configs.length === 0 && (
                  <p className="text-xs text-muted-foreground">
                    请先在设置中配置 Provider。
                  </p>
                )}
              </div>

              <div className="space-y-2">
                <Label>模型名</Label>
                {availableModels.length > 0 ? (
                  <Combobox
                    id="debug-model"
                    items={modelOptions}
                    value={model}
                    onValueChange={setModel}
                    placeholder={selectedConfigId ? "选择模型" : "请先选择 Provider"}
                    searchPlaceholder="搜索模型"
                    emptyText="未找到模型"
                    disabled={!selectedConfigId}
                  />
                ) : (
                  <Input
                    placeholder={selectedConfigId ? "输入模型名" : "请先选择 Provider"}
                    value={model}
                    onChange={(event) => setModel(event.target.value)}
                    disabled={!selectedConfigId}
                  />
                )}
              </div>

              <div className="space-y-2">
                <Label htmlFor="debug-temperature">温度</Label>
                <Input
                  id="debug-temperature"
                  type="number"
                  min="0"
                  max="2"
                  step="0.1"
                  value={temperature}
                  onChange={(event) => setTemperature(event.target.value)}
                />
                <p className="text-[11px] text-muted-foreground">推荐 0.2 - 1.0，越高越发散。</p>
              </div>

              <div className="space-y-2">
                <Label htmlFor="debug-max-tokens">最大输出 tokens</Label>
                <Input
                  id="debug-max-tokens"
                  type="number"
                  min="1"
                  step="1"
                  value={maxTokens}
                  onChange={(event) => setMaxTokens(event.target.value)}
                />
                <p className="text-[11px] text-muted-foreground">限制输出长度，避免过长响应。</p>
              </div>
            </CardContent>
          </Card>

          <div className="space-y-6">
            <Card>
              <CardHeader>
                <CardTitle className="text-lg">输入</CardTitle>
                <CardDescription>输入单轮提示词，点击运行查看响应。</CardDescription>
              </CardHeader>
              <CardContent>
                <form className="space-y-4" onSubmit={handleRun}>
                  <div className="space-y-2">
                    <Label htmlFor="debug-input">提示词</Label>
                    <Textarea
                      id="debug-input"
                      value={input}
                      onChange={(event) => setInput(event.target.value)}
                      placeholder="例如：总结下面这段文字..."
                      rows={6}
                    />
                  </div>

                  {error && (
                    <div className="rounded-md border border-destructive/30 bg-destructive/5 px-3 py-2 text-xs text-destructive">
                      {error}
                    </div>
                  )}

                  <div className="flex items-center gap-3">
                    <Button type="submit" disabled={isRunning}>
                      {isRunning ? (
                        <Loader2 className="h-4 w-4 mr-2 animate-spin" />
                      ) : (
                        <Play className="h-4 w-4 mr-2" />
                      )}
                      运行调试
                    </Button>
                    <span className={cn("text-xs", isRunning ? "text-primary" : "text-muted-foreground")}>
                      {isRunning ? "模型正在生成响应..." : "单轮调用，不会保存上下文"}
                    </span>
                  </div>
                </form>
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle className="text-lg">响应</CardTitle>
                <CardDescription>模型返回的文本内容。</CardDescription>
              </CardHeader>
              <CardContent>
                <div className="min-h-[220px] rounded-md border border-border/60 bg-muted/30 p-4 text-sm">
                  {response ? (
                    <pre className="whitespace-pre-wrap font-sans text-foreground">{response}</pre>
                  ) : (
                    <div className="text-muted-foreground text-sm">
                      暂无响应内容
                    </div>
                  )}
                </div>
              </CardContent>
            </Card>
          </div>
        </div>
      </div>
    </div>
  )
}
