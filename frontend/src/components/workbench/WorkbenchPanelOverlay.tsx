import type { ReactNode } from "react"
import { X } from "lucide-react"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"

interface WorkbenchPanelOverlayProps {
  visible: boolean
  onClose: () => void
  children: ReactNode
  className?: string
}

export function WorkbenchPanelOverlay({
  visible,
  onClose,
  children,
  className,
}: WorkbenchPanelOverlayProps) {
  return (
    <div className={cn("group/panel relative flex min-h-0 min-w-0 flex-1 flex-col", className)}>
      {visible ? (
        <div className="pointer-events-none absolute inset-x-0 top-0 z-20 flex justify-end p-2 opacity-0 transition-opacity group-hover/panel:opacity-100">
          <Button
            type="button"
            variant="secondary"
            size="icon"
            className="pointer-events-auto h-8 w-8 rounded-full border border-border/70 bg-background/90 shadow-sm backdrop-blur-sm"
            onClick={onClose}
            aria-label="关闭面板"
          >
            <X className="h-4 w-4" />
          </Button>
        </div>
      ) : null}
      {children}
    </div>
  )
}
