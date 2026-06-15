import React from "react"

import { cn } from "@/lib/utils"

type ContextUsageDialProps = {
  percent?: number | null
  loading?: boolean
  className?: string
}

function clampPercent(value?: number | null) {
  if (typeof value !== "number" || !Number.isFinite(value)) {
    return null
  }
  return Math.max(0, Math.min(value, 100))
}

function toneClasses(percent: number | null) {
  if (percent == null) {
    return {
      track: "text-muted-foreground/25",
      progress: "text-muted-foreground/55",
      needle: "text-muted-foreground/80",
      center: "fill-muted-foreground/80",
    }
  }
  if (percent >= 85) {
    return {
      track: "text-rose-500/20",
      progress: "text-rose-500",
      needle: "text-rose-500",
      center: "fill-rose-500",
    }
  }
  if (percent >= 65) {
    return {
      track: "text-amber-500/20",
      progress: "text-amber-500",
      needle: "text-amber-500",
      center: "fill-amber-500",
    }
  }
  return {
    track: "text-emerald-500/20",
    progress: "text-emerald-500",
    needle: "text-emerald-500",
    center: "fill-emerald-500",
  }
}

export function ContextUsageDial({ percent, loading = false, className }: ContextUsageDialProps) {
  const safePercent = clampPercent(percent)
  const angle = safePercent == null ? -110 : -110 + safePercent * 2.2
  const tone = toneClasses(safePercent)
  const needleLength = 6.9
  const needleRadians = ((angle - 90) * Math.PI) / 180
  const needleX = 12 + needleLength * Math.cos(needleRadians)
  const needleY = 16 + needleLength * Math.sin(needleRadians)

  return (
    <span
      className={cn(
        "inline-flex h-5 w-6 shrink-0 items-center justify-center",
        loading && "animate-pulse",
        className,
      )}
      aria-hidden="true"
    >
      <svg viewBox="3 7 18 11" className="h-5 w-6 overflow-visible">
        <path
          d="M4.5 16a7.5 7.5 0 0 1 15 0"
          pathLength={100}
          fill="none"
          stroke="currentColor"
          strokeWidth="2.45"
          strokeLinecap="round"
          className={tone.track}
        />
        <path
          d="M4.5 16a7.5 7.5 0 0 1 15 0"
          pathLength={100}
          fill="none"
          stroke="currentColor"
          strokeWidth="2.45"
          strokeLinecap="round"
          strokeDasharray={`${safePercent ?? 0} 100`}
          className={tone.progress}
        />
        <path d="M6.7 14.2 7.8 15.05" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" className="text-muted-foreground/45" />
        <path d="M12 12.45v1.75" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" className="text-muted-foreground/45" />
        <path d="m17.2 15.05 1.1-0.85" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" className="text-muted-foreground/45" />
        <path
          d={`M12 16 L${needleX.toFixed(3)} ${needleY.toFixed(3)}`}
          stroke="currentColor"
          strokeWidth="2.1"
          strokeLinecap="round"
          className={cn(tone.needle, "transition-all duration-500 ease-out")}
        />
        <circle cx="12" cy="16" r="2.2" className={tone.center} />
      </svg>
    </span>
  )
}
