import React, { useCallback, useEffect, useState } from "react"
import { Outlet } from "react-router-dom"
import { Settings } from "lucide-react"

import { Button } from "@/components/ui/button"
import { ElectronSettingsDialog } from "@/components/electron/ElectronSettingsDialog"
import { ElectronOpenSettingsContext, type ElectronOpenSettingsOptions } from "@/components/electron/electron-main-contexts"
import { WorkbenchChatTitleToolbarSetterContext } from "@/components/electron/WorkbenchChatTitleToolbarContext"
import { ElectronWorkbenchChromeLeading, ElectronWorkbenchChromeProvider } from "@/components/electron/ElectronWorkbenchChrome"
import { ElectronWindowChromeSpacer } from "@/components/layout/ElectronWindowChrome"
import { getElectronWindowChromeCSSVars, usesElectronCustomTitleBar } from "@/lib/electron"
import { useWorkbench } from "@/components/workbench/WorkbenchProvider"
import { WechatReloginDialog } from "@/components/settings/WechatReloginDialog"

export function MainLayout() {
  const { tabs, activeTabId } = useWorkbench()
  const [electronSettingsOpen, setElectronSettingsOpen] = useState(false)
  const [electronSettingsInitialPanel, setElectronSettingsInitialPanel] = useState<"settings" | "workspaces" | "projects" | "memory-libraries" | "rag-libraries" | "skills" | "mcp" | "skills-sources" | "logs" | "usage" | "prompts" | "debug">("settings")
  const [electronUsageTaskId, setElectronUsageTaskId] = useState<number | undefined>(undefined)
  const [electronSettingsInitialProjectId, setElectronSettingsInitialProjectId] = useState<number | undefined>(undefined)
  const [electronChatTitleToolbar, setElectronChatTitleToolbar] = useState<React.ReactNode>(null)

  const activeWorkspaceTitle = tabs.find((tab) => tab.id === activeTabId)?.title || "工作台"
  const activeWorkspaceTab = tabs.find((tab) => tab.id === activeTabId) ?? null

  useEffect(() => {
    if (typeof document !== "undefined") {
      document.title = activeWorkspaceTitle
    }
    if (!activeWorkspaceTab || activeWorkspaceTab.kind !== "workspace") {
      return
    }
    void window.electronAPI?.setCurrentWorkspaceWindow?.({
      id: activeWorkspaceTab.workspaceId,
      name: activeWorkspaceTab.title,
    })
  }, [activeWorkspaceTab, activeWorkspaceTitle])

  const openElectronSettings = useCallback((options?: ElectronOpenSettingsOptions) => {
    const el = document.activeElement
    if (el instanceof HTMLElement) {
      el.blur()
    }
    setElectronSettingsInitialPanel(options?.panel || "settings")
    setElectronUsageTaskId(options?.usageTaskId)
    setElectronSettingsInitialProjectId(options?.initialProjectId)
    setElectronSettingsOpen(true)
  }, [])

  const settingsButton = (
    <Button
      type="button"
      variant="ghost"
      size="icon"
      className="h-8 w-8 text-muted-foreground hover:text-foreground"
      title="设置与工具"
      aria-label="设置与工具"
      onClick={() => openElectronSettings()}
    >
      <Settings className="h-4 w-4" />
    </Button>
  )

  const titleBarTrailing = (
    <>
      {electronChatTitleToolbar ? (
        <>
          <div className="flex shrink-0 items-center gap-0.5 px-1">{electronChatTitleToolbar}</div>
          <div className="mx-1.5 h-5 w-px shrink-0 bg-border/80 [-webkit-app-region:no-drag]" aria-hidden />
        </>
      ) : null}
      {settingsButton}
    </>
  )

  return (
    <ElectronWorkbenchChromeProvider>
      <WorkbenchChatTitleToolbarSetterContext.Provider value={setElectronChatTitleToolbar}>
        <ElectronOpenSettingsContext.Provider value={openElectronSettings}>
          <div className="flex h-screen w-full flex-col overflow-hidden bg-background text-foreground">
            {usesElectronCustomTitleBar() ? (
              <ElectronWindowChromeSpacer
                className="z-[60]"
                leading={<ElectronWorkbenchChromeLeading />}
                center={activeWorkspaceTitle}
                trailing={titleBarTrailing}
              />
            ) : (
              <div
                className="relative flex h-9 shrink-0 items-center border-b border-border/50 bg-muted/15 px-2 [-webkit-app-region:no-drag]"
                style={getElectronWindowChromeCSSVars()}
              >
                <ElectronWorkbenchChromeLeading />
                <div className="min-w-0 flex-1 [-webkit-app-region:drag]" />
                <div className="pointer-events-none absolute inset-0 flex items-center justify-center px-4 text-sm font-medium text-foreground/80">
                  <div className="max-w-[420px] truncate">{activeWorkspaceTitle}</div>
                </div>
                <div className="flex shrink-0 items-center [-webkit-app-region:no-drag]">{titleBarTrailing}</div>
              </div>
            )}

            <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
              <main className="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden [-webkit-app-region:no-drag]">
                <Outlet />
              </main>
            </div>

            <ElectronSettingsDialog
              open={electronSettingsOpen}
              onOpenChange={setElectronSettingsOpen}
              initialPanel={electronSettingsInitialPanel}
              usageTaskId={electronUsageTaskId}
              initialProjectId={electronSettingsInitialProjectId}
            />
            <WechatReloginDialog />
          </div>
        </ElectronOpenSettingsContext.Provider>
      </WorkbenchChatTitleToolbarSetterContext.Provider>
    </ElectronWorkbenchChromeProvider>
  )
}
