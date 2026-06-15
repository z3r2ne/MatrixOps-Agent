import React, { useEffect, useLayoutEffect, useRef, useState } from "react"

import BatchWorkerConfigDialog from "@/components/workers/BatchWorkerConfigDialog"
import WorkerDialog from "@/components/workers/WorkerDialog"
import { LLMConfigSection } from "@/components/settings/LLMConfigSection"
import { ModelSettingsSection } from "@/components/settings/ModelSettingsSection"
import { SETTINGS_NAV_ITEMS, type SettingsTabId } from "@/components/settings/settingsNavConfig"
import { SystemSettingsSection } from "@/components/settings/SystemSettingsSection"
import { WorkersSection } from "@/components/settings/WorkersSection"
import { api, Worker, WorkerCreate, WorkerUpdate, LLMConfig, ModelSettings, type WorkerBulkApplyConfigRequest } from "@/lib/api"
import { useNotification } from "@/components/NotificationProvider"
import { cn } from "@/lib/utils"

export function SettingsPage() {
  const [workers, setWorkers] = useState<Worker[]>([])
  const [llmConfigs, setLlmConfigs] = useState<LLMConfig[]>([])
  const [modelSettings, setModelSettings] = useState<ModelSettings[]>([])
  const [activeTab, setActiveTab] = useState<SettingsTabId>("workers")
  const [workerDialogOpen, setWorkerDialogOpen] = useState(false)
  const [workerDialogMode, setWorkerDialogMode] = useState<"create" | "edit">("create")
  const [editingWorker, setEditingWorker] = useState<Worker | null>(null)
  const [batchConfigDialogOpen, setBatchConfigDialogOpen] = useState(false)
  const [batchConfigSelectedIds, setBatchConfigSelectedIds] = useState<number[]>([])
  const { notify } = useNotification()
  const settingsScrollRef = useRef<HTMLDivElement | null>(null)

  /** Tab 切换时回到顶部，避免在不同内容高度间切换时停留在旧滚动位置。 */
  useLayoutEffect(() => {
    const el = settingsScrollRef.current
    if (el) {
      el.scrollTop = 0
    }
  }, [activeTab])

  const loadWorkers = async () => {
    try {
      const data = await api.getWorkers()
      setWorkers(data)
    } catch (error) {
      notify({ type: "error", title: "加载 Worker 失败", description: error instanceof Error ? error.message : "未知错误" })
    }
  }

  const loadLLMConfigs = async () => {
    try {
      const data = await api.getLLMConfigs()
      setLlmConfigs(data)
    } catch (error) {
      notify({ type: "error", title: "加载 LLM 配置失败", description: error instanceof Error ? error.message : "未知错误" })
    }
  }

  const loadModelSettings = async () => {
    try {
      const data = await api.getModelSettings()
      setModelSettings(data)
    } catch (error) {
      notify({ type: "error", title: "加载模型配置失败", description: error instanceof Error ? error.message : "未知错误" })
    }
  }

  useEffect(() => {
    switch (activeTab) {
      case "workers":
        loadWorkers()
        loadLLMConfigs()
        loadModelSettings()
        break
      case "providers":
        loadLLMConfigs()
        break
      case "models":
        loadModelSettings()
        break
      case "system":
        // System settings handles its own loading internally via useEffect, 
        // but for a true "reload on tab switch" we'd need to expose a load method 
        // or add a key={activeTab} to force unmount/remount.
        break
    }
  }, [activeTab])

  const openCreateWorker = () => {
    loadLLMConfigs()
    loadModelSettings()
    setWorkerDialogMode("create")
    setEditingWorker(null)
    setWorkerDialogOpen(true)
  }

  const openEditWorker = (worker: Worker) => {
    loadLLMConfigs()
    loadModelSettings()
    setWorkerDialogMode("edit")
    setEditingWorker(worker)
    setWorkerDialogOpen(true)
  }

  const openBatchConfigDialog = (ids: number[]) => {
    loadLLMConfigs()
    loadModelSettings()
    setBatchConfigSelectedIds(ids)
    setBatchConfigDialogOpen(true)
  }

  const handleWorkerSubmit = async (payload: WorkerCreate | WorkerUpdate) => {
    if (workerDialogMode === "create") {
      await api.createWorker(payload as WorkerCreate)
      notify({ type: "success", title: "Worker 已创建" })
    } else if (editingWorker) {
      await api.updateWorker(editingWorker.id, payload as WorkerUpdate)
      notify({ type: "success", title: "Worker 已更新" })
    }
    await loadWorkers()
  }

  const deleteWorker = async (worker: Worker) => {
    try {
      await api.deleteWorker(worker.id)
      notify({ type: "success", title: "Worker 已删除" })
      await loadWorkers()
    } catch (error) {
      notify({ type: "error", title: "删除失败", description: error instanceof Error ? error.message : "未知错误" })
    }
  }

  const deleteWorkers = async (ids: number[]) => {
    if (ids.length === 0) return
    try {
      for (const id of ids) {
        await api.deleteWorker(id)
      }
      notify({ type: "success", title: "Worker 已删除", description: `已删除 ${ids.length} 个` })
      await loadWorkers()
    } catch (error) {
      notify({ type: "error", title: "删除失败", description: error instanceof Error ? error.message : "未知错误" })
    }
  }

  const exportWorkers = async (ids: number[]) => {
    if (ids.length === 0) return
    try {
      const blob = await api.exportWorkers(ids)
      const url = URL.createObjectURL(blob)
      const link = document.createElement("a")
      link.href = url
      link.download = `workers-${new Date().toISOString().slice(0, 19).replace(/[:T]/g, "-")}.zip`
      link.click()
      URL.revokeObjectURL(url)
      notify({ type: "success", title: "导出完成" })
    } catch (error) {
      notify({ type: "error", title: "导出失败", description: error instanceof Error ? error.message : "未知错误" })
    }
  }

  const importWorkers = async (file: File) => {
    try {
      const result = await api.importWorkers(file)
      notify({ type: "success", title: "导入完成", description: `新增 ${result.imported} 个，覆盖 ${result.updated} 个` })
      await loadWorkers()
    } catch (error) {
      notify({ type: "error", title: "导入失败", description: error instanceof Error ? error.message : "未知错误" })
    }
  }

  const restoreDefaultWorkers = async () => {
    try {
      await api.restoreDefaultWorkers()
      notify({ type: "success", title: "默认 Workers 已恢复", description: "所有内置 Workers 已重置为默认配置" })
      await loadWorkers()
    } catch (error) {
      notify({ type: "error", title: "恢复失败", description: error instanceof Error ? error.message : "未知错误" })
    }
  }

  const bulkApplyWorkerConfig = async (payload: WorkerBulkApplyConfigRequest) => {
    try {
      const result = await api.bulkApplyWorkerConfig(payload)
      notify({ type: "success", title: "批量配置已应用", description: `已更新 ${result.updated} 个 Worker` })
      await loadWorkers()
    } catch (error) {
      notify({ type: "error", title: "批量配置失败", description: error instanceof Error ? error.message : "未知错误" })
      throw error
    }
  }

  return (
    <div
      ref={settingsScrollRef}
      className="flex min-h-0 flex-1 overflow-y-auto overscroll-y-contain [scrollbar-gutter:stable]"
    >
      <div className="mx-auto flex w-full max-w-6xl min-w-0 flex-col gap-6 p-4 sm:p-6 lg:p-8">
        <section className="border border-border/60 bg-card/50 p-4 shadow-sm sm:p-5">
          <div className="flex flex-col gap-4">
            <nav aria-label="设置分区" className="grid gap-2 sm:grid-cols-2 xl:grid-cols-4">
              {SETTINGS_NAV_ITEMS.map((item) => {
                const Icon = item.icon
                const isActive = activeTab === item.id

                return (
                  <button
                    key={item.id}
                    type="button"
                    onClick={() => setActiveTab(item.id)}
                    className={cn(
                      "flex min-w-0 items-start gap-3 border px-4 py-3 text-left transition-colors",
                      "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2",
                      isActive
                        ? "border-primary/30 bg-primary/5 text-foreground shadow-sm"
                        : "border-border/60 bg-background hover:bg-accent/50"
                    )}
                  >
                    <Icon className={cn("mt-0.5 h-4 w-4 shrink-0", isActive ? "text-primary" : "text-muted-foreground")} />
                    <div className="min-w-0">
                      <div className="text-sm font-medium">{item.label}</div>
                      <div className="mt-1 text-xs text-muted-foreground">{item.description}</div>
                    </div>
                  </button>
                )
              })}
            </nav>
          </div>
        </section>

        <div className="min-w-0 overflow-x-clip">
          {activeTab === "system" && (
            <SystemSettingsSection key="system" />
          )}

          {activeTab === "providers" && (
            <LLMConfigSection key="providers" />
          )}

          {activeTab === "models" && (
            <ModelSettingsSection key="models" onChanged={loadModelSettings} />
          )}

          {activeTab === "workers" && (
            <WorkersSection
              key="workers"
              workers={workers}
              onCreateWorker={openCreateWorker}
              onEditWorker={openEditWorker}
              onDeleteWorker={deleteWorker}
              onRestoreDefaults={restoreDefaultWorkers}
              onBulkDelete={deleteWorkers}
              onBulkExport={exportWorkers}
              onBulkConfigure={openBatchConfigDialog}
              onImportWorkers={importWorkers}
            />
          )}
        </div>

        <WorkerDialog
          open={workerDialogOpen}
          mode={workerDialogMode}
          providers={[]}
          llmConfigs={llmConfigs}
          modelSettings={modelSettings}
          initial={editingWorker}
          onOpenChange={setWorkerDialogOpen}
          onSubmit={handleWorkerSubmit}
        />
        <BatchWorkerConfigDialog
          open={batchConfigDialogOpen}
          workers={workers}
          llmConfigs={llmConfigs}
          modelSettings={modelSettings}
          initialSelectedIds={batchConfigSelectedIds}
          onOpenChange={setBatchConfigDialogOpen}
          onSubmit={bulkApplyWorkerConfig}
        />
      </div>
    </div>
  )
}
