import { MessageSquare, MonitorPlay } from "lucide-react"

import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"

export type WorkbenchViewMode = "chat" | "simulation"

interface WorkbenchViewTabsProps {
  value: WorkbenchViewMode
  onChange: (mode: WorkbenchViewMode) => void
  className?: string
}

export function WorkbenchViewTabs({ value, onChange, className }: WorkbenchViewTabsProps) {
  return (
    <div
      className={cn(
        "flex items-center rounded-lg border border-border/70 bg-muted/40 p-0.5",
        className,
      )}
    >
      <Button
        type="button"
        variant={value === "chat" ? "secondary" : "ghost"}
        size="sm"
        className="h-7 gap-1 px-2.5 text-xs"
        onClick={() => onChange("chat")}
      >
        <MessageSquare className="h-3.5 w-3.5" />
        对话
      </Button>
      <Button
        type="button"
        variant={value === "simulation" ? "secondary" : "ghost"}
        size="sm"
        className="h-7 gap-1 px-2.5 text-xs"
        onClick={() => onChange("simulation")}
      >
        <MonitorPlay className="h-3.5 w-3.5" />
        仿真
      </Button>
    </div>
  )
}
