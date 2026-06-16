import { useCallback, useEffect, useMemo, useState } from "react"
import {
  ChevronDown,
  ChevronRight,
  File,
  Folder,
  FolderOpen,
  Loader2,
  Save,
} from "lucide-react"
import { Button } from "@/components/ui/button"
import { Textarea } from "@/components/ui/textarea"
import { api, type TaskFilesystemEntry, type TaskFilesystemRoot } from "@/lib/api"
import { cn } from "@/lib/utils"
import { toast } from "sonner"

interface WorkbenchFilesystemPanelProps {
  taskId: number
  visible?: boolean
}

interface TreeNodeState {
  loading: boolean
  loaded: boolean
  entries: TaskFilesystemEntry[]
  error?: string
}

function normalizePath(path: string): string {
  return path.replace(/\\/g, "/").replace(/^\/+/, "")
}

export function WorkbenchFilesystemPanel({
  taskId,
  visible = true,
}: WorkbenchFilesystemPanelProps) {
  const [roots, setRoots] = useState<TaskFilesystemRoot[]>([])
  const [rootsLoading, setRootsLoading] = useState(false)
  const [activeRootId, setActiveRootId] = useState<string | null>(null)
  const [expandedPaths, setExpandedPaths] = useState<Set<string>>(new Set())
  const [treeStateByPath, setTreeStateByPath] = useState<Record<string, TreeNodeState>>({})
  const [selectedFilePath, setSelectedFilePath] = useState<string | null>(null)
  const [editorContent, setEditorContent] = useState("")
  const [savedContent, setSavedContent] = useState("")
  const [fileLoading, setFileLoading] = useState(false)
  const [fileBinary, setFileBinary] = useState(false)
  const [saving, setSaving] = useState(false)

  const activeRoot = useMemo(
    () => roots.find((root) => root.id === activeRootId) ?? null,
    [activeRootId, roots],
  )

  const isDirty = selectedFilePath != null && editorContent !== savedContent

  const loadDirectory = useCallback(async (rootId: string, path: string) => {
    const key = `${rootId}:${normalizePath(path)}`
    setTreeStateByPath((prev) => ({
      ...prev,
      [key]: {
        loading: true,
        loaded: prev[key]?.loaded ?? false,
        entries: prev[key]?.entries ?? [],
        error: undefined,
      },
    }))
    try {
      const entries = await api.listTaskFilesystem(taskId, rootId, path)
      setTreeStateByPath((prev) => ({
        ...prev,
        [key]: {
          loading: false,
          loaded: true,
          entries,
        },
      }))
    } catch (error) {
      const message = error instanceof Error ? error.message : "加载目录失败"
      setTreeStateByPath((prev) => ({
        ...prev,
        [key]: {
          loading: false,
          loaded: true,
          entries: [],
          error: message,
        },
      }))
    }
  }, [taskId])

  useEffect(() => {
    let cancelled = false
    setRootsLoading(true)
    setRoots([])
    setActiveRootId(null)
    setExpandedPaths(new Set())
    setTreeStateByPath({})
    setSelectedFilePath(null)
    setEditorContent("")
    setSavedContent("")
    setFileBinary(false)

    void api.getTaskFilesystemRoots(taskId)
      .then((response) => {
        if (cancelled) return
        setRoots(response.roots)
        setActiveRootId(response.roots[0]?.id ?? null)
      })
      .catch((error) => {
        if (cancelled) return
        toast.error(error instanceof Error ? error.message : "加载文件根目录失败")
      })
      .finally(() => {
        if (!cancelled) setRootsLoading(false)
      })

    return () => {
      cancelled = true
    }
  }, [taskId])

  useEffect(() => {
    if (!activeRootId) return
    void loadDirectory(activeRootId, "")
  }, [activeRootId, loadDirectory])

  const toggleFolder = useCallback((path: string) => {
    if (!activeRootId) return
    const normalized = normalizePath(path)
    setExpandedPaths((prev) => {
      const next = new Set(prev)
      if (next.has(normalized)) {
        next.delete(normalized)
      } else {
        next.add(normalized)
        const key = `${activeRootId}:${normalized}`
        const state = treeStateByPath[key]
        if (!state?.loaded && !state?.loading) {
          void loadDirectory(activeRootId, normalized)
        }
      }
      return next
    })
  }, [activeRootId, loadDirectory, treeStateByPath])

  const openFile = useCallback(async (path: string) => {
    if (!activeRootId) return
    const normalized = normalizePath(path)
    setSelectedFilePath(normalized)
    setFileLoading(true)
    setFileBinary(false)
    try {
      const response = await api.readTaskFilesystem(taskId, activeRootId, normalized)
      if (response.binary) {
        setFileBinary(true)
        setEditorContent("")
        setSavedContent("")
      } else {
        setEditorContent(response.content)
        setSavedContent(response.content)
      }
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "读取文件失败")
      setSelectedFilePath(null)
    } finally {
      setFileLoading(false)
    }
  }, [activeRootId, taskId])

  const handleSave = useCallback(async () => {
    if (!activeRootId || !selectedFilePath || fileBinary) return
    setSaving(true)
    try {
      await api.writeTaskFilesystem(taskId, {
        root: activeRootId,
        path: selectedFilePath,
        content: editorContent,
      })
      setSavedContent(editorContent)
      toast.success("已保存")
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "保存失败")
    } finally {
      setSaving(false)
    }
  }, [activeRootId, editorContent, fileBinary, selectedFilePath, taskId])

  const renderEntries = useCallback((entries: TaskFilesystemEntry[], depth: number) => {
    return entries.map((entry) => {
      const normalized = normalizePath(entry.path)
      const isExpanded = expandedPaths.has(normalized)
      const childKey = activeRootId ? `${activeRootId}:${normalized}` : ""
      const childState = childKey ? treeStateByPath[childKey] : undefined

      if (entry.isDir) {
        return (
          <div key={entry.path}>
            <button
              type="button"
              onClick={() => toggleFolder(normalized)}
              className="flex w-full items-center rounded px-2 py-1 text-left text-sm hover:bg-accent/50"
              style={{ paddingLeft: `${depth * 12 + 8}px` }}
            >
              {isExpanded ? (
                <ChevronDown className="mr-1 h-3.5 w-3.5 shrink-0 text-muted-foreground" />
              ) : (
                <ChevronRight className="mr-1 h-3.5 w-3.5 shrink-0 text-muted-foreground" />
              )}
              {isExpanded ? (
                <FolderOpen className="mr-1.5 h-4 w-4 shrink-0 text-blue-500" />
              ) : (
                <Folder className="mr-1.5 h-4 w-4 shrink-0 text-blue-500" />
              )}
              <span className="truncate">{entry.name}</span>
            </button>
            {isExpanded ? (
              <div>
                {childState?.loading ? (
                  <div
                    className="flex items-center gap-2 px-2 py-1 text-xs text-muted-foreground"
                    style={{ paddingLeft: `${(depth + 1) * 12 + 8}px` }}
                  >
                    <Loader2 className="h-3 w-3 animate-spin" />
                    加载中...
                  </div>
                ) : null}
                {childState?.error ? (
                  <div
                    className="px-2 py-1 text-xs text-destructive"
                    style={{ paddingLeft: `${(depth + 1) * 12 + 8}px` }}
                  >
                    {childState.error}
                  </div>
                ) : null}
                {childState?.entries?.length
                  ? renderEntries(childState.entries, depth + 1)
                  : null}
                {childState?.loaded && !childState.loading && !childState.error && childState.entries.length === 0 ? (
                  <div
                    className="px-2 py-1 text-xs text-muted-foreground"
                    style={{ paddingLeft: `${(depth + 1) * 12 + 8}px` }}
                  >
                    空目录
                  </div>
                ) : null}
              </div>
            ) : null}
          </div>
        )
      }

      return (
        <button
          key={entry.path}
          type="button"
          onClick={() => void openFile(normalized)}
          className={cn(
            "flex w-full items-center rounded px-2 py-1 text-left text-sm hover:bg-accent/50",
            selectedFilePath === normalized && "bg-accent text-accent-foreground",
          )}
          style={{ paddingLeft: `${depth * 12 + 28}px` }}
        >
          <File className="mr-1.5 h-4 w-4 shrink-0 text-muted-foreground" />
          <span className="truncate">{entry.name}</span>
        </button>
      )
    })
  }, [activeRootId, expandedPaths, openFile, selectedFilePath, toggleFolder, treeStateByPath])

  const rootEntries = activeRootId
    ? treeStateByPath[`${activeRootId}:`]?.entries ?? []
    : []
  const rootLoading = activeRootId
    ? treeStateByPath[`${activeRootId}:`]?.loading ?? false
    : false
  const rootError = activeRootId
    ? treeStateByPath[`${activeRootId}:`]?.error
    : undefined

  return (
    <div
      className={cn(
        "flex min-h-0 flex-1 flex-col bg-background",
        !visible && "pointer-events-none",
      )}
      aria-hidden={!visible}
    >
      <div className="flex items-center gap-1 border-b border-border/60 px-2 py-1.5">
        {rootsLoading ? (
          <div className="flex items-center gap-2 px-2 text-xs text-muted-foreground">
            <Loader2 className="h-3.5 w-3.5 animate-spin" />
            加载目录...
          </div>
        ) : (
          roots.map((root) => (
            <button
              key={root.id}
              type="button"
              className={cn(
                "rounded-md px-2.5 py-1 text-xs transition-colors",
                activeRootId === root.id
                  ? "bg-muted font-medium text-foreground"
                  : "text-muted-foreground hover:bg-muted/50 hover:text-foreground",
              )}
              onClick={() => {
                setActiveRootId(root.id)
                setExpandedPaths(new Set())
                setSelectedFilePath(null)
                setEditorContent("")
                setSavedContent("")
                setFileBinary(false)
              }}
              title={root.path}
            >
              {root.label}
            </button>
          ))
        )}
      </div>

      <div className="grid min-h-0 flex-1 grid-cols-[minmax(12rem,18rem)_1fr]">
        <div className="min-h-0 overflow-auto border-r border-border/60 bg-muted/10">
          {rootLoading ? (
            <div className="flex items-center gap-2 p-3 text-xs text-muted-foreground">
              <Loader2 className="h-3.5 w-3.5 animate-spin" />
              读取文件树...
            </div>
          ) : rootError ? (
            <div className="p-3 text-xs text-destructive">{rootError}</div>
          ) : rootEntries.length > 0 ? (
            <div className="py-1">{renderEntries(rootEntries, 0)}</div>
          ) : (
            <div className="p-3 text-xs text-muted-foreground">
              {activeRoot?.path ? "目录为空或路径不存在" : "没有可用目录"}
            </div>
          )}
        </div>

        <div className="flex min-h-0 min-w-0 flex-1 flex-col">
          <div className="flex items-center justify-between gap-2 border-b border-border/60 px-3 py-2">
            <div className="min-w-0 truncate text-xs text-muted-foreground">
              {selectedFilePath ?? "选择文件以编辑"}
            </div>
            <Button
              type="button"
              size="sm"
              variant="outline"
              className="h-7 gap-1.5 px-2 text-xs"
              disabled={!selectedFilePath || fileBinary || !isDirty || saving}
              onClick={() => void handleSave()}
            >
              {saving ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Save className="h-3.5 w-3.5" />}
              保存
            </Button>
          </div>

          <div className="min-h-0 flex-1">
            {fileLoading ? (
              <div className="flex h-full items-center justify-center text-sm text-muted-foreground">
                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                读取文件...
              </div>
            ) : fileBinary ? (
              <div className="flex h-full items-center justify-center p-6 text-sm text-muted-foreground">
                该文件为二进制内容，无法在编辑器中打开
              </div>
            ) : selectedFilePath ? (
              <Textarea
                value={editorContent}
                onChange={(event) => setEditorContent(event.target.value)}
                className="h-full min-h-0 resize-none rounded-none border-0 bg-background font-mono text-xs shadow-none focus-visible:ring-0"
                spellCheck={false}
              />
            ) : (
              <div className="flex h-full items-center justify-center p-6 text-sm text-muted-foreground">
                从左侧文件树选择文件
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}
