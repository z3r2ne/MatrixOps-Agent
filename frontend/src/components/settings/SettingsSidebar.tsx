import { ChevronRight } from "lucide-react"

import { cn } from "@/lib/utils"

import { SETTINGS_NAV_ITEMS, type SettingsTabId } from "./settingsNavConfig"

export type SettingsSidebarProps = {
  activeTab: SettingsTabId
  onSelectTab: (tab: SettingsTabId) => void
}

/** 设置页左侧导航：固定列宽；不使用 sticky，避免 Tab 切换时内容高度骤变导致粘附态闪动 */
export function SettingsSidebar({ activeTab, onSelectTab }: SettingsSidebarProps) {
  return (
    <nav
      aria-label="设置分区"
      className={cn(
        "w-full min-w-0 self-start rounded-lg border border-border/50 bg-muted/20 p-1"
      )}
    >
      <div className="space-y-1">
        {SETTINGS_NAV_ITEMS.map((item) => {
          const Icon = item.icon
          const isActive = activeTab === item.id

          return (
            <button
              key={item.id}
              type="button"
              onClick={() => onSelectTab(item.id)}
              className={cn(
                "w-full flex items-center gap-3 px-4 py-3 rounded-lg text-left transition-colors",
                "hover:bg-accent/50",
                "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-inset",
                isActive ? "bg-accent text-accent-foreground shadow-sm" : "text-muted-foreground"
              )}
            >
              <Icon className={cn("h-5 w-5 shrink-0", isActive && "text-primary")} />
              <div className="flex-1 min-w-0">
                <div className={cn("text-sm font-medium", isActive && "text-foreground")}>{item.label}</div>
                <div className="text-xs text-muted-foreground mt-0.5 truncate">{item.description}</div>
              </div>
              <span className="inline-flex w-4 shrink-0 justify-end" aria-hidden>
                {isActive ? <ChevronRight className="h-4 w-4 text-primary" /> : null}
              </span>
            </button>
          )
        })}
      </div>
    </nav>
  )
}
