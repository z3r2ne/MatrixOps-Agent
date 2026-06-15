import React, { useMemo } from "react"
import { DiffModeEnum } from "@git-diff-view/react"
import { cn } from "@/lib/utils"
import { resolvePatchDisplaySections } from "./patchDiffText"
import { UnifiedDiffContent } from "./UnifiedDiffContent"

/** patch / unified diff 文本，使用与 Diff 弹窗一致的不分栏（Unified）展示 */
export function PatchDiffViewer({ text, className }: { text: string; className?: string }) {
  const sections = useMemo(() => resolvePatchDisplaySections(text), [text])

  return (
    <div className={cn("max-h-[min(360px,55vh)] overflow-auto space-y-4", className)}>
      {sections.map((section, index) => (
        <UnifiedDiffContent
          key={`${section.path}:${index}`}
          filePath={section.path}
          unifiedDiff={section.unifiedDiff}
          mode={DiffModeEnum.Unified}
          showPathHeader={sections.length > 1}
        />
      ))}
    </div>
  )
}
