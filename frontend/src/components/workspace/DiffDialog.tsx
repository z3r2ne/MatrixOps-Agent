import React, { useMemo, useState, useEffect, useCallback, useRef } from "react"
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter, DialogClose } from "@/components/ui/dialog"
import { Button } from "@/components/ui/button"
import { Label } from "@/components/ui/label"
import { Textarea } from "@/components/ui/textarea"
import { ScrollArea } from "@/components/ui/scroll-area"
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { PanelDividerHandle } from "@/components/ui/panel-divider-handle"
import { useConfirmDialog } from "@/components/ConfirmDialogProvider"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import {
  Loader2,
  FileCode,
  FileText,
  FilePlus,
  FileMinus,
  SplitSquareHorizontal,
  List,
  GitBranch,
  GitCommit,
  GitMerge,
  Bot,
  Folder,
  FolderOpen,
  ChevronRight,
  ChevronDown,
  ChevronLeft,
  Clock,
  RotateCcw,
  CircleDot,
  GitCompare,
  MoreVertical,
  X,
} from "lucide-react"
import { DiffModeEnum } from "@git-diff-view/react"
import "@git-diff-view/react/styles/diff-view.css"
import { UnifiedDiffContent } from "./UnifiedDiffContent"
import "@/diff-overrides.css"
import { api, type Task, type Project } from "@/lib/api"
import { cn } from "@/lib/utils"
import { toast } from "sonner"
import { DiffCodeSearchBar } from "./DiffCodeSearchBar"

interface DiffDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  taskId: number
  taskTitle: string
  task?: Task
}

interface DiffFile {
  path: string
  action: string
  unified_diff?: string
  oldContent?: string
  newContent?: string
}

interface FileTreeNode {
  name: string
  path: string
  type: "file" | "folder"
  children?: FileTreeNode[]
  action?: string
  diffFile?: DiffFile
}

type PatchRow = {
  id?: string
  partId: string
  hash: string
  snapshot: string
  startSnapshot?: string
  sessionId?: string
  timestamp: number
  description?: string
}

type DiffTarget =
  | { kind: "worktree" }
  | { kind: "commit"; hash: string; subject?: string }
  | { kind: "snapshot"; partId: string; id?: string }

type TimelineRow =
  | { key: string; kind: "commit"; timestamp: number; hash: string; subject: string }
  | { key: string; kind: "snapshot"; timestamp: number; patch: PatchRow }

const DIFF_DIALOG_TIMELINE_COLLAPSED_KEY = "diff-dialog:timeline-collapsed"
const DIFF_DIALOG_SIDEBAR_COLLAPSED_KEY = "diff-dialog:sidebar-collapsed"
const DIFF_DIALOG_FILES_PANEL_WIDTH_KEY = "diff-dialog:files-panel-width"

function readStoredBoolean(key: string, fallback = false) {
  if (typeof window === "undefined") return fallback
  const value = window.localStorage.getItem(key)
  if (value == null) return fallback
  return value === "1"
}

function readStoredWidth(key: string, fallback: number) {
  if (typeof window === "undefined") return fallback
  const raw = window.localStorage.getItem(key)
  const parsed = Number(raw)
  if (!Number.isFinite(parsed)) return fallback
  return Math.max(220, Math.min(640, parsed))
}

function buildFileTree(diffs: DiffFile[]): FileTreeNode[] {
  const root: FileTreeNode[] = []

  diffs.forEach((diff) => {
    const parts = diff.path.split("/")
    let currentLevel = root
    let currentPath = ""

    parts.forEach((part, index) => {
      currentPath = currentPath ? `${currentPath}/${part}` : part
      const isFile = index === parts.length - 1

      let existingNode = currentLevel.find((node) => node.name === part)

      if (!existingNode) {
        const newNode: FileTreeNode = {
          name: part,
          path: currentPath,
          type: isFile ? "file" : "folder",
          children: isFile ? undefined : [],
        }

        if (isFile) {
          newNode.action = diff.action
          newNode.diffFile = diff
        }

        currentLevel.push(newNode)
        existingNode = newNode
      }

      if (!isFile && existingNode.children) {
        currentLevel = existingNode.children
      }
    })
  })

  const sortNodes = (nodes: FileTreeNode[]): FileTreeNode[] => {
    return nodes
      .sort((a, b) => {
        if (a.type !== b.type) {
          return a.type === "folder" ? -1 : 1
        }
        return a.name.localeCompare(b.name)
      })
      .map((node) => {
        if (node.children) {
          node.children = sortNodes(node.children)
        }
        return node
      })
  }

  return sortNodes(root)
}

function targetKey(t: DiffTarget): string {
  if (t.kind === "worktree") return "worktree"
  if (t.kind === "commit") return `commit:${t.hash}`
  return `snap:${t.id || t.partId}`
}

function normalizeSnapshotTs(ts: number): number {
  if (ts > 1_000_000_000_000) return Math.floor(ts / 1000)
  return ts
}

function findPatchInRows(rows: TimelineRow[], partId: string, id?: string): PatchRow | null {
  for (const r of rows) {
    if (r.kind !== "snapshot") continue
    if (r.patch.partId === partId || (id && r.patch.id === id)) return r.patch
  }
  return null
}

const diffFetchOpts = { includePatches: false as const }

export function DiffDialog({ open, onOpenChange, taskId, taskTitle, task: taskProp }: DiffDialogProps) {
  const { confirm } = useConfirmDialog()
  const [loading, setLoading] = useState(true)
  const [diffs, setDiffs] = useState<DiffFile[]>([])
  const [timelineRows, setTimelineRows] = useState<TimelineRow[]>([])
  const [timelineMeta, setTimelineMeta] = useState<{ baseBranch: string; baseCommitHash: string } | null>(null)

  const [target, setTarget] = useState<DiffTarget>({ kind: "worktree" })
  const [worktreeBasis, setWorktreeBasis] = useState<"default" | "base" | "parent">("default")
  const [snapshotTab, setSnapshotTab] = useState<"step" | "total">("step")

  const [viewMode, setViewMode] = useState<DiffModeEnum>(() => {
    const saved = localStorage.getItem("diff-view-mode")
    return saved === "split" ? DiffModeEnum.Split : DiffModeEnum.Unified
  })
  const [selectedFile, setSelectedFile] = useState<string | null>(null)
  const [task, setTask] = useState<Task | null>(taskProp || null)
  const [project, setProject] = useState<Project | null>(null)
  const [expandedFolders, setExpandedFolders] = useState<Set<string>>(new Set())

  const [commitDialogOpen, setCommitDialogOpen] = useState(false)
  const [commitMessage, setCommitMessage] = useState("")
  const [isCommitting, setIsCommitting] = useState(false)
  const [isMerging, setIsMerging] = useState(false)
  const [isRestoring, setIsRestoring] = useState(false)
  const [isGenerating, setIsGenerating] = useState(false)

  const [compareAnchor, setCompareAnchor] = useState<DiffTarget | null>(null)
  const [compareSelecting, setCompareSelecting] = useState(false)
  const [timelineCollapsed, setTimelineCollapsed] = useState(() => readStoredBoolean(DIFF_DIALOG_TIMELINE_COLLAPSED_KEY, false))
  const [sidebarCollapsed, setSidebarCollapsed] = useState(() => readStoredBoolean(DIFF_DIALOG_SIDEBAR_COLLAPSED_KEY, false))
  const [filesPanelWidth, setFilesPanelWidth] = useState(() => readStoredWidth(DIFF_DIALOG_FILES_PANEL_WIDTH_KEY, 320))
  const diffCodePanelRef = useRef<HTMLDivElement>(null)
  const handleCopyLabel = useCallback((text: string, label: string) => {
    navigator.clipboard.writeText(text)
    toast.success(`已复制${label}`)
  }, [])

  const isWorktree = useMemo(() => {
    if (!task || !project) return false
    return !!(task.workDir && task.workDir !== project.path)
  }, [task, project])

  const findPatch = useCallback(
    (partId: string, id?: string) => findPatchInRows(timelineRows, partId, id),
    [timelineRows],
  )

  const selectedPatch = useMemo(() => {
    if (target.kind !== "snapshot") return null
    return findPatchInRows(timelineRows, target.partId, target.id)
  }, [timelineRows, target])

  useEffect(() => {
    if (!open) {
      setTarget({ kind: "worktree" })
      setWorktreeBasis("default")
      setSnapshotTab("step")
      setCompareAnchor(null)
      setCompareSelecting(false)
    }
  }, [open])

  useEffect(() => {
    if (typeof window === "undefined") return
    window.localStorage.setItem(DIFF_DIALOG_TIMELINE_COLLAPSED_KEY, timelineCollapsed ? "1" : "0")
  }, [timelineCollapsed])

  useEffect(() => {
    if (typeof window === "undefined") return
    window.localStorage.setItem(DIFF_DIALOG_SIDEBAR_COLLAPSED_KEY, sidebarCollapsed ? "1" : "0")
  }, [sidebarCollapsed])

  useEffect(() => {
    if (typeof window === "undefined") return
    window.localStorage.setItem(DIFF_DIALOG_FILES_PANEL_WIDTH_KEY, String(filesPanelWidth))
  }, [filesPanelWidth])

  useEffect(() => {
    if (!open || !taskId) return

    const loadTaskAndProject = async () => {
      try {
        const taskData = taskProp || (await api.getTask(taskId))
        setTask(taskData)
        if (taskData.projectId) {
          const projectData = await api.getProject(taskData.projectId)
          setProject(projectData)
        }
      } catch (error) {
        console.error("Failed to load task/project:", error)
      }
    }

    void loadTaskAndProject()
  }, [open, taskId, taskProp])

  const loadTimeline = useCallback(async () => {
    if (!taskId) return
    try {
      const data = await api.getTaskGitTimeline(taskId)
      setTimelineMeta({ baseBranch: data.baseBranch, baseCommitHash: data.baseCommitHash })
      const rows: TimelineRow[] = []
      for (const it of data.items) {
        if (it.kind === "commit") {
          rows.push({
            key: `commit:${it.commit.hash}`,
            kind: "commit",
            timestamp: it.timestamp,
            hash: it.commit.hash,
            subject: it.commit.subject,
          })
        } else {
          const s = it.snapshot
          rows.push({
            key: `snap:${s.id || s.partId}`,
            kind: "snapshot",
            timestamp: it.timestamp,
            patch: {
              id: s.id,
              partId: s.partId,
              hash: s.hash,
              snapshot: s.snapshot,
              startSnapshot: s.startSnapshot,
              sessionId: s.sessionId,
              timestamp: s.timestamp,
              description: s.description,
            },
          })
        }
      }
      setTimelineRows(rows)
    } catch (e) {
      console.error(e)
      toast.error("加载 Git 时间线失败")
    }
  }, [taskId])

  useEffect(() => {
    if (!open || !taskId) return
    void loadTimeline()
  }, [open, taskId, loadTimeline])

  const loadDiffs = useCallback(async () => {
    if (!taskId) return

    setLoading(true)
    setDiffs([])
    setSelectedFile(null)

    try {
      let result: Awaited<ReturnType<typeof api.getTaskDiff>>

      if (target.kind === "commit") {
        result = await api.getTaskDiff(taskId, { atCommit: target.hash, ...diffFetchOpts })
      } else if (target.kind === "worktree") {
        result = await api.getTaskDiff(taskId, { basis: worktreeBasis, ...diffFetchOpts })
      } else {
        const patch = findPatchInRows(timelineRows, target.partId, target.id)
        if (!patch) {
          result = await api.getTaskDiff(taskId, { basis: "default", ...diffFetchOpts })
        } else if (snapshotTab === "step") {
          result = await api.getTaskDiff(taskId, { hash: patch.hash, toHash: patch.snapshot, ...diffFetchOpts })
        } else {
          const fromHash = task?.baseCommitHash?.trim() || patch.startSnapshot || undefined
          if (!fromHash) {
            toast.error("缺少 base 提交哈希，无法对比")
            setLoading(false)
            return
          }
          result = await api.getTaskDiff(taskId, { hash: fromHash, toHash: patch.snapshot, ...diffFetchOpts })
        }
      }

      applyFiles(result)
    } catch (error) {
      console.error("Failed to load git diff:", error)
      toast.error("加载 diff 失败", {
        description: error instanceof Error ? error.message : "未知错误",
      })
    } finally {
      setLoading(false)
    }

    function applyFiles(res: Awaited<ReturnType<typeof api.getTaskDiff>>) {
      if (res.files && res.files.length > 0) {
        const diffFiles: DiffFile[] = res.files.map((file) => ({
          path: file.path,
          action: "edit",
          unified_diff: file.diff,
        }))
        setDiffs(diffFiles)
        if (diffFiles.length > 0) {
          setSelectedFile(diffFiles[0].path)
        }
      } else {
        setDiffs([])
      }
    }
  }, [taskId, target, worktreeBasis, snapshotTab, task?.baseCommitHash, timelineRows])

  useEffect(() => {
    if (!open || !taskId) return
    void loadDiffs()
  }, [open, taskId, target, worktreeBasis, snapshotTab, loadDiffs])

  const compareRefs = useCallback(
    (a: DiffTarget, b: DiffTarget): { mode: "git"; from: string; to: string } | { mode: "snap"; from: string; to: string } | null => {
      if (a.kind === "worktree" || b.kind === "worktree") {
        const commitSide = a.kind === "commit" ? a : b.kind === "commit" ? b : null
        if (!commitSide || commitSide.kind !== "commit") return null
        const other = targetKey(a) === targetKey(commitSide) ? b : a
        if (other.kind !== "worktree") return null
        return { mode: "git", from: commitSide.hash, to: "HEAD" }
      }
      if (a.kind === "commit" && b.kind === "commit") {
        const ta = timelineRows.find((r) => r.kind === "commit" && r.hash === a.hash)?.timestamp ?? 0
        const tb = timelineRows.find((r) => r.kind === "commit" && r.hash === b.hash)?.timestamp ?? 0
        const [first, second] = ta <= tb ? [a.hash, b.hash] : [b.hash, a.hash]
        return { mode: "git", from: first, to: second }
      }
      if (a.kind === "snapshot" && b.kind === "snapshot") {
        const pa = findPatch(a.partId, a.id)
        const pb = findPatch(b.partId, b.id)
        if (!pa || !pb) return null
        const tsa = normalizeSnapshotTs(pa.timestamp)
        const tsb = normalizeSnapshotTs(pb.timestamp)
        const [first, second] = tsa <= tsb ? [pa.snapshot, pb.snapshot] : [pb.snapshot, pa.snapshot]
        return { mode: "snap", from: first, to: second }
      }
      return null
    },
    [findPatch, timelineRows],
  )

  const handleViewModeChange = (mode: DiffModeEnum) => {
    setViewMode(mode)
    localStorage.setItem("diff-view-mode", mode === DiffModeEnum.Split ? "split" : "unified")
  }

  const handleOpenCommitDialog = () => {
    setCommitMessage("")
    setCommitDialogOpen(true)
  }

  const handleCommit = async () => {
    if (!commitMessage.trim()) {
      toast.error("请输入提交消息")
      return
    }

    setIsCommitting(true)
    try {
      const result = await api.gitCommit(taskId, commitMessage.trim())
      toast.success(`提交成功: ${result.commit}`)
      setCommitDialogOpen(false)
      setCommitMessage("")
      await loadTimeline()
      await loadDiffs()
    } catch (error: unknown) {
      toast.error(`提交失败: ${error instanceof Error ? error.message : "未知错误"}`)
    } finally {
      setIsCommitting(false)
    }
  }

  const handleMerge = async () => {
    if (!task?.branch) {
      toast.error("无法获取分支信息")
      return
    }

    const confirmed = await confirm({
      title: "合并分支",
      description: `确定要将分支 "${task.branch}" 合并到主分支吗？\n\n这将使用任务名称作为提交消息。`,
      confirmLabel: "合并",
      tone: "destructive",
    })
    if (!confirmed) return

    setIsMerging(true)
    try {
      const result = await api.gitMerge(taskId, taskTitle)
      toast.success(result.message || "合并成功")
      onOpenChange(false)
    } catch (error: unknown) {
      toast.error(`合并失败: ${error instanceof Error ? error.message : "未知错误"}`)
    } finally {
      setIsMerging(false)
    }
  }

  const applyTaskSnapshotMode = async (mode: "undo_step" | "restore_checkpoint") => {
    if (!selectedPatch?.partId) {
      toast.error("无法应用：缺少快照部件信息")
      return
    }

    const confirmed = await confirm({
      title: mode === "undo_step" ? "撤销本次更改" : "恢复到此快照",
      description:
        mode === "undo_step"
          ? "确定要撤销本次更改吗？\n\n工作区将恢复到该步之前的状态，并删除此检查点记录。"
          : "确定要恢复到此快照吗？\n\n工作区将恢复到该步完成后的状态，并移除时间线上之后的所有检查点。",
      confirmLabel: mode === "undo_step" ? "撤销" : "恢复",
      tone: "destructive",
    })
    if (!confirmed) return

    setIsRestoring(true)
    try {
      const result = await api.applyTaskSnapshot(taskId, {
        mode,
        codeSnapshotId: selectedPatch.id,
        partId: selectedPatch.partId,
      })
      toast.success(result.message || "操作成功")
      setTarget({ kind: "worktree" })
      setSnapshotTab("step")
      await loadTimeline()
      await loadDiffs()
    } catch (error: unknown) {
      toast.error(`${error instanceof Error ? error.message : "未知错误"}`)
    } finally {
      setIsRestoring(false)
    }
  }

  const handleGenerateMessage = async () => {
    if (diffs.length === 0) {
      toast.error("没有可用的代码变更")
      return
    }

    setIsGenerating(true)
    try {
      const allDiffs = diffs.map((d) => `文件: ${d.path}\n${d.unified_diff || ""}`).join("\n\n")
      const result = await api.generateCommitMessage(allDiffs)
      setCommitMessage(result.message)
      toast.success("提交消息已生成")
    } catch (error: unknown) {
      toast.error(`生成失败: ${error instanceof Error ? error.message : "未知错误"}`)
    } finally {
      setIsGenerating(false)
    }
  }

  const selectedDiff = useMemo(() => diffs.find((d) => d.path === selectedFile), [diffs, selectedFile])

  const stats = useMemo(() => {
    let totalFiles = diffs.length
    let additions = 0
    let deletions = 0

    diffs.forEach((diff) => {
      if (diff.unified_diff) {
        const lines = diff.unified_diff.split("\n")
        lines.forEach((line) => {
          if (line.startsWith("+") && !line.startsWith("+++")) additions++
          if (line.startsWith("-") && !line.startsWith("---")) deletions++
        })
      }
    })

    return { totalFiles, additions, deletions }
  }, [diffs])

  const getFileIcon = (action: string) => {
    switch (action) {
      case "write":
        return <FilePlus className="h-4 w-4 text-green-600" />
      case "delete":
        return <FileMinus className="h-4 w-4 text-red-600" />
      case "edit":
        return <FileText className="h-4 w-4 text-blue-600" />
      default:
        return <FileCode className="h-4 w-4 text-muted-foreground" />
    }
  }

  const fileTree = useMemo(() => buildFileTree(diffs), [diffs])

  const toggleFolder = (path: string) => {
    setExpandedFolders((prev) => {
      const next = new Set(prev)
      if (next.has(path)) next.delete(path)
      else next.add(path)
      return next
    })
  }

  useEffect(() => {
    if (diffs.length > 0) {
      const allFolderPaths = new Set<string>()
      diffs.forEach((diff) => {
        const parts = diff.path.split("/")
        let currentPath = ""
        for (let i = 0; i < parts.length - 1; i++) {
          currentPath = currentPath ? `${currentPath}/${parts[i]}` : parts[i]
          allFolderPaths.add(currentPath)
        }
      })
      setExpandedFolders(allFolderPaths)
    }
  }, [diffs])

  const renderTreeNode = (node: FileTreeNode, depth: number = 0) => {
    const isExpanded = expandedFolders.has(node.path)

    if (node.type === "folder") {
      return (
        <div key={node.path}>
          <button
            type="button"
            onClick={() => toggleFolder(node.path)}
            className={cn(
              "flex w-full items-center rounded transition-colors hover:bg-accent/50",
              "group",
            )}
          >
            <div
              className="flex min-w-max items-center gap-1.5 px-2 py-1.5 text-left text-sm"
              style={{ paddingLeft: `${depth * 12 + 8}px` }}
            >
              {isExpanded ? (
                <ChevronDown className="h-3.5 w-3.5 text-muted-foreground shrink-0" />
              ) : (
                <ChevronRight className="h-3.5 w-3.5 text-muted-foreground shrink-0" />
              )}
              {isExpanded ? (
                <FolderOpen className="h-4 w-4 text-blue-500 shrink-0" />
              ) : (
                <Folder className="h-4 w-4 text-blue-500 shrink-0" />
              )}
              <span className="font-medium whitespace-nowrap">{node.name}</span>
            </div>
          </button>
          {isExpanded && node.children && (
            <div>{node.children.map((child) => renderTreeNode(child, depth + 1))}</div>
          )}
        </div>
      )
    }

    return (
      <button
        type="button"
        key={node.path}
        onClick={() => setSelectedFile(node.path)}
        className={cn(
          "mb-0.5 flex w-full items-center rounded text-left transition-colors",
          selectedFile === node.path ? "bg-accent text-accent-foreground" : "hover:bg-accent/50",
        )}
      >
        <div
          className="flex min-w-max items-center gap-1.5 px-2 py-1.5 text-sm"
          style={{ paddingLeft: `${depth * 12 + 8 + 18}px` }}
        >
          {node.action && getFileIcon(node.action)}
          <span className="whitespace-nowrap">{node.name}</span>
        </div>
      </button>
    )
  }

  const onPickTimelineRow = (row: TimelineRow) => {
    if (compareSelecting && compareAnchor) {
      if (row.kind === "commit") {
        void finalizeCompare({ kind: "commit", hash: row.hash, subject: row.subject })
      } else {
        void finalizeCompare({ kind: "snapshot", partId: row.patch.partId, id: row.patch.id })
      }
      return
    }
    if (row.kind === "commit") {
      setTarget({ kind: "commit", hash: row.hash, subject: row.subject })
    } else {
      setTarget({ kind: "snapshot", partId: row.patch.partId, id: row.patch.id })
    }
  }

  async function finalizeCompare(second: DiffTarget) {
    if (!compareAnchor) return
    const refs = compareRefs(compareAnchor, second)
    if (!refs) {
      toast.error("无法对比这两种节点")
      setCompareSelecting(false)
      return
    }
    setCompareSelecting(false)
    setLoading(true)
    setDiffs([])
    setSelectedFile(null)
    try {
      let result: Awaited<ReturnType<typeof api.getTaskDiff>>
      if (refs.mode === "git") {
        result = await api.getTaskDiff(taskId, { gitFrom: refs.from, gitTo: refs.to, ...diffFetchOpts })
      } else {
        result = await api.getTaskDiff(taskId, { hash: refs.from, toHash: refs.to, ...diffFetchOpts })
      }
      if (result.files && result.files.length > 0) {
        const diffFiles: DiffFile[] = result.files.map((file) => ({
          path: file.path,
          action: "edit",
          unified_diff: file.diff,
        }))
        setDiffs(diffFiles)
        setSelectedFile(diffFiles[0]?.path ?? null)
      }
      setTarget(second)
      setCompareAnchor(null)
    } catch (e) {
      toast.error("对比加载失败")
    } finally {
      setLoading(false)
    }
  }

  const openMenuCompare = (t: DiffTarget) => {
    setCompareAnchor(t)
    setCompareSelecting(true)
    toast.message("对比模式：请点击时间线上另一个节点")
  }

  const restorePatchToSnapshot = async (patch: PatchRow) => {
    const confirmed = await confirm({
      title: "恢复检查点结束状态",
      description: `将工作区恢复为检查点结束状态？\n${patch.description || patch.snapshot.slice(0, 7)}`,
      confirmLabel: "恢复",
      tone: "destructive",
    })
    if (!confirmed) return
    setIsRestoring(true)
    try {
      await api.restoreSnapshot(taskId, patch.snapshot, true)
      toast.success("已恢复")
      await loadTimeline()
      setTarget({ kind: "worktree" })
    } catch (e) {
      toast.error("恢复失败")
    } finally {
      setIsRestoring(false)
    }
  }

  const restoreToCommit = async (hash: string) => {
    const confirmed = await confirm({
      title: "恢复到提交",
      description: `将工作区恢复到提交 ${hash.slice(0, 7)}？`,
      confirmLabel: "恢复",
      tone: "destructive",
    })
    if (!confirmed) return
    setIsRestoring(true)
    try {
      await api.restoreWorktreeRef(taskId, hash, true)
      toast.success("已恢复到该提交")
      await loadTimeline()
      setTarget({ kind: "worktree" })
    } catch (e) {
      toast.error("恢复失败")
    } finally {
      setIsRestoring(false)
    }
  }

  const discardAllWorktreeChanges = async () => {
    const confirmed = await confirm({
      title: "撤销所有未提交更改",
      description: "将当前工作区恢复到当前 HEAD 提交，并丢弃所有未提交修改。确定继续吗？",
      confirmLabel: "撤销全部",
      tone: "destructive",
    })
    if (!confirmed) return

    setIsRestoring(true)
    try {
      await api.restoreWorktreeRef(taskId, "HEAD", true)
      toast.success("已撤销当前工作区的所有未提交更改")
      await loadTimeline()
      await loadDiffs()
      setTarget({ kind: "worktree" })
    } catch (error) {
      toast.error("撤销失败")
    } finally {
      setIsRestoring(false)
    }
  }

  const viewPatchVsBase = (patch: PatchRow) => {
    const fromHash = task?.baseCommitHash?.trim() || patch.startSnapshot
    if (!fromHash) {
      toast.error("缺少 base 哈希")
      return
    }
    setTarget({ kind: "snapshot", partId: patch.partId, id: patch.id })
    setSnapshotTab("total")
  }

  const undoPatch = async (patch: PatchRow) => {
    const confirmed = await confirm({
      title: "撤销检查点更改",
      description: "撤销该检查点对应更改？",
      confirmLabel: "撤销",
      tone: "destructive",
    })
    if (!confirmed) return
    setIsRestoring(true)
    try {
      await api.applyTaskSnapshot(taskId, {
        mode: "undo_step",
        codeSnapshotId: patch.id,
        partId: patch.partId,
      })
      toast.success("已撤销")
      await loadTimeline()
      setTarget({ kind: "worktree" })
    } catch (e) {
      toast.error("撤销失败")
    } finally {
      setIsRestoring(false)
    }
  }

  const baseLabel = timelineMeta?.baseBranch || task?.baseBranch || "main"
  const baseHashShort = (timelineMeta?.baseCommitHash || task?.baseCommitHash || "").slice(0, 7)

  return (
    <>
      <Dialog open={open} onOpenChange={onOpenChange}>
        <DialogContent
          hideClose
          srOnlyTitle="代码差异"
          onOpenAutoFocus={(event) => event.preventDefault()}
          className={cn(
            "flex flex-col p-0 gap-0 overflow-hidden",
            "!fixed !left-0 !right-0 !bottom-0 !top-[var(--electron-window-chrome-top,0px)] !h-[calc(100dvh-var(--electron-window-chrome-top,0px))] !w-screen !max-w-none !translate-x-0 !translate-y-0",
            "rounded-none border-0 sm:rounded-none",
            "data-[state=open]:animate-none data-[state=closed]:animate-none",
            "data-[state=open]:zoom-in-100 data-[state=closed]:zoom-out-100",
          )}
        >
          <DialogHeader className="shrink-0 space-y-3 border-b px-6 py-4">
            <div className="flex items-start gap-4">
              <div className="min-w-0 flex flex-1 flex-col gap-2">
                <div className="flex items-center gap-3 flex-wrap">
                  {compareSelecting && (
                    <span className="text-xs bg-amber-100 text-amber-900 px-2 py-0.5 rounded">
                      对比模式：请选择第二个节点
                    </span>
                  )}
                </div>
                <div className="flex min-w-0 items-center gap-2 overflow-hidden text-xs">
                  <p className="min-w-0 max-w-[220px] shrink truncate text-sm text-muted-foreground" title={taskTitle}>
                    {taskTitle}
                  </p>
                  {task?.branch ? (
                    <>
                      <button
                        type="button"
                        className="flex h-7 w-[112px] shrink-0 cursor-copy items-center gap-1.5 rounded bg-blue-50 px-2 text-[11px] text-blue-700"
                        title={task.branch}
                        onClick={() => handleCopyLabel(task.branch, "分支名")}
                      >
                        <GitBranch className="h-3 w-3 shrink-0" />
                        <span className="truncate font-medium">{task.branch}</span>
                      </button>
                      <button
                        type="button"
                        className="flex h-7 w-[170px] shrink-0 items-center gap-1.5 rounded bg-muted px-2 text-muted-foreground"
                        title={`${baseLabel}${baseHashShort ? ` @${baseHashShort}` : ""}`}
                        onClick={() => handleCopyLabel(baseLabel, "base 分支")}
                      >
                        <span className="shrink-0">base</span>
                        <span className="min-w-0 truncate font-mono">{baseLabel}</span>
                        {baseHashShort ? <span className="shrink-0 opacity-70">@{baseHashShort}</span> : null}
                      </button>
                    </>
                  ) : null}
                  {target.kind === "worktree" && (
                    <Tabs value={worktreeBasis} onValueChange={(v) => setWorktreeBasis(v as typeof worktreeBasis)}>
                      <TabsList className="h-8 border border-border bg-muted/40 p-0.5">
                        <TabsTrigger value="default" className="h-7 px-3 text-xs">
                          未提交变更
                        </TabsTrigger>
                        <TabsTrigger value="base" className="h-7 px-3 text-xs">
                          对比 base
                        </TabsTrigger>
                        <TabsTrigger value="parent" className="h-7 px-3 text-xs">
                          对比上一提交
                        </TabsTrigger>
                      </TabsList>
                    </Tabs>
                  )}

                  {target.kind === "snapshot" && selectedPatch && (
                    <Tabs value={snapshotTab} onValueChange={(v) => setSnapshotTab(v as typeof snapshotTab)}>
                      <TabsList className="h-8 border border-border bg-muted/40 p-0.5">
                        <TabsTrigger value="step" className="h-7 px-3 text-xs">
                          本次更改
                        </TabsTrigger>
                        <TabsTrigger value="total" className="h-7 px-3 text-xs">
                          对比 base
                        </TabsTrigger>
                      </TabsList>
                    </Tabs>
                  )}
                </div>
              </div>

              <div className="ml-auto flex items-center gap-2 flex-wrap justify-end shrink-0">
                <div className="flex items-center gap-3 text-xs text-muted-foreground shrink-0">
                  <span className="flex items-center gap-1">
                    <FileCode className="h-3.5 w-3.5" />
                    {stats.totalFiles} 个文件
                  </span>
                  <span className="text-green-600">+{stats.additions}</span>
                  <span className="text-red-600">-{stats.deletions}</span>
                </div>
                <div className="h-4 w-px bg-border" />

                {target.kind === "worktree" && (
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => void discardAllWorktreeChanges()}
                    disabled={isRestoring || isCommitting || isMerging}
                    className="text-amber-600 hover:text-amber-700 hover:bg-amber-50 border-amber-200"
                  >
                    <RotateCcw className="h-4 w-4 mr-1" />
                    {isRestoring ? "处理中…" : "撤销所有更改"}
                  </Button>
                )}

                {target.kind === "snapshot" && selectedPatch && (
                  <>
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => void applyTaskSnapshotMode("undo_step")}
                      disabled={isRestoring || isCommitting || isMerging}
                      className="text-amber-600 hover:text-amber-700 hover:bg-amber-50 border-amber-200"
                    >
                      <RotateCcw className="h-4 w-4 mr-1" />
                      {isRestoring ? "处理中…" : "撤销本次更改"}
                    </Button>
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => void applyTaskSnapshotMode("restore_checkpoint")}
                      disabled={isRestoring || isCommitting || isMerging}
                      className="text-blue-700 hover:text-blue-800 hover:bg-blue-50 border-blue-200"
                    >
                      <RotateCcw className="h-4 w-4 mr-1" />
                      {isRestoring ? "处理中…" : "恢复此快照"}
                    </Button>
                  </>
                )}

                <Button
                  variant="outline"
                  size="sm"
                  onClick={handleOpenCommitDialog}
                  disabled={isCommitting || isMerging || diffs.length === 0}
                >
                  <GitCommit className="h-4 w-4 mr-1" />
                  提交
                </Button>

                {isWorktree && (
                  <Button
                    variant="default"
                    size="sm"
                    onClick={() => void handleMerge()}
                    disabled={isCommitting || isMerging || diffs.length === 0 || !task?.branch}
                  >
                    <GitMerge className="h-4 w-4 mr-1" />
                    {isMerging ? "合并中…" : "合并"}
                  </Button>
                )}

                <div className="h-4 w-px bg-border" />

                <Button
                  variant={viewMode === DiffModeEnum.Unified ? "secondary" : "ghost"}
                  size="sm"
                  onClick={() => handleViewModeChange(DiffModeEnum.Unified)}
                >
                  <List className="h-4 w-4 mr-1" />
                  统一
                </Button>
                <Button
                  variant={viewMode === DiffModeEnum.Split ? "secondary" : "ghost"}
                  size="sm"
                  onClick={() => handleViewModeChange(DiffModeEnum.Split)}
                >
                  <SplitSquareHorizontal className="h-4 w-4 mr-1" />
                  分栏
                </Button>
              </div>
              <DialogClose asChild>
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  className="h-8 w-8 shrink-0"
                  aria-label="关闭"
                >
                  <X className="h-4 w-4" />
                </Button>
              </DialogClose>
            </div>
          </DialogHeader>

          <div className="flex-1 flex min-h-0 overflow-hidden">
            {/* Git 时间线 */}
            <div className={cn(
              "flex shrink-0 flex-col overflow-hidden bg-muted/20 transition-[width,opacity] duration-200 ease-out",
              sidebarCollapsed || timelineCollapsed ? "w-0 opacity-0" : "w-[220px] opacity-100",
            )}>
              <div className="px-3 py-2 border-b bg-muted/40">
                <p className="text-xs font-medium text-muted-foreground">变更时间线</p>
                <p className="text-[10px] text-muted-foreground mt-0.5 font-mono truncate" title={timelineMeta?.baseCommitHash}>
                  base: {baseLabel}
                  {baseHashShort ? ` @${baseHashShort}` : ""}
                </p>
              </div>
              <ScrollArea className="flex-1">
                <div className="p-2 space-y-0.5">
                  <div
                    className={cn(
                      "flex items-stretch gap-0 rounded-md border border-transparent",
                      target.kind === "worktree" && "border-border bg-accent/30",
                    )}
                  >
                    <button
                      type="button"
                      className="flex flex-1 items-center gap-2 px-2 py-2 text-left text-xs transition-colors hover:bg-accent/40 rounded-l-md"
                      onClick={() => {
                        if (compareSelecting && compareAnchor) {
                          void finalizeCompare({ kind: "worktree" })
                          return
                        }
                        setCompareAnchor(null)
                        setCompareSelecting(false)
                        setTarget({ kind: "worktree" })
                      }}
                    >
                      <Clock className="h-3.5 w-3.5 shrink-0" />
                      <span className="font-medium truncate">当前工作区</span>
                    </button>
                    <DropdownMenu>
                      <DropdownMenuTrigger asChild>
                        <Button variant="ghost" size="icon" className="h-auto w-8 shrink-0 rounded-l-none rounded-r-md" aria-label="工作区操作">
                          <MoreVertical className="h-4 w-4" />
                        </Button>
                      </DropdownMenuTrigger>
                      <DropdownMenuContent align="end" className="w-56">
                        <DropdownMenuItem
                          onClick={() => {
                            setCompareAnchor(null)
                            setCompareSelecting(false)
                            setTarget({ kind: "worktree" })
                          }}
                        >
                          查看此变更
                        </DropdownMenuItem>
                        <DropdownMenuItem
                          onClick={() => {
                            setWorktreeBasis("base")
                            setTarget({ kind: "worktree" })
                          }}
                        >
                          对比 base
                        </DropdownMenuItem>
                        <DropdownMenuItem
                          onClick={() => {
                            setWorktreeBasis("parent")
                            setTarget({ kind: "worktree" })
                          }}
                        >
                          对比上一提交
                        </DropdownMenuItem>
                        <DropdownMenuSeparator />
                        <DropdownMenuItem onClick={() => openMenuCompare({ kind: "worktree" })}>
                          <GitCompare className="h-3.5 w-3.5 mr-2" />
                          对比…
                        </DropdownMenuItem>
                      </DropdownMenuContent>
                    </DropdownMenu>
                  </div>

                  {timelineRows.map((row) => {
                    const active =
                      row.kind === "commit"
                        ? target.kind === "commit" && target.hash === row.hash
                        : target.kind === "snapshot" &&
                          (target.partId === row.patch.partId ||
                            (Boolean(row.patch.id) && Boolean(target.id) && target.id === row.patch.id))

                    if (row.kind === "commit") {
                      return (
                        <div
                          key={row.key}
                          className={cn(
                            "flex items-stretch gap-0 rounded-md border border-transparent",
                            active && "border-border bg-accent/30",
                          )}
                        >
                          <button
                            type="button"
                            className="flex flex-1 items-start gap-2 px-2 py-2 text-left transition-colors hover:bg-accent/40 rounded-l-md"
                            onClick={() => onPickTimelineRow(row)}
                          >
                            <GitCommit className="h-4 w-4 shrink-0 mt-0.5 opacity-80" />
                            <div className="min-w-0 flex-1">
                              <div className="font-mono text-[10px] opacity-70">{row.hash.slice(0, 7)}</div>
                              <div className="text-xs line-clamp-2 leading-snug">{row.subject}</div>
                              <div className="text-[10px] text-muted-foreground mt-0.5">
                                {new Date(row.timestamp * 1000).toLocaleString()}
                              </div>
                            </div>
                          </button>
                          <DropdownMenu>
                            <DropdownMenuTrigger asChild>
                              <Button variant="ghost" size="icon" className="h-auto w-8 shrink-0 rounded-l-none rounded-r-md" aria-label="提交操作">
                                <MoreVertical className="h-4 w-4" />
                              </Button>
                            </DropdownMenuTrigger>
                            <DropdownMenuContent align="end" className="w-56">
                              <DropdownMenuItem
                                onClick={() => {
                                  setTarget({ kind: "commit", hash: row.hash, subject: row.subject })
                                }}
                              >
                                查看此提交
                              </DropdownMenuItem>
                              <DropdownMenuItem
                                onClick={() => {
                                  const base = task?.baseCommitHash?.trim() || timelineMeta?.baseCommitHash
                                  if (!base) {
                                    toast.error("缺少 base 提交")
                                    return
                                  }
                                  void (async () => {
                                    setLoading(true)
                                    try {
                                      const res = await api.getTaskDiff(taskId, { gitFrom: base, gitTo: row.hash, ...diffFetchOpts })
                                      if (res.files?.length) {
                                        setDiffs(
                                          res.files.map((f) => ({
                                            path: f.path,
                                            action: "edit",
                                            unified_diff: f.diff,
                                          })),
                                        )
                                        setSelectedFile(res.files[0].path)
                                      }
                                      setTarget({ kind: "commit", hash: row.hash })
                                    } finally {
                                      setLoading(false)
                                    }
                                  })()
                                }}
                              >
                                对比 base
                              </DropdownMenuItem>
                              <DropdownMenuItem
                                onClick={() => {
                                  void restoreToCommit(row.hash)
                                }}
                              >
                                恢复工作区到此提交
                              </DropdownMenuItem>
                              <DropdownMenuSeparator />
                              <DropdownMenuItem onClick={() => openMenuCompare({ kind: "commit", hash: row.hash, subject: row.subject })}>
                                <GitCompare className="h-3.5 w-3.5 mr-2" />
                                对比…
                              </DropdownMenuItem>
                            </DropdownMenuContent>
                          </DropdownMenu>
                        </div>
                      )
                    }

                    const p = row.patch
                    return (
                      <div
                        key={row.key}
                        className={cn(
                          "flex items-stretch gap-0 rounded-md border border-transparent",
                          active && "border-border bg-accent/30",
                        )}
                      >
                        <button
                          type="button"
                          className="flex flex-1 items-start gap-1.5 px-2 py-1.5 text-left transition-colors hover:bg-accent/40 rounded-l-md min-w-0"
                          onClick={() => onPickTimelineRow(row)}
                        >
                          <CircleDot className="h-3 w-3 shrink-0 mt-1 text-primary opacity-90" />
                          <div className="min-w-0 flex-1">
                            <div className="text-[10px] font-mono opacity-70">
                              {p.hash.slice(0, 7)}→{p.snapshot.slice(0, 7)}
                            </div>
                            {p.description && (
                              <div className="text-[10px] line-clamp-2 text-muted-foreground italic">{p.description}</div>
                            )}
                            <div className="text-[9px] text-muted-foreground">
                              {new Date(normalizeSnapshotTs(p.timestamp) * 1000).toLocaleTimeString()}
                            </div>
                          </div>
                        </button>
                        <DropdownMenu>
                          <DropdownMenuTrigger asChild>
                            <Button variant="ghost" size="icon" className="h-auto w-7 shrink-0 rounded-l-none rounded-r-md px-0" aria-label="检查点操作">
                              <MoreVertical className="h-3.5 w-3.5" />
                            </Button>
                          </DropdownMenuTrigger>
                          <DropdownMenuContent align="end" className="w-56">
                            <DropdownMenuItem
                              onClick={() => {
                                setSnapshotTab("step")
                                setTarget({ kind: "snapshot", partId: p.partId, id: p.id })
                              }}
                            >
                              查看此更改
                            </DropdownMenuItem>
                            <DropdownMenuItem onClick={() => viewPatchVsBase(p)}>对比 base</DropdownMenuItem>
                            <DropdownMenuItem onClick={() => void undoPatch(p)}>撤销此次更改</DropdownMenuItem>
                            <DropdownMenuItem onClick={() => void restorePatchToSnapshot(p)}>恢复代码到此检查点</DropdownMenuItem>
                            <DropdownMenuSeparator />
                            <DropdownMenuItem onClick={() => openMenuCompare({ kind: "snapshot", partId: p.partId, id: p.id })}>
                              <GitCompare className="h-3.5 w-3.5 mr-2" />
                              对比…
                            </DropdownMenuItem>
                          </DropdownMenuContent>
                        </DropdownMenu>
                      </div>
                    )
                  })}
                </div>
              </ScrollArea>
            </div>

            <PanelDividerHandle
              hidden={sidebarCollapsed}
              collapsed={timelineCollapsed}
              onCollapsedChange={setTimelineCollapsed}
              collapsible
              collapseTitleCollapsed="展开变更时间线"
              collapseTitleExpanded="折叠变更时间线"
            />

            {/* 文件树 */}
            <div className={cn(
              "flex shrink-0 flex-col overflow-hidden transition-[width,opacity] duration-200 ease-out",
              sidebarCollapsed ? "w-0 opacity-0" : "opacity-100",
            )}
            style={!sidebarCollapsed ? { width: `${filesPanelWidth}px`, flexBasis: `${filesPanelWidth}px` } : undefined}
            >
              <div className="flex h-full w-full flex-col overflow-hidden">
              <div className="px-4 py-2 border-b bg-muted/50">
                <p className="text-xs font-medium text-muted-foreground">变更文件 ({diffs.length})</p>
              </div>
              <div className="flex-1 min-h-0 overflow-auto scrollbar-thin scrollbar-thumb-muted-foreground/20 scrollbar-track-transparent">
                <div className="min-h-full min-w-max p-2">
                  {loading ? (
                    <div className="flex items-center justify-center py-8">
                      <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
                    </div>
                  ) : diffs.length === 0 ? (
                    <div className="text-center py-8 text-sm text-muted-foreground">没有代码变更</div>
                  ) : (
                    <div className="space-y-0.5">{fileTree.map((node) => renderTreeNode(node, 0))}</div>
                  )}
                </div>
              </div>
              </div>
            </div>

            <PanelDividerHandle
              collapsed={sidebarCollapsed}
              onCollapsedChange={setSidebarCollapsed}
              collapsible
              collapseTitleCollapsed="展开左侧面板"
              collapseTitleExpanded="折叠左侧面板"
              draggable
              size={filesPanelWidth}
              onSizeChange={setFilesPanelWidth}
              minSize={220}
              maxSize={640}
              resizeSign={1}
            />

            {/* Diff */}
            <div ref={diffCodePanelRef} className="relative flex-1 flex flex-col min-w-0 min-h-0">
              <DiffCodeSearchBar
                containerRef={diffCodePanelRef}
                enabled={!loading && Boolean(selectedDiff?.unified_diff)}
                contentKey={`${selectedFile ?? ""}:${viewMode}:${loading ? "loading" : "ready"}`}
              />
              {loading ? (
                <div className="flex-1 flex items-center justify-center">
                  <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
                </div>
              ) : !selectedDiff ? (
                <div className="flex-1 flex items-center justify-center text-sm text-muted-foreground">请选择左侧文件</div>
              ) : !selectedDiff.unified_diff ? (
                <div className="flex-1 flex items-center justify-center text-sm text-muted-foreground">此文件没有 diff</div>
              ) : (
                <div className="flex-1 min-h-0 min-w-0 overflow-auto">
                  <div className="w-full min-w-0 p-4">
                    <UnifiedDiffContent
                      filePath={selectedDiff.path}
                      unifiedDiff={selectedDiff.unified_diff}
                      mode={viewMode}
                      showPathHeader
                    />
                  </div>
                </div>
              )}
            </div>
          </div>

        </DialogContent>
      </Dialog>

      <Dialog open={commitDialogOpen} onOpenChange={setCommitDialogOpen}>
        <DialogContent className="sm:max-w-[500px]">
          <DialogHeader>
            <DialogTitle>提交代码变更</DialogTitle>
            <DialogDescription>请输入提交消息来描述这些变更。</DialogDescription>
          </DialogHeader>

          <div className="space-y-4 py-4">
            <div className="space-y-2">
              <div className="flex items-center justify-between">
                <Label htmlFor="commit-message">提交消息 *</Label>
                <Button type="button" variant="outline" size="sm" onClick={() => void handleGenerateMessage()} disabled={isGenerating || isCommitting} className="h-7">
                  {isGenerating ? (
                    <>
                      <Loader2 className="h-3.5 w-3.5 mr-1.5 animate-spin" />
                      生成中…
                    </>
                  ) : (
                    <>
                      <Bot className="h-3.5 w-3.5 mr-1.5" />
                      AI 生成
                    </>
                  )}
                </Button>
              </div>
              <Textarea
                id="commit-message"
                placeholder="描述此次变更的内容和目的…"
                value={commitMessage}
                onChange={(e) => setCommitMessage(e.target.value)}
                className="min-h-[100px]"
                disabled={isCommitting}
              />
            </div>

            <div className="flex items-center gap-3 text-xs text-muted-foreground px-3 py-2 bg-muted rounded">
              <span className="flex items-center gap-1">
                <FileCode className="h-3 w-3" />
                {stats.totalFiles} 个文件
              </span>
              <span className="text-green-600">+{stats.additions}</span>
              <span className="text-red-600">-{stats.deletions}</span>
            </div>
          </div>

          <DialogFooter>
            <Button variant="outline" onClick={() => setCommitDialogOpen(false)} disabled={isCommitting}>
              取消
            </Button>
            <Button onClick={() => void handleCommit()} disabled={isCommitting || !commitMessage.trim()}>
              {isCommitting ? (
                <>
                  <Loader2 className="h-4 w-4 mr-2 animate-spin" />
                  提交中…
                </>
              ) : (
                <>
                  <GitCommit className="h-4 w-4 mr-2" />
                  提交
                </>
              )}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}
