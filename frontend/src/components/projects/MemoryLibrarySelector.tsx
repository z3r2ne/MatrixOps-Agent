import React, { useMemo } from "react"
import { BookOpen, CheckSquare2 } from "lucide-react"

import { Checkbox } from "@/components/ui/checkbox"
import { ScrollArea } from "@/components/ui/scroll-area"
import { Badge } from "@/components/ui/badge"
import { type MemoryLibrary } from "@/lib/api"
import { cn } from "@/lib/utils"

interface MemoryLibrarySelectorProps {
  libraries: MemoryLibrary[]
  selectedIds: number[]
  onChange: (ids: number[]) => void
  disabled?: boolean
  className?: string
}

function buildPreview(content: string) {
  const normalized = content.replace(/\s+/g, " ").trim()
  if (!normalized) return "暂无介绍"
  return normalized.slice(0, 72)
}

export function MemoryLibrarySelector({
  libraries,
  selectedIds,
  onChange,
  disabled = false,
  className,
}: MemoryLibrarySelectorProps) {
  const selectedSet = useMemo(() => new Set(selectedIds), [selectedIds])
  const selectedLibraries = useMemo(
    () => libraries.filter((library) => selectedSet.has(library.id)),
    [libraries, selectedSet],
  )

  const toggle = (id: number, checked: boolean) => {
    const next = checked ? [...selectedIds, id] : selectedIds.filter((item) => item !== id)
    onChange(Array.from(new Set(next)))
  }

  return (
    <div className={cn("space-y-2", className)}>
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2 text-sm font-medium text-foreground">
          <BookOpen className="h-4 w-4 text-muted-foreground" />
          关联记忆库
        </div>
        <Badge variant="outline">{selectedIds.length} 个已关联</Badge>
      </div>
      {selectedLibraries.length > 0 ? (
        <div className="flex flex-wrap gap-1.5 rounded-lg border border-border/60 bg-muted/20 p-2">
          {selectedLibraries.map((library) => (
            <Badge key={library.id} variant="secondary" className="gap-1">
              <CheckSquare2 className="h-3 w-3" />
              {library.name}
            </Badge>
          ))}
        </div>
      ) : null}
      <div className="rounded-lg border border-border/60 bg-background/80">
        {libraries.length === 0 ? (
          <div className="px-3 py-4 text-sm text-muted-foreground">
            暂无记忆库，可先到设置中的「记忆库」页面创建。
          </div>
        ) : (
          <ScrollArea className="max-h-56">
            <div className="divide-y">
              {libraries.map((library) => {
                const checked = selectedSet.has(library.id)
                return (
                  <label
                    key={library.id}
                    className={cn(
                      "flex cursor-pointer items-start gap-3 px-3 py-2.5 transition-colors hover:bg-muted/30",
                      disabled && "cursor-not-allowed opacity-60",
                    )}
                  >
                    <Checkbox
                      checked={checked}
                      disabled={disabled}
                      onCheckedChange={(value) => toggle(library.id, value === true)}
                      className="mt-0.5"
                    />
                    <div className="min-w-0 flex-1">
                      <div className="truncate text-sm font-medium text-foreground">{library.name}</div>
                      <div className="mt-0.5 text-xs text-muted-foreground">{buildPreview(library.content)}</div>
                    </div>
                  </label>
                )
              })}
            </div>
          </ScrollArea>
        )}
      </div>
      <p className="text-xs text-muted-foreground">
        关联后，项目会在任务启动时注入这些记忆库的介绍作为背景上下文。
      </p>
    </div>
  )
}
