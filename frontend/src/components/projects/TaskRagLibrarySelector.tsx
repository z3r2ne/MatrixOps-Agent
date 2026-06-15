import React, { useMemo } from "react"
import { Database } from "lucide-react"

import { Combobox, type ComboboxOption } from "@/components/ui/combobox"
import { type MemoryLibrary, type TaskMemoryLibraryMode } from "@/lib/api"
import { cn } from "@/lib/utils"

interface TaskRagLibrarySelectorProps {
  libraries: MemoryLibrary[]
  mode: TaskMemoryLibraryMode
  selectedIds: number[]
  onModeChange: (mode: TaskMemoryLibraryMode) => void
  onSelectedIdsChange: (ids: number[]) => void
  disabled?: boolean
  className?: string
  layout?: "inline" | "stacked"
}

const MODE_OPTIONS: Array<{ value: TaskMemoryLibraryMode; label: string; description?: string }> = [
  { value: "none", label: "不使用 RAG", description: "不启用语义检索" },
  { value: "temporary", label: "临时 RAG", description: "仅当前任务，可后续转正" },
  { value: "libraries", label: "选择 RAG 知识库", description: "使用已有长期知识库" },
]

function buildPreview(content: string) {
  const normalized = content.replace(/\s+/g, " ").trim()
  if (!normalized) return "暂无内容"
  return normalized.slice(0, 72)
}

function buildModeOptions(): ComboboxOption[] {
  return MODE_OPTIONS.map((option) => ({
    value: option.value,
    label: option.label,
    description: option.description,
    searchText: `${option.label} ${option.description || ""}`,
  }))
}

function buildLibraryOptions(libraries: MemoryLibrary[]): ComboboxOption[] {
  return libraries.map((library) => ({
    value: String(library.id),
    label: library.name,
    description: buildPreview(library.content),
    searchText: `${library.name} ${library.content}`,
  }))
}

export function TaskRagLibrarySelector({
  libraries,
  mode,
  selectedIds,
  onModeChange,
  onSelectedIdsChange,
  disabled = false,
  className,
  layout = "inline",
}: TaskRagLibrarySelectorProps) {
  const permanentLibraries = useMemo(
    () => libraries.filter((library) => !library.isTemporary),
    [libraries],
  )
  const modeOptions = useMemo(() => buildModeOptions(), [])
  const libraryOptions = useMemo(() => buildLibraryOptions(permanentLibraries), [permanentLibraries])
  const selectedLibraryId = selectedIds[0] ? String(selectedIds[0]) : ""

  const handleModeChange = (value: string) => {
    const nextMode = value as TaskMemoryLibraryMode
    onModeChange(nextMode)
    if (nextMode !== "libraries") {
      onSelectedIdsChange([])
    }
  }

  const handleLibraryChange = (value: string) => {
    const libraryId = Number(value)
    if (!libraryId) {
      onSelectedIdsChange([])
      return
    }
    onSelectedIdsChange([libraryId])
  }

  const modeCombobox = (
    <div className="relative">
      <Database className="pointer-events-none absolute left-3 top-1/2 z-10 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
      <Combobox
        id="task-rag-mode"
        items={modeOptions}
        value={mode}
        onValueChange={handleModeChange}
        placeholder="RAG"
        searchPlaceholder="搜索 RAG 选项"
        emptyText="未找到选项"
        disabled={disabled}
        inputClassName="pl-9"
        renderItem={(item) => (
          <div className="min-w-0 flex-1">
            <div className="truncate text-sm">{item.label}</div>
            {item.description ? (
              <div className="truncate text-xs text-muted-foreground">{item.description}</div>
            ) : null}
          </div>
        )}
      />
    </div>
  )

  const libraryCombobox =
    mode === "libraries" ? (
      <div className="relative">
        <Database className="pointer-events-none absolute left-3 top-1/2 z-10 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
        <Combobox
          id="task-rag-library"
          items={libraryOptions}
          value={selectedLibraryId}
          onValueChange={handleLibraryChange}
          placeholder={libraryOptions.length === 0 ? "暂无 RAG 知识库" : "选择 RAG 知识库"}
          searchPlaceholder="搜索 RAG 知识库"
          emptyText="未找到 RAG 知识库"
          disabled={disabled || libraryOptions.length === 0}
          inputClassName="pl-9"
          renderItem={(item) => (
            <div className="min-w-0 flex-1">
              <div className="truncate text-sm">{item.label}</div>
              {item.description ? (
                <div className="truncate text-xs text-muted-foreground">{item.description}</div>
              ) : null}
            </div>
          )}
        />
      </div>
    ) : null

  if (layout === "stacked") {
    return (
      <div className={cn("space-y-3", className)}>
        {modeCombobox}
        {libraryCombobox}
        {mode === "temporary" ? (
          <p className="text-xs text-muted-foreground">
            创建任务时会自动生成临时 RAG 知识库，可在任务执行过程中写入内容；需要长期保留时可在 RAG 页面转正。
          </p>
        ) : null}
      </div>
    )
  }

  return (
    <div className={cn("grid gap-3 md:col-span-3 md:grid-cols-3", className)}>
      {modeCombobox}
      {libraryCombobox}
      {mode === "libraries" ? <div className="hidden md:block" /> : null}
    </div>
  )
}

export function validateTaskRagLibrarySelection(
  mode: TaskMemoryLibraryMode,
  selectedIds: number[],
): string | null {
  if (mode === "libraries" && selectedIds.length === 0) {
    return "请选择一个 RAG 知识库"
  }
  return null
}

export function buildTaskRagLibraryPayload(
  mode: TaskMemoryLibraryMode,
  selectedIds: number[],
): Pick<import("@/lib/api").TaskCreate, "memoryLibraryMode" | "memoryLibraryIds"> {
  return {
    memoryLibraryMode: mode,
    memoryLibraryIds: mode === "libraries" ? selectedIds : undefined,
  }
}

/** @deprecated 使用 TaskRagLibrarySelector */
export const TaskMemoryLibrarySelector = TaskRagLibrarySelector
export const validateTaskMemoryLibrarySelection = validateTaskRagLibrarySelection
export const buildTaskMemoryLibraryPayload = buildTaskRagLibraryPayload
