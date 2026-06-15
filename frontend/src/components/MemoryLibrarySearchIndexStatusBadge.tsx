import { Badge } from "@/components/ui/badge"
import { cn } from "@/lib/utils"

export function MemoryLibrarySearchIndexStatusBadge({
  status,
  progress,
  className,
}: {
  status?: string
  progress?: number
  className?: string
}) {
  const normalized = (status || "idle").toLowerCase()
  let label = "未索引"
  let variant: "default" | "secondary" | "destructive" | "outline" = "secondary"

  switch (normalized) {
    case "completed":
    case "ready":
      label = "已索引"
      variant = "default"
      break
    case "running":
      label = typeof progress === "number" && progress > 0 ? `索引中 ${progress}%` : "索引中"
      variant = "outline"
      break
    case "pending":
      label = "排队中"
      variant = "outline"
      break
    case "failed":
      label = "索引失败"
      variant = "destructive"
      break
    default:
      label = "未索引"
      variant = "secondary"
  }

  return (
    <Badge variant={variant} className={cn("whitespace-nowrap", className)}>
      {label}
    </Badge>
  )
}
