import React, { useMemo } from "react"
import { DiffView, DiffModeEnum } from "@git-diff-view/react"
import "@git-diff-view/react/styles/diff-view.css"
import { cn } from "@/lib/utils"
import { buildDiffViewData } from "./patchDiffText"

export function UnifiedDiffFallbackLines({ text, className }: { text: string; className?: string }) {
  return (
    <div className={cn("border rounded overflow-x-auto", className)}>
      <div className="bg-muted px-3 py-2 border-b text-xs font-medium">代码变更</div>
      <div className="bg-card min-w-max">
        {text.split("\n").map((line, index) => {
          const isAddition = line.startsWith("+")
          const isDeletion = line.startsWith("-")
          const isContext = !isAddition && !isDeletion
          return (
            <div
              key={index}
              data-diff-fallback-line
              className={cn(
                "px-4 py-0.5 text-xs font-mono border-l-2",
                isAddition && "bg-green-50 border-l-green-500 text-green-800",
                isDeletion && "bg-red-50 border-l-red-500 text-red-800",
                isContext && "border-l-transparent",
              )}
            >
              <span className="inline-block w-8 text-muted-foreground select-none">{index + 1}</span>
              <span>{line || " "}</span>
            </div>
          )
        })}
      </div>
    </div>
  )
}

export function UnifiedDiffContent({
  filePath,
  unifiedDiff,
  mode = DiffModeEnum.Unified,
  showPathHeader = false,
  className,
}: {
  filePath?: string
  unifiedDiff: string
  mode?: DiffModeEnum
  showPathHeader?: boolean
  className?: string
}) {
  const diffData = useMemo(() => {
    if (!unifiedDiff) return null
    try {
      return buildDiffViewData(filePath || "file.txt", unifiedDiff)
    } catch (error) {
      console.error("Failed to parse diff:", error)
      return null
    }
  }, [filePath, unifiedDiff])

  const canUseDiffView = Boolean(diffData && unifiedDiff.includes("---") && unifiedDiff.includes("+++"))

  return (
    <div className={className}>
      {showPathHeader && filePath ? (
        <div className="mb-3 pb-2 border-b min-w-0">
          <p className="text-sm font-medium text-muted-foreground truncate min-w-0" title={filePath}>
            {filePath}
          </p>
        </div>
      ) : null}
      {canUseDiffView && diffData ? (
        <div className="diff-view-shell w-full overflow-x-auto rounded border">
          <DiffView
            data={diffData}
            diffViewWrap={false}
            diffViewTheme="light"
            diffViewHighlight
            diffViewMode={mode}
            diffViewFontSize={13}
          />
        </div>
      ) : (
        <UnifiedDiffFallbackLines text={unifiedDiff} />
      )}
    </div>
  )
}
