import React, { useEffect, useState, useMemo } from "react"

import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Combobox, type ComboboxOption } from "@/components/ui/combobox"
import { Textarea } from "@/components/ui/textarea"
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { api, Provider, Worker, WorkerCreate, WorkerUpdate, LLMConfig, ToolInfo, ModelSettings } from "@/lib/api"

interface WorkerDialogProps {
  open: boolean
  mode: "create" | "edit"
  providers: Provider[]
  llmConfigs: LLMConfig[]
  modelSettings: ModelSettings[]
  initial?: Worker | null
  onOpenChange: (open: boolean) => void
  onSubmit: (payload: WorkerCreate | WorkerUpdate) => Promise<void>
}

const OCCUPATIONS = [
  { value: "analyst", label: "分析师" },
  { value: "coder", label: "研发工程师" },
  { value: "reviewer", label: "验收师" },
  { value: "orchestrator", label: "指挥师" },
  { value: "planner", label: "规划师" },
]

const OCCUPATION_OPTIONS: ComboboxOption[] = OCCUPATIONS.map((item) => ({
  value: item.value,
  label: item.label,
  searchText: `${item.label} ${item.value}`,
}))

const COMPACTION_WORKER_NAME = "compaction"

function formatLimitK(value: number) {
  if (!value) return "0K"
  const inK = value / 1000
  return Number.isInteger(inK) ? `${inK}K` : `${inK.toFixed(1)}K`
}

const WorkerDialog: React.FC<WorkerDialogProps> = ({
  open,
  mode,
  providers,
  llmConfigs,
  modelSettings,
  initial,
  onOpenChange,
  onSubmit,
}) => {
  const [name, setName] = useState("")
  const [occupation, setOccupation] = useState("coder")
  const [llmConfigId, setLlmConfigId] = useState("")
  const [model, setModel] = useState("")
  const [modelSettingsName, setModelSettingsName] = useState("default_model_config")
  const [description, setDescription] = useState("")
  const [temperature, setTemperature] = useState("0.5")
  const [systemPrompt, setSystemPrompt] = useState("")
  const [enabledTools, setEnabledTools] = useState<string[]>([])
  const [enabledSkills, setEnabledSkills] = useState<string[]>([])
  const [toolInfos, setToolInfos] = useState<ToolInfo[]>([])
  const [installedSkills, setInstalledSkills] = useState<{ name: string; description: string }[]>([])
  const [resourceTab, setResourceTab] = useState<"tools" | "skills">("tools")
  const [isSubmitting, setIsSubmitting] = useState(false)

  const toolItems = useMemo(() => {
    return toolInfos.map((toolInfo) => ({
      key: toolInfo.name,
      label: `${toolInfo.verbosName || toolInfo.name} (${toolInfo.name})`,
      description: toolInfo.description || "",
    }))
  }, [toolInfos])

  const allToolNames = useMemo(() => toolItems.map((item) => item.key), [toolItems])
  const enabledSkillSet = useMemo(() => new Set(enabledSkills), [enabledSkills])

  const llmConfigOptions = useMemo<ComboboxOption[]>(
    () =>
      llmConfigs.map((item) => ({
        value: String(item.id),
        label: item.name,
        description: item.type,
        searchText: `${item.name} ${item.type} ${item.model || ""}`,
      })),
    [llmConfigs]
  )

  useEffect(() => {
    if (!open) return
    if (mode === "edit" && initial) {
      setName(initial.name)
      setOccupation(initial.occupation || "coder")
      
      // 优先使用 llmConfigId，如果没有则通过 provider 名称查找
      if (initial.llmConfigId) {
        setLlmConfigId(String(initial.llmConfigId))
      } else if (initial.provider) {
        const config = llmConfigs.find(c => c.name === initial.provider)
        setLlmConfigId(config ? String(config.id) : "")
      } else {
        setLlmConfigId("")
      }
      
      setModel(initial.model || "")
      setModelSettingsName(initial.modelSettingsName || "default_model_config")
      setDescription(initial.description || "")
      setTemperature(String(initial.temperature ?? 0.5))
      setSystemPrompt(initial.systemPrompt || "")
      setEnabledTools(parseEnabledList(initial.enabledTools, allToolNames))
      setEnabledSkills(parseEnabledList(initial.enabledSkills, []))
      return
    }
    setName("")
    setOccupation("coder")
    setLlmConfigId("")
    setModel("")
    setModelSettingsName("default_model_config")
    setDescription("")
    setTemperature("")
    setSystemPrompt("")
    setEnabledTools(allToolNames)
    setEnabledSkills([])
  }, [open, mode, initial, llmConfigs, allToolNames])

  useEffect(() => {
    if (!open) return
    const loadTools = async () => {
      try {
        const data = await api.getTools()
        setToolInfos(data)
      } catch {
        setToolInfos([])
      }
    }
    loadTools()
    const loadSkills = async () => {
      try {
        const data = await api.getSkills(true)
        setInstalledSkills(
          data
            .map((item) => ({
              name: item.name.trim(),
              description: item.description?.trim() || "",
            }))
            .filter((item) => item.name.length > 0)
            .sort((a, b) => a.name.localeCompare(b.name))
        )
      } catch {
        setInstalledSkills([])
      }
    }
    loadSkills()
  }, [open])

  const selectedLLMConfig = llmConfigs.find(c => String(c.id) === llmConfigId)
  const availableModels = selectedLLMConfig?.model.split(',').map(m => m.trim()).filter(Boolean) || []
  const selectedModelSettings = modelSettings.find((item) => item.name === modelSettingsName)
  const selectableModelSettings = useMemo(() => {
    if (modelSettings.some((item) => item.name === "default_model_config")) {
      return modelSettings
    }
    return [
      {
        name: "default_model_config",
        contextLimit: 0,
        outputLimit: 0,
        prompt: "",
        systemPromptPlacement: "system" as const,
        nativeOpenAIToolCalls: false,
      },
      ...modelSettings,
    ]
  }, [modelSettings])
  const modelOptions = useMemo<ComboboxOption[]>(
    () =>
      availableModels.map((item) => ({
        value: item,
        label: item,
        searchText: item,
      })),
    [availableModels]
  )
  const modelSettingOptions = useMemo<ComboboxOption[]>(
    () =>
      selectableModelSettings.map((item) => ({
        value: item.name,
        label: item.name,
        description: `上下文 ${formatLimitK(item.contextLimit)} · 输出 ${formatLimitK(item.outputLimit)}`,
        searchText: `${item.name} ${item.systemPromptPlacement}`,
      })),
    [selectableModelSettings]
  )
  const enabledToolSet = useMemo(() => new Set(enabledTools), [enabledTools])
  const isCompactionWorker =
    (mode === "edit" && initial?.name === COMPACTION_WORKER_NAME) ||
    name.trim().toLowerCase() === COMPACTION_WORKER_NAME

  const handleSubmit = async () => {
    if (!name.trim() || !llmConfigId || !model.trim()) {
      return
    }
    if (mode === "create" && name.trim().toLowerCase() === COMPACTION_WORKER_NAME) {
      return
    }
    setIsSubmitting(true)
    try {
      const toolsPayload = isCompactionWorker ? "[]" : JSON.stringify([...enabledTools].sort())
      const skillsPayload = isCompactionWorker ? "[]" : JSON.stringify([...enabledSkills].sort())
      const temperatureValue = temperature.trim() === "" ? undefined : Number(temperature)
      const temperaturePayload = temperatureValue !== undefined && Number.isFinite(temperatureValue) ? temperatureValue : undefined
      if (mode === "create") {
        const payload: WorkerCreate = {
          name: name.trim(),
          provider: selectedLLMConfig?.name || "llm",
          model: model.trim(),
          modelSettingsName: modelSettingsName || "default_model_config",
          description: description.trim() || OCCUPATIONS.find(o => o.value === occupation)?.label || "",
          temperature: temperaturePayload,
          systemPrompt: systemPrompt.trim(),
          occupation,
          enabledTools: toolsPayload,
          enabledSkills: skillsPayload,
          llmConfigId: llmConfigId ? Number(llmConfigId) : undefined,
          workingDir: "",
        }
        await onSubmit(payload)
      } else {
        const payload: WorkerUpdate = {
          name: name.trim(),
          provider: selectedLLMConfig?.name || "llm",
          model: model.trim(),
          modelSettingsName: modelSettingsName || "default_model_config",
          description: description.trim() || OCCUPATIONS.find(o => o.value === occupation)?.label || "",
          temperature: temperaturePayload,
          systemPrompt: systemPrompt.trim(),
          occupation,
          enabledTools: toolsPayload,
          enabledSkills: skillsPayload,
          llmConfigId: llmConfigId ? Number(llmConfigId) : undefined,
        }
        await onSubmit(payload)
      }
      onOpenChange(false)
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="!flex h-[calc(100dvh-var(--electron-window-chrome-top,0px)-2rem)] max-h-[calc(100dvh-var(--electron-window-chrome-top,0px)-2rem)] w-[min(96vw,88rem)] max-w-7xl flex-col gap-0 overflow-hidden p-0">
        <DialogHeader className="hidden shrink-0 border-b px-6 py-5">
          <DialogTitle>{mode === "create" ? "Create worker" : "Edit worker"}</DialogTitle>
        </DialogHeader>

        <div className="min-h-0 flex-1 overflow-y-auto px-6 py-5 xl:overflow-hidden">
          <div className="grid h-full min-h-0 gap-6 xl:grid-cols-[minmax(0,360px)_minmax(0,1fr)] xl:items-stretch">
            <div className="flex min-h-0 flex-col gap-4 overflow-y-auto pr-1 xl:max-h-full">
              <div className="grid shrink-0 gap-4 md:grid-cols-2 xl:grid-cols-1">
                <div className="space-y-2 md:col-span-2 xl:col-span-1">
                  <Label htmlFor="worker-name">名称</Label>
                  <Input
                    id="worker-name"
                    placeholder="例如：高级研发工程师"
                    value={name}
                    onChange={(event) => setName(event.target.value)}
                    disabled={isCompactionWorker && mode === "edit"}
                  />
                </div>

                <div className="space-y-2">
                  <Label>职业</Label>
                  <Combobox
                    id="worker-occupation"
                    items={OCCUPATION_OPTIONS}
                    value={occupation}
                    onValueChange={setOccupation}
                    placeholder="选择职业"
                    searchPlaceholder="搜索职业"
                    emptyText="未找到职业"
                  />
                </div>

                <div className="space-y-2">
                  <Label htmlFor="worker-temperature">温度</Label>
                  <Input
                    id="worker-temperature"
                    type="number"
                    step="0.1"
                    min="0"
                    max="2"
                    value={temperature}
                    onChange={(event) => setTemperature(event.target.value)}
                  />
                </div>

                <div className="space-y-2 md:col-span-2 xl:col-span-1">
                  <Label htmlFor="worker-description">描述</Label>
                  <Input
                    id="worker-description"
                    placeholder="例如：负责代码实现与优化"
                    value={description}
                    onChange={(event) => setDescription(event.target.value)}
                  />
                </div>

                <div className="space-y-2">
                  <Label>Provider 配置</Label>
                  <Combobox
                    id="worker-llm-config"
                    items={llmConfigOptions}
                    value={llmConfigId}
                    onValueChange={(val) => {
                      setLlmConfigId(val)
                      setModel("")
                    }}
                    placeholder="选择大模型配置"
                    searchPlaceholder="搜索 Provider 配置"
                    emptyText="未找到配置"
                  />
                </div>

                <div className="space-y-2">
                  <Label>模型名</Label>
                  <Combobox
                    id="worker-model"
                    items={modelOptions}
                    value={model}
                    onValueChange={setModel}
                    placeholder={llmConfigId ? "选择模型" : "请先选择 Provider 配置"}
                    searchPlaceholder="搜索模型"
                    emptyText={llmConfigId ? "未找到模型" : "请先选择 Provider 配置"}
                    disabled={!llmConfigId || modelOptions.length === 0}
                  />
                </div>

                <div className="space-y-2 md:col-span-2 xl:col-span-1">
                  <Label>模型配置</Label>
                  <Combobox
                    id="worker-model-settings"
                    items={modelSettingOptions}
                    value={modelSettingsName}
                    onValueChange={setModelSettingsName}
                    placeholder="选择模型配置"
                    searchPlaceholder="搜索模型配置"
                    emptyText="未找到模型配置"
                  />
                  <p className="text-[10px] text-muted-foreground leading-snug">
                    为当前 Worker 指定上下文限制、模型提示词和原生工具调用开关。
                  </p>
                  {selectedModelSettings && (
                    <div className="rounded-md border border-border/60 bg-muted/30 px-3 py-2 text-[11px] text-muted-foreground">
                      <div>上下文: {formatLimitK(selectedModelSettings.contextLimit)}</div>
                      <div>输出: {formatLimitK(selectedModelSettings.outputLimit)}</div>
                    </div>
                  )}
                </div>
              </div>

              <div className="flex shrink-0 flex-col gap-2">
                <Label>{isCompactionWorker ? "额外压缩提示词" : "系统提示词"}</Label>
                <Textarea
                  value={systemPrompt}
                  onChange={(event) => setSystemPrompt(event.target.value)}
                  placeholder={
                    isCompactionWorker
                      ? "可选：补充项目/团队特有的记忆压缩侧重点，会追加到压缩任务 prompt 末尾..."
                      : "输入这个 Worker 的系统提示词..."
                  }
                  className="min-h-[10rem] resize-y"
                  rows={8}
                />
                {isCompactionWorker ? (
                  <p className="text-[11px] text-muted-foreground leading-snug">
                    此提示词不会作为 agent 主循环 system prompt，仅在记忆压缩时作为额外侧重点注入。
                  </p>
                ) : null}
              </div>
            </div>

            {!isCompactionWorker ? (
            <div className="flex min-h-0 flex-col overflow-hidden xl:max-h-full">
              <Tabs
                value={resourceTab}
                onValueChange={(value) => setResourceTab(value as "tools" | "skills")}
                className="shrink-0"
              >
                <TabsList className="grid h-9 w-full grid-cols-2">
                  <TabsTrigger value="tools">启用工具 ({enabledTools.length})</TabsTrigger>
                  <TabsTrigger value="skills">预加载技能 ({enabledSkills.length})</TabsTrigger>
                </TabsList>
              </Tabs>

              {resourceTab === "tools" ? (
                <div className="mt-3 flex min-h-0 flex-1 flex-col gap-3 overflow-hidden">
                  <p className="shrink-0 text-xs text-muted-foreground">
                    Worker 只会看到并尝试调用这里启用的工具，真正是否放行由项目权限决定。
                  </p>
                  <SelectionToolbar
                    summary={`已启用 ${enabledTools.length} / ${toolItems.length} 个工具`}
                    onSelectAll={() => setEnabledTools(allToolNames)}
                    onClear={() => setEnabledTools([])}
                  />
                  <div className="min-h-0 flex-1 overflow-y-auto overscroll-contain pr-1">
                    <div className="grid gap-3 pb-1 sm:grid-cols-2 2xl:grid-cols-3">
                      {toolItems.map((item) => (
                        <SelectionCard
                          key={item.key}
                          checked={enabledToolSet.has(item.key)}
                          title={item.label}
                          description={item.description}
                          onToggle={() =>
                            setEnabledTools((prev) => {
                              const next = new Set(prev)
                              if (next.has(item.key)) {
                                next.delete(item.key)
                              } else {
                                next.add(item.key)
                              }
                              return Array.from(next).sort()
                            })
                          }
                        />
                      ))}
                    </div>
                  </div>
                </div>
              ) : (
                <div className="mt-3 flex min-h-0 flex-1 flex-col gap-3 overflow-hidden">
                  <p className="shrink-0 text-xs text-muted-foreground">
                    选中的技能会在使用该 Worker 时自动注入上下文，无需手动调用 load_skill。
                  </p>
                  <SelectionToolbar
                    summary={`已启用 ${enabledSkills.length} / ${installedSkills.length} 个技能`}
                    onSelectAll={() => setEnabledSkills(installedSkills.map((item) => item.name))}
                    onClear={() => setEnabledSkills([])}
                    selectAllDisabled={installedSkills.length === 0}
                  />
                  <div className="min-h-0 flex-1 overflow-y-auto overscroll-contain pr-1">
                    {installedSkills.length === 0 ? (
                      <div className="rounded-md border border-dashed border-border/60 px-3 py-4 text-xs text-muted-foreground">
                        暂无已安装技能。请先在技能广场安装技能后再配置。
                      </div>
                    ) : (
                      <div className="grid gap-3 pb-1 sm:grid-cols-2">
                        {installedSkills.map((item) => (
                          <SelectionCard
                            key={item.name}
                            checked={enabledSkillSet.has(item.name)}
                            title={item.name}
                            description={item.description}
                            onToggle={() =>
                              setEnabledSkills((prev) => {
                                const next = new Set(prev)
                                if (next.has(item.name)) {
                                  next.delete(item.name)
                                } else {
                                  next.add(item.name)
                                }
                                return [...next].sort()
                              })
                            }
                          />
                        ))}
                      </div>
                    )}
                  </div>
                </div>
              )}
            </div>
            ) : (
              <div className="flex min-h-0 flex-col rounded-md border border-dashed border-border/60 bg-muted/20 px-4 py-6 text-sm text-muted-foreground">
                <p className="font-medium text-foreground">记忆压缩专用 Worker</p>
                <p className="mt-2 leading-relaxed">
                  此 Worker 不参与 agent 主循环，不能配置工具与技能。请在左侧配置 Provider、模型与额外压缩提示词；所有记忆压缩请求都会使用这里的配置。
                </p>
              </div>
            )}
          </div>
        </div>

        <DialogFooter className="shrink-0 border-t px-6 py-4">
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={isSubmitting}>
            Cancel
          </Button>
          <Button onClick={handleSubmit} disabled={isSubmitting}>
            {mode === "create" ? "Create worker" : "Save"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

export default WorkerDialog

interface SelectionToolbarProps {
  summary: string
  onSelectAll: () => void
  onClear: () => void
  selectAllDisabled?: boolean
}

function SelectionToolbar({ summary, onSelectAll, onClear, selectAllDisabled }: SelectionToolbarProps) {
  return (
    <div className="flex shrink-0 items-center justify-between rounded-md border border-border/60 bg-muted/30 px-3 py-2 text-xs text-muted-foreground">
      <span>{summary}</span>
      <div className="flex items-center gap-2">
        <Button type="button" variant="outline" size="sm" className="h-7" onClick={onSelectAll} disabled={selectAllDisabled}>
          全选
        </Button>
        <Button type="button" variant="outline" size="sm" className="h-7" onClick={onClear}>
          清空
        </Button>
      </div>
    </div>
  )
}

interface SelectionCardProps {
  checked: boolean
  title: string
  description?: string
  onToggle: () => void
}

function SelectionCard({ checked, title, description, onToggle }: SelectionCardProps) {
  return (
    <button
      type="button"
      className={`flex w-full items-start gap-3 rounded-md border p-3 text-left transition-colors ${
        checked ? "border-primary/40 bg-primary/5" : "border-border/60 hover:border-border hover:bg-muted/30"
      }`}
      onClick={onToggle}
    >
      <input type="checkbox" checked={checked} readOnly className="mt-0.5 h-4 w-4 shrink-0 rounded border border-border" />
      <div className="min-w-0">
        <p className="text-sm font-medium">{title}</p>
        {description ? <p className="text-xs text-muted-foreground">{description}</p> : null}
      </div>
    </button>
  )
}

function parseEnabledList(raw: string | undefined, fallback: string[]) {
  if (!raw?.trim()) {
    return [...fallback]
  }

  try {
    const parsed = JSON.parse(raw)
    if (!Array.isArray(parsed)) {
      return [...fallback]
    }
    return parsed
      .filter((value): value is string => typeof value === "string" && value.trim().length > 0)
      .sort()
  } catch {
    return [...fallback]
  }
}
