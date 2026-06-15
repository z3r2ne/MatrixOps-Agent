import React, { useCallback, useEffect, useState } from "react"
import { Plus, Edit, Trash2, SlidersHorizontal } from "lucide-react"

import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Switch } from "@/components/ui/switch"
import { Textarea } from "@/components/ui/textarea"
import { useConfirmDialog } from "@/components/ConfirmDialogProvider"
import { api, type ModelSettings, type ModelSettingsCreate } from "@/lib/api"
import { toast } from "sonner"

interface ModelSettingsSectionProps {
  onChanged?: () => void | Promise<void>
}

function formatLimitK(value: number) {
  if (!value) return "0K"
  const inK = value / 1000
  return Number.isInteger(inK) ? `${inK}K` : `${inK.toFixed(1)}K`
}

function formatOptionalLimitK(value?: number | null) {
  if (!value) return "未设置"
  return formatLimitK(value)
}

function parseKInput(value: string) {
  const parsed = Number(value)
  if (!Number.isFinite(parsed) || parsed <= 0) {
    return 0
  }
  return Math.round(parsed * 1000)
}

function toKInputValue(value?: number) {
  if (!value) return "0"
  const inK = value / 1000
  return Number.isInteger(inK) ? String(inK) : inK.toFixed(1)
}

function toOptionalKInputValue(value?: number | null) {
  if (!value) return ""
  const inK = value / 1000
  return Number.isInteger(inK) ? String(inK) : inK.toFixed(1)
}

function parseOptionalKInput(value: string) {
  const trimmed = value.trim()
  if (trimmed === "") return undefined
  return parseKInput(trimmed)
}

function numberInputValue(value?: number | null) {
  return value ?? ""
}

function parseOptionalNumber(value: string) {
  const trimmed = value.trim()
  if (trimmed === "") return undefined
  const parsed = Number(trimmed)
  return Number.isFinite(parsed) ? parsed : undefined
}

const DEFAULT_MODEL_SETTINGS_NAME = "default_model_config"

type ThinkingMode = "unset" | "enabled" | "disabled"

interface ModelSettingsFormState {
  name: string
  contextLimit: string
  outputLimit: string
  budgetTokens: string
  systemPromptPlacement: "system" | "instruction" | "user_input"
  topP: string
  topK: string
  frequencyPenalty: string
  enableThinking: ThinkingMode
  /** API thinking.type；未设置则不发送该字段 */
  thinkingType: ThinkingMode
  reasoningEffort: "" | "low" | "medium" | "high" | "xhigh" | "none" | "max"
  textVerbosity: "" | "low" | "medium" | "high" | "xhigh"
  enableEncryptedReasoning: ThinkingMode
  parallelToolCalls: ThinkingMode
  enablePromptCacheKey: ThinkingMode
  prompt: string
  nativeOpenAIToolCalls: boolean
  enableSilentToolWatchdog: boolean
}

export function ModelSettingsSection({ onChanged }: ModelSettingsSectionProps) {
  const { confirm } = useConfirmDialog()
  const [settings, setSettings] = useState<ModelSettings[]>([])
  const [loading, setLoading] = useState(true)
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editingName, setEditingName] = useState<string | null>(null)
  const [formData, setFormData] = useState<ModelSettingsFormState>({
    name: "",
    contextLimit: "",
    outputLimit: "",
    budgetTokens: "",
    systemPromptPlacement: "instruction",
    topP: "",
    topK: "",
    frequencyPenalty: "",
    enableThinking: "unset",
    thinkingType: "unset",
    reasoningEffort: "",
    textVerbosity: "",
    enableEncryptedReasoning: "unset",
    parallelToolCalls: "unset",
    enablePromptCacheKey: "unset",
    prompt: "",
    nativeOpenAIToolCalls: false,
  })

  const loadData = useCallback(async () => {
    setLoading(true)
    try {
      const data = await api.getModelSettings()
      setSettings(data.sort((left, right) => left.name.localeCompare(right.name, "zh-CN")))
    } catch (error: any) {
      toast.error(`加载失败: ${error.message}`)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    loadData()
  }, [loadData])

  const resetForm = () => {
    setEditingName(null)
    setFormData({
      name: "",
      contextLimit: "",
      outputLimit: "",
      budgetTokens: "",
      systemPromptPlacement: "instruction",
      topP: "",
      topK: "",
      frequencyPenalty: "",
      enableThinking: "unset",
      thinkingType: "unset",
      reasoningEffort: "",
      textVerbosity: "",
      enableEncryptedReasoning: "unset",
      parallelToolCalls: "unset",
      enablePromptCacheKey: "unset",
      prompt: "",
      nativeOpenAIToolCalls: false,
      enableSilentToolWatchdog: false,
    })
  }

  const handleOpenCreate = () => {
    resetForm()
    setDialogOpen(true)
  }

  const handleOpenEdit = (setting: ModelSettings) => {
    setEditingName(setting.name)
    setFormData({
      name: setting.name,
      contextLimit: toOptionalKInputValue(setting.contextLimit),
      outputLimit: toOptionalKInputValue(setting.outputLimit),
      budgetTokens: setting.budgetTokens == null ? "" : String(setting.budgetTokens),
      systemPromptPlacement: setting.systemPromptPlacement,
      topP: setting.topP == null ? "" : String(setting.topP),
      topK: setting.topK == null ? "" : String(setting.topK),
      frequencyPenalty: setting.frequencyPenalty == null ? "" : String(setting.frequencyPenalty),
      enableThinking:
        setting.enableThinking == null
          ? "unset"
          : setting.enableThinking
            ? "enabled"
            : "disabled",
      thinkingType:
        setting.thinkingType === "enabled" || setting.thinkingType === "disabled"
          ? setting.thinkingType
          : "unset",
      reasoningEffort: setting.reasoningEffort ?? "",
      textVerbosity: setting.textVerbosity ?? "",
      enableEncryptedReasoning:
        setting.enableEncryptedReasoning == null
          ? "unset"
          : setting.enableEncryptedReasoning
            ? "enabled"
            : "disabled",
      parallelToolCalls:
        setting.parallelToolCalls == null
          ? "unset"
          : setting.parallelToolCalls
            ? "enabled"
            : "disabled",
      enablePromptCacheKey:
        setting.enablePromptCacheKey == null
          ? "unset"
          : setting.enablePromptCacheKey
            ? "enabled"
            : "disabled",
      prompt: setting.prompt,
      nativeOpenAIToolCalls: setting.nativeOpenAIToolCalls,
    })
    setDialogOpen(true)
  }

  const handleSave = async () => {
    try {
      const basePayload = {
        name: formData.name.trim(),
        contextLimit: parseOptionalKInput(formData.contextLimit),
        outputLimit: parseOptionalKInput(formData.outputLimit),
        budgetTokens: parseOptionalNumber(formData.budgetTokens),
        systemPromptPlacement: formData.systemPromptPlacement,
        topP: parseOptionalNumber(formData.topP),
        topK: parseOptionalNumber(formData.topK),
        frequencyPenalty: parseOptionalNumber(formData.frequencyPenalty),
        enableThinking:
          formData.enableThinking === "unset"
            ? undefined
            : formData.enableThinking === "enabled",
        thinkingType:
          formData.thinkingType === "unset"
            ? ""
            : formData.thinkingType === "enabled"
              ? "enabled"
              : "disabled",
        reasoningEffort: formData.reasoningEffort || undefined,
        textVerbosity: formData.textVerbosity || undefined,
        enableEncryptedReasoning:
          formData.enableEncryptedReasoning === "unset"
            ? undefined
            : formData.enableEncryptedReasoning === "enabled",
        parallelToolCalls:
          formData.parallelToolCalls === "unset"
            ? undefined
            : formData.parallelToolCalls === "enabled",
        enablePromptCacheKey:
          formData.enablePromptCacheKey === "unset"
            ? undefined
            : formData.enablePromptCacheKey === "enabled",
        prompt: formData.prompt,
        nativeOpenAIToolCalls: formData.nativeOpenAIToolCalls,
        enableSilentToolWatchdog: formData.enableSilentToolWatchdog,
      } satisfies ModelSettingsCreate
      if (editingName) {
        await api.updateModelSetting(editingName, {
          ...basePayload,
          contextLimit: formData.contextLimit.trim() === "" ? null : basePayload.contextLimit,
          outputLimit: formData.outputLimit.trim() === "" ? null : basePayload.outputLimit,
          budgetTokens: formData.budgetTokens.trim() === "" ? null : basePayload.budgetTokens,
          topP: formData.topP.trim() === "" ? null : basePayload.topP,
          topK: formData.topK.trim() === "" ? null : basePayload.topK,
          frequencyPenalty: formData.frequencyPenalty.trim() === "" ? null : basePayload.frequencyPenalty,
          enableThinking: formData.enableThinking === "unset" ? null : basePayload.enableThinking,
          thinkingType: formData.thinkingType === "unset" ? null : basePayload.thinkingType,
          reasoningEffort: formData.reasoningEffort === "" ? null : basePayload.reasoningEffort,
          textVerbosity: formData.textVerbosity === "" ? null : basePayload.textVerbosity,
          enableEncryptedReasoning: formData.enableEncryptedReasoning === "unset" ? null : basePayload.enableEncryptedReasoning,
          parallelToolCalls: formData.parallelToolCalls === "unset" ? null : basePayload.parallelToolCalls,
          enablePromptCacheKey: formData.enablePromptCacheKey === "unset" ? null : basePayload.enablePromptCacheKey,
        })
        toast.success("模型配置已更新")
      } else {
        await api.createModelSetting(basePayload)
        toast.success("模型配置已创建")
      }
      setDialogOpen(false)
      resetForm()
      await loadData()
      await onChanged?.()
    } catch (error: any) {
      toast.error(`保存失败: ${error.message}`)
    }
  }

  const isEditingDefaultSetting = editingName === DEFAULT_MODEL_SETTINGS_NAME

  const handleDelete = async (name: string) => {
    const confirmed = await confirm({
      title: "删除模型配置",
      description: `确定删除模型配置“${name}”吗？`,
      confirmLabel: "删除",
      tone: "destructive",
    })
    if (!confirmed) {
      return
    }
    try {
      await api.deleteModelSetting(name)
      toast.success("模型配置已删除")
      await loadData()
      await onChanged?.()
    } catch (error: any) {
      toast.error(`删除失败: ${error.message}`)
    }
  }

  return (
    <Card>
      <CardHeader>
        <div className="flex flex-wrap items-center justify-between gap-4">
          <div className="min-w-0 flex-1">
            <CardTitle className="flex items-center gap-2">
              <SlidersHorizontal className="h-5 w-5" />
              模型配置
            </CardTitle>
            <CardDescription className="mt-2">
              以模型配置名为唯一键，独立管理上下文限制、模型提示词和原生工具调用开关。
            </CardDescription>
          </div>
          <Button className="shrink-0" onClick={handleOpenCreate}>
            <Plus className="mr-2 h-4 w-4" />
            新建模型配置
          </Button>
        </div>
      </CardHeader>

      <CardContent className="space-y-3">
        {loading ? (
          <div className="py-8 text-center text-sm text-muted-foreground">加载中...</div>
        ) : settings.length === 0 ? (
          <div className="py-8 text-center text-sm text-muted-foreground">暂无模型配置</div>
        ) : (
          settings.map((setting) => (
            <div
              key={setting.name}
              className="flex flex-col gap-3 border border-border/70 bg-background p-4 sm:flex-row sm:items-center sm:justify-between"
            >
              <div className="min-w-0 flex-1">
                <div className="mb-1 flex flex-wrap items-center gap-2">
                  <h4 className="truncate font-medium text-foreground">{setting.name}</h4>
                  {setting.name === DEFAULT_MODEL_SETTINGS_NAME ? (
                    <span className="border border-emerald-200 bg-emerald-50 px-1.5 py-0.5 text-[10px] text-emerald-700">默认</span>
                  ) : null}
                  {setting.nativeOpenAIToolCalls ? (
                    <span className="border border-sky-200 bg-sky-50 px-1.5 py-0.5 text-[10px] text-sky-700">原生工具调用</span>
                  ) : null}
                </div>
                <div className="flex flex-wrap items-center gap-4 text-xs text-muted-foreground">
                  <span>上下文: {formatOptionalLimitK(setting.contextLimit)}</span>
                  <span>输出: {formatOptionalLimitK(setting.outputLimit)}</span>
                  <span>Thinking Budget: {setting.budgetTokens == null ? "未设置" : setting.budgetTokens}</span>
                  <span>提示词位置: {setting.systemPromptPlacement}</span>
                  <span>Top-P: {setting.topP == null ? "未设置" : setting.topP}</span>
                  <span>Top-K: {setting.topK == null ? "未设置" : setting.topK}</span>
                  {setting.prompt ? <span>含模型提示词</span> : <span>无模型提示词</span>}
                </div>
              </div>
              <div className="flex items-center gap-2 sm:ml-4">
                <Button variant="ghost" size="icon" className="h-8 w-8" onClick={() => handleOpenEdit(setting)}>
                  <Edit className="h-4 w-4" />
                </Button>
                <Button
                  variant="ghost"
                  size="icon"
                  className="h-8 w-8 text-destructive hover:text-destructive"
                  onClick={() => handleDelete(setting.name)}
                  disabled={setting.name === DEFAULT_MODEL_SETTINGS_NAME}
                >
                  <Trash2 className="h-4 w-4" />
                </Button>
              </div>
            </div>
          ))
        )}
      </CardContent>

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className="flex max-h-[min(88vh,820px)] w-[calc(100vw-1.5rem)] max-w-2xl flex-col overflow-hidden sm:max-w-[720px]">
          <DialogHeader>
            <DialogTitle>{editingName ? "编辑模型配置" : "新建模型配置"}</DialogTitle>
            <DialogDescription>
              模型配置与 Provider 无关，可被多个 Worker 复用。
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-4 overflow-y-auto py-4 pr-1">
            <div className="space-y-2">
              <Label htmlFor="name">模型配置名 *</Label>
              <Input
                id="name"
                placeholder="例如：default_model_config"
                value={formData.name}
                onChange={(event) => setFormData({ ...formData, name: event.target.value })}
                disabled={isEditingDefaultSetting}
              />
              {isEditingDefaultSetting ? (
                <p className="text-xs text-muted-foreground">
                  默认模型配置名称固定，其他模型配置可在编辑时修改名称。
                </p>
              ) : null}
            </div>

            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <div className="space-y-2">
                <Label htmlFor="contextLimit">上下文限制 (K)</Label>
                <Input
                  id="contextLimit"
                  type="number"
                  step="0.1"
                  value={formData.contextLimit}
                  onChange={(event) => setFormData({ ...formData, contextLimit: event.target.value })}
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="outputLimit">输出限制 (K)</Label>
                <Input
                  id="outputLimit"
                  type="number"
                  step="0.1"
                  value={formData.outputLimit}
                  onChange={(event) => setFormData({ ...formData, outputLimit: event.target.value })}
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="budgetTokens">Thinking Budget Tokens</Label>
                <Input
                  id="budgetTokens"
                  type="number"
                  min="1"
                  step="1"
                  value={formData.budgetTokens}
                  onChange={(event) => setFormData({ ...formData, budgetTokens: event.target.value })}
                  placeholder="留空则不传 budget_tokens"
                />
                <p className="text-xs text-muted-foreground">仅 Anthropic/Claude 原生 thinking 使用；留空则不发送。</p>
              </div>
              <div className="space-y-2">
                <Label htmlFor="systemPromptPlacement">System Prompt Placement</Label>
                <Select
                  value={formData.systemPromptPlacement}
                  onValueChange={(value: "system" | "instruction" | "user_input") =>
                    setFormData({ ...formData, systemPromptPlacement: value })
                  }
                >
                  <SelectTrigger id="systemPromptPlacement">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="system">system</SelectItem>
                    <SelectItem value="instruction">instruction</SelectItem>
                    <SelectItem value="user_input">user_input</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>

            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <div className="space-y-2">
                <Label htmlFor="topP">Top-P</Label>
                <Input
                  id="topP"
                  type="number"
                  min="0"
                  max="1"
                  step="0.1"
                  value={formData.topP}
                  onChange={(event) => setFormData({ ...formData, topP: event.target.value })}
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="topK">Top-K</Label>
                <Input
                  id="topK"
                  type="number"
                  min="0"
                  step="1"
                  value={formData.topK}
                  onChange={(event) => setFormData({ ...formData, topK: event.target.value })}
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="frequencyPenalty">Frequency Penalty</Label>
                <Input
                  id="frequencyPenalty"
                  type="number"
                  min="0"
                  max="2"
                  step="0.1"
                  value={formData.frequencyPenalty}
                  onChange={(event) => setFormData({ ...formData, frequencyPenalty: event.target.value })}
                />
              </div>
            </div>

            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <div className="space-y-2">
                <Label htmlFor="enableThinking">Enable Thinking</Label>
                <Select
                  value={formData.enableThinking}
                  onValueChange={(value: ThinkingMode) => setFormData({ ...formData, enableThinking: value })}
                >
                  <SelectTrigger id="enableThinking">
                    <SelectValue placeholder="未设置" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="unset">未设置</SelectItem>
                    <SelectItem value="enabled">开启</SelectItem>
                    <SelectItem value="disabled">关闭</SelectItem>
                  </SelectContent>
                </Select>
              </div>

              <div className="space-y-2">
                <Label htmlFor="thinkingType">Thinking Type</Label>
                <Select
                  value={formData.thinkingType}
                  onValueChange={(value: ThinkingMode) => setFormData({ ...formData, thinkingType: value })}
                >
                  <SelectTrigger id="thinkingType">
                    <SelectValue placeholder="未设置" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="unset">未设置</SelectItem>
                    <SelectItem value="enabled">enabled</SelectItem>
                    <SelectItem value="disabled">disabled</SelectItem>
                  </SelectContent>
                </Select>
                <p className="text-xs text-muted-foreground">对应原生 OpenAI 请求体 thinking.type；未设置则不发送。</p>
              </div>

              <div className="space-y-2">
                <Label htmlFor="reasoningEffort">Reasoning Effort</Label>
                <Select
                  value={formData.reasoningEffort || "unset"}
                  onValueChange={(
                    value: "unset" | "low" | "medium" | "high" | "xhigh" | "none" | "max"
                  ) => setFormData({ ...formData, reasoningEffort: value === "unset" ? "" : value })}
                >
                  <SelectTrigger id="reasoningEffort">
                    <SelectValue placeholder="未设置" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="unset">未设置</SelectItem>
                    <SelectItem value="low">low</SelectItem>
                    <SelectItem value="medium">medium</SelectItem>
                    <SelectItem value="high">high</SelectItem>
                    <SelectItem value="xhigh">xhigh</SelectItem>
                    <SelectItem value="none">none</SelectItem>
                    <SelectItem value="max">max</SelectItem>
                  </SelectContent>
                </Select>
              </div>

              <div className="space-y-2">
                <Label htmlFor="textVerbosity">Text Verbosity</Label>
                <Select
                  value={formData.textVerbosity || "unset"}
                  onValueChange={(value: "unset" | "low" | "medium" | "high" | "xhigh") =>
                    setFormData({ ...formData, textVerbosity: value === "unset" ? "" : value })
                  }
                >
                  <SelectTrigger id="textVerbosity">
                    <SelectValue placeholder="未设置" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="unset">未设置</SelectItem>
                    <SelectItem value="low">low</SelectItem>
                    <SelectItem value="medium">medium</SelectItem>
                    <SelectItem value="high">high</SelectItem>
                    <SelectItem value="xhigh">xhigh</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>

            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <div className="space-y-2">
                <Label htmlFor="enableEncryptedReasoning">启用 Encrypted Reasoning</Label>
                <Select
                  value={formData.enableEncryptedReasoning}
                  onValueChange={(value: ThinkingMode) => setFormData({ ...formData, enableEncryptedReasoning: value })}
                >
                  <SelectTrigger id="enableEncryptedReasoning">
                    <SelectValue placeholder="未设置" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="unset">未设置</SelectItem>
                    <SelectItem value="enabled">开启</SelectItem>
                    <SelectItem value="disabled">关闭</SelectItem>
                  </SelectContent>
                </Select>
              </div>

              <div className="space-y-2">
                <Label htmlFor="parallelToolCalls">Parallel Tool Calls</Label>
                <Select
                  value={formData.parallelToolCalls}
                  onValueChange={(value: ThinkingMode) => setFormData({ ...formData, parallelToolCalls: value })}
                >
                  <SelectTrigger id="parallelToolCalls">
                    <SelectValue placeholder="未设置" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="unset">未设置</SelectItem>
                    <SelectItem value="enabled">开启</SelectItem>
                    <SelectItem value="disabled">关闭</SelectItem>
                  </SelectContent>
                </Select>
              </div>

              <div className="space-y-2">
                <Label htmlFor="enablePromptCacheKey">Cache Key</Label>
                <Select
                  value={formData.enablePromptCacheKey}
                  onValueChange={(value: ThinkingMode) => setFormData({ ...formData, enablePromptCacheKey: value })}
                >
                  <SelectTrigger id="enablePromptCacheKey">
                    <SelectValue placeholder="未设置" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="unset">未设置</SelectItem>
                    <SelectItem value="enabled">开启</SelectItem>
                    <SelectItem value="disabled">关闭</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>

            <div className="flex items-start justify-between gap-4 border p-3">
              <div className="min-w-0 space-y-0.5 pr-2">
                <Label htmlFor="nativeOpenAIToolCalls" className="text-sm font-medium">
                  原生工具调用（OpenAI 兼容）
                </Label>
                <p className="text-xs text-muted-foreground leading-relaxed">
                  开启后，该模型配置会优先使用 API 原生 tools / tool_calls。
                </p>
              </div>
              <Switch
                id="nativeOpenAIToolCalls"
                className="mt-0.5 shrink-0"
                checked={!!formData.nativeOpenAIToolCalls}
                onCheckedChange={(checked) => setFormData({ ...formData, nativeOpenAIToolCalls: checked })}
              />
            </div>

            <div className="flex items-start justify-between gap-4 border p-3">
              <div className="min-w-0 space-y-0.5 pr-2">
                <Label htmlFor="enableSilentToolWatchdog" className="text-sm font-medium">
                  阶段性总结看门狗
                </Label>
                <p className="text-xs text-muted-foreground leading-relaxed">
                  开启后，若模型连续 10 次调用工具且期间没有任何思考或文本输出，将注入补充系统提示，要求先用一句话总结已完成内容与下一步计划，再继续执行。
                </p>
              </div>
              <Switch
                id="enableSilentToolWatchdog"
                className="mt-0.5 shrink-0"
                checked={!!formData.enableSilentToolWatchdog}
                onCheckedChange={(checked) => setFormData({ ...formData, enableSilentToolWatchdog: checked })}
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="prompt">模型提示词</Label>
              <Textarea
                id="prompt"
                placeholder="输入模型配置对应的提示词..."
                value={formData.prompt || ""}
                onChange={(event) => setFormData({ ...formData, prompt: event.target.value })}
                rows={8}
                className="resize-none"
              />
            </div>
          </div>

          <DialogFooter>
            <Button variant="outline" onClick={() => setDialogOpen(false)}>
              取消
            </Button>
            <Button onClick={handleSave} disabled={!formData.name?.trim()}>
              保存
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </Card>
  )
}
