import React, { useEffect, useMemo, useState } from "react"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import { Combobox, type ComboboxOption } from "@/components/ui/combobox"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Label } from "@/components/ui/label"
import { Worker, LLMConfig, ModelSettings, type WorkerBulkApplyConfigRequest } from "@/lib/api"
import { cn } from "@/lib/utils"

interface BatchWorkerConfigDialogProps {
  open: boolean
  workers: Worker[]
  llmConfigs: LLMConfig[]
  modelSettings: ModelSettings[]
  initialSelectedIds: number[]
  onOpenChange: (open: boolean) => void
  onSubmit: (payload: WorkerBulkApplyConfigRequest) => Promise<void>
}

function formatLimitK(value: number) {
  if (!value) return "0K"
  const inK = value / 1000
  return Number.isInteger(inK) ? `${inK}K` : `${inK.toFixed(1)}K`
}

export default function BatchWorkerConfigDialog({
  open,
  workers,
  llmConfigs,
  modelSettings,
  initialSelectedIds,
  onOpenChange,
  onSubmit,
}: BatchWorkerConfigDialogProps) {
  const [llmConfigId, setLlmConfigId] = useState("")
  const [model, setModel] = useState("")
  const [modelSettingsName, setModelSettingsName] = useState("default_model_config")
  const [selectedIds, setSelectedIds] = useState<Set<number>>(new Set())
  const [isSubmitting, setIsSubmitting] = useState(false)

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

  const selectedLLMConfig = useMemo(
    () => llmConfigs.find((item) => String(item.id) === llmConfigId),
    [llmConfigId, llmConfigs]
  )

  const availableModels = useMemo(
    () => selectedLLMConfig?.model.split(",").map((item) => item.trim()).filter(Boolean) || [],
    [selectedLLMConfig]
  )

  const modelOptions = useMemo<ComboboxOption[]>(
    () =>
      availableModels.map((item) => ({
        value: item,
        label: item,
        searchText: item,
      })),
    [availableModels]
  )

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

  const selectedModelSettings = useMemo(
    () => selectableModelSettings.find((item) => item.name === modelSettingsName),
    [modelSettingsName, selectableModelSettings]
  )

  const selectedCount = selectedIds.size
  const isAllSelected = workers.length > 0 && workers.every((worker) => selectedIds.has(worker.id))

  useEffect(() => {
    if (!open) return
    setLlmConfigId("")
    setModel("")
    setModelSettingsName("default_model_config")
    setSelectedIds(new Set(initialSelectedIds.filter((id) => workers.some((worker) => worker.id === id))))
  }, [open, initialSelectedIds, workers])

  useEffect(() => {
    if (selectedIds.size === 0) return
    const validIds = new Set(workers.map((worker) => worker.id))
    setSelectedIds((prev) => {
      const next = new Set<number>()
      prev.forEach((id) => {
        if (validIds.has(id)) {
          next.add(id)
        }
      })
      return next
    })
  }, [workers, selectedIds.size])

  const toggleWorker = (workerId: number) => {
    setSelectedIds((prev) => {
      const next = new Set(prev)
      if (next.has(workerId)) {
        next.delete(workerId)
      } else {
        next.add(workerId)
      }
      return next
    })
  }

  const handleToggleAll = () => {
    if (isAllSelected) {
      setSelectedIds(new Set())
      return
    }
    setSelectedIds(new Set(workers.map((worker) => worker.id)))
  }

  const handleSubmit = async () => {
    if (!selectedLLMConfig || !model.trim() || selectedCount === 0) {
      return
    }
    setIsSubmitting(true)
    try {
      await onSubmit({
        workerIds: Array.from(selectedIds),
        provider: selectedLLMConfig.name,
        model: model.trim(),
        modelSettingsName: modelSettingsName || "default_model_config",
        llmConfigId: Number(llmConfigId),
      })
      onOpenChange(false)
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="flex max-h-[90vh] w-[min(96vw,72rem)] max-w-5xl flex-col overflow-hidden">
        <DialogHeader>
          <DialogTitle>批量配置 Worker</DialogTitle>
          <DialogDescription>
            统一设置 Provider、模型名和模型配置，然后应用到选中的 Worker。
          </DialogDescription>
        </DialogHeader>

        <div className="grid min-h-0 flex-1 gap-6 overflow-hidden lg:grid-cols-[minmax(0,360px)_minmax(0,1fr)]">
          <div className="min-h-0 space-y-4 overflow-hidden">
            <div className="space-y-2">
              <Label>Provider 配置</Label>
              <Combobox
                id="worker-bulk-llm-config"
                items={llmConfigOptions}
                value={llmConfigId}
                onValueChange={(value) => {
                  setLlmConfigId(value)
                  setModel("")
                }}
                placeholder="选择 Provider 配置"
                searchPlaceholder="搜索 Provider 配置"
                emptyText="未找到配置"
              />
            </div>

            <div className="space-y-2">
              <Label>模型名</Label>
              <Combobox
                id="worker-bulk-model"
                items={modelOptions}
                value={model}
                onValueChange={setModel}
                placeholder={llmConfigId ? "选择模型" : "请先选择 Provider 配置"}
                searchPlaceholder="搜索模型"
                emptyText={llmConfigId ? "未找到模型" : "请先选择 Provider 配置"}
                disabled={!llmConfigId || modelOptions.length === 0}
              />
            </div>

            <div className="space-y-2">
              <Label>模型配置</Label>
              <Combobox
                id="worker-bulk-model-settings"
                items={modelSettingOptions}
                value={modelSettingsName}
                onValueChange={setModelSettingsName}
                placeholder="选择模型配置"
                searchPlaceholder="搜索模型配置"
                emptyText="未找到模型配置"
              />
              {selectedModelSettings ? (
                <div className="rounded-md border border-border/60 bg-muted/30 px-3 py-2 text-xs text-muted-foreground">
                  上下文 {formatLimitK(selectedModelSettings.contextLimit)} · 输出 {formatLimitK(selectedModelSettings.outputLimit)}
                </div>
              ) : null}
            </div>
          </div>

          <div className="flex min-h-0 flex-col overflow-hidden rounded-md border border-border/60">
            <div className="flex flex-wrap items-center justify-between gap-2 border-b border-border/60 bg-muted/20 px-4 py-3">
              <div>
                <div className="text-sm font-medium">选择 Worker</div>
                <div className="text-xs text-muted-foreground">已选 {selectedCount} / {workers.length}</div>
              </div>
              <div className="flex items-center gap-2">
                <Button type="button" variant="outline" size="sm" className="h-8" onClick={handleToggleAll}>
                  {isAllSelected ? "取消全选" : "全选 Worker"}
                </Button>
                <Button type="button" variant="outline" size="sm" className="h-8" onClick={() => setSelectedIds(new Set())}>
                  清空
                </Button>
              </div>
            </div>

            <div className="min-h-0 flex-1 overflow-y-auto p-3">
              {workers.length === 0 ? (
                <div className="flex h-full items-center justify-center text-sm text-muted-foreground">
                  暂无可配置的 Worker
                </div>
              ) : (
                <div className="grid gap-3 md:grid-cols-2">
                  {workers.map((worker) => {
                    const checked = selectedIds.has(worker.id)
                    return (
                      <button
                        key={worker.id}
                        type="button"
                        className={cn(
                          "flex items-start gap-3 rounded-md border p-3 text-left transition-colors",
                          checked
                            ? "border-primary/40 bg-primary/5"
                            : "border-border/60 hover:border-border hover:bg-muted/30"
                        )}
                        onClick={() => toggleWorker(worker.id)}
                      >
                        <Checkbox
                          checked={checked}
                          className="mt-0.5"
                          onCheckedChange={() => toggleWorker(worker.id)}
                          onClick={(event) => event.stopPropagation()}
                        />
                        <div className="min-w-0 flex-1">
                          <div className="flex flex-wrap items-center gap-2">
                            <span className="text-sm font-medium break-all">{worker.name}</span>
                            {worker.occupation ? <Badge variant="outline">{worker.occupation}</Badge> : null}
                          </div>
                          <div className="mt-1 text-xs text-muted-foreground">
                            {worker.provider || "未设置"} · {worker.model || "未设置"} · {worker.modelSettingsName || "default_model_config"}
                          </div>
                        </div>
                      </button>
                    )
                  })}
                </div>
              )}
            </div>
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={isSubmitting}>
            取消
          </Button>
          <Button onClick={handleSubmit} disabled={isSubmitting || !selectedLLMConfig || !model.trim() || selectedCount === 0}>
            应用到 {selectedCount} 个 Worker
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
