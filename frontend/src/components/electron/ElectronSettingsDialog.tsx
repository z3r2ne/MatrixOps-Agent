import type { ReactNode } from "react"
import React, { useLayoutEffect, useState } from "react"
import {
  BrainCircuit,
  BarChart3,
  BookOpen,
  Database,
  FlaskConical,
  FolderOpen,
  LayoutGrid,
  MessageCircle,
  MessageSquare,
  Sparkles,
  Terminal,
  Plug,
  Search,
  Settings,
  X,
} from "lucide-react"
import { Dialog, DialogContent } from "@/components/ui/dialog"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"
import { getElectronWindowChromeCSSVars } from "@/lib/electron"
import { DebugPage } from "@/pages/DebugPage"
import { LogsPage } from "@/pages/LogsPage"
import { MemoryLibrariesPage } from "@/pages/MemoryLibrariesPage"
import { RagLibrariesPage } from "@/pages/RagLibrariesPage"
import { ProjectsPage } from "@/pages/ProjectsPage"
import { PromptsPage } from "@/pages/PromptsPage"
import { SettingsPage } from "@/pages/SettingsPage"
import { SkillSourcesPage } from "@/pages/SkillSourcesPage"
import { ILinkSection } from "@/components/settings/ILinkSection"
import { McpServersPage } from "@/pages/McpServersPage"
import { EmbeddingConfigsPage } from "@/pages/EmbeddingConfigsPage"
import { SearchConfigsPage } from "@/pages/SearchConfigsPage"
import { SkillsPage } from "@/pages/SkillsPage"
import { UsageAnalyticsPage } from "@/pages/UsageAnalyticsPage"
import { WorkspacesPage } from "@/pages/WorkspacesPage"
import {
  ElectronSettingsShellCloseContext,
  ElectronSettingsPanelContext,
  ElectronUsageAnalyticsFilterContext,
  ElectronSettingsInitialProjectIdContext,
  type ElectronSettingsPanel,
} from "@/components/electron/electron-main-contexts"

const NAV_ITEMS: { panel: ElectronSettingsPanel; label: string; title: string; icon: typeof Settings }[] = [
  { panel: "settings", label: "设置", title: "设置", icon: Settings },
  { panel: "workspaces", label: "工作区", title: "工作区", icon: LayoutGrid },
  { panel: "projects", label: "项目", title: "项目", icon: FolderOpen },
  { panel: "memory-libraries", label: "记忆库", title: "记忆库", icon: BookOpen },
  { panel: "rag-libraries", label: "RAG", title: "RAG 知识库", icon: Database },
  { panel: "skills", label: "技能", title: "技能广场", icon: Sparkles },
  { panel: "mcp", label: "MCP", title: "MCP 服务器", icon: Plug },
  { panel: "search", label: "搜索", title: "搜索配置", icon: Search },
  { panel: "embedding", label: "Embedding", title: "Embedding 配置", icon: BrainCircuit },
  { panel: "ilink", label: "iLink", title: "iLink 微信", icon: MessageCircle },
  { panel: "logs", label: "日志", title: "系统日志", icon: Terminal },
  { panel: "usage", label: "统计", title: "数据统计", icon: BarChart3 },
  { panel: "prompts", label: "提示词", title: "提示词", icon: MessageSquare },
  { panel: "debug", label: "调试", title: "调试", icon: FlaskConical },
]

function ElectronSettingsChrome({
  panel,
  setPanel,
}: {
  panel: ElectronSettingsPanel
  setPanel: (p: ElectronSettingsPanel) => void
}) {
  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-hidden sm:flex-row">
      <aside className="flex w-full shrink-0 flex-col border-b border-border/50 bg-muted/15 sm:w-52 sm:border-b-0 sm:border-r">
        <nav className="flex max-h-[40vh] flex-row gap-1 overflow-x-auto px-2 py-3 sm:max-h-none sm:flex-1 sm:flex-col sm:gap-0.5 sm:overflow-y-auto sm:py-3">
          {NAV_ITEMS.map(({ panel: id, label, title, icon: Icon }) => {
            const isActive = panel === id
            return (
              <button
                key={id}
                type="button"
                title={title}
                onClick={() => setPanel(id)}
                className={cn(
                  "flex items-center gap-2 rounded-md px-2 py-2 text-left text-sm transition-colors",
                  isActive ? "bg-muted font-medium text-foreground" : "text-muted-foreground hover:bg-muted/60 hover:text-foreground"
                )}
              >
                <Icon className="h-4 w-4 shrink-0" />
                <span className="whitespace-nowrap">{label}</span>
              </button>
            )
          })}
        </nav>
      </aside>
      <main className="min-h-0 min-w-0 flex-1 overflow-y-auto overflow-x-hidden">{renderPanel(panel)}</main>
    </div>
  )
}

function renderPanel(panel: ElectronSettingsPanel): ReactNode {
  switch (panel) {
    case "settings":
      return <SettingsPage />
    case "workspaces":
      return <WorkspacesPage />
    case "projects":
      return <ProjectsPage />
    case "memory-libraries":
      return <MemoryLibrariesPage />
    case "rag-libraries":
      return <RagLibrariesPage />
    case "skills":
      return <SkillsPage />
    case "mcp":
      return <McpServersPage />
    case "search":
      return <SearchConfigsPage />
    case "embedding":
      return <EmbeddingConfigsPage />
    case "skills-sources":
      return <SkillSourcesPage />
    case "logs":
      return <LogsPage />
    case "usage":
      return <UsageAnalyticsPage />
    case "prompts":
      return <PromptsPage />
    case "ilink":
      return <ILinkSection />
    case "debug":
      return <DebugPage />
    default:
      return <SettingsPage />
  }
}

/** 固定视口内尺寸，切换左侧 tab 时弹窗整体大小不变 */
const SETTINGS_DIALOG_WIDTH = "min(1180px, calc(100vw - 2rem))"
const SETTINGS_DIALOG_HEIGHT = "min(720px, calc(100dvh - var(--electron-window-chrome-top, 0px) - 2rem))"

export function ElectronSettingsDialog({
  open,
  onOpenChange,
  initialPanel = "settings",
  usageTaskId,
  initialProjectId,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  initialPanel?: ElectronSettingsPanel
  usageTaskId?: number
  initialProjectId?: number
}) {
  const [panel, setPanel] = useState<ElectronSettingsPanel>(initialPanel)

  useLayoutEffect(() => {
    if (open) {
      setPanel(initialPanel)
    }
  }, [initialPanel, open])

  const close = () => onOpenChange(false)

  const handleDialogOpenChange = (next: boolean) => {
    if (next) {
      const active = document.activeElement
      if (active instanceof HTMLElement) {
        active.blur()
      }
    }
    onOpenChange(next)
  }

  return (
    <Dialog open={open} onOpenChange={handleDialogOpenChange}>
      <DialogContent
        hideClose
        srOnlyTitle="设置"
        className={cn(
          "!flex !max-w-none !flex-col gap-0 overflow-hidden !p-0 sm:!max-w-none",
          "max-h-none"
        )}
        style={{
          ...getElectronWindowChromeCSSVars(),
          width: SETTINGS_DIALOG_WIDTH,
          maxWidth: SETTINGS_DIALOG_WIDTH,
          height: SETTINGS_DIALOG_HEIGHT,
          minHeight: SETTINGS_DIALOG_HEIGHT,
          maxHeight: SETTINGS_DIALOG_HEIGHT,
        }}
      >
        {open ? (
          <ElectronSettingsShellCloseContext.Provider value={close}>
            <ElectronSettingsPanelContext.Provider value={{ panel, setPanel }}>
              <ElectronUsageAnalyticsFilterContext.Provider value={{ taskId: usageTaskId }}>
                <ElectronSettingsInitialProjectIdContext.Provider value={initialProjectId}>
                  <div className="relative flex min-h-0 flex-1 flex-col overflow-hidden">
                  <div className="group/close absolute right-0 top-0 z-40 h-14 w-28 [-webkit-app-region:no-drag]">
                    <div className="flex h-full w-full items-start justify-end p-1.5">
                      <Button
                        type="button"
                        variant="ghost"
                        size="icon"
                        className="h-8 w-8 bg-background/95 opacity-0 shadow-sm ring-1 ring-border/50 transition-opacity group-hover/close:opacity-100 focus-visible:opacity-100 dark:bg-background/90"
                        onClick={close}
                        aria-label="关闭"
                      >
                        <X className="h-4 w-4" />
                      </Button>
                    </div>
                  </div>
                  <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
                    <ElectronSettingsChrome panel={panel} setPanel={setPanel} />
                  </div>
                </div>
                </ElectronSettingsInitialProjectIdContext.Provider>
              </ElectronUsageAnalyticsFilterContext.Provider>
            </ElectronSettingsPanelContext.Provider>
          </ElectronSettingsShellCloseContext.Provider>
        ) : null}
      </DialogContent>
    </Dialog>
  )
}
