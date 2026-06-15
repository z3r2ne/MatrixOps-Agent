import React, { createContext, useCallback, useContext, useMemo, useState, type ReactNode } from "react"
import { PanelLeft, PanelLeftClose, Search, SquareStack } from "lucide-react"
import { Button } from "@/components/ui/button"

export type ElectronWorkbenchChromeValue = {
  taskSidebarCollapsed: boolean
  setTaskSidebarCollapsed: (v: boolean) => void
  toggleTaskSidebar: () => void
  searchPaletteOpen: boolean
  setSearchPaletteOpen: (v: boolean) => void
  openSearchPalette: () => void
}

const ElectronWorkbenchChromeContext = createContext<ElectronWorkbenchChromeValue | null>(null)

export function useElectronWorkbenchChrome(): ElectronWorkbenchChromeValue | null {
  return useContext(ElectronWorkbenchChromeContext)
}

export function ElectronWorkbenchChromeProvider({ children }: { children: ReactNode }) {
  const [taskSidebarCollapsed, setTaskSidebarCollapsed] = useState(false)
  const [searchPaletteOpen, setSearchPaletteOpen] = useState(false)

  const toggleTaskSidebar = useCallback(() => {
    setTaskSidebarCollapsed((c) => !c)
  }, [])

  const openSearchPalette = useCallback(() => {
    const el = document.activeElement
    if (el instanceof HTMLElement) {
      el.blur()
    }
    setSearchPaletteOpen(true)
  }, [])

  const value = useMemo<ElectronWorkbenchChromeValue>(
    () => ({
      taskSidebarCollapsed,
      setTaskSidebarCollapsed,
      toggleTaskSidebar,
      searchPaletteOpen,
      setSearchPaletteOpen,
      openSearchPalette,
    }),
    [taskSidebarCollapsed, searchPaletteOpen, toggleTaskSidebar, openSearchPalette]
  )

  return <ElectronWorkbenchChromeContext.Provider value={value}>{children}</ElectronWorkbenchChromeContext.Provider>
}

/** Electron 工作台顶栏：搜索、折叠任务侧栏（需在 Provider 内使用） */
export function ElectronWorkbenchChromeLeading() {
  const chrome = useContext(ElectronWorkbenchChromeContext)
  if (!chrome) {
    return null
  }

  return (
    <div className="flex shrink-0 items-center gap-0.5 px-1">
      <Button
        type="button"
        variant="ghost"
        size="icon"
        className="h-8 w-8 text-muted-foreground hover:text-foreground"
        title="打开初始化窗口"
        aria-label="打开初始化窗口"
        onClick={() => {
          void window.electronAPI?.openLauncherWindow?.()
        }}
      >
        <SquareStack className="h-4 w-4" />
      </Button>
      <Button
        type="button"
        variant="ghost"
        size="icon"
        className="h-8 w-8 text-muted-foreground hover:text-foreground"
        title="搜索任务"
        aria-label="搜索任务"
        onClick={chrome.openSearchPalette}
      >
        <Search className="h-4 w-4" />
      </Button>
      <Button
        type="button"
        variant="ghost"
        size="icon"
        className="h-8 w-8 text-muted-foreground hover:text-foreground"
        title={chrome.taskSidebarCollapsed ? "展开任务列表" : "收起任务列表"}
        aria-label={chrome.taskSidebarCollapsed ? "展开任务列表" : "收起任务列表"}
        onClick={chrome.toggleTaskSidebar}
      >
        {chrome.taskSidebarCollapsed ? <PanelLeft className="h-4 w-4" /> : <PanelLeftClose className="h-4 w-4" />}
      </Button>
    </div>
  )
}
