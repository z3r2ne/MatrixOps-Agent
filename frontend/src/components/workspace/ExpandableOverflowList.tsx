import React, { useEffect, useLayoutEffect, useMemo, useRef, useState } from "react"
import { ChevronDown, ChevronUp, Wrench } from "lucide-react"

import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"
import { useAutoScroll } from "@/hooks/useAutoScroll"

type ExpandableOverflowListMode = "peek" | "full" | "expanded"

interface ExpandableOverflowListProps {
  children: React.ReactNode
  defaultMode?: ExpandableOverflowListMode
  peekHeightClassName?: string
  collapsedSummary: (expand: () => void) => React.ReactNode
  collapseLabel?: React.ReactNode
  className?: string
}

export function ExpandableOverflowList({
  children,
  defaultMode = "peek",
  peekHeightClassName = "max-h-56",
  collapsedSummary,
  collapseLabel = "收起工具调用",
  className,
}: ExpandableOverflowListProps) {
  const [mode, setMode] = useState<ExpandableOverflowListMode>(defaultMode)
  const contentRef = useRef<HTMLDivElement>(null)
  const [hasOverflow, setHasOverflow] = useState(false)
  const [canScrollUp, setCanScrollUp] = useState(false)
  const [canScrollDown, setCanScrollDown] = useState(false)

  const { ref: viewportRef, handleScroll } = useAutoScroll({
    deps: [children],
    enabled: mode === "peek",
  })

  useEffect(() => {
    setMode(defaultMode)
  }, [defaultMode])

  useLayoutEffect(() => {
    if (mode !== "peek") {
      setHasOverflow(false)
      setCanScrollUp(false)
      setCanScrollDown(false)
      return
    }

    const viewport = viewportRef.current
    const content = contentRef.current
    if (!viewport || !content) {
      return
    }

    const updateState = () => {
      const nextHasOverflow = viewport.scrollHeight > viewport.clientHeight + 1
      const nextCanScrollUp = viewport.scrollTop > 2
      const nextCanScrollDown = viewport.scrollTop + viewport.clientHeight < viewport.scrollHeight - 2
      setHasOverflow(nextHasOverflow)
      setCanScrollUp(nextHasOverflow && nextCanScrollUp)
      setCanScrollDown(nextHasOverflow && nextCanScrollDown)
    }

    updateState()
    viewport.addEventListener("scroll", updateState, { passive: true })

    let observer: ResizeObserver | null = null
    if (typeof ResizeObserver !== "undefined") {
      observer = new ResizeObserver(() => updateState())
      observer.observe(viewport)
      observer.observe(content)
    }

    return () => {
      viewport.removeEventListener("scroll", updateState)
      observer?.disconnect()
    }
  }, [children, mode, viewportRef])

  const restoreMode = useMemo<ExpandableOverflowListMode>(() => {
    return defaultMode === "expanded" ? "peek" : defaultMode
  }, [defaultMode])

  if (mode === "full") {
    return <>{collapsedSummary(() => setMode("expanded"))}</>
  }

  const contentNode = <div ref={contentRef}>{children}</div>

  if (mode === "peek") {
    return (
      <div className={cn("relative", className)}>
        <div
          ref={viewportRef}
          onScroll={handleScroll}
          className={cn(
            "relative overflow-y-auto pr-1 scrollbar-thin scrollbar-thumb-muted-foreground/20 scrollbar-track-transparent",
            peekHeightClassName,
          )}
        >
          {contentNode}
        </div>

        {hasOverflow && canScrollUp && (
          <button
            type="button"
            className="absolute inset-x-0 top-0 z-10 flex h-10 items-start justify-center bg-gradient-to-b from-background via-background/90 to-transparent pt-1 text-muted-foreground/70 transition hover:text-foreground"
            onClick={() => setMode("expanded")}
          >
            <ChevronUp className="h-4 w-4" />
          </button>
        )}

        {hasOverflow && canScrollDown && (
          <button
            type="button"
            className="absolute inset-x-0 bottom-0 z-10 flex h-14 items-end justify-center bg-gradient-to-t from-background via-background/90 to-transparent pb-1.5 text-muted-foreground/80 transition hover:text-foreground"
            onClick={() => setMode("expanded")}
          >
            <span className="flex items-center gap-1 rounded-full border border-border/60 bg-background/85 px-2 py-0.5 text-[10px]">
              <Wrench className="h-3 w-3" />
              展开工具调用
              <ChevronDown className="h-3 w-3" />
            </span>
          </button>
        )}
      </div>
    )
  }

  return (
    <div className={className}>
      {contentNode}
      {restoreMode !== "expanded" && (
        <div className="ml-8 mb-4 flex justify-center">
          <Button
            variant="ghost"
            size="sm"
            className="h-6 text-[10px] text-muted-foreground hover:text-foreground"
            onClick={() => setMode(restoreMode)}
          >
            {collapseLabel}
          </Button>
        </div>
      )}
    </div>
  )
}
