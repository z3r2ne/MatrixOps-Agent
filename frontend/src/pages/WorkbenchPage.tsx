import React, { useEffect, useMemo, useState } from "react"
import { useNavigate, useSearchParams } from "react-router-dom"
import { Settings } from "lucide-react"
import { WorkspaceDetailPage } from "@/pages/WorkspaceDetailPage"
import { useWorkbench } from "@/components/workbench/WorkbenchProvider"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "@/components/ui/card"
import { takePendingElectronWorkbenchOpen } from "@/lib/electronPendingWorkbench"
import { useElectronOpenSettings } from "@/components/electron/electron-main-contexts"

export function WorkbenchPage() {
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const openElectronSettings = useElectronOpenSettings()
  const { tabs, activeTabId, openWorkspaceTab } = useWorkbench()

  const activeTab = useMemo(
    () => tabs.find(tab => tab.id === activeTabId) ?? null,
    [activeTabId, tabs]
  )

  useEffect(() => {
    const workspaceId = Number(searchParams.get("workspaceId") || "")
    const workspaceName = searchParams.get("workspaceName")
    if (!workspaceId || !workspaceName) {
      return
    }
    openWorkspaceTab({ id: workspaceId, name: workspaceName })
    navigate("/workbench", { replace: true })
  }, [navigate, openWorkspaceTab, searchParams])

  useEffect(() => {
    const pending = takePendingElectronWorkbenchOpen()
    if (!pending) {
      return
    }
    if (pending.kind === "workspace") {
      openWorkspaceTab({ id: pending.id, name: pending.name })
    }
  }, [openWorkspaceTab])

  const activeWorkbenchTab = activeTab ?? tabs[0] ?? null

  let mainContent: React.ReactNode
  if (tabs.length === 0) {
    mainContent = (
      <div className="flex flex-1 items-center justify-center p-6">
        <Card className="w-full max-w-xl">
          <CardHeader>
            <CardTitle>尚未打开工作区</CardTitle>
            <CardDescription>
              请点击标题栏上的「设置」按钮，在「工作区」中选择一个工作区；或创建后再打开。
            </CardDescription>
          </CardHeader>
          <CardFooter className="flex flex-wrap gap-2">
            <Button
              type="button"
              onClick={() => openElectronSettings?.()}
              disabled={!openElectronSettings}
            >
              <Settings className="mr-2 h-4 w-4" />
              打开设置
            </Button>
          </CardFooter>
        </Card>
      </div>
    )
  } else {
    mainContent = (
      <div className="flex min-h-0 flex-1 flex-col">
        <WorkspaceDetailPage
          workspaceId={activeWorkbenchTab.workspaceId}
          showProjectHeader={false}
          selectionStorageKey={`matrixops.workbench.tab.${activeWorkbenchTab.id}.selected-task`}
          publishWorkbenchTitleToolbar
        />
      </div>
    )
  }

  return (
    <div className="flex h-full min-h-0 flex-col bg-background">
      {mainContent}
    </div>
  )
}
