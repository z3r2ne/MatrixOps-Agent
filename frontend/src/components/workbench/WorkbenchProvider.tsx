import React, { createContext, useCallback, useContext, useEffect, useMemo, useState } from "react"

type WorkbenchWorkspaceTab = {
  id: string
  kind: "workspace"
  title: string
  workspaceId: number
}

export type WorkbenchTab = WorkbenchWorkspaceTab

interface WorkbenchContextValue {
  tabs: WorkbenchTab[]
  activeTabId: string | null
  openWorkspaceTab: (workspace: { id: number; name: string }) => void
}

const STORAGE_KEY = "matrixops.workbench.current-workspace.v1"

const WorkbenchContext = createContext<WorkbenchContextValue | null>(null)

function buildWorkspaceEntry(workspace: { id: number; name: string }): WorkbenchTab {
  return {
    id: `workspace:${workspace.id}`,
    kind: "workspace",
    title: workspace.name,
    workspaceId: workspace.id,
  }
}

function loadStoredWorkbenchState(): { tabs: WorkbenchTab[]; activeTabId: string | null } {
  if (typeof window === "undefined") {
    return { tabs: [], activeTabId: null }
  }

  try {
    const raw = window.localStorage.getItem(STORAGE_KEY)
    if (!raw) {
      return { tabs: [], activeTabId: null }
    }

    const parsed = JSON.parse(raw) as { workspaceId?: number; workspaceName?: string }
    if (!Number.isFinite(parsed.workspaceId) || !parsed.workspaceName) {
      return { tabs: [], activeTabId: null }
    }

    const tab = buildWorkspaceEntry({ id: parsed.workspaceId, name: parsed.workspaceName })
    return { tabs: [tab], activeTabId: tab.id }
  } catch (error) {
    console.error("Failed to load current workspace:", error)
    return { tabs: [], activeTabId: null }
  }
}

export function WorkbenchProvider({ children }: { children: React.ReactNode }) {
  const [tabs, setTabs] = useState<WorkbenchTab[]>(() => loadStoredWorkbenchState().tabs)
  const [activeTabId, setActiveTabId] = useState<string | null>(() => loadStoredWorkbenchState().activeTabId)

  useEffect(() => {
    if (tabs.length === 0) {
      if (activeTabId !== null) {
        setActiveTabId(null)
      }
      return
    }

    if (!activeTabId || !tabs.some((tab) => tab.id === activeTabId)) {
      setActiveTabId(tabs[0].id)
    }
  }, [activeTabId, tabs])

  useEffect(() => {
    if (typeof window === "undefined") {
      return
    }

    const active = tabs.find((tab) => tab.id === activeTabId) ?? tabs[0] ?? null
    if (!active) {
      window.localStorage.removeItem(STORAGE_KEY)
      return
    }

    window.localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({
        workspaceId: active.workspaceId,
        workspaceName: active.title,
      }),
    )
  }, [activeTabId, tabs])

  const openWorkspaceTab = useCallback((workspace: { id: number; name: string }) => {
    const next = buildWorkspaceEntry(workspace)
    setTabs([next])
    setActiveTabId(next.id)
  }, [])

  const value = useMemo<WorkbenchContextValue>(() => ({
    tabs,
    activeTabId,
    openWorkspaceTab,
  }), [activeTabId, openWorkspaceTab, tabs])

  return (
    <WorkbenchContext.Provider value={value}>
      {children}
    </WorkbenchContext.Provider>
  )
}

export function useWorkbench() {
  const context = useContext(WorkbenchContext)
  if (!context) {
    throw new Error("useWorkbench must be used within a WorkbenchProvider")
  }
  return context
}
