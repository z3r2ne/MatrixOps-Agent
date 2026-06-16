import React, { useState, useEffect, useMemo, useCallback, useTransition, useRef } from "react"
import { useNavigate, useParams, useSearchParams } from "react-router-dom"
import { 
  ResizableHandle, 
  ResizablePanel, 
  ResizablePanelGroup 
} from "@/components/ui/resizable"
import { Plus, RefreshCw, Loader2, Search, Layers, FolderOpen, FolderPlus, Bot, GitBranch, Terminal, X, ChevronRight, MoreVertical } from "lucide-react"
import { api, Task, TaskExecution, Project, MessageTokens, Worker, BranchInfo, type WorkspaceResponse, type TaskListGroupMode, type TaskMessageQueueItem, type MemoryLibrary, type TaskMemoryLibraryMode } from "@/lib/api"
import { normalizeTaskQueuePayload } from "@/lib/taskQueue"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import { TaskCard, type TaskCardProps } from "@/components/workspace/TaskCard"
import { ChatInterfaceV2 } from "@/components/workspace/ChatInterfaceV2"
import { WorkspaceTerminalPanel } from "@/components/workspace/WorkspaceTerminalPanel"
import { ExecutionLogsDialog } from "@/components/workspace/ExecutionLogsDialog"
import { SessionListDialog } from "@/components/workspace/SessionListDialog"
import { SessionInfoDialog } from "@/components/workspace/SessionInfoDialog"
import { TaskInfoDialog } from "@/components/workspace/TaskInfoDialog"
import { DiffDialog } from "@/components/workspace/DiffDialog"
import { PatchDiffViewer } from "@/components/workspace/PatchDiffViewer"
import { MemoryManagerDialog } from "@/components/workspace/MemoryManagerDialog"
import { MemoryOrganizationDialog } from "@/components/workspace/MemoryOrganizationDialog"
import { TaskSideWorkbench } from "@/components/workspace/TaskSideWorkbench"
import { TestWorkspaceView } from "@/components/workspace/TestWorkspaceView"
import { ScrollArea } from "@/components/ui/scroll-area"
import { toast } from "sonner"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Label } from "@/components/ui/label"
import { Combobox, type ComboboxOption } from "@/components/ui/combobox"
import { Switch } from "@/components/ui/switch"
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu"
import { cn } from "@/lib/utils"
import { getApiErrorMessage } from "@/lib/httpClient"
import { playSystemNotificationSound, sendSystemNotification } from "@/lib/systemNotification"
import { useTaskMessages, useArchivedSessionHistory, type UserMessagePart } from "@/hooks/useGlobalWebSocket"
import { Reorder, useDragControls } from "framer-motion"
import { buildBranchOptions } from "@/lib/branches"
import { getElectronWindowChromeCSSVars } from "@/lib/electron"
import { useElectronWorkbenchChrome } from "@/components/electron/ElectronWorkbenchChrome"
import { useElectronOpenSettings } from "@/components/electron/electron-main-contexts"
import {
  DEFAULT_TASK_LIST_GROUP_MODE,
  TASK_GROUP_MODE_OPTIONS,
  normalizeTaskListGroupMode,
} from "@/lib/taskGrouping"
import {
  TaskRagLibrarySelector,
  buildTaskRagLibraryPayload,
  validateTaskRagLibrarySelection,
} from "@/components/projects/TaskRagLibrarySelector"

const IMPORT_PROJECT_OPTION_VALUE = "__import_project__"
/** 暂时关闭对话页右侧悬浮工作台 */
const TASK_SIDE_WORKBENCH_ENABLED = false

interface TaskGroup {
  key: string
  kind: TaskListGroupMode
  label: string
  tasks: Task[]
  projectId?: number
  workDir?: string
}

function generateCreateTaskBranchName() {
  return `matrixops/${Math.random().toString(36).slice(2, 10)}`
}

function mergeTaskOrder(tasks: Task[], newOrderIds: number[]): Task[] {
  const taskMap = new Map(tasks.map((t) => [t.id, t]))
  if (newOrderIds.some((id) => !taskMap.has(id))) return tasks
  return newOrderIds.map((id) => taskMap.get(id)!)
}

interface SortableTaskRowProps {
  value: number
  onDragStartClear: () => void
  onDragEndPersist: () => void
  children: React.ReactElement<TaskCardProps>
}

function SortableTaskRow({ value, onDragStartClear, onDragEndPersist, children }: SortableTaskRowProps) {
  const dragControls = useDragControls()
  const enhanced = React.isValidElement(children)
    ? React.cloneElement(children, {
        showReorderHandle: true,
        onReorderPointerDown: (e: React.PointerEvent) => {
          e.preventDefault()
          dragControls.start(e)
        },
      } as Partial<TaskCardProps>)
    : children
  return (
    <Reorder.Item
      value={value}
      as="div"
      layout="position"
      dragListener={false}
      dragControls={dragControls}
      className="relative z-0 mb-2 w-full list-none"
      onDragStart={onDragStartClear}
      onDragEnd={() => void onDragEndPersist()}
    >
      {enhanced}
    </Reorder.Item>
  )
}

interface WorkspaceDetailPageProps {
  workspaceId?: number
  selectionStorageKey?: string
  /** 浏览器工作台多 tab 时仅激活页面向标题栏注册聊天工具条 */
  publishWorkbenchTitleToolbar?: boolean
}

function buildWorkerOptions(workers: Worker[]): ComboboxOption[] {
  return workers.map((worker) => ({
    value: worker.name,
    label: worker.name,
    description: worker.description || worker.model || "",
    searchText: `${worker.name} ${worker.description || ""} ${worker.model || ""}`,
  }))
}

export function WorkspaceDetailPage({
  workspaceId: workspaceIdProp,
  selectionStorageKey,
  publishWorkbenchTitleToolbar = true,
}: WorkspaceDetailPageProps = {}) {
  const { id: routeWorkspaceId } = useParams<{ id?: string }>()
  const [searchParams] = useSearchParams()

  const workspaceId = workspaceIdProp ?? (routeWorkspaceId ? Number(routeWorkspaceId) : 0)
  const initialTaskId = Number(searchParams.get("taskId") || "") || null
  const navigate = useNavigate()
  const openElectronSettings = useElectronOpenSettings()
  const effectiveWorkspaceId = workspaceId
  const fallbackSelectionStorageKey = effectiveWorkspaceId
    ? `matrixops.workspace-detail.selected-task.workspace.${effectiveWorkspaceId}`
    : ""
  const effectiveSelectionStorageKey = selectionStorageKey || fallbackSelectionStorageKey

  const [tasks, setTasks] = useState<Task[]>([])
  const [workspace, setWorkspace] = useState<WorkspaceResponse | null>(null)
  const [isLoading, setIsLoading] = useState(true)
  const [selectedTaskId, setSelectedTaskId] = useState<number | null>(null)
  const selectedTask = useMemo(
    () => tasks.find((task) => task.id === selectedTaskId),
    [tasks, selectedTaskId],
  )
  const [taskNavigationStack, setTaskNavigationStack] = useState<number[]>([])
  const [isPending, startTransition] = useTransition()
  const [isCreatingTask, setIsCreatingTask] = useState(false)
  const [createTaskLoading, setCreateTaskLoading] = useState(false)
  const [createTaskSubmitting, setCreateTaskSubmitting] = useState(false)
  const [createTaskProjects, setCreateTaskProjects] = useState<Project[]>([])
  const [createTaskProjectId, setCreateTaskProjectId] = useState("0")
  const [createTaskProjectIsGit, setCreateTaskProjectIsGit] = useState(false)
  const [isCreateTaskProjectDialogOpen, setIsCreateTaskProjectDialogOpen] = useState(false)
  const [createTaskNewProjectName, setCreateTaskNewProjectName] = useState("")
  const [createTaskNewProjectPath, setCreateTaskNewProjectPath] = useState("")
  const [createTaskNewProjectSubmitting, setCreateTaskNewProjectSubmitting] = useState(false)
  const [createTaskWorkers, setCreateTaskWorkers] = useState<Worker[]>([])
  const [createTaskBranches, setCreateTaskBranches] = useState<BranchInfo[]>([])
  const [createTaskWorkerName, setCreateTaskWorkerName] = useState("chat")
  const [createTaskBranch, setCreateTaskBranch] = useState("")
  const [createTaskUseWorktree, setCreateTaskUseWorktree] = useState(false)
  const [createTaskNewBranch, setCreateTaskNewBranch] = useState("")
  const [createTaskInput, setCreateTaskInput] = useState("")
  const [createTaskMemoryLibraries, setCreateTaskMemoryLibraries] = useState<MemoryLibrary[]>([])
  const [createTaskMemoryLibraryMode, setCreateTaskMemoryLibraryMode] = useState<TaskMemoryLibraryMode>("none")
  const [createTaskMemoryLibraryIds, setCreateTaskMemoryLibraryIds] = useState<number[]>([])
  const [terminalSessionIdsByTaskId, setTerminalSessionIdsByTaskId] = useState<Map<number, string>>(new Map())
  const [showTerminalForTaskId, setShowTerminalForTaskId] = useState<number | null>(null)
  const [creatingTerminal, setCreatingTerminal] = useState(false)
  
  // Search and Group
  const [searchQuery, setSearchQuery] = useState("")
  const [groupMode, setGroupMode] = useState<TaskListGroupMode>(DEFAULT_TASK_LIST_GROUP_MODE)
  const [savingGroupMode, setSavingGroupMode] = useState(false)
  const [collapsedProjectGroups, setCollapsedProjectGroups] = useState<Set<string>>(new Set())
  const wbChrome = useElectronWorkbenchChrome()
  const electronWorkbenchLayout = Boolean(wbChrome)
  const showTaskListPanel = !wbChrome || !wbChrome.taskSidebarCollapsed
  const searchPaletteInputRef = useRef<HTMLInputElement>(null)
  
  // Execution Logs Dialog
  const [logsTaskId, setLogsTaskId] = useState<number | null>(null)

  // Session List Dialog
  const [sessionDialogTaskId, setSessionDialogTaskId] = useState<number | null>(null)
  const [selectedExecution, setSelectedExecution] = useState<TaskExecution | null>(null)
  const [toolRejectReason, setToolRejectReason] = useState("")

  // Session Info Dialog
  const [showSessionInfo, setShowSessionInfo] = useState(false)

  // Active Processes
  const [activeProcessTaskIds, setActiveProcessTaskIds] = useState<number[]>([])

  // Task Info Dialog
  const [taskInfoDialogTask, setTaskInfoDialogTask] = useState<Task | null>(null)
  const [renameTask, setRenameTask] = useState<Task | null>(null)
  const [renameName, setRenameName] = useState("")

  // Diff Dialog
  const [diffDialogTaskId, setDiffDialogTaskId] = useState<number | null>(null)
  const [memoryDialogTaskId, setMemoryDialogTaskId] = useState<number | null>(null)
  const [memoryOrganizationDialogTaskId, setMemoryOrganizationDialogTaskId] = useState<number | null>(null)

  const [taskMemoDraft, setTaskMemoDraft] = useState("")
  const [isSavingTaskMemo, setIsSavingTaskMemo] = useState(false)

  const [sessionTitles, setSessionTitles] = useState<Map<number, string>>(new Map())
  const [wechatBindingsByTaskId, setWechatBindingsByTaskId] = useState<Map<number, string>>(new Map())
  const [sessionIds, setSessionIds] = useState<Map<number, string>>(new Map())
  const [sessionTokens, setSessionTokens] = useState<Map<string, MessageTokens>>(new Map())
  const [taskErrors, setTaskErrors] = useState<Map<number, string>>(new Map())
  const [waitUserInputs, setWaitUserInputs] = useState<Map<number, { id: string; question: Record<string, any> }[]>>(new Map())
  const [waitUserAnswers, setWaitUserAnswers] = useState<Record<string, string>>({})
  const taskStatusRef = useRef<Map<number, string>>(new Map())
  const taskNameRef = useRef<Map<number, string>>(new Map())
  const taskParentRef = useRef<Map<number, number | undefined>>(new Map())

  useEffect(() => {
    taskNameRef.current = new Map(tasks.map((task) => [task.id, task.name?.trim() || task.content?.trim() || `任务 #${task.id}`]))
    taskParentRef.current = new Map(tasks.map((task) => [task.id, task.parentTaskId]))
  }, [tasks])

  const readStoredSelectedTaskId = useCallback(() => {
    if (!effectiveSelectionStorageKey || typeof window === "undefined") return null
    const raw = window.localStorage.getItem(effectiveSelectionStorageKey)
    const value = Number(raw || "")
    return Number.isFinite(value) && value > 0 ? value : null
  }, [effectiveSelectionStorageKey])

  useEffect(() => {
    if (!effectiveSelectionStorageKey || typeof window === "undefined" || selectedTaskId === null) {
      return
    }
    window.localStorage.setItem(effectiveSelectionStorageKey, String(selectedTaskId))
  }, [effectiveSelectionStorageKey, selectedTaskId])

  const onTaskStatusWs = useCallback((taskId: number, status: string, sessionId?: string, workDir?: string) => {
    const previousStatus = status ? taskStatusRef.current.get(taskId) : undefined
    if (status) {
      taskStatusRef.current.set(taskId, status)
    }

    setTasks(prev => prev.map(task => {
      if (task.id !== taskId) return task
      const patch: Partial<Task> = {}
      if (status) patch.status = status as Task['status']
      if (sessionId) patch.sessionId = sessionId
      if (workDir) patch.workDir = workDir
      if (Object.keys(patch).length === 0) return task
      return { ...task, ...patch }
    }))

    if (sessionId) {
      setSessionIds(prev => {
        const next = new Map(prev)
        next.set(taskId, sessionId)
        return next
      })
    }

    if (status && status !== "failed") {
      setTaskErrors(prev => {
        if (!prev.has(taskId)) return prev
        const next = new Map(prev)
        next.delete(taskId)
        return next
      })
    }

    const isRootTask = taskParentRef.current.has(taskId) && !taskParentRef.current.get(taskId)

    if (status && isRootTask && (status === "done" || status === "failed" || status === "cancelled") && previousStatus !== status) {
      const taskName = taskNameRef.current.get(taskId) || `任务 #${taskId}`
      const notificationTitle =
        status === "done" ? "任务执行成功" : status === "cancelled" ? "任务已取消" : "任务执行失败"
      const notificationBody =
        status === "done"
          ? `${taskName} 已成功完成`
          : status === "cancelled"
            ? `${taskName} 已被用户取消`
            : `${taskName} 执行失败`
      playSystemNotificationSound(status === "done" ? "success" : "error")
      void sendSystemNotification(
        notificationTitle,
        {
          body: notificationBody,
          tag: `task-finished-${taskId}-${status}`,
          onClick: () => {
            setSelectedTaskId(taskId)
          },
        }
      )
    }
  }, [])

  const onErrorWs = useCallback((taskId: number | undefined, error: string) => {
    console.error('[WorkspaceDetail] 收到错误:', taskId, error)

    // AI 任务执行期错误统一在聊天消息的 assistant error part 中展示，
    // 这里仅保留状态缓存，避免右下角重复 toast 干扰阅读。
    if (taskId) {
      setTaskErrors(prev => {
        const next = new Map(prev)
        next.set(taskId, error)
        return next
      })
    }
  }, [])

  const onSessionTitleWs = useCallback((taskId: number, title: string) => {
    console.log('[WorkspaceDetail] 收到会话标题更新:', taskId, title)

    if (typeof title !== 'string') {
      console.error('[WorkspaceDetail] 会话标题类型错误:', typeof title)
      return
    }

    setSessionTitles(prev => {
      const newMap = new Map(prev)
      newMap.set(taskId, title)
      return newMap
    })

    // 彻底方案：任务列表数据本身携带 sessionTitle，WS 更新也直接写回 task，避免刷新后丢失
    setTasks(prev => prev.map(task => task.id === taskId ? { ...task, sessionTitle: title } : task))
  }, [])

  const handleRetryLastUserMessage = useCallback(async (messageId: string) => {
    if (!selectedTaskId) {
      toast.error("请先选择一个任务")
      return
    }
    try {
      await api.retryLastUserMessage(selectedTaskId, messageId)
      setSelectedExecution(null)
      toast.success("已重新发送这条用户消息")
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "重试消息失败")
    }
  }, [selectedTaskId])

  const onWaitUserInputWs = useCallback((taskId: number, payload: Record<string, any>) => {
    const id = payload?.id
    const question = payload?.question
    if (typeof id !== 'string' || !question || typeof question !== 'object') {
      console.error('[WorkspaceDetail] wait_user_input payload 格式错误:', payload)
      return
    }

    setWaitUserInputs(prev => {
      const next = new Map(prev)
      const queue = next.get(taskId) ?? []
      if (queue.some((entry) => entry.id === id)) {
        return prev
      }
      next.set(taskId, [...queue, { id, question: question as Record<string, any> }])
      return next
    })
  }, [])

  const loadWechatBindings = useCallback(async () => {
    try {
      const accounts = await api.getWechatAccounts()
      const map = new Map<number, string>()
      for (const acc of accounts) {
        if (acc.boundTaskId != null && acc.boundTaskId > 0) {
          map.set(acc.boundTaskId, acc.ilinkUserId || acc.botId)
        }
      }
      setWechatBindingsByTaskId(map)
    } catch {
      setWechatBindingsByTaskId(new Map())
    }
  }, [])

  const loadData = useCallback(async () => {
    setIsLoading(true)
    try {
      if (!effectiveWorkspaceId) {
        setTasks([])
        setWechatBindingsByTaskId(new Map())
        return
      }
      const [tasksData] = await Promise.all([
        api.getWorkspaceTasks(effectiveWorkspaceId),
        loadWechatBindings(),
      ])
      const nextTasks = Array.isArray(tasksData) ? tasksData : []
      setTasks(nextTasks)
      taskStatusRef.current = new Map(nextTasks.map((task) => [task.id, task.status]))
      taskNameRef.current = new Map(nextTasks.map((task) => [task.id, task.name?.trim() || task.content?.trim() || `任务 #${task.id}`]))
      taskParentRef.current = new Map(nextTasks.map((task) => [task.id, task.parentTaskId]))
    } catch (error) {
      console.error('[WorkspaceDetail] ❌ 加载数据失败:', error)
      console.error('[WorkspaceDetail] 错误详情:', JSON.stringify(error, null, 2))
      toast.error("加载数据失败")
    } finally {
      setIsLoading(false)
    }
  }, [effectiveWorkspaceId, loadWechatBindings])

  const loadActiveProcesses = useCallback(async () => {
    try {
      const data = await api.getActiveProcesses()
      setActiveProcessTaskIds(data.taskIds)
    } catch (error) {
      console.error("加载活跃进程失败", error)
    }
  }, [])

  // 全局 WebSocket：单例连接 + 单份 taskStates（回调与订阅合并到同一 hook，避免重复处理每条消息）
  const {
    messagesV2: wsMessagesV2,
    isConnected,
    sendMessage,
    stopTask,
    cancelTaskTool,
    restartTask,
    shouldRetry,
    dismissRetry,
    isWorking,
    reloadHistory,
    loadMoreHistory,
    hasMoreHistory,
    isHistoryLoading,
    messageQueue,
    messageQueueAutoSend,
    taskPlan,
    loadTaskQueueForTask,
    setTaskQueueState,
    waitUserInput,
  } = useTaskMessages(selectedExecution ? null : selectedTaskId, {
    onTaskStatus: onTaskStatusWs,
    onError: onErrorWs,
    onSessionTitle: onSessionTitleWs,
    onWaitUserInput: onWaitUserInputWs,
  })

  const {
    messages: archivedMessagesV2,
    hasMoreHistory: archivedHasMoreHistory,
    isHistoryLoading: archivedHistoryLoading,
    loadMoreHistory: loadMoreArchivedHistory,
  } = useArchivedSessionHistory(selectedExecution?.agentSessionId)

  const handleUpdateTaskQueue = useCallback(async (
    taskId: number,
    queue: TaskMessageQueueItem[],
    options?: { autoSend?: boolean },
  ) => {
    try {
      const data = await api.updateTaskQueue(taskId, queue, options)
      const normalized = normalizeTaskQueuePayload(data)
      setTaskQueueState(taskId, normalized)
    } catch (error) {
      toast.error("更新队列失败")
    }
  }, [setTaskQueueState])

  const handleTaskQueueAutoSendChange = useCallback(async (taskId: number, autoSend: boolean) => {
    try {
      const data = await api.updateTaskQueue(taskId, messageQueue, { autoSend })
      setTaskQueueState(taskId, normalizeTaskQueuePayload(data))
    } catch (error) {
      toast.error("更新队列开关失败")
    }
  }, [messageQueue, setTaskQueueState])

  const handleSendNextQueueItem = useCallback(async (taskId: number, itemId: string) => {
    const isSupplement = isWorking || selectedTask?.status === "running"
    try {
      const data = await api.sendNextTaskQueueItem(taskId, itemId)
      setTaskQueueState(taskId, normalizeTaskQueuePayload({
        queue: data.queue || [],
        autoSend: messageQueueAutoSend,
      }))
      toast.success(
        data.message || (isSupplement ? "已加入本轮补充" : "已发送"),
      )
    } catch (error) {
      toast.error(isSupplement ? "补充失败" : "发送失败")
    }
  }, [isWorking, selectedTask?.status, setTaskQueueState])

  const sendMessageWithQueue = useCallback((
    content: string,
    parts?: UserMessagePart[],
  ) => {
    if (!selectedTaskId) return
    const trimmed = content.trim()
    const hasPayload = trimmed.length > 0 || (parts && parts.length > 0)
    if (!hasPayload) return

    const shouldEnqueue = isWorking || selectedTask?.status === "running"
    if (shouldEnqueue) {
      const optimisticItem: TaskMessageQueueItem = {
        id: `queue-local-${Date.now()}`,
        content: trimmed || "(附件)",
        parts: parts?.map((part) =>
          part.type === "file"
            ? {
                type: "file" as const,
                path: part.path,
                url: part.url,
                mime: part.mime,
                filename: part.filename,
                inputSource: part.inputSource,
              }
            : {
                type: "text" as const,
                text: part.text,
              },
        ),
        createdAt: Date.now(),
      }
      setTaskQueueState(selectedTaskId, {
        queue: [...messageQueue, optimisticItem],
        autoSend: messageQueueAutoSend,
      })
    }
    sendMessage(content, parts)
  }, [
    isWorking,
    messageQueue,
    messageQueueAutoSend,
    selectedTask?.status,
    selectedTaskId,
    sendMessage,
    setTaskQueueState,
  ])

  // 处理选择特定会话（只读查看模式，消息历史走 V2 分页 API）
  const handleSelectSession = useCallback((execution: TaskExecution) => {
    if (execution.taskId !== selectedTaskId) {
      setTaskNavigationStack([])
    }
    setSelectedExecution(execution)
    setSelectedTaskId(execution.taskId)
    if (!execution.agentSessionId?.trim()) {
      toast.message("该执行记录没有可查看的会话消息")
    }
  }, [selectedTaskId])

  // 清除选中的特定会话（返回实时模式）
  const clearSelectedExecution = useCallback(() => {
    setSelectedExecution(null)
  }, [])

  // 清空历史，开始新会话（重新执行任务）
  // 显示会话信息的回调（需要记忆化以避免重新渲染）
  const handleShowSessionInfo = useCallback(() => {
    setShowSessionInfo(true)
  }, [])

  const handleOpenTaskConversation = useCallback(async (taskId: number) => {
    if (!taskId) return
    try {
      let nextTask = tasks.find((task) => task.id === taskId)
      if (!nextTask) {
        const fetched = await api.getTask(taskId)
        const projectName = tasks.find((task) => task.projectId === fetched.projectId)?.projectName
        nextTask = { ...fetched, projectName }
        setTasks((prev) => {
          if (prev.some((task) => task.id === fetched.id)) {
            return prev.map((task) => (task.id === fetched.id ? { ...task, ...nextTask } : task))
          }
          return [...prev, nextTask!]
        })
      }
      if (selectedTaskId && selectedTaskId !== taskId) {
        setTaskNavigationStack((prev) => (
          prev[prev.length - 1] === selectedTaskId ? prev : [...prev, selectedTaskId]
        ))
      }
      setIsCreatingTask(false)
      setSelectedTaskId(taskId)
      clearSelectedExecution()
    } catch (error) {
      console.error("Failed to open subtask conversation:", error)
      toast.error("打开子任务失败", {
        description: error instanceof Error ? error.message : "未知错误",
      })
    }
  }, [clearSelectedExecution, selectedTaskId, tasks])

  const handleReturnToParentConversation = useCallback(() => {
    const targetTaskId = taskNavigationStack[taskNavigationStack.length - 1]
    if (!targetTaskId) return

    setTaskNavigationStack((prev) => prev.slice(0, -1))
    setIsCreatingTask(false)
    setSelectedTaskId(targetTaskId)
    clearSelectedExecution()
  }, [clearSelectedExecution, taskNavigationStack])

  const handleSelectTaskConversation = useCallback((taskId: number) => {
    startTransition(() => {
      setTaskNavigationStack([])
      setIsCreatingTask(false)
      setSelectedTaskId(taskId)
      clearSelectedExecution()
    })
  }, [clearSelectedExecution, startTransition])
  
  const handleClearHistory = useCallback(() => {
    if (selectedTaskId) {
      setSelectedExecution(null)
      restartTask()
    }
  }, [selectedTaskId, restartTask])

  // Handle retry from error (实时错误重试)
  const handleRetryFromError = useCallback(() => {
    if (!selectedTaskId) return
    
    // 清除重试标记
    dismissRetry()
    
    // 重新启动任务
    restartTask()

    toast.success("正在重新执行任务")
  }, [selectedTaskId, dismissRetry, restartTask])

  const waitUserInputQueue = selectedTaskId ? (waitUserInputs.get(selectedTaskId) ?? []) : []
  const activeWaitUserInput = waitUserInputQueue[0]
  const pendingWaitUserInputCount = waitUserInputQueue.length
  const isProjectToolPermissionRequest = activeWaitUserInput?.question?.kind === "project_tool_permission"

  const displayMessagesV2 = useMemo(() => {
    return selectedExecution ? archivedMessagesV2 : wsMessagesV2
  }, [selectedExecution, wsMessagesV2, archivedMessagesV2])

  const displayHasMoreHistory = selectedExecution ? archivedHasMoreHistory : hasMoreHistory
  const displayHistoryLoading = selectedExecution ? archivedHistoryLoading : isHistoryLoading
  const handleLoadMoreHistory = useCallback(() => {
    if (selectedExecution) {
      void loadMoreArchivedHistory()
      return
    }
    void loadMoreHistory()
  }, [selectedExecution, loadMoreArchivedHistory, loadMoreHistory])

  const waitQuestions = useMemo(() => {
    if (activeWaitUserInput?.question?.kind === "project_tool_permission") {
      return []
    }
    if (!activeWaitUserInput) return []
    const questionPayload = activeWaitUserInput.question
    const items = Array.isArray(questionPayload)
      ? questionPayload
      : Array.isArray(questionPayload?.questions)
        ? questionPayload.questions
        : []

    const usedKeys = new Set<string>()
    return items.map((item: any, index: number) => {
      const text = typeof item?.question === 'string' ? item.question.trim() : ""
      let key = text || `question_${index}`
      if (usedKeys.has(key)) {
        key = `question_${index}`
      }
      usedKeys.add(key)
      return {
        key,
        question: text || `问题 ${index + 1}`,
        options: Array.isArray(item?.options) ? item.options : undefined,
        required: Boolean(item?.required)
      }
    })
  }, [activeWaitUserInput])

  useEffect(() => {
    if (!activeWaitUserInput) return
    setWaitUserAnswers({})
    setToolRejectReason("")
  }, [activeWaitUserInput?.id])

  const submitWaitUserInputResult = useCallback((entryId: string, result: Record<string, any>) => {
    if (!selectedTaskId) return
    waitUserInput(selectedTaskId, entryId, result)
    setWaitUserInputs(prev => {
      const next = new Map(prev)
      const queue = next.get(selectedTaskId) ?? []
      const filtered = queue.filter((entry) => entry.id !== entryId)
      if (filtered.length === 0) {
        next.delete(selectedTaskId)
      } else {
        next.set(selectedTaskId, filtered)
      }
      return next
    })
    setWaitUserAnswers({})
  }, [selectedTaskId, waitUserInput])

  const handleSubmitWaitUserInput = useCallback(() => {
    if (!selectedTaskId || !activeWaitUserInput) return
    const missing = waitQuestions.filter(q => q.required && !(waitUserAnswers[q.key] || "").trim())
    if (missing.length > 0) {
      toast.error("请填写所有必填问题")
      return
    }

    const result: Record<string, any> = {}
    waitQuestions.forEach(q => {
      result[q.key] = waitUserAnswers[q.key] || ""
    })

    submitWaitUserInputResult(activeWaitUserInput.id, result)
  }, [selectedTaskId, activeWaitUserInput, waitQuestions, waitUserAnswers, submitWaitUserInputResult])

  const handleCancelWaitUserInput = useCallback(() => {
    if (!selectedTaskId || !activeWaitUserInput) return
    if (activeWaitUserInput.question?.kind === "project_tool_permission") {
      submitWaitUserInputResult(activeWaitUserInput.id, { decision: "reject", reason: toolRejectReason.trim() })
      return
    }
    submitWaitUserInputResult(activeWaitUserInput.id, { refused: true })
  }, [selectedTaskId, activeWaitUserInput, submitWaitUserInputResult, toolRejectReason])

  const projectToolPermissionPayload = useMemo(() => {
    if (!isProjectToolPermissionRequest) {
      return null
    }
    const question = activeWaitUserInput?.question
    return {
      projectName: question?.project?.name || "当前项目",
      workerName: question?.worker?.name || "worker",
      toolLabel: question?.tool?.label || question?.tool?.name || "tool",
      toolName: question?.tool?.name || "tool",
      description: question?.tool?.description || "",
      path: question?.request?.path || "",
      command: question?.request?.command || "",
      contentPreview: question?.request?.contentPreview || "",
      patchPreview: question?.request?.patchPreview || "",
    }
  }, [activeWaitUserInput, isProjectToolPermissionRequest])

  const handleProjectToolPermissionDecision = useCallback((decision: "allow" | "reject") => {
    if (!activeWaitUserInput) return
    submitWaitUserInputResult(
      activeWaitUserInput.id,
      decision === "reject"
        ? { decision, reason: toolRejectReason.trim() }
        : { decision }
    )
  }, [activeWaitUserInput, submitWaitUserInputResult, toolRejectReason])

  const activeProjectId = useMemo(() => {
    if (isCreatingTask && Number(createTaskProjectId) > 0) {
      return Number(createTaskProjectId)
    }
    if (selectedTaskId) {
      return tasks.find(t => t.id === selectedTaskId)?.projectId || 0
    }
    return 0
  }, [createTaskProjectId, isCreatingTask, selectedTaskId, tasks])
  const activeProjectPath = useMemo(() => {
    if (selectedTaskId) {
      const task = tasks.find(t => t.id === selectedTaskId)
      const wd = task?.workDir?.trim()
      if (wd) return wd
    }
    if (isCreatingTask && activeProjectId > 0) {
      return createTaskProjects.find((project) => project.id === activeProjectId)?.path
    }
    return tasks.find(t => t.projectId === activeProjectId)?.workDir
  }, [activeProjectId, createTaskProjects, isCreatingTask, selectedTaskId, tasks])

  const terminalSessionId = useMemo(() => {
    if (!selectedTaskId) return null
    return terminalSessionIdsByTaskId.get(selectedTaskId) ?? null
  }, [selectedTaskId, terminalSessionIdsByTaskId])

  const showTerminal = useMemo(() => {
    return !!selectedTaskId && showTerminalForTaskId === selectedTaskId
  }, [selectedTaskId, showTerminalForTaskId])

  useEffect(() => {
    // 右侧内容与任务卡片绑定：切换任务时自动退出终端视图
    setShowTerminalForTaskId(null)
  }, [selectedTaskId])

  const handleToggleTerminal = useCallback(async () => {
    if (!selectedTaskId) {
      toast.error("请先选择一个任务再打开终端")
      return
    }
    if (showTerminal) {
      setShowTerminalForTaskId(null)
      return
    }
    if (terminalSessionId) {
      setShowTerminalForTaskId(selectedTaskId)
      return
    }
    setCreatingTerminal(true)
    try {
      const session = await api.createTerminalSession({ workDir: activeProjectPath || "" })
      setTerminalSessionIdsByTaskId(prev => {
        const next = new Map(prev)
        next.set(selectedTaskId, session.id)
        return next
      })
      setShowTerminalForTaskId(selectedTaskId)
    } catch (error) {
      console.error("Failed to create terminal session:", error)
      toast.error(error instanceof Error ? error.message : "打开终端失败")
    } finally {
      setCreatingTerminal(false)
    }
  }, [activeProjectPath, selectedTaskId, showTerminal, terminalSessionId])

  const handleCloseTerminal = useCallback(async () => {
    const sessionId = terminalSessionId
    setShowTerminalForTaskId(null)
    if (selectedTaskId) {
      setTerminalSessionIdsByTaskId(prev => {
        if (!prev.has(selectedTaskId)) return prev
        const next = new Map(prev)
        next.delete(selectedTaskId)
        return next
      })
    }
    if (!sessionId) {
      return
    }
    try {
      await api.closeTerminalSession(sessionId)
    } catch (error) {
      console.error("Failed to close terminal session:", error)
    }
  }, [selectedTaskId, terminalSessionId])

  const createTaskProjectOptions = useMemo<ComboboxOption[]>(
    () => [
      {
        value: "0",
        label: "仅工作区",
        description: "不附加项目上下文",
        searchText: "workspace only",
      },
      ...createTaskProjects.map((item) => ({
        value: String(item.id),
        label: item.name,
        description: item.path,
        searchText: `${item.name} ${item.path}`,
      })),
      {
        value: IMPORT_PROJECT_OPTION_VALUE,
        label: "导入项目",
        description: "创建并添加一个新项目",
        searchText: "import create project 导入项目",
      },
    ],
    [createTaskProjects]
  )

  const createTaskWorkerOptions = useMemo(
    () => buildWorkerOptions(createTaskWorkers),
    [createTaskWorkers]
  )

  const createTaskBranchOptions = useMemo(
    () => buildBranchOptions(createTaskBranches),
    [createTaskBranches]
  )

  const ensureCreateTaskWorktreeBranch = useCallback((force = false) => {
    setCreateTaskNewBranch((current) => {
      if (!force && current.trim()) return current
      return generateCreateTaskBranchName()
    })
  }, [])

  const loadCreateTaskBranches = useCallback(async (projectId: number) => {
    if (!projectId) {
      setCreateTaskBranches([])
      setCreateTaskBranch("")
      return
    }
    const branchList = await api.getBranches(projectId)
    setCreateTaskBranches(branchList)
    const currentBranch = branchList.find((branch) => branch.isCurrent)
    if (currentBranch) {
      setCreateTaskBranch(currentBranch.name)
      return
    }
    setCreateTaskBranch(branchList[0]?.name || "")
  }, [])

  const refreshCreateTaskProjectGitState = useCallback(async (projectId: number) => {
    if (!projectId) {
      setCreateTaskProjectIsGit(false)
      setCreateTaskUseWorktree(false)
      setCreateTaskBranches([])
      setCreateTaskBranch("")
      return
    }
    const result = await api.checkGitRepo(projectId)
    setCreateTaskProjectIsGit(Boolean(result.isGitRepo))
    if (!result.isGitRepo) {
      setCreateTaskUseWorktree(false)
      setCreateTaskBranches([])
      setCreateTaskBranch("")
      return
    }
    await loadCreateTaskBranches(projectId)
  }, [loadCreateTaskBranches])

  useEffect(() => {
    if (!isCreatingTask) return

    let cancelled = false
    const loadCreateContext = async () => {
      setCreateTaskLoading(true)
      try {
        const [workersData, projectsData, memoryLibrariesData] = await Promise.all([
          api.getWorkers().catch(() => []),
          effectiveWorkspaceId ? api.getProjects(effectiveWorkspaceId).catch(() => []) : Promise.resolve([]),
          api.getRagLibraries().catch(() => []),
        ])

        if (cancelled) return

        setCreateTaskWorkers(workersData)
        setCreateTaskProjects(projectsData)
        setCreateTaskMemoryLibraries(memoryLibrariesData)
        setCreateTaskMemoryLibraryMode("none")
        setCreateTaskMemoryLibraryIds([])
        const defaultWorkerName =
          workersData.find((worker) => worker.name === "chat")?.name ||
          workersData[0]?.name ||
          "chat"
        setCreateTaskWorkerName(defaultWorkerName)
        const presetProjectId =
          createTaskProjectId &&
          projectsData.some((item) => String(item.id) === createTaskProjectId)
            ? createTaskProjectId
            : ""
        const defaultProjectId = presetProjectId || "0"
        setCreateTaskProjectId(defaultProjectId)
        setCreateTaskUseWorktree(false)
        setCreateTaskNewBranch("")
        await refreshCreateTaskProjectGitState(Number(defaultProjectId))
      } catch (error) {
        console.error("Failed to initialize inline create task:", error)
        toast.error("加载新建任务配置失败")
      } finally {
        if (!cancelled) {
          setCreateTaskLoading(false)
        }
      }
    }

    void loadCreateContext()
    return () => {
      cancelled = true
    }
  }, [createTaskProjectId, effectiveWorkspaceId, isCreatingTask, refreshCreateTaskProjectGitState])

  const handleInlineCreateTask = useCallback(async (messageOverride?: string, partsOverride?: UserMessagePart[]) => {
    const source = typeof messageOverride === "string" ? messageOverride : createTaskInput
    const trimmed = source.trim()
    const inputParts = partsOverride && partsOverride.length > 0 ? partsOverride : undefined
    if (!trimmed && (!inputParts || inputParts.length === 0)) {
      toast.error("请输入任务内容或添加附件")
      return
    }
    if (!effectiveWorkspaceId) {
      toast.error("未找到工作区")
      return
    }
    if (!createTaskWorkerName) {
      toast.error("请选择 Worker")
      return
    }
    if (Number(createTaskProjectId) > 0 && createTaskProjectIsGit && !createTaskBranch) {
      toast.error("请选择分支")
      return
    }
    if (createTaskUseWorktree && !createTaskNewBranch.trim()) {
      toast.error("请输入新分支名")
      return
    }
    const memoryError = validateTaskRagLibrarySelection(createTaskMemoryLibraryMode, createTaskMemoryLibraryIds)
    if (memoryError) {
      toast.error(memoryError)
      return
    }

    setCreateTaskSubmitting(true)
    try {
      const newTask = await api.runTask(effectiveWorkspaceId, {
        name: trimmed || undefined,
        content: trimmed || undefined,
        projectId: Number(createTaskProjectId) > 0 ? Number(createTaskProjectId) : undefined,
        workerName: createTaskWorkerName,
        branch: Number(createTaskProjectId) > 0 && createTaskProjectIsGit && !createTaskUseWorktree ? createTaskBranch : undefined,
        newBranch: Number(createTaskProjectId) > 0 && createTaskProjectIsGit && createTaskUseWorktree ? createTaskNewBranch.trim() : undefined,
        baseBranch: Number(createTaskProjectId) > 0 && createTaskProjectIsGit && createTaskUseWorktree ? createTaskBranch : undefined,
        inputParts,
        ...buildTaskRagLibraryPayload(createTaskMemoryLibraryMode, createTaskMemoryLibraryIds),
      })
      await loadData()
      setTaskNavigationStack([])
      setSelectedTaskId(newTask.id)
      clearSelectedExecution()
      setCreateTaskInput("")
      setIsCreatingTask(false)
      toast.success("任务已创建并开始执行")
    } catch (error) {
      console.error("Failed to create task inline:", error)
      toast.error(getApiErrorMessage(error, "创建任务失败"))
    } finally {
      setCreateTaskSubmitting(false)
    }
  }, [clearSelectedExecution, createTaskBranch, createTaskInput, createTaskMemoryLibraryIds, createTaskMemoryLibraryMode, createTaskNewBranch, createTaskProjectId, createTaskProjectIsGit, createTaskUseWorktree, createTaskWorkerName, effectiveWorkspaceId, loadData])

  const handleCreateTaskProjectChange = useCallback((value: string) => {
    if (value === IMPORT_PROJECT_OPTION_VALUE) {
      setIsCreateTaskProjectDialogOpen(true)
      return
    }
    setCreateTaskProjectId(value)
    setCreateTaskUseWorktree(false)
    setCreateTaskNewBranch("")
    void refreshCreateTaskProjectGitState(Number(value))
  }, [refreshCreateTaskProjectGitState])

  const handleEnableCreateTaskWorktree = useCallback((checked: boolean) => {
    setCreateTaskUseWorktree(checked)
    if (checked) {
      ensureCreateTaskWorktreeBranch(false)
    }
  }, [ensureCreateTaskWorktreeBranch])

  const handleCreateTaskProjectImport = useCallback(async () => {
    if (!effectiveWorkspaceId) {
      toast.error("未找到工作区")
      return
    }
    if (!createTaskNewProjectName.trim() || !createTaskNewProjectPath.trim()) {
      toast.error("请填写项目名称和路径")
      return
    }

    setCreateTaskNewProjectSubmitting(true)
    try {
      const newProject = await api.createProject(effectiveWorkspaceId, {
        name: createTaskNewProjectName.trim(),
        path: createTaskNewProjectPath.trim(),
      })
      setCreateTaskProjects((prev) => [...prev, newProject])
      setCreateTaskProjectId(String(newProject.id))
      await refreshCreateTaskProjectGitState(newProject.id)
      setIsCreateTaskProjectDialogOpen(false)
      setCreateTaskNewProjectName("")
      setCreateTaskNewProjectPath("")
      toast.success("项目已创建并选中")
    } catch (error) {
      console.error("Failed to import project for task:", error)
      toast.error(error instanceof Error ? error.message : "导入项目失败")
    } finally {
      setCreateTaskNewProjectSubmitting(false)
    }
  }, [createTaskNewProjectName, createTaskNewProjectPath, effectiveWorkspaceId, refreshCreateTaskProjectGitState])

  useEffect(() => {
    let cancelled = false

    if (!effectiveWorkspaceId) {
      setGroupMode(DEFAULT_TASK_LIST_GROUP_MODE)
      return
    }

    void api.getWorkspace(effectiveWorkspaceId)
      .then((workspace) => {
        if (cancelled) return
        setGroupMode(normalizeTaskListGroupMode(workspace.groupMode))
        setWorkspace(workspace)
      })
      .catch((error) => {
        console.error("Failed to load workspace group mode:", error)
        if (!cancelled) {
          setGroupMode(DEFAULT_TASK_LIST_GROUP_MODE)
        }
      })

    return () => {
      cancelled = true
    }
  }, [effectiveWorkspaceId])

  useEffect(() => {
    // 工作区模式或项目模式都需要加载数据
    if (effectiveWorkspaceId) {
      loadData()
      loadActiveProcesses()
      
      // 定期刷新活跃进程（每10秒）
      const interval = setInterval(loadActiveProcesses, 10000)
      return () => clearInterval(interval)
    }
  }, [effectiveWorkspaceId, loadData, loadActiveProcesses])

  const rootTasks = useMemo(() => tasks.filter(task => !task.parentTaskId), [tasks])

  // 自动选择第一个任务（首次加载时）
  useEffect(() => {
    if (!isCreatingTask && !isLoading && tasks.length > 0 && selectedTaskId === null) {
      const storedTaskId = readStoredSelectedTaskId()
      const nextTaskId =
        initialTaskId && tasks.some(task => task.id === initialTaskId)
          ? initialTaskId
          : storedTaskId && tasks.some(task => task.id === storedTaskId)
            ? storedTaskId
            : rootTasks[0]?.id ?? tasks[0].id
      setTaskNavigationStack([])
      setSelectedTaskId(nextTaskId)
    }
  }, [initialTaskId, isCreatingTask, rootTasks, tasks, isLoading, readStoredSelectedTaskId, selectedTaskId])

  useEffect(() => {
    if (selectedTaskId === null) {
      return
    }

    const taskExists = tasks.some(task => task.id === selectedTaskId)
    if (taskExists) {
      return
    }

    setTaskNavigationStack([])
    const storedTaskId = readStoredSelectedTaskId()
    const nextTaskId =
      storedTaskId && tasks.some(task => task.id === storedTaskId)
        ? storedTaskId
        : rootTasks.length > 0
          ? rootTasks[0].id
          : tasks.length > 0
            ? tasks[0].id
          : null
    setSelectedTaskId(nextTaskId)
    clearSelectedExecution()
  }, [rootTasks, tasks, selectedTaskId, clearSelectedExecution, readStoredSelectedTaskId])

  // Filter and group tasks
  const filteredTasks = useMemo(() => {
    if (!searchQuery.trim()) return rootTasks
    const query = searchQuery.toLowerCase()
    return rootTasks.filter(task => 
      task.name?.toLowerCase().includes(query) ||
      task.content.toLowerCase().includes(query) ||
      task.workerName?.toLowerCase().includes(query)
    )
  }, [rootTasks, searchQuery])

  const groupedTasks = useMemo((): TaskGroup[] => {
    if (groupMode === "none") {
      return [{ key: "all", kind: "none", label: "", tasks: filteredTasks }]
    }

    if (groupMode === "project") {
      const groups = new Map<number, TaskGroup>()
      filteredTasks.forEach((task) => {
        const projectId = task.projectId || 0
        if (!groups.has(projectId)) {
          groups.set(projectId, {
            key: `project:${projectId}`,
            kind: "project",
            label: task.projectName || `项目 #${projectId || "未知"}`,
            tasks: [],
            projectId,
            workDir: task.workDir,
          })
        }
        const group = groups.get(projectId)!
        group.tasks.push(task)
        if (!group.workDir && task.workDir) {
          group.workDir = task.workDir
        }
      })
      return Array.from(groups.values())
    }
    
    const groups: Record<string, Task[]> = {}
    filteredTasks.forEach(task => {
      const date = new Date(task.createdAt).toLocaleDateString("zh-CN", {
        year: "numeric",
        month: "long",
        day: "numeric"
      })
      if (!groups[date]) groups[date] = []
      groups[date].push(task)
    })
    
    return Object.entries(groups)
      .sort((a, b) => {
        const dateA = new Date(filteredTasks.find(t => 
          new Date(t.createdAt).toLocaleDateString("zh-CN", { year: "numeric", month: "long", day: "numeric" }) === a[0]
        )?.createdAt || 0)
        const dateB = new Date(filteredTasks.find(t => 
          new Date(t.createdAt).toLocaleDateString("zh-CN", { year: "numeric", month: "long", day: "numeric" }) === b[0]
        )?.createdAt || 0)
        return dateB.getTime() - dateA.getTime()
      })
      .map(([label, tasks]) => ({ key: `date:${label}`, kind: "date" as const, label, tasks }))
  }, [filteredTasks, groupMode])

  const handleGroupModeChange = useCallback(async (value: string) => {
    const nextMode = normalizeTaskListGroupMode(value)
    if (nextMode === groupMode) {
      return
    }

    const previousMode = groupMode
    setGroupMode(nextMode)

    if (!effectiveWorkspaceId) {
      return
    }

    setSavingGroupMode(true)
    try {
      await api.updateWorkspace(effectiveWorkspaceId, { groupMode: nextMode })
    } catch (error) {
      setGroupMode(previousMode)
      toast.error(error instanceof Error ? error.message : "保存任务分组方式失败")
    } finally {
      setSavingGroupMode(false)
    }
  }, [effectiveWorkspaceId, groupMode])

  const toggleProjectGroupCollapsed = useCallback((groupKey: string) => {
    setCollapsedProjectGroups((prev) => {
      const next = new Set(prev)
      if (next.has(groupKey)) {
        next.delete(groupKey)
      } else {
        next.add(groupKey)
      }
      return next
    })
  }, [])

  const taskReorderEnabled = groupMode === "none" && !searchQuery.trim()

  useEffect(() => {
    if (!wbChrome?.searchPaletteOpen) {
      return
    }
    const id = window.requestAnimationFrame(() => {
      searchPaletteInputRef.current?.focus()
    })
    return () => window.cancelAnimationFrame(id)
  }, [wbChrome?.searchPaletteOpen])

  const pendingFramerReorderRef = useRef<number[]>([])

  const clearPendingReorderOnDragStart = useCallback(() => {
    pendingFramerReorderRef.current = []
  }, [])

  const handleFramerSegmentReorder = useCallback((newOrderIds: number[]) => {
    pendingFramerReorderRef.current = newOrderIds
    setTasks((prev) => mergeTaskOrder(prev, newOrderIds))
  }, [])

  const handleFramerDragEndPersist = useCallback(
    async () => {
      const ids = pendingFramerReorderRef.current
      pendingFramerReorderRef.current = []
      if (!ids?.length) return
      if (!effectiveWorkspaceId) return
      try {
        await api.reorderTasks(effectiveWorkspaceId, ids)
      } catch (err: unknown) {
        const msg =
          err && typeof err === "object" && "message" in err
            ? String((err as { message: string }).message)
            : "排序保存失败"
        toast.error(msg)
        await loadData()
      }
    },
    [effectiveWorkspaceId, loadData]
  )

  const openCreateDialog = useCallback(() => {
    setCreateTaskInput("")
    setTaskNavigationStack([])
    setSelectedTaskId(null)
    setIsCreatingTask(true)
    setSelectedExecution(null)
  }, [])

  const openCreateDialogForProject = useCallback((projectId: number) => {
    setCreateTaskInput("")
    setCreateTaskUseWorktree(false)
    setCreateTaskNewBranch("")
    setTaskNavigationStack([])
    setSelectedTaskId(null)
    setCreateTaskProjectId(String(projectId))
    setIsCreatingTask(true)
    setSelectedExecution(null)
  }, [])

  const handleRestartTask = async (taskId: number) => {
    try {
      await api.restartTask(taskId)
      toast.success("任务已重新启动")
      loadData()
      setTaskNavigationStack([])
      setSelectedTaskId(taskId) // Switch to this task to see logs
    } catch (error) {
      toast.error("重启失败")
    }
  }

  const handleUpdateStatus = async (task: Task, status: Task["status"]) => {
    try {
      await api.updateTask(task.id, { status })
      setTasks(prev => prev.map(t => t.id === task.id ? { ...t, status } : t))
      toast.success("状态已更新")
    } catch (error) {
      toast.error("更新失败")
    }
  }

  const handleDeleteTask = async (taskId: number) => {
    try {
      await api.deleteTask(taskId)
      setTasks(prev => prev.filter(t => t.id !== taskId))
      setTaskNavigationStack(prev => prev.filter(id => id !== taskId))
      if (selectedTaskId === taskId) setSelectedTaskId(null)
      toast.success("任务已删除")
    } catch (error) {
      toast.error("删除失败")
    }
  }

  const openRenameDialog = (task: Task) => {
    setRenameTask(task)
    setRenameName(task.name || "")
  }

  const handleRenameTask = async () => {
    if (!renameTask) return
    try {
      await api.updateTask(renameTask.id, { name: renameName.trim() || undefined })
      setTasks(prev => prev.map(t => t.id === renameTask.id ? { ...t, name: renameName.trim() || undefined } : t))
      if (selectedTaskId === renameTask.id) {
        setSelectedTaskId(renameTask.id) // trigger re-render title
      }
      toast.success("任务名称已更新")
      setRenameTask(null)
      setRenameName("")
    } catch (error: any) {
      toast.error(error.message || "重命名失败")
    }
  }

  const handleOpenInEditor = useCallback(async (task: Task) => {
    try {
      await api.openProject({ path: task.workDir })
      toast.success("已尝试在编辑器中打开")
    } catch (error: any) {
      toast.error(error.message || "打开编辑器失败")
    }
  }, [])

  const handleOpenProjectFolder = useCallback(async (projectId: number, workDir?: string) => {
    try {
      let path = workDir?.trim() || ""
      if (!path) {
        const projectData = await api.getProject(projectId)
        path = projectData.path || ""
      }
      if (!path) {
        toast.error("未找到项目路径")
        return
      }
      await api.openInFileManager({ path })
      toast.success("已尝试打开项目文件夹")
    } catch (error: any) {
      toast.error(error.message || "打开项目文件夹失败")
    }
  }, [])

  useEffect(() => {
    setTaskMemoDraft(selectedTask?.memo || "")
    setIsSavingTaskMemo(false)
  }, [selectedTask?.id, selectedTask?.memo])

  useEffect(() => {
    if (!selectedTaskId || !selectedTask) return
    const nextMemo = taskMemoDraft
    const currentMemo = selectedTask.memo || ""
    if (nextMemo === currentMemo) {
      setIsSavingTaskMemo(false)
      return
    }

    setIsSavingTaskMemo(true)
    const timer = window.setTimeout(async () => {
      try {
        const updatedTask = await api.updateTask(selectedTaskId, { memo: nextMemo })
        setTasks((prev) => prev.map((task) => (task.id === selectedTaskId ? { ...task, ...updatedTask } : task)))
      } catch (error) {
        console.error("Failed to save task memo:", error)
        toast.error("保存备忘录失败", {
          description: error instanceof Error ? error.message : "未知错误",
        })
      } finally {
        setIsSavingTaskMemo(false)
      }
    }, 3000)

    return () => {
      window.clearTimeout(timer)
    }
  }, [selectedTask, selectedTaskId, taskMemoDraft])

  const currentSessionId = useMemo(() => {
    if (selectedExecution?.agentSessionId) {
      return selectedExecution.agentSessionId
    }
    if (selectedTask?.sessionId) {
      return selectedTask.sessionId
    }
    if (selectedTaskId) {
      return sessionIds.get(selectedTaskId)
    }
    return undefined
  }, [selectedExecution?.agentSessionId, selectedTask?.sessionId, selectedTaskId, sessionIds])

  useEffect(() => {
    if (!currentSessionId) return
    if (sessionTokens.has(currentSessionId)) return

    let cancelled = false
    api.getSessionInfo(currentSessionId)
      .then((session) => {
        if (cancelled || !session.tokens) return
        setSessionTokens(prev => {
          const next = new Map(prev)
          next.set(currentSessionId, session.tokens!)
          return next
        })
      })
      .catch((error) => {
        if (!cancelled) {
          console.error("Failed to load session info:", error)
        }
      })

    return () => {
      cancelled = true
    }
  }, [currentSessionId, sessionTokens])

  useEffect(() => {
    if (!initialTaskId) return
    if (!tasks.some((task) => task.id === initialTaskId)) return
    setTaskNavigationStack([])
    setSelectedTaskId(initialTaskId)
    clearSelectedExecution()
  }, [clearSelectedExecution, initialTaskId, tasks])

  const sessionTitle = selectedTaskId ? sessionTitles.get(selectedTaskId) : undefined

  const chatTitle = selectedExecution
    ? `会话 #${selectedExecution.id} (任务 #${selectedTaskId})`
    : selectedTask?.parentTaskId
      ? `子任务 #${selectedTask.id} · ${selectedTask.workerName || "worker"}`
    : sessionTitle
      ? `${sessionTitle.length > 50 ? sessionTitle.slice(0, 50) + '...' : sessionTitle} #${selectedTaskId}`
      : selectedTask
        ? `${(selectedTask.name || selectedTask.content).length > 50
              ? (selectedTask.name || selectedTask.content).slice(0, 50) + '...'
              : (selectedTask.name || selectedTask.content)} #${selectedTaskId}`
        : selectedTaskId
          ? `任务 #${selectedTaskId}`
          : "AI 助手"

  const handleSessionTransferImported = useCallback(async () => {
    if (!selectedTaskId) return

    await reloadHistory()

    if (!currentSessionId) return

    try {
      const session = await api.getSessionInfo(currentSessionId)

      if (session.tokens) {
        setSessionTokens(prev => {
          const next = new Map(prev)
          next.set(currentSessionId, session.tokens!)
          return next
        })
      }

      if (session.title) {
        setSessionTitles(prev => {
          const next = new Map(prev)
          next.set(selectedTaskId, session.title)
          return next
        })
      }
    } catch (error) {
      console.error("Failed to refresh session after import:", error)
    }
  }, [currentSessionId, reloadHistory, selectedTaskId])

  return (
    <>
      {workspace?.type === "test" ? (
        <TestWorkspaceView workspace={workspace} />
      ) : (
        <div className="h-full flex flex-col bg-background">
      <ResizablePanelGroup
        key={showTaskListPanel ? "workspace-layout-both" : "workspace-layout-chat"}
        orientation="horizontal"
        autoSaveId={showTaskListPanel ? "workspace-layout-v3" : "workspace-layout-v3-fullchat"}
      >
        {showTaskListPanel ? (
          <>
            <ResizablePanel defaultSize={24} minSize={4} className="border-r min-w-0">
          <div className="flex h-full flex-col">
            {/* Header with Search & Group */}
            <div className="border-b px-2 py-2">
              {!electronWorkbenchLayout ? (
                <div className="relative min-w-0">
                  <Search className="absolute left-2 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-muted-foreground" />
                  <Input
                    value={searchQuery}
                    onChange={e => setSearchQuery(e.target.value)}
                    placeholder="搜索任务..."
                    className="h-7 pl-7 text-xs"
                  />
                </div>
              ) : null}
              <div className={cn("flex items-center gap-1.5", !electronWorkbenchLayout && "mt-2")}>
                <div className="relative min-w-0 flex-1">
                  <Layers className="pointer-events-none absolute left-2 top-1/2 h-3 w-3 -translate-y-1/2 text-muted-foreground" />
                  <Combobox
                    id="workspace-group-mode"
                    items={TASK_GROUP_MODE_OPTIONS}
                    value={groupMode}
                    onValueChange={handleGroupModeChange}
                    placeholder="选择分组方式"
                    searchPlaceholder="搜索分组方式"
                    emptyText="未找到分组选项"
                    inputClassName="h-7 pl-7 pr-2 text-xs"
                    disabled={savingGroupMode}
                  />
                </div>
                <Button variant="ghost" size="icon" className="h-7 w-7 shrink-0" onClick={loadData} disabled={isLoading}>
                  <RefreshCw className={cn("h-3.5 w-3.5", isLoading && "animate-spin")} />
                </Button>
                <Button size="icon" className="h-7 w-7 shrink-0" onClick={openCreateDialog}>
                  <Plus className="h-3.5 w-3.5" />
                </Button>
              </div>
            </div>
            
            {/* Task List */}
            <ScrollArea className="flex-1">
              <div className="p-2">
                {filteredTasks.length === 0 && !isLoading && (
                  <div className="text-center text-xs text-muted-foreground py-10">
                    {searchQuery ? "没有匹配的任务" : "暂无任务"}
                  </div>
                )}
                {groupedTasks.map((group, groupIndex) => {
                  const isProjectGroup = group.kind === "project"
                  const isCollapsed = isProjectGroup && collapsedProjectGroups.has(group.key)

                  return (
                  <div key={group.key || groupIndex} className={cn(isProjectGroup && "group/project")}>
                    {group.kind === "date" && group.label ? (
                      <div className="sticky top-0 mb-2 border-b bg-background py-2 text-xs font-medium text-muted-foreground">
                        {group.label}
                      </div>
                    ) : null}

                    {isProjectGroup ? (
                      <div className="mb-2 flex items-center justify-between border-b py-2">
                        <button
                          type="button"
                          onClick={() => toggleProjectGroupCollapsed(group.key)}
                          className="flex min-w-0 flex-1 items-center gap-2 text-left"
                        >
                          <ChevronRight
                            className={cn(
                              "h-3.5 w-3.5 shrink-0 text-muted-foreground transition-transform",
                              !isCollapsed && "rotate-90"
                            )}
                          />
                          <span className="truncate text-xs font-medium text-foreground">{group.label}</span>
                          <span className="shrink-0 text-[11px] text-muted-foreground">{group.tasks.length}</span>
                        </button>
                        <DropdownMenu>
                          <DropdownMenuTrigger asChild>
                            <Button
                              type="button"
                              variant="ghost"
                              size="icon"
                              className="h-6 w-6 shrink-0 opacity-0 transition-opacity group-hover/project:opacity-100 focus-visible:opacity-100"
                              onClick={(event) => event.stopPropagation()}
                            >
                              <MoreVertical className="h-3.5 w-3.5" />
                            </Button>
                          </DropdownMenuTrigger>
                          <DropdownMenuContent align="end">
                            <DropdownMenuItem
                              onClick={() => group.projectId && openCreateDialogForProject(group.projectId)}
                              disabled={!group.projectId}
                            >
                              <Plus className="mr-2 h-4 w-4" />
                              创建任务
                            </DropdownMenuItem>
                            <DropdownMenuItem
                              onClick={() => group.projectId && handleOpenProjectFolder(group.projectId, group.workDir)}
                              disabled={!group.projectId}
                            >
                              <FolderOpen className="mr-2 h-4 w-4" />
                              打开文件夹
                            </DropdownMenuItem>
                          </DropdownMenuContent>
                        </DropdownMenu>
                      </div>
                    ) : null}

                    {!isCollapsed && (taskReorderEnabled
                      ? (
                          <Reorder.Group
                            key={`reorder-group-${group.label || groupIndex}`}
                            axis="y"
                            as="div"
                            values={group.tasks.map((t) => t.id)}
                            onReorder={(newOrder) => handleFramerSegmentReorder(newOrder)}
                            className="mb-0"
                          >
                            {group.tasks.map((task) => (
                              <SortableTaskRow
                                key={task.id}
                                value={task.id}
                                onDragStartClear={clearPendingReorderOnDragStart}
                                onDragEndPersist={handleFramerDragEndPersist}
                              >
                                <TaskCard
                                  task={task}
                                  isSelected={selectedTaskId === task.id}
                                  sessionTitle={task.sessionTitle || sessionTitles.get(task.id)}
                                  wechatBindingLabel={wechatBindingsByTaskId.get(task.id)}
                                  showProjectName={!isProjectGroup}
                                  onClick={() => handleSelectTaskConversation(task.id)}
                                  onDelete={(e) => handleDeleteTask(task.id)}
                                  onStatusChange={(status) => handleUpdateStatus(task, status)}
                                  onRestart={() => handleRestartTask(task.id)}
                                  onViewInfo={() => setTaskInfoDialogTask(task)}
                                  onViewAnalytics={() => {
                                    if (openElectronSettings) {
                                      openElectronSettings({ panel: "usage", usageTaskId: task.id })
                                      return
                                    }
                                    navigate(`/usage?filter=task&taskId=${task.id}`)
                                  }}
                                  onViewLogs={() => setLogsTaskId(task.id)}
                                  onViewSessions={() => setSessionDialogTaskId(task.id)}
                                  onViewDiff={() => setDiffDialogTaskId(task.id)}
                                  onManageMemory={() => setMemoryDialogTaskId(task.id)}
                                  onOrganizeMemory={() => setMemoryOrganizationDialogTaskId(task.id)}
                                  onManageProjectPrompt={() => openElectronSettings?.({ panel: "prompts", initialProjectId: task.projectId })}
                                  onOpenInEditor={() => handleOpenInEditor(task)}
                                  onRename={() => openRenameDialog(task)}
                                  hasResidentProcess={activeProcessTaskIds.includes(task.id)}
                                />
                              </SortableTaskRow>
                            ))}
                          </Reorder.Group>
                        )
                      : group.tasks.map((task) => (
                          <div key={task.id}>
                            <TaskCard
                              task={task}
                              isSelected={selectedTaskId === task.id}
                              sessionTitle={task.sessionTitle || sessionTitles.get(task.id)}
                              wechatBindingLabel={wechatBindingsByTaskId.get(task.id)}
                              showProjectName={!isProjectGroup}
                              onClick={() => handleSelectTaskConversation(task.id)}
                              onDelete={(e) => handleDeleteTask(task.id)}
                              onStatusChange={(status) => handleUpdateStatus(task, status)}
                              onRestart={() => handleRestartTask(task.id)}
                              onViewInfo={() => setTaskInfoDialogTask(task)}
                              onViewAnalytics={() => {
                                if (openElectronSettings) {
                                  openElectronSettings({ panel: "usage", usageTaskId: task.id })
                                  return
                                }
                                navigate(`/usage?filter=task&taskId=${task.id}`)
                              }}
                              onViewLogs={() => setLogsTaskId(task.id)}
                              onViewSessions={() => setSessionDialogTaskId(task.id)}
                              onViewDiff={() => setDiffDialogTaskId(task.id)}
                              onManageMemory={() => setMemoryDialogTaskId(task.id)}
                              onOrganizeMemory={() => setMemoryOrganizationDialogTaskId(task.id)}
                              onManageProjectPrompt={() => openElectronSettings?.({ panel: "prompts", initialProjectId: task.projectId })}
                              onOpenInEditor={() => handleOpenInEditor(task)}
                              onRename={() => openRenameDialog(task)}
                              hasResidentProcess={activeProcessTaskIds.includes(task.id)}
                            />
                          </div>
                        )))}
                  </div>
                )})}
              </div>
            </ScrollArea>
          </div>
        </ResizablePanel>

        <ResizableHandle />
          </>
        ) : null}

        {/* RIGHT PANEL: Chat / Logs */}
        <ResizablePanel defaultSize={showTaskListPanel ? 76 : 100} minSize={20} className="min-w-0">
          {isCreatingTask ? (
            <ChatInterfaceV2
              messages={[]}
              taskId={undefined}
              sessionId={undefined}
              projectId={activeProjectId || undefined}
              projectPath={activeProjectPath}
              isConnected={false}
              centeredComposer
              centeredTitle="有什么可以帮忙的？"
              chatChrome="workbench"
              publishWorkbenchTitleToolbar={publishWorkbenchTitleToolbar}
              onSendMessage={(message, parts) => {
                void handleInlineCreateTask(message, parts)
              }}
              isRunning={createTaskSubmitting}
              isWorking={createTaskSubmitting}
              placeholder="输入任务内容..."
              readOnly={false}
              composerExtra={
                <div className="space-y-3 min-h-[320px]">
                  <div className="grid gap-3 md:grid-cols-3 auto-rows-max">
                    <div className="relative md:col-span-1">
                      <FolderOpen className="pointer-events-none absolute left-3 top-1/2 z-10 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                      <Combobox
                        items={createTaskProjectOptions}
                        value={createTaskProjectId}
                        onValueChange={handleCreateTaskProjectChange}
                        placeholder="项目上下文"
                        searchPlaceholder="搜索项目"
                        emptyText="未找到项目"
                        disabled={createTaskLoading || createTaskSubmitting}
                        inputClassName="pl-9"
                        renderItem={(item, selected) => {
                          if (item.value === IMPORT_PROJECT_OPTION_VALUE) {
                            return (
                              <div className="flex items-center gap-2 text-primary">
                                <FolderPlus className="h-4 w-4" />
                                <div className="min-w-0 flex-1">
                                  <div className="truncate text-sm font-medium">{item.label}</div>
                                  {item.description ? (
                                    <div className="truncate text-xs text-muted-foreground">{item.description}</div>
                                  ) : null}
                                </div>
                              </div>
                            )
                          }
                          return (
                            <div className="min-w-0 flex-1">
                              <div className="truncate text-sm">{item.label}</div>
                              {item.description ? (
                                <div className="truncate text-xs text-muted-foreground">{item.description}</div>
                              ) : null}
                            </div>
                          )
                        }}
                      />
                    </div>

                    <div className="relative">
                      <Bot className="pointer-events-none absolute left-3 top-1/2 z-10 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                      <Combobox
                        items={createTaskWorkerOptions}
                        value={createTaskWorkerName}
                        onValueChange={setCreateTaskWorkerName}
                        placeholder="Worker"
                        searchPlaceholder="搜索 Worker"
                        emptyText="未找到 Worker"
                        disabled={createTaskLoading || createTaskSubmitting || createTaskWorkerOptions.length === 0}
                        inputClassName="pl-9"
                      />
                    </div>

                    <div className="relative">
                      <GitBranch className="pointer-events-none absolute left-3 top-1/2 z-10 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                      <Combobox
                        items={createTaskBranchOptions}
                        value={createTaskBranch}
                        onValueChange={setCreateTaskBranch}
                        placeholder={
                          Number(createTaskProjectId) === 0
                            ? "先选择项目"
                            : createTaskProjectIsGit
                              ? (createTaskUseWorktree ? "基准分支" : "分支")
                              : "当前项目不是 Git 项目"
                        }
                        searchPlaceholder="搜索分支"
                        emptyText={createTaskProjectIsGit ? "未找到分支" : "当前项目不是 Git 项目"}
                        disabled={createTaskLoading || createTaskSubmitting || !createTaskProjectIsGit || createTaskBranchOptions.length === 0}
                        inputClassName="pl-9"
                      />
                    </div>

                    <TaskRagLibrarySelector
                      libraries={createTaskMemoryLibraries}
                      mode={createTaskMemoryLibraryMode}
                      selectedIds={createTaskMemoryLibraryIds}
                      onModeChange={setCreateTaskMemoryLibraryMode}
                      onSelectedIdsChange={setCreateTaskMemoryLibraryIds}
                      disabled={createTaskLoading || createTaskSubmitting}
                    />

                    <div className="space-y-4 md:col-span-3">
                      <div className="min-h-[72px]">
                        <div className="grid gap-3 md:grid-cols-[180px_minmax(0,1fr)] md:items-center">
                          <div className="flex h-10 items-center justify-between rounded-md border border-border/60 bg-muted/30 px-3">
                            <div className="flex items-center gap-2 text-muted-foreground">
                              <GitBranch className="h-4 w-4" />
                              <span className="text-xs">worktree</span>
                            </div>
                            <Switch
                              checked={createTaskUseWorktree}
                              onCheckedChange={handleEnableCreateTaskWorktree}
                              disabled={createTaskLoading || createTaskSubmitting || !createTaskProjectIsGit}
                            />
                          </div>

                          <div className="relative">
                            <GitBranch className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                            <Input
                              value={createTaskNewBranch}
                              onChange={(e) => setCreateTaskNewBranch(e.target.value)}
                              placeholder={createTaskProjectIsGit ? "新分支，例如：feature/my-task" : "当前项目不是 Git 项目"}
                              disabled={createTaskLoading || createTaskSubmitting || !createTaskProjectIsGit || !createTaskUseWorktree}
                              className={createTaskUseWorktree && createTaskProjectIsGit ? "pl-9 opacity-100" : "pl-9 opacity-40"}
                            />
                          </div>
                        </div>
                      </div>
                    </div>
                  </div>
                </div>
              }
            />
          ) : (
          <div className="relative h-full min-h-0">
            <ChatInterfaceV2 
              messages={displayMessagesV2}
              taskId={selectedTaskId || undefined}
              sessionId={currentSessionId}
              projectId={activeProjectId || undefined}
              projectPath={activeProjectPath}
              chatChrome="workbench"
              publishWorkbenchTitleToolbar={publishWorkbenchTitleToolbar}
              isConnected={selectedExecution ? false : isConnected}
              onSendMessage={selectedExecution ? undefined : sendMessageWithQueue}
              onStop={selectedExecution ? undefined : stopTask}
              onCancelTool={selectedExecution ? undefined : cancelTaskTool}
              onOpenTask={handleOpenTaskConversation}
              onBack={taskNavigationStack.length > 0 ? handleReturnToParentConversation : undefined}
              onRestart={selectedExecution ? undefined : restartTask}
              onOrganizeMemory={selectedExecution || !selectedTaskId ? undefined : () => setMemoryOrganizationDialogTaskId(selectedTaskId)}
              onClearHistory={handleClearHistory}
              isRunning={selectedExecution ? false : selectedTask?.status === "running" || isWorking}
              isWorking={isWorking}
              title={chatTitle}
              backLabel={taskNavigationStack.length > 1 ? "返回上一级" : "返回主 Worker"}
              placeholder={selectedExecution
                ? "此会话无法继续对话"
                : selectedTask?.status === "running" || isWorking
                  ? "执行中发送将加入待发送队列（Enter 发送）…"
                  : "输入指令..."
              }
              readOnly={!!selectedExecution}
              taskStatus={selectedTask?.status}
              errorMessage={selectedTaskId ? taskErrors.get(selectedTaskId) : undefined}
              shouldRetry={shouldRetry}
              onRetryFromError={handleRetryFromError}
              onDismissRetry={dismissRetry}
              hasMoreHistory={displayHasMoreHistory}
              isHistoryLoading={displayHistoryLoading}
              onLoadMoreHistory={handleLoadMoreHistory}
              sessionTokens={currentSessionId ? (sessionTokens.get(currentSessionId) || null) : null}
              onSessionTransferImported={handleSessionTransferImported}
              onRetryUserMessage={selectedExecution ? undefined : handleRetryLastUserMessage}
              composerWorkerLabel={
                selectedTaskId && selectedTask
                  ? selectedTask.workerName?.trim() || "chat"
                  : undefined
              }
              taskPlan={taskPlan}
              messageQueue={messageQueue}
              messageQueueAutoSend={messageQueueAutoSend}
              onMessageQueueAutoSendChange={
                selectedTaskId
                  ? (autoSend) => void handleTaskQueueAutoSendChange(selectedTaskId, autoSend)
                  : undefined
              }
              onMessageQueueChange={selectedTaskId ? (queue) => void handleUpdateTaskQueue(selectedTaskId, queue) : undefined}
              onSendNextQueueItem={selectedTaskId ? (itemId) => void handleSendNextQueueItem(selectedTaskId, itemId) : undefined}
              headerExtra={
                <>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="h-7 w-7"
                    onClick={() => {
                      void handleToggleTerminal()
                    }}
                    disabled={creatingTerminal || !activeProjectPath}
                  >
                    {creatingTerminal ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Terminal className="h-3.5 w-3.5" />}
                  </Button>
                  {terminalSessionId ? (
                    <Button
                      variant="ghost"
                      size="icon"
                      className="h-7 w-7 text-muted-foreground hover:text-destructive"
                      onClick={() => {
                        void handleCloseTerminal()
                      }}
                    >
                      <X className="h-3.5 w-3.5" />
                    </Button>
                  ) : null}
                </>
              }
              alternateContent={
                terminalSessionId ? (
                  <WorkspaceTerminalPanel sessionId={terminalSessionId} visible={showTerminal} />
                ) : (
                  <div className="flex min-h-0 flex-1 items-center justify-center text-sm text-muted-foreground">
                    正在准备终端...
                  </div>
                )
              }
              showAlternateContent={showTerminal}
            />
            {TASK_SIDE_WORKBENCH_ENABLED ? (
              <TaskSideWorkbench
                task={selectedTask || null}
                memoValue={taskMemoDraft}
                onMemoChange={setTaskMemoDraft}
              />
            ) : null}
          </div>
          )}
        </ResizablePanel>
      </ResizablePanelGroup>

      <Dialog
        open={isCreateTaskProjectDialogOpen}
        onOpenChange={(open) => {
          setIsCreateTaskProjectDialogOpen(open)
          if (!open) {
            setCreateTaskNewProjectName("")
            setCreateTaskNewProjectPath("")
          }
        }}
      >
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>导入项目</DialogTitle>
            <DialogDescription>先创建一个项目并加入当前工作区，然后自动选中它作为任务上下文。</DialogDescription>
          </DialogHeader>
          <div className="grid gap-4 py-2">
            <div className="grid gap-2">
              <Label htmlFor="create-task-project-name">项目名称</Label>
              <Input
                id="create-task-project-name"
                value={createTaskNewProjectName}
                onChange={(e) => setCreateTaskNewProjectName(e.target.value)}
                placeholder="my-project"
                disabled={createTaskNewProjectSubmitting}
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="create-task-project-path">项目路径</Label>
              <Input
                id="create-task-project-path"
                value={createTaskNewProjectPath}
                onChange={(e) => setCreateTaskNewProjectPath(e.target.value)}
                placeholder="/path/to/project"
                disabled={createTaskNewProjectSubmitting}
              />
            </div>
          </div>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => setIsCreateTaskProjectDialogOpen(false)}
              disabled={createTaskNewProjectSubmitting}
            >
              取消
            </Button>
            <Button onClick={() => void handleCreateTaskProjectImport()} disabled={createTaskNewProjectSubmitting}>
              {createTaskNewProjectSubmitting ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : null}
              创建并选择
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {wbChrome ? (
        <Dialog open={wbChrome.searchPaletteOpen} onOpenChange={wbChrome.setSearchPaletteOpen}>
          <DialogContent
            hideClose
            className={cn(
              "!flex w-[min(720px,calc(100vw-2rem))] max-w-[720px] !flex-col gap-0 overflow-hidden p-0 sm:max-w-[720px]",
              "!min-h-0"
            )}
            style={{
              ...getElectronWindowChromeCSSVars(),
              height: "min(640px, calc(100dvh - var(--electron-window-chrome-top, 0px) - 2rem))",
              maxHeight: "min(640px, calc(100dvh - var(--electron-window-chrome-top, 0px) - 2rem))",
            }}
          >
            <DialogHeader className="sr-only">
              <DialogTitle>搜索任务</DialogTitle>
              <DialogDescription>按名称或内容筛选任务</DialogDescription>
            </DialogHeader>
            <div className="shrink-0 border-b px-4 py-4">
              <div className="relative">
                <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                <Input
                  ref={searchPaletteInputRef}
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  placeholder="搜索任务..."
                  className="h-11 pl-10 pr-3 text-base"
                />
              </div>
            </div>
            <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
              <ScrollArea className="h-full min-h-0">
                <div className="space-y-0.5 p-2 pr-3">
                  {filteredTasks.length === 0 ? (
                    <div className="py-12 text-center text-sm text-muted-foreground">
                      {searchQuery.trim() ? "没有匹配的任务" : "暂无任务"}
                    </div>
                  ) : (
                    filteredTasks.map((task) => {
                      const title = (task.name || task.content || `任务 #${task.id}`).trim()
                      const short = title.length > 160 ? `${title.slice(0, 160)}…` : title
                      return (
                        <button
                          key={task.id}
                          type="button"
                          className={cn(
                            "flex w-full flex-col gap-0.5 rounded-md border border-transparent px-3 py-2.5 text-left text-sm transition-colors",
                            "hover:bg-accent hover:text-accent-foreground",
                            selectedTaskId === task.id && "border-border bg-muted/60"
                          )}
                          onClick={() => {
                            handleSelectTaskConversation(task.id)
                            wbChrome.setSearchPaletteOpen(false)
                          }}
                        >
                          <span className="font-medium leading-snug">{short}</span>
                          {task.workerName ? (
                            <span className="text-xs text-muted-foreground">{task.workerName}</span>
                          ) : null}
                        </button>
                      )
                    })
                  )}
                </div>
              </ScrollArea>
            </div>
          </DialogContent>
        </Dialog>
      ) : null}

      <Dialog
        open={!!activeWaitUserInput}
        onOpenChange={(open) => {
          if (!open) {
            handleCancelWaitUserInput()
          }
        }}
      >
        <DialogContent className="max-w-lg max-h-[80vh] flex flex-col">
          <DialogHeader>
            <DialogTitle>
              {isProjectToolPermissionRequest ? "工具权限确认" : "需要你的输入"}
              {pendingWaitUserInputCount > 1 ? `（${pendingWaitUserInputCount} 项待处理）` : ""}
            </DialogTitle>
            <DialogDescription>
              {isProjectToolPermissionRequest
                ? pendingWaitUserInputCount > 1
                  ? "当前项目把这个工具设置成了询问模式。请依次处理队列中的权限请求。"
                  : "当前项目把这个工具设置成了询问模式，请选择是否放行本次调用。"
                : "请回答以下问题以继续执行任务"}
            </DialogDescription>
          </DialogHeader>
          <ScrollArea className="flex-1 pr-4">
            {isProjectToolPermissionRequest && projectToolPermissionPayload ? (
              <div className="space-y-4 py-2">
                <div className="rounded-lg border border-border/60 bg-muted/30 p-4 space-y-3">
                  <div>
                    <p className="text-sm font-medium">{projectToolPermissionPayload.toolLabel}</p>
                    <p className="text-xs text-muted-foreground">
                      {projectToolPermissionPayload.toolName}
                    </p>
                  </div>
                  {projectToolPermissionPayload.description && (
                    <p className="text-xs text-muted-foreground">{projectToolPermissionPayload.description}</p>
                  )}
                  <div className="grid gap-2 text-xs">
                    <div>
                      <span className="text-muted-foreground">项目：</span>
                      <span>{projectToolPermissionPayload.projectName}</span>
                    </div>
                    <div>
                      <span className="text-muted-foreground">Worker：</span>
                      <span>{projectToolPermissionPayload.workerName}</span>
                    </div>
                    {projectToolPermissionPayload.path && (
                      <div>
                        <span className="text-muted-foreground">路径：</span>
                        <code className="break-all">{projectToolPermissionPayload.path}</code>
                      </div>
                    )}
                  </div>
                </div>

                {projectToolPermissionPayload.command && (
                  <div className="space-y-2">
                    <Label>命令预览</Label>
                    <pre className="rounded-lg border border-border/60 bg-muted/30 p-3 text-xs whitespace-pre-wrap break-all">
                      {projectToolPermissionPayload.command}
                    </pre>
                  </div>
                )}

                {projectToolPermissionPayload.contentPreview && (
                  <div className="space-y-2">
                    <Label>内容预览</Label>
                    <pre className="rounded-lg border border-border/60 bg-muted/30 p-3 text-xs whitespace-pre-wrap break-all">
                      {projectToolPermissionPayload.contentPreview}
                    </pre>
                  </div>
                )}

                {projectToolPermissionPayload.patchPreview && (
                  <div className="space-y-2">
                    <Label>补丁预览</Label>
                    <PatchDiffViewer text={projectToolPermissionPayload.patchPreview} />
                  </div>
                )}

                <div className="space-y-2">
                  <Label htmlFor="tool-reject-reason">拒绝原因</Label>
                  <Textarea
                    id="tool-reject-reason"
                    value={toolRejectReason}
                    onChange={(e) => setToolRejectReason(e.target.value)}
                    placeholder="可选。比如：这个命令会改太多文件，先换一个更小范围的方案。"
                    rows={3}
                  />
                  <p className="text-xs text-muted-foreground">
                    如果你点击拒绝，这里的内容会一起回传给模型。
                  </p>
                </div>
              </div>
            ) : (
              <div className="space-y-4 py-2">
                {waitQuestions.length === 0 ? (
                  <p className="text-sm text-muted-foreground">暂无可用问题</p>
                ) : (
                  waitQuestions.map((q) => (
                    <div key={q.key} className="space-y-2">
                      <Label>
                        {q.question}
                        {q.required && <span className="text-destructive"> *</span>}
                      </Label>
                      <Input
                        value={waitUserAnswers[q.key] || ""}
                        onChange={(e) =>
                          setWaitUserAnswers(prev => ({ ...prev, [q.key]: e.target.value }))
                        }
                        placeholder="请输入"
                      />
                      {q.options && q.options.length > 0 && (
                        <div className="flex flex-wrap gap-2">
                          {q.options.map((option) => (
                            <Button
                              key={option}
                              type="button"
                              variant="outline"
                              size="sm"
                              className="h-7 text-xs"
                              onClick={() =>
                                setWaitUserAnswers(prev => ({ ...prev, [q.key]: option }))
                              }
                            >
                              {option}
                            </Button>
                          ))}
                        </div>
                      )}
                    </div>
                  ))
                )}
              </div>
            )}
          </ScrollArea>
          <DialogFooter>
            {isProjectToolPermissionRequest ? (
              <>
                <Button variant="outline" onClick={() => handleProjectToolPermissionDecision("reject")}>
                  拒绝
                </Button>
                <Button onClick={() => handleProjectToolPermissionDecision("allow")}>
                  允许本次调用
                </Button>
              </>
            ) : (
              <>
                <Button variant="outline" onClick={handleCancelWaitUserInput}>
                  暂不回答
                </Button>
                <Button onClick={handleSubmitWaitUserInput} disabled={!activeWaitUserInput || waitQuestions.length === 0}>
                  提交
                </Button>
              </>
            )}
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <ExecutionLogsDialog
        taskId={logsTaskId || 0}
        isOpen={logsTaskId !== null}
        onClose={() => setLogsTaskId(null)}
      />

      {/* RENAME DIALOG */}
      <Dialog open={!!renameTask} onOpenChange={(open) => !open && setRenameTask(null)}>
        <DialogContent className="max-w-sm">
          <DialogHeader>
            <DialogTitle>重命名任务</DialogTitle>
            <DialogDescription>更新任务名称以便区分</DialogDescription>
          </DialogHeader>
          <div className="space-y-3 py-2">
            <div className="space-y-2">
              <Label>任务名称</Label>
              <Input 
                value={renameName}
                onChange={e => setRenameName(e.target.value)}
                placeholder="输入新的任务名称"
                autoFocus
              />
            </div>
            <p className="text-xs text-muted-foreground">
              任务内容保持不变，仅更新显示名称。
            </p>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setRenameTask(null)}>取消</Button>
            <Button onClick={handleRenameTask}>保存</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* SESSION LIST DIALOG */}
      <SessionListDialog
        taskId={sessionDialogTaskId}
        open={sessionDialogTaskId !== null}
        onOpenChange={(open) => !open && setSessionDialogTaskId(null)}
        onSelectSession={handleSelectSession}
      />

      {/* SESSION INFO DIALOG */}
      <SessionInfoDialog
        execution={selectedExecution}
        open={showSessionInfo}
        onOpenChange={setShowSessionInfo}
        onViewProcessLogs={() => selectedExecution && setLogsTaskId(selectedExecution.taskId)}
      />

      {/* DIFF DIALOG */}
      {diffDialogTaskId && (
        <DiffDialog
          open={diffDialogTaskId !== null}
          onOpenChange={(open) => !open && setDiffDialogTaskId(null)}
          taskId={diffDialogTaskId}
          taskTitle={tasks.find(t => t.id === diffDialogTaskId)?.content || `任务 #${diffDialogTaskId}`}
        />
      )}

      {memoryDialogTaskId !== null && (
        <MemoryManagerDialog
          open={memoryDialogTaskId !== null}
          onOpenChange={(open) => !open && setMemoryDialogTaskId(null)}
          sessionId={
            tasks.find(t => t.id === memoryDialogTaskId)?.sessionId ||
            sessionIds.get(memoryDialogTaskId)
          }
          isRunning={tasks.find(t => t.id === memoryDialogTaskId)?.status === "running"}
          taskTitle={
            tasks.find(t => t.id === memoryDialogTaskId)?.name ||
            tasks.find(t => t.id === memoryDialogTaskId)?.content ||
            `任务 #${memoryDialogTaskId}`
          }
        />
      )}

      {memoryOrganizationDialogTaskId !== null && (
        <MemoryOrganizationDialog
          open={memoryOrganizationDialogTaskId !== null}
          onOpenChange={(open) => !open && setMemoryOrganizationDialogTaskId(null)}
          sessionId={
            tasks.find(t => t.id === memoryOrganizationDialogTaskId)?.sessionId ||
            sessionIds.get(memoryOrganizationDialogTaskId)
          }
          taskId={memoryOrganizationDialogTaskId ?? undefined}
          taskTitle={
            tasks.find(t => t.id === memoryOrganizationDialogTaskId)?.name ||
            tasks.find(t => t.id === memoryOrganizationDialogTaskId)?.content ||
            `任务 #${memoryOrganizationDialogTaskId}`
          }
        />
      )}

      {/* TASK INFO DIALOG */}
      <TaskInfoDialog
        task={taskInfoDialogTask}
        open={taskInfoDialogTask !== null}
        onOpenChange={(open) => !open && setTaskInfoDialogTask(null)}
      />

    </div>
      )}
    </>
  )
}
