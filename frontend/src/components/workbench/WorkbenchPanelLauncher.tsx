import { useCallback, useRef, useState } from "react"
import { Loader2, PanelTop } from "lucide-react"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"
import type { WorkbenchPanelDefinition, WorkbenchPanelId } from "./types"

interface WorkbenchPanelLauncherProps {
  panels: WorkbenchPanelDefinition[]
  activePanelId: WorkbenchPanelId | null
  panelLoadingId?: WorkbenchPanelId | null
  disabled?: boolean
  onSelectPanel: (panelId: WorkbenchPanelId) => void
}

export function WorkbenchPanelLauncher({
  panels,
  activePanelId,
  panelLoadingId = null,
  disabled = false,
  onSelectPanel,
}: WorkbenchPanelLauncherProps) {
  const [open, setOpen] = useState(false)
  const closeTimerRef = useRef<number | null>(null)

  const clearCloseTimer = useCallback(() => {
    if (closeTimerRef.current != null) {
      window.clearTimeout(closeTimerRef.current)
      closeTimerRef.current = null
    }
  }, [])

  const handleMouseEnter = useCallback(() => {
    clearCloseTimer()
    setOpen(true)
  }, [clearCloseTimer])

  const handleMouseLeave = useCallback(() => {
    clearCloseTimer()
    closeTimerRef.current = window.setTimeout(() => setOpen(false), 120)
  }, [clearCloseTimer])

  const activePanel = activePanelId ? panels.find((panel) => panel.id === activePanelId) : null
  const TriggerIcon = activePanel?.icon ?? PanelTop

  return (
    <div
      className="relative"
      onMouseEnter={handleMouseEnter}
      onMouseLeave={handleMouseLeave}
    >
      <Button
        type="button"
        variant="ghost"
        size="icon"
        className={cn(
          "h-7 w-7",
          activePanelId && "bg-muted text-foreground",
        )}
        disabled={disabled}
        aria-expanded={open}
        aria-haspopup="menu"
      >
        <TriggerIcon className="h-3.5 w-3.5" />
      </Button>

      {open ? (
        <div className="absolute right-0 top-full z-50 mt-1 w-[min(18rem,calc(100vw-2rem))] rounded-lg border border-border/80 bg-popover p-2 shadow-lg">
          <div className="mb-1.5 px-1 text-[10px] font-medium uppercase tracking-wide text-muted-foreground">
            工作台
          </div>
          <div className="grid grid-cols-4 gap-1.5">
            {panels.map((panel) => {
              const Icon = panel.icon
              const isActive = activePanelId === panel.id
              const isLoading = panelLoadingId === panel.id
              return (
                <button
                  key={panel.id}
                  type="button"
                  className={cn(
                    "flex min-h-[4.25rem] flex-col items-center justify-center gap-1 rounded-md border px-1.5 py-2 text-center transition-colors",
                    isActive
                      ? "border-primary/40 bg-primary/10 text-foreground"
                      : "border-border/60 bg-background hover:border-border hover:bg-muted/40",
                  )}
                  onClick={() => onSelectPanel(panel.id)}
                  disabled={disabled || isLoading}
                >
                  {isLoading ? (
                    <Loader2 className="h-4 w-4 animate-spin text-muted-foreground" />
                  ) : (
                    <Icon className="h-4 w-4 shrink-0" />
                  )}
                  <span className="w-full truncate text-[10px] leading-tight">{panel.label}</span>
                </button>
              )
            })}
          </div>
        </div>
      ) : null}
    </div>
  )
}
