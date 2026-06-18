import React, { useState, useEffect, useLayoutEffect, useRef, useMemo, useCallback } from "react"
import { useVirtualizer } from "@tanstack/react-virtual"
import { Send, Bot, AlertCircle, ChevronRight, ChevronDown, ChevronUp, ArrowDown, ArrowLeft, FileCode, Brain, Loader2, Square, Trash2, Copy, Check, FileText, Braces, Archive, Download, Upload, Plus, X, Paperclip, Sparkles, Wrench, RefreshCw, FolderOpen, GitBranch, PanelRightOpen } from "lucide-react"

import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { ScrollArea } from "@/components/ui/scroll-area"
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { AlertDialog, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from "@/components/ui/alert-dialog"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { cn } from "@/lib/utils"
import { toast } from "sonner"
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip"
import { api, SessionContextResponse, WithParts, Part, Memory, MessageTokens, SessionPromptResponse, SessionTransferPayload, TaskMessageQueueItem, TaskPlanDocument, ToolInfo, SkillCard } from "@/lib/api"
import { loadedSkillsFromMessages } from "@/lib/loadedSkillsFromMessages"
import { buildBranchMentionItems } from "@/lib/branches"
import { LexicalMentionInput } from "./LexicalMentionInput"
import { MentionItem } from "./MarkdownMentionInput"
import { MarkdownRenderer } from "./MarkdownRenderer"
import { ToolCallView } from "./ToolCallView"
import { isToolRunningStatus } from "./toolDisplayUtils"
import { ReviewDialog } from "./ReviewDialog"
import { ContextUsageDial } from "./ContextUsageDial"
import { ExpandableOverflowList } from "./ExpandableOverflowList"
import { TaskMessageQueue } from "./TaskMessageQueue"
import { MessageAttachmentBubble, isRenderableFilePart } from "./MessageAttachmentBubble"
import type { UserInputFileSource, UserMessagePart } from "@/hooks/useGlobalWebSocket"
import { useWorkbenchChatTitleToolbarSetter } from "@/components/electron/WorkbenchChatTitleToolbarContext"
import { useAutoScroll } from "@/hooks/useAutoScroll"

import {
  CHAT_ATTACHMENT_MAX_BYTES,
  filePreviewURL,
  type LocalChatAttachment,
  isUserAuthoredMessage,
  isScrollContainerNearBottom,
  VIRTUAL_MESSAGE_LIST_THRESHOLD,
  RUNNING_TOOLS_MESSAGE_LOOKBACK,
  mergeConsecutiveAssistantMessages,
  formatCompactByteCount,
  buildSessionTransferFilename,
} from "./chat/chat-utils"
import { MemoryDialog, ProjectFilePromptsPanel } from "./chat/ChatDialogs"
import { ChatV2MessageBlock } from "./chat/ChatV2MessageBlock"
import { ChatComposer } from "./chat/ChatComposer"
import { ChatSessionDialogs } from "./chat/ChatSessionDialogs"
import { WorkbenchViewTabs, type WorkbenchViewMode } from "./WorkbenchViewTabs"

interface ChatInterfaceV2Props {
  messages?: WithParts[]
  hasMoreHistory?: boolean
  isHistoryLoading?: boolean
  onLoadMoreHistory?: () => void | Promise<void>
  taskId?: number
  isConnected?: boolean
  isConnecting?: boolean
  onSendMessage?: (message: string, parts?: UserMessagePart[]) => void
  onStop?: () => void
  onCancelTool?: (callID: string) => void
  onOpenTask?: (taskId: number) => void | Promise<void>
  onBack?: () => void
  onRestart?: () => void
  onRetry?: () => void
  onShowInfo?: () => void
  onOrganizeMemory?: () => void
  onClearHistory?: () => void
  isRunning?: boolean
  isWorking?: boolean
  title?: string
  placeholder?: string
  readOnly?: boolean
  taskStatus?: string
  backLabel?: string
  headerExtra?: React.ReactNode
  sessionId?: string
  shouldRetry?: boolean
  onRetryFromError?: () => void
  onDismissRetry?: () => void
  projectId?: number
  projectPath?: string
  errorMessage?: string
  sessionTokens?: MessageTokens | null
  onSessionTransferImported?: () => void | Promise<void>
  onRetryUserMessage?: (messageId: string) => void | Promise<void>
  /** 在输入区上方展示当前任务/会话使用的 Worker 名称 */
  composerWorkerLabel?: string
  /** 任务计划（writeplan / updateplan 实时同步） */
  taskPlan?: TaskPlanDocument | null
  /** 在输入区下方追加自定义配置区 */
  composerExtra?: React.ReactNode
  /** 居中展示输入区（用于无历史消息的创建态） */
  centeredComposer?: boolean
  /** 居中展示时的标题 */
  centeredTitle?: string
  alternateContent?: React.ReactNode
  showAlternateContent?: boolean
  /** workbench：隐藏聊天顶栏，将操作按钮挂到窗口标题栏（Electron）或消息区上方细条（浏览器） */
  chatChrome?: "default" | "workbench"
  /** 多 tab 时仅当前激活页面向标题栏注册工具条，避免互相覆盖 */
  publishWorkbenchTitleToolbar?: boolean
  /** 记忆总结侧栏是否展开 */
  memorySummaryPanelOpen?: boolean
  /** 切换记忆总结侧栏 */
  onToggleMemorySummaryPanel?: () => void
  /** 工作台主视图：对话 / 仿真 */
  workbenchViewMode?: WorkbenchViewMode
  onWorkbenchViewModeChange?: (mode: WorkbenchViewMode) => void
  /** 任务消息队列 */
  messageQueue?: TaskMessageQueueItem[]
  /** 队列是否在对话结束后自动发送 */
  messageQueueAutoSend?: boolean
  onMessageQueueAutoSendChange?: (autoSend: boolean) => void
  /** 队列变更回调 */
  onMessageQueueChange?: (queue: TaskMessageQueueItem[]) => void
  /** 将队列项作为下一轮用户输入发送 */
  onSendNextQueueItem?: (itemId: string) => void
}


export function ChatInterfaceV2({
  messages = [],
  hasMoreHistory = false,
  isHistoryLoading = false,
  onLoadMoreHistory,
  taskId,
  isConnected,
  isConnecting,
  onSendMessage,
  onStop,
  onCancelTool,
  onOpenTask,
  onBack,
  onRestart,
  onRetry,
  onShowInfo,
  onOrganizeMemory,
  onClearHistory,
  isRunning = false,
  isWorking = false,
  title = "AI 助手 (V2)",
  placeholder = "输入指令...",
  readOnly = false,
  taskStatus,
  backLabel = "返回",
  headerExtra,
  sessionId,
  shouldRetry = false,
  onRetryFromError,
  onDismissRetry,
  projectId,
  projectPath,
  errorMessage,
  sessionTokens = null,
  onSessionTransferImported,
  onRetryUserMessage,
  composerWorkerLabel,
  taskPlan,
  composerExtra,
  centeredComposer = false,
  centeredTitle = "有什么可以帮忙的？",
  alternateContent,
  showAlternateContent = false,
  chatChrome = "default",
  publishWorkbenchTitleToolbar = true,
  memorySummaryPanelOpen = false,
  onToggleMemorySummaryPanel,
  workbenchViewMode = "chat",
  onWorkbenchViewModeChange,
  messageQueue,
  messageQueueAutoSend = true,
  onMessageQueueAutoSendChange,
  onMessageQueueChange,
  onSendNextQueueItem,
}: ChatInterfaceV2Props) {
  const [input, setInput] = useState("")
  const [pendingAttachments, setPendingAttachments] = useState<LocalChatAttachment[]>([])
  const [highlightedMessageId, setHighlightedMessageId] = useState<string | null>(null)
  const [showReviewDialog, setShowReviewDialog] = useState(false)
  const [showErrorDialog, setShowErrorDialog] = useState(false)
  const [showMemoryDialog, setShowMemoryDialog] = useState(false)
  const [memoryDialogLoading, setMemoryDialogLoading] = useState(false)
  const [showSessionInfoDialog, setShowSessionInfoDialog] = useState(false)
  const [showContextDialog, setShowContextDialog] = useState(false)
  const [showProjectFilePromptsDialog, setShowProjectFilePromptsDialog] = useState(false)
  const [selectedMemory, setSelectedMemory] = useState<Memory | null>(null)
  const [selectedPrompt, setSelectedPrompt] = useState<string | undefined>(undefined)
  const [selectedRawResponse, setSelectedRawResponse] = useState<string | undefined>(undefined)
  const [showPromptDialog, setShowPromptDialog] = useState(false)
  const [loadingPrompt, setLoadingPrompt] = useState(false)
  const [currentPrompt, setCurrentPrompt] = useState<SessionPromptResponse | null>(null)
  const [promptCopied, setPromptCopied] = useState(false)
  const [promptDialogTab, setPromptDialogTab] = useState<"prompt" | "response" | "history">("prompt")
  const [sessionContext, setSessionContext] = useState<SessionContextResponse | null>(null)
  const [sessionContextLoading, setSessionContextLoading] = useState(false)
  const [installedSkillsCatalog, setInstalledSkillsCatalog] = useState<SkillCard[]>([])
  const [isExportingSession, setIsExportingSession] = useState(false)
  const [isImportingSession, setIsImportingSession] = useState(false)
  const [pendingImportFile, setPendingImportFile] = useState<File | null>(null)
  const [pendingImportFilename, setPendingImportFilename] = useState("")
  const [showImportConfirm, setShowImportConfirm] = useState(false)
  /** 为 true 时新消息自动滚到底部；用户上滑后为 false，点「回到底部」再置 true */
  const [stickToBottom, setStickToBottom] = useState(true)
  const bottomRef = useRef<HTMLDivElement>(null)
  const renderMessages = useMemo(() => mergeConsecutiveAssistantMessages(messages), [messages])
  const loadedSkillsFromChat = useMemo(() => loadedSkillsFromMessages(messages), [messages])
  const hasInlineAssistantError = useMemo(
    () => renderMessages.some((msg) => msg.info.role === "assistant" && msg.parts.some((part) => part.type === "error")),
    [renderMessages],
  )

  useEffect(() => {
    setStickToBottom(true)
  }, [taskId, sessionId])
  useEffect(() => {
    setSessionContext(null)
    setShowSessionInfoDialog(false)
    setShowContextDialog(false)
    setShowMemoryDialog(false)
    setMemoryDialogLoading(false)
    setSelectedMemory(null)
    setSelectedPrompt(undefined)
    setSelectedRawResponse(undefined)
  }, [sessionId, composerWorkerLabel])
  const inputRef = useRef<HTMLTextAreaElement>(null)
  const importInputRef = useRef<HTMLInputElement>(null)
  const attachmentPickerRef = useRef<HTMLInputElement>(null)

  const ingestAttachmentFiles = useCallback(async (files: File[], source: UserInputFileSource) => {
    if (readOnly || !files.length) return
    const next: LocalChatAttachment[] = []
    for (const file of files) {
      if (file.size > CHAT_ATTACHMENT_MAX_BYTES) {
        toast.error("文件过大", { description: `${file.name} 超过 ${formatCompactByteCount(CHAT_ATTACHMENT_MAX_BYTES)}` })
        continue
      }
      next.push({
        id: `${Date.now()}-${Math.random().toString(36).slice(2, 9)}`,
        file,
        previewUrl: filePreviewURL(file),
        mime: file.type || "application/octet-stream",
        name: file.name || "未命名",
        source,
      })
    }
    if (next.length) {
      setPendingAttachments((prev) => [...prev, ...next])
    }
  }, [readOnly])

  const removePendingAttachment = useCallback((id: string) => {
    setPendingAttachments((prev) => prev.filter((a) => a.id !== id))
  }, [])

  const handleAttachmentPickerChange = useCallback(
    async (e: React.ChangeEvent<HTMLInputElement>) => {
      const list = e.target.files
      e.target.value = ""
      if (!list?.length) return
      await ingestAttachmentFiles(Array.from(list), "picker")
    },
    [ingestAttachmentFiles],
  )

  const handleComposerPasteCapture = useCallback(
    (e: React.ClipboardEvent) => {
      if (readOnly) return
      const cd = e.clipboardData
      if (!cd) return
      const fromItems: File[] = []
      for (let i = 0; i < cd.items.length; i++) {
        const it = cd.items[i]
        if (it.kind === "file") {
          const f = it.getAsFile()
          if (f) fromItems.push(f)
        }
      }
      const fromFiles = cd.files?.length ? Array.from(cd.files) : []
      const candidates = fromItems.length > 0 ? fromItems : fromFiles
      if (!candidates.length) return
      e.preventDefault()
      e.stopPropagation()
      void ingestAttachmentFiles(candidates, "paste")
    },
    [readOnly, ingestAttachmentFiles],
  )

  const handleViewMemory = useCallback(async (messageId: string) => {
    if (!sessionId && !taskId) {
      toast.error("当前没有可查看的会话信息")
      return
    }

    setShowMemoryDialog(true)
    setMemoryDialogLoading(true)
    setSelectedMemory(null)
    setSelectedPrompt(undefined)
    setSelectedRawResponse(undefined)

    try {
      const [promptResult, contextResult] = await Promise.all([
        sessionId
          ? api.getSessionPrompt(sessionId, { skipErrorNotification: true, messageId })
          : api.getTaskPrompt(taskId!, { skipErrorNotification: true, messageId }),
        sessionId
          ? api.getSessionContext(sessionId, composerWorkerLabel, { skipErrorNotification: true })
          : Promise.resolve(null),
      ])

      if (promptResult) {
        setSelectedPrompt(promptResult.prompt)
        setSelectedRawResponse(promptResult.rawResponse)
        setCurrentPrompt(promptResult)
      }

      if (contextResult?.prompts) {
        setSelectedMemory(contextResult.prompts)
      }

      if (!promptResult?.prompt && !selectedMemory) {
        toast.error("当前没有可展示的提示词")
        setShowMemoryDialog(false)
      }
    } catch (error) {
      console.error("Failed to load memory dialog data:", error)
      toast.error("加载提示词失败", {
        description: error instanceof Error ? error.message : "未知错误",
      })
      if (!selectedMemory) {
        setShowMemoryDialog(false)
      }
    } finally {
      setMemoryDialogLoading(false)
    }
  }, [composerWorkerLabel, selectedMemory, sessionId, taskId])

  const handleOpenPromptDialog = useCallback(async () => {
    if (!sessionId && !taskId) {
      toast.error("当前没有可查看的 Prompt")
      return
    }

    setLoadingPrompt(true)
    try {
      const promptData = sessionId
        ? await api.getSessionPrompt(sessionId, { skipErrorNotification: true })
        : await api.getTaskPrompt(taskId!, { skipErrorNotification: true })
      setCurrentPrompt(promptData)
      setShowPromptDialog(true)
    } catch (error: unknown) {
      console.error("Failed to load task prompt:", error)
      const status = typeof error === "object" && error !== null && "status" in error ? (error as { status: number }).status : 0
      if (status === 404) {
        toast.error("暂无可用会话", {
          description: "请先发送一条消息开始对话，之后再查看 Prompt。",
        })
      } else {
        toast.error("加载 Prompt 失败", {
          description: error instanceof Error ? error.message : "未知错误",
        })
      }
    } finally {
      setLoadingPrompt(false)
    }
  }, [sessionId, taskId])

  const loadSessionContext = useCallback(async (options?: { silent?: boolean }) => {
    if (!sessionId) {
      if (!options?.silent) {
        toast.error("当前还没有会话")
      }
      return null
    }
    try {
      setSessionContextLoading(true)
      const data = await api.getSessionContext(sessionId, composerWorkerLabel, { skipErrorNotification: true })
      setSessionContext(data)
      return data
    } catch (error) {
      console.error("Failed to load session context:", error)
      if (!options?.silent) {
        toast.error("加载会话信息失败", {
          description: (error as { message?: string } | undefined)?.message || "未知错误",
        })
      }
      return null
    } finally {
      setSessionContextLoading(false)
    }
  }, [composerWorkerLabel, sessionId])

  useEffect(() => {
    if (!sessionId) {
      setSessionContext(null)
      return
    }
    void loadSessionContext({ silent: true })
  }, [loadSessionContext, sessionId])

  const handleOpenSessionInfoDialog = useCallback(async () => {
    if (!sessionId) return
    try {
      setSessionContextLoading(true)
      const [data, installedSkills] = await Promise.all([
        api.getSessionContext(sessionId, composerWorkerLabel, { skipErrorNotification: true }),
        api.getSkills(true),
      ])
      setSessionContext(data)
      setInstalledSkillsCatalog(installedSkills.filter((skill) => skill.installed))
      setShowSessionInfoDialog(true)
    } catch (error) {
      console.error("Failed to load session tools/skills:", error)
      toast.error("加载技能与工具失败")
    } finally {
      setSessionContextLoading(false)
    }
  }, [composerWorkerLabel, sessionId])

  const handleOpenProjectFilePromptsDialog = useCallback(async () => {
    const data = await loadSessionContext()
    if (data) {
      setShowProjectFilePromptsDialog(true)
    }
  }, [loadSessionContext])

  const handleOpenContextDialog = useCallback(async () => {
    const data = await loadSessionContext()
    if (data) {
      setShowContextDialog(true)
    }
  }, [loadSessionContext])

  const handleCopyText = useCallback((text: string, successLabel: string) => {
    navigator.clipboard.writeText(text)
    toast.success(successLabel)
  }, [])

  const runningTools = useMemo(() => {
    const tools: Array<{ id: string; name: string; status: string }> = []
    const seen = new Set<string>()
    const tailStart = Math.max(0, messages.length - RUNNING_TOOLS_MESSAGE_LOOKBACK)
    for (let mi = tailStart; mi < messages.length; mi++) {
      const message = messages[mi]
      for (const part of message.parts) {
        if (part.type !== "tool" && part.type !== "tool-delta") continue
        if (!part.tool) continue
        if (!isToolRunningStatus(part.tool.state.status)) continue
        const callID = part.tool.callID
        if (!callID || seen.has(callID)) continue
        seen.add(callID)
        tools.push({
          id: callID,
          name: part.tool.tool || "工具",
          status: part.tool.state.status || "running",
        })
      }
    }
    return tools
  }, [messages])

  const lastRetryableUserMessageId = useMemo(() => {
    for (let i = renderMessages.length - 1; i >= 0; i--) {
      const msg = renderMessages[i]
      if (!isUserAuthoredMessage(msg) || !msg?.info?.id) continue
      const hasText = msg.parts.some((part) => part.type === "text" && (part.text || "").trim() !== "")
      if (hasText) {
        return msg.info.id
      }
    }
    return null
  }, [renderMessages])

  const hasRunningTools = runningTools.length > 0
  const virtualizeMessages = renderMessages.length >= VIRTUAL_MESSAGE_LIST_THRESHOLD && !hasRunningTools

  const scrollContainerRef = useRef<HTMLDivElement | null>(null)
  const historyLoadSentinelRef = useRef<HTMLDivElement | null>(null)
  const prependHistoryAnchorRef = useRef<{
    scrollHeight: number
    scrollTop: number
    messageCount: number
  } | null>(null)
  const historyLoadArmedRef = useRef(false)

  useEffect(() => {
    historyLoadArmedRef.current = false
    prependHistoryAnchorRef.current = null
  }, [sessionId, taskId])

  const virtualizer = useVirtualizer({
    count: virtualizeMessages ? renderMessages.length : 0,
    getScrollElement: () => scrollContainerRef.current,
    estimateSize: () => 140,
    overscan: 10,
  })
  const virtualizerRef = useRef(virtualizer)
  virtualizerRef.current = virtualizer
  const handleVirtualizedMessageLayoutChange = useCallback(() => {
    if (!virtualizeMessages) return
    requestAnimationFrame(() => {
      virtualizerRef.current.measure()
    })
  }, [virtualizeMessages])

  const scrollToBottom = useCallback(
    (behavior: ScrollBehavior = "smooth") => {
      setStickToBottom(true)
      if (renderMessages.length === 0) return
      if (virtualizeMessages) {
        virtualizerRef.current.scrollToIndex(messages.length - 1, { align: "end", behavior })
      } else {
        bottomRef.current?.scrollIntoView({ behavior, block: "end" })
      }
    },
    [messages.length, virtualizeMessages],
  )

  const handleMessagesScroll = useCallback(() => {
    const el = scrollContainerRef.current
    if (!el) return
    setStickToBottom(isScrollContainerNearBottom(el))
  }, [])

  useLayoutEffect(() => {
    const anchor = prependHistoryAnchorRef.current
    const el = scrollContainerRef.current
    if (!anchor || !el) return

    if (messages.length > anchor.messageCount) {
      const delta = el.scrollHeight - anchor.scrollHeight
      el.scrollTop = anchor.scrollTop + Math.max(delta, 0)
      prependHistoryAnchorRef.current = null
      return
    }

    if (!isHistoryLoading) {
      prependHistoryAnchorRef.current = null
    }
  }, [isHistoryLoading, messages.length, virtualizeMessages])

  const beginLoadMoreHistory = useCallback(() => {
    if (!hasMoreHistory || isHistoryLoading || !onLoadMoreHistory || prependHistoryAnchorRef.current) {
      return false
    }
    const el = scrollContainerRef.current
    if (el) {
      prependHistoryAnchorRef.current = {
        scrollHeight: el.scrollHeight,
        scrollTop: el.scrollTop,
        messageCount: messages.length,
      }
    }
    historyLoadArmedRef.current = false
    void onLoadMoreHistory()
    return true
  }, [hasMoreHistory, isHistoryLoading, messages.length, onLoadMoreHistory])

  useEffect(() => {
    const root = scrollContainerRef.current
    const target = historyLoadSentinelRef.current
    if (!root || !target || typeof IntersectionObserver === "undefined") {
      return
    }

    const observer = new IntersectionObserver(
      (entries) => {
        const entry = entries[0]
        if (!entry) return

        if (!entry.isIntersecting) {
          historyLoadArmedRef.current = true
          return
        }

        if (!historyLoadArmedRef.current) return
        beginLoadMoreHistory()
      },
      {
        root,
        threshold: 0,
      },
    )

    observer.observe(target)
    return () => observer.disconnect()
  }, [beginLoadMoreHistory])

  const promptDialogHistoryText = useMemo(() => {
    return messages
      .map((message) => {
        const role = message.info.role === "assistant" ? "assistant" : message.info.role === "user" ? "user" : String(message.info.role || "unknown")
        const blocks = message.parts
          .map((part) => {
            if ((part.type === "text" || part.type === "text-delta" || part.type === "reasoning" || part.type === "reasoning-delta") && (part.text || "").trim()) {
              return part.text || ""
            }
            if (part.type === "error") {
              const errText = part.error?.message || part.text || ""
              return errText.trim() ? `[错误]\n${errText}` : ""
            }
            if (part.type === "file") {
              return `[文件] ${part.filename || part.url || "未命名附件"}`
            }
            if ((part.type === "tool" || part.type === "tool-delta") && part.tool) {
              const toolName = part.tool.tool || "unknown-tool"
              const callID = part.tool.callID || "unknown-call-id"
              const state = part.tool.state
              const details: string[] = []
              if (state?.status) details.push(`状态: ${state.status}`)
              if (state?.title) details.push(`标题: ${state.title}`)
              if (state?.input !== undefined) {
                const inputText = typeof state.input === "string" ? state.input : JSON.stringify(state.input, null, 2)
                if (inputText && inputText.trim()) details.push(`参数:\n${inputText}`)
              }
              if (state?.raw && state.raw.trim()) details.push(`原始参数:\n${state.raw}`)
              if (state?.output && state.output.trim()) details.push(`输出:\n${state.output}`)
              if (state?.error && state.error.trim()) details.push(`错误:\n${state.error}`)
              if (state?.metadata && Object.keys(state.metadata).length > 0) {
                details.push(`元数据:\n${JSON.stringify(state.metadata, null, 2)}`)
              }
              return [`[工具调用] ${toolName} (${callID})`, ...details].join("\n")
            }
            if (part.type === "memory-organization") {
              return "[记忆整理]"
            }
            if (part.type === "compaction") {
              return "[记忆压缩]"
            }
            return ""
          })
          .filter((item) => item.trim() !== "")
        if (blocks.length === 0) {
          return ""
        }
        return `[${role}]\n${blocks.join("\n\n")}`
      })
      .filter((item) => item.trim() !== "")
      .join("\n\n")
  }, [messages])

  const handleCopyPrompt = useCallback(() => {
    if (promptDialogTab === "history") {
      if (!promptDialogHistoryText.trim()) return
      navigator.clipboard.writeText(promptDialogHistoryText)
      setPromptCopied(true)
      setTimeout(() => setPromptCopied(false), 2000)
      return
    }

    if (promptDialogTab === "response") {
      if (!currentPrompt?.rawResponse) return
      navigator.clipboard.writeText(currentPrompt.rawResponse)
      setPromptCopied(true)
      setTimeout(() => setPromptCopied(false), 2000)
      return
    }

    if (!currentPrompt?.prompt) return
    navigator.clipboard.writeText(currentPrompt.prompt)
    setPromptCopied(true)
    setTimeout(() => setPromptCopied(false), 2000)
  }, [currentPrompt?.prompt, currentPrompt?.rawResponse, promptDialogHistoryText, promptDialogTab])

  useEffect(() => {
    if (!showPromptDialog) return
    if (promptDialogTab === "response" && !currentPrompt?.rawResponse) {
      setPromptDialogTab("prompt")
    }
  }, [currentPrompt?.rawResponse, promptDialogTab, showPromptDialog])

  const handleExportSessionTransfer = useCallback(async () => {
    if (!sessionId) {
      toast.error("当前还没有会话，暂时不能导出")
      return
    }

    setIsExportingSession(true)
    try {
      const blob = await api.exportSessionTransfer(sessionId)
      const url = URL.createObjectURL(blob)
      const link = document.createElement("a")
      link.href = url
      link.download = buildSessionTransferFilename(title)
      document.body.appendChild(link)
      link.click()
      document.body.removeChild(link)
      URL.revokeObjectURL(url)

      toast.success("导出成功")
    } catch (error) {
      console.error("Failed to export session transfer:", error)
      toast.error("导出失败", {
        description: (error as { message?: string } | undefined)?.message || "未知错误",
      })
    } finally {
      setIsExportingSession(false)
    }
  }, [sessionId, title])

  const handlePickImportFile = useCallback(() => {
    if (readOnly) {
      toast.error("请先回到当前实时会话，再导入覆盖")
      return
    }
    if (!sessionId) {
      toast.error("当前还没有会话，暂时不能导入")
      return
    }
    importInputRef.current?.click()
  }, [readOnly, sessionId])

  const handleImportFileChange = useCallback(async (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0]
    event.target.value = ""

    if (!file) {
      return
    }

    // 直接保存文件，后端会解析 ZIP 或 JSON
    setPendingImportFile(file)
    setPendingImportFilename(file.name)
    setShowImportConfirm(true)
  }, [])

  const clearPendingImport = useCallback(() => {
    setShowImportConfirm(false)
    setPendingImportFile(null)
    setPendingImportFilename("")
  }, [])

  const handleConfirmImport = useCallback(async () => {
    if (!sessionId || !pendingImportFile) {
      return
    }

    setIsImportingSession(true)
    try {
      const result = await api.importSessionTransfer(sessionId, pendingImportFile)
      setCurrentPrompt(null)
      setSelectedMemory(null)
      setSelectedPrompt(undefined)
      setShowMemoryDialog(false)
      await onSessionTransferImported?.()
      toast.success("导入成功", {
        description: `已覆盖为 ${result.importedMessages} 条聊天消息和 ${result.importedMemories} 条记忆`,
      })
      clearPendingImport()
    } catch (error) {
      console.error("Failed to import session transfer:", error)
      toast.error("导入失败", {
        description: (error as { message?: string } | undefined)?.message || "未知错误",
      })
    } finally {
      setIsImportingSession(false)
    }
  }, [clearPendingImport, onSessionTransferImported, pendingImportFile, sessionId])

  const hasUserMessages = useMemo(() => messages.some(isUserAuthoredMessage), [messages])

  const hasMeaningfulTokens = useCallback((tokens?: MessageTokens | null) => {
    if (!tokens) return false
    return (
      tokens.input > 0 ||
      tokens.output > 0 ||
      tokens.reasoning > 0 ||
      (tokens.cache?.read || 0) > 0 ||
      (tokens.cache?.write || 0) > 0
    )
  }, [])

  const latestAssistantTokens = useMemo<MessageTokens | null>(() => {
    for (let index = messages.length - 1; index >= 0; index -= 1) {
      const message = messages[index]
      if (message.info.role === 'assistant' && hasMeaningfulTokens(message.info.tokens)) {
        return message.info.tokens
      }
    }
    return sessionTokens
  }, [messages, sessionTokens, hasMeaningfulTokens])

  const currentContextUsageTokens = useMemo(() => {
    if (!latestAssistantTokens) {
      return 0
    }
    return latestAssistantTokens.input + (latestAssistantTokens.cache?.read || 0)
  }, [latestAssistantTokens])

  const contextLimitTokens = sessionContext?.effectiveContextLimit || sessionContext?.contextLimit || 0

  const contextUsagePercent = useMemo(() => {
    if (!contextLimitTokens || currentContextUsageTokens <= 0) {
      return null
    }
    return (currentContextUsageTokens * 100) / contextLimitTokens
  }, [contextLimitTokens, currentContextUsageTokens])

  useEffect(() => {
    if (!sessionId && !taskId) {
      return
    }

    let cancelled = false

    const loadPrompt = async () => {
      try {
        const promptData = sessionId
          ? await api.getSessionPrompt(sessionId, { skipErrorNotification: true })
          : await api.getTaskPrompt(taskId!, { skipErrorNotification: true })

        if (!cancelled) {
          setCurrentPrompt(promptData)
        }
      } catch (error) {
        if (!cancelled) {
          console.error("Failed to preload task prompt:", error)
        }
      }
    }

    void loadPrompt()

    return () => {
      cancelled = true
    }
  }, [sessionId, taskId])

  const currentContextPrompt = currentPrompt?.prompt || ""

  const formatTokenCount = useCallback((value: number) => {
    if (value >= 1_000_000) {
      return `${(value / 1_000_000).toFixed(value >= 10_000_000 ? 0 : 1)}M`
    }
    if (value >= 1_000) {
      return `${(value / 1_000).toFixed(value >= 10_000 ? 0 : 1)}k`
    }
    return `${value}`
  }, [])

  const formatByteCount = useCallback((value: number) => {
    if (value >= 1024 * 1024) {
      return `${(value / (1024 * 1024)).toFixed(value >= 10 * 1024 * 1024 ? 0 : 1)}MB`
    }
    if (value >= 1024) {
      return `${(value / 1024).toFixed(value >= 10 * 1024 ? 0 : 1)}KB`
    }
    return `${value}B`
  }, [])

  const contextButtonTitle = useMemo(() => {
    if (!contextLimitTokens) {
      return "上下文上限未知"
    }
    const percentLabel = contextUsagePercent == null ? "0%" : `${contextUsagePercent.toFixed(contextUsagePercent >= 10 ? 0 : 1)}%`
    return `上下文占用 ${percentLabel} · ${formatTokenCount(currentContextUsageTokens)} / ${formatTokenCount(contextLimitTokens)} tokens`
  }, [contextLimitTokens, contextUsagePercent, currentContextUsageTokens, formatTokenCount])

  const tokenStats = useMemo(() => {
    const stats: Array<{ label: string; value: number; formatter?: (value: number) => string }> = []

    if (latestAssistantTokens) {
      const contextSize = latestAssistantTokens.input + (latestAssistantTokens.cache?.read || 0)
      stats.push(
        { label: '上下文', value: contextSize },
        { label: '输出', value: latestAssistantTokens.output },
      )

      if (latestAssistantTokens.reasoning > 0) {
        stats.push({ label: '推理', value: latestAssistantTokens.reasoning })
      }
      if ((latestAssistantTokens.cache?.read || 0) > 0) {
        stats.push({ label: '缓存', value: latestAssistantTokens.cache.read })
      }
    }

    if (currentContextPrompt) {
      stats.push({
        label: '上下文大小',
        value: new TextEncoder().encode(currentContextPrompt).length,
        formatter: formatByteCount,
      })
    }

    return stats
  }, [currentContextPrompt, formatByteCount, latestAssistantTokens])

  const headerStats = useMemo(() => {
    const stats = [...tokenStats]

    if (messages.length > 0) {
      stats.push({ label: '消息', value: messages.length })
    }

    return stats
  }, [messages.length, tokenStats])

  // 默认的 mention 项目（添加分类）
  const defaultMentionItems: MentionItem[] = useMemo(() => {
    if (readOnly || !sessionId) {
      return []
    }

    return [
      {
        id: 'command-compress',
        type: 'command',
        label: 'compress',
        value: 'command://default?name=compress',
        description: '压缩并整理当前会话记忆',
        icon: <Archive className="h-3.5 w-3.5" />,
        category: 'command',
      },
      {
        id: 'command-summary',
        type: 'command',
        label: 'summary',
        value: 'command://default?name=summary',
        description: '总结当前会话并保存到记忆库',
        icon: <Archive className="h-3.5 w-3.5" />,
        category: 'command',
      },
      {
        id: 'command-new-worktree',
        type: 'command',
        label: 'new-worktree',
        value: 'command://default?name=new-worktree',
        description: '基于当前分支创建新的 worktree',
        icon: <GitBranch className="h-3.5 w-3.5" />,
        category: 'command',
      },
    ]
  }, [readOnly, sessionId])

  // 处理 mention 搜索
  const handleMentionSearch = useCallback(async (query: string): Promise<MentionItem[]> => {
    const itemsByCategory = new Map<string, MentionItem[]>()
    const appendItems = (category: string, nextItems: MentionItem[]) => {
      if (nextItems.length === 0) {
        return
      }
      const currentItems = itemsByCategory.get(category) || []
      itemsByCategory.set(category, [...currentItems, ...nextItems])
    }
    
    // 1. 添加默认命令
    const commandItems = !query.trim() 
      ? defaultMentionItems 
      : defaultMentionItems.filter(item =>
          item.label.toLowerCase().includes(query.toLowerCase()) ||
          (item.description && item.description.toLowerCase().includes(query.toLowerCase()))
        )
    appendItems('command', commandItems)

    // 2. 如果有 projectId，加载资源列表
    if (projectId) {
      const [resourcesResult, branchesResult] = await Promise.allSettled([
        api.getResources(projectId),
        api.getBranches(projectId),
      ])

      if (resourcesResult.status === 'fulfilled') {
        resourcesResult.value.forEach((resourceGroup) => {
          Object.entries(resourceGroup).forEach(([category, paths]) => {
            if (category === 'branch') {
              return
            }

            const categoryItems = paths.flatMap((path) => {
              const label = path.split('/').pop() || path
              const type = category === 'command' || category === 'file' || category === 'worker' || category === 'skill'
                ? (category as 'command' | 'file' | 'worker' | 'skill')
                : 'custom'

              const shouldInclude = category === 'file'
                ? true
                : (!query.trim() ||
                    label.toLowerCase().includes(query.toLowerCase()) ||
                    path.toLowerCase().includes(query.toLowerCase()))

              if (!shouldInclude) {
                return []
              }

              const value = type === 'file'
                ? `file://default?filePath=${encodeURIComponent(path)}`
                : type === 'worker'
                  ? `worker://default?name=${encodeURIComponent(path)}`
                  : type === 'skill'
                    ? `skill://default?name=${encodeURIComponent(path)}`
                    : path

              return [{
                id: `resource-${category}-${path}`,
                type,
                label,
                value,
                description: path,
                category,
                isSpecialCommand: type === 'command' && path === 'review',
              }]
            })

            appendItems(category, categoryItems)
          })
        })
      } else {
        console.error('获取资源列表失败:', resourcesResult.reason)
      }

      if (branchesResult.status === 'fulfilled') {
        appendItems('branch', buildBranchMentionItems(branchesResult.value, query))
      } else {
        console.error('获取分支列表失败:', branchesResult.reason)
      }
    }

    const categoryOrder = ['command', 'file', 'branch', 'worker', 'skill']
    const orderedItems: MentionItem[] = []

    categoryOrder.forEach((category) => {
      orderedItems.push(...(itemsByCategory.get(category) || []))
      itemsByCategory.delete(category)
    })

    itemsByCategory.forEach((groupItems) => {
      orderedItems.push(...groupItems)
    })

    return orderedItems
  }, [defaultMentionItems, projectId])

  const handleFileSearch = useCallback(async (query: string): Promise<MentionItem[]> => {
    if (!projectId || !query.trim()) return []
    try {
      const results = await api.searchResources(projectId, query.trim())
      return results.map((path) => {
        const label = path.split('/').pop() || path
        return {
          id: `resource-file-${path}`,
          type: 'file',
          label,
          value: `file://default?filePath=${encodeURIComponent(path)}`,
          description: path,
          category: 'file'
        }
      })
    } catch (error) {
      console.error('搜索文件失败:', error)
      return []
    }
  }, [projectId])

  // 仅在用户位于底部时自动跟随新消息，避免上滑阅读时被拽回底部
  useEffect(() => {
    if (renderMessages.length === 0 || !stickToBottom) return
    const timer = setTimeout(() => {
      if (virtualizeMessages) {
        virtualizerRef.current.scrollToIndex(renderMessages.length - 1, { align: "end", behavior: "auto" })
      } else {
        bottomRef.current?.scrollIntoView({ behavior: "auto", block: "end" })
      }
    }, 50)
    return () => clearTimeout(timer)
  }, [renderMessages, virtualizeMessages, renderMessages.length, stickToBottom])

  useEffect(() => {
    if (!stickToBottom || renderMessages.length === 0) return
    const timer = setTimeout(() => {
      if (virtualizeMessages) {
        virtualizerRef.current.scrollToIndex(renderMessages.length - 1, { align: "end", behavior: "auto" })
      } else {
        const el = scrollContainerRef.current
        if (el) {
          el.scrollTop = el.scrollHeight
        } else {
          bottomRef.current?.scrollIntoView({ behavior: "auto", block: "end" })
        }
      }
    }, 0)
    return () => clearTimeout(timer)
  }, [virtualizeMessages, stickToBottom, renderMessages.length])

  const handleSend = useCallback(async () => {
    const text = input.trim()
    if (readOnly) return
    if (!text && pendingAttachments.length === 0) return
    setStickToBottom(true)
    let parts: UserMessagePart[] | undefined
    try {
      if (pendingAttachments.length > 0) {
        const uploaded: UserMessagePart[] = []
        for (const attachment of pendingAttachments) {
          const res = await api.uploadTempFiles([{ file: attachment.file, inputSource: attachment.source }])
          for (const file of res.files) {
            uploaded.push({
              type: "file",
              path: file.path,
              mime: file.mime,
              filename: file.filename,
              inputSource: (file.inputSource as UserInputFileSource | undefined) ?? attachment.source,
              url: file.url,
            })
          }
        }
        parts = uploaded
      }
    } catch (error: any) {
      toast.error("附件上传失败", { description: error?.message || "请稍后重试" })
      return
    }
    onSendMessage?.(text, parts)
    setInput("")
    for (const attachment of pendingAttachments) {
      URL.revokeObjectURL(attachment.previewUrl)
    }
    setPendingAttachments([])
  }, [input, readOnly, onSendMessage, pendingAttachments, taskId])

  const handleStopClick = useCallback((e?: React.MouseEvent | React.FormEvent) => {
    e?.preventDefault()
    e?.stopPropagation()
    onStop?.()
  }, [onStop])

  // 处理特殊命令
  const handleSpecialCommand = useCallback((command: string) => {
    if (command === 'review') {
      setShowReviewDialog(true)
      return
    }
  }, [])

  // 处理 review 对话框确认
  const [pendingMention, setPendingMention] = useState<MentionItem | null>(null)

  const handleReviewConfirm = useCallback((command: string) => {
    const match = command.match(/\[(.+?)\]\((review:\/\/[^)]+)\)/)
    if (match) {
      setPendingMention({
        id: `review-${Date.now()}`,
        type: 'command',
        label: match[1],
        value: match[2],
        description: '代码审查对比',
        category: '命令',
      })
      return
    }
    setInput(prev => prev ? `${prev} ${command}` : command)
  }, [])

  const scrollToMessage = useCallback((direction: 'prev' | 'next') => {
    if (renderMessages.length === 0) return

    const userMessages = renderMessages.filter(isUserAuthoredMessage)
    if (userMessages.length === 0) return

    const viewport = scrollContainerRef.current
    if (!viewport) return

    const applyHighlight = (id: string) => {
      setHighlightedMessageId(id)
      setTimeout(() => {
        setHighlightedMessageId((prev) => (prev === id ? null : prev))
      }, 2000)
    }

    if (virtualizeMessages) {
      const userIndices = userMessages
        .map((m) => renderMessages.findIndex((x) => x.info.id === m.info.id))
        .filter((i) => i >= 0)
      if (userIndices.length === 0) return

      const vis = virtualizerRef.current.getVirtualItems()
      const firstVisible = vis.length ? vis[0].index : 0

      let targetIdx: number
      if (direction === "prev") {
        const candidates = userIndices.filter((i) => i < firstVisible)
        targetIdx = candidates.length ? candidates[candidates.length - 1] : userIndices[0]
      } else {
        const next = userIndices.find((i) => i > firstVisible)
        targetIdx = next !== undefined ? next : userIndices[userIndices.length - 1]
      }

      const targetId = renderMessages[targetIdx]?.info?.id
      if (targetId) {
        virtualizerRef.current.scrollToIndex(targetIdx, { align: "start", behavior: "smooth" })
        applyHighlight(targetId)
      }
      return
    }

    const messageElements = userMessages
      .map((msg) => ({
        id: msg.info.id,
        el: document.getElementById(`msg-${msg.info.id}`),
      }))
      .filter((m) => m.el) as { id: string; el: HTMLElement }[]

    if (messageElements.length === 0) return

    const viewportRect = viewport.getBoundingClientRect()
    const threshold = 20

    let targetMessage: { id: string; el: HTMLElement } | null = null

    if (direction === "prev") {
      for (let i = messageElements.length - 1; i >= 0; i--) {
        const rect = messageElements[i].el.getBoundingClientRect()
        if (rect.top < viewportRect.top - threshold) {
          targetMessage = messageElements[i]
          break
        }
      }
      if (!targetMessage) targetMessage = messageElements[0]
    } else {
      for (let i = 0; i < messageElements.length; i++) {
        const rect = messageElements[i].el.getBoundingClientRect()
        if (rect.top > viewportRect.top + threshold) {
          targetMessage = messageElements[i]
          break
        }
      }
    }

    if (targetMessage) {
      targetMessage.el.scrollIntoView({ behavior: "smooth", block: "start" })
      applyHighlight(targetMessage.id)
    } else {
      scrollToBottom("smooth")
    }
  }, [renderMessages, virtualizeMessages, scrollToBottom])

  const setWorkbenchChatTitleToolbar = useWorkbenchChatTitleToolbarSetter()

  const showProjectWorkbenchToolbar =
    chatChrome === "workbench" && !!projectId && publishWorkbenchTitleToolbar

  const workbenchTitleToolbarActions = useMemo(() => {
    if (!showProjectWorkbenchToolbar) return null
    return (
      <TooltipProvider delayDuration={300}>
        <div className="flex items-center gap-0.5">
          {onWorkbenchViewModeChange ? (
            <WorkbenchViewTabs
              value={workbenchViewMode}
              onChange={onWorkbenchViewModeChange}
              className="mr-1.5"
            />
          ) : null}
          {headerExtra}
          <div className="flex items-center gap-0.5 border-r border-border/60 pr-2 mr-1">
            <Tooltip>
              <TooltipTrigger asChild>
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  className="h-8 w-8"
                  onClick={() => scrollToMessage("prev")}
                  disabled={!hasUserMessages}
                >
                  <ChevronUp className="h-4 w-4" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>上一条</TooltipContent>
            </Tooltip>
            <Tooltip>
              <TooltipTrigger asChild>
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  className="h-8 w-8"
                  onClick={() => scrollToMessage("next")}
                  disabled={!hasUserMessages}
                >
                  <ChevronDown className="h-4 w-4" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>下一条</TooltipContent>
            </Tooltip>
          </div>

          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                type="button"
                variant="ghost"
                size="icon"
                className="h-8 w-8"
                onClick={() => void handleExportSessionTransfer()}
                disabled={isExportingSession}
              >
                {isExportingSession ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Download className="h-3.5 w-3.5" />}
              </Button>
            </TooltipTrigger>
            <TooltipContent>导出记忆和聊天记录</TooltipContent>
          </Tooltip>

          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                type="button"
                variant="ghost"
                size="icon"
                className="h-8 w-8"
                onClick={handlePickImportFile}
                disabled={isImportingSession || readOnly}
              >
                {isImportingSession ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Upload className="h-3.5 w-3.5" />}
              </Button>
            </TooltipTrigger>
            <TooltipContent>导入并覆盖当前记忆和聊天记录</TooltipContent>
          </Tooltip>

          {onClearHistory ? (
            <Tooltip>
              <TooltipTrigger asChild>
                <Button type="button" variant="ghost" size="icon" className="h-8 w-8" onClick={onClearHistory}>
                  <Trash2 className="h-3.5 w-3.5" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>清空历史</TooltipContent>
            </Tooltip>
          ) : null}

          {sessionId || taskId ? (
            <Tooltip>
              <TooltipTrigger asChild>
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  className="h-8 w-8"
                  onClick={() => void handleOpenPromptDialog()}
                  disabled={loadingPrompt}
                >
                  {loadingPrompt ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Braces className="h-3.5 w-3.5" />}
                </Button>
              </TooltipTrigger>
              <TooltipContent>查看当前 Prompt</TooltipContent>
            </Tooltip>
          ) : null}

          {onToggleMemorySummaryPanel ? (
            <Tooltip>
              <TooltipTrigger asChild>
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  className={cn("h-8 w-8", memorySummaryPanelOpen && "bg-accent text-accent-foreground")}
                  onClick={onToggleMemorySummaryPanel}
                  disabled={!sessionId}
                >
                  <PanelRightOpen className="h-3.5 w-3.5" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>{memorySummaryPanelOpen ? "收起记忆总结" : "展开记忆总结"}</TooltipContent>
            </Tooltip>
          ) : null}
        </div>
      </TooltipProvider>
    )
  }, [
    showProjectWorkbenchToolbar,
    headerExtra,
    hasUserMessages,
    scrollToMessage,
    handleExportSessionTransfer,
    isExportingSession,
    handlePickImportFile,
    isImportingSession,
    readOnly,
    onClearHistory,
    handleOpenPromptDialog,
    loadingPrompt,
    sessionId,
    taskId,
    onToggleMemorySummaryPanel,
    memorySummaryPanelOpen,
    onWorkbenchViewModeChange,
    workbenchViewMode,
  ])

  useLayoutEffect(() => {
    if (!setWorkbenchChatTitleToolbar) return
    if (!workbenchTitleToolbarActions) {
      return
    }
    setWorkbenchChatTitleToolbar(workbenchTitleToolbarActions)
    return () => {
      setWorkbenchChatTitleToolbar(null)
    }
  }, [setWorkbenchChatTitleToolbar, workbenchTitleToolbarActions])

  const hasComposerPayload = Boolean(input.trim()) || pendingAttachments.length > 0
  const showSendWhileRunning = isRunning && hasComposerPayload

  return (
    <div className="flex h-full min-w-0 flex-col bg-background">
      <input
        ref={importInputRef}
        type="file"
        accept=".zip,application/json,.json"
        className="hidden"
        onChange={handleImportFileChange}
      />

      {centeredComposer ? (
        <div className="flex min-h-0 flex-1 items-center justify-center px-6 py-8">
          <div className="w-full max-w-4xl">
            <div className="mb-6 text-center">
              <h2 className="text-2xl font-semibold tracking-tight text-foreground">{centeredTitle}</h2>
            </div>
            <ChatComposer
              readOnly={readOnly}
              isRunning={isRunning}
              taskStatus={taskStatus}
              showSendWhileRunning={showSendWhileRunning}
              hasComposerPayload={hasComposerPayload}
              input={input}
              onInputChange={setInput}
              onSend={handleSend}
              onStop={handleStopClick}
              placeholder={placeholder}
              pendingAttachments={pendingAttachments}
              onRemoveAttachment={removePendingAttachment}
              onAttachmentPickerChange={handleAttachmentPickerChange}
              onComposerPasteCapture={handleComposerPasteCapture}
              attachmentPickerRef={attachmentPickerRef}
              composerWorkerLabel={composerWorkerLabel}
              taskTitle={title}
              taskPlan={taskPlan}
              taskId={taskId}
              sessionId={sessionId}
              sessionContextLoading={sessionContextLoading}
              onOpenSessionInfoDialog={handleOpenSessionInfoDialog}
              onOpenProjectFilePromptsDialog={handleOpenProjectFilePromptsDialog}
              onOpenContextDialog={handleOpenContextDialog}
              contextButtonTitle={contextButtonTitle}
              contextUsagePercent={contextUsagePercent}
              defaultMentionItems={defaultMentionItems}
              onMentionSearch={handleMentionSearch}
              onFileSearch={handleFileSearch}
              onSpecialCommand={handleSpecialCommand}
              pendingMention={pendingMention}
              onInsertMentionComplete={() => setPendingMention(null)}
              composerExtra={composerExtra}
            />
          </div>
        </div>
      ) : (
        <>

      {chatChrome === "default" ? (
      <div className="flex items-center justify-between border-b px-4 py-2">
        <div className="flex min-w-0 items-start gap-2">
          {onBack && (
            <div className="flex shrink-0 items-center gap-1.5">
              <Button
                type="button"
                variant="ghost"
                size="sm"
                className="h-7 gap-1 rounded-md px-2 text-xs text-muted-foreground hover:bg-muted hover:text-foreground focus-visible:ring-2 focus-visible:ring-primary/40"
                onClick={onBack}
                aria-label={backLabel}
              >
                <ArrowLeft className="h-3.5 w-3.5" />
                <span>{backLabel}</span>
              </Button>
              <div className="h-4 w-px bg-border" />
            </div>
          )}
          <div className="mt-0.5 flex h-7 w-7 shrink-0 items-center justify-center rounded-md bg-muted/50 text-muted-foreground">
            <Bot className="h-4 w-4" />
          </div>
          <div className="min-w-0 flex-1">
            <div className="flex min-w-0 flex-wrap items-center gap-2">
              {onBack && (
                <Badge variant="outline" className="rounded-full border-border/70 bg-muted/30 px-1.5 py-0 text-[10px] font-medium text-muted-foreground">
                  子 Worker 会话
                </Badge>
              )}
              <h3 className="truncate text-sm font-medium">{title}</h3>
              {taskStatus && (
                taskStatus === "failed" && errorMessage ? (
                  <button
                    type="button"
                    className={cn(
                      "px-1.5 py-0.5 text-xs underline-offset-2 hover:underline",
                      "bg-red-100 text-red-700"
                    )}
                    onClick={() => setShowErrorDialog(true)}
                  >
                    {taskStatus}
                  </button>
                ) : (
                  <span className={cn(
                    "px-1.5 py-0.5 text-xs",
                    taskStatus === "running" ? "bg-blue-100 text-blue-700" :
                    taskStatus === "done" ? "bg-emerald-100 text-emerald-700" :
                    taskStatus === "cancelled" ? "bg-amber-100 text-amber-800" :
                    taskStatus === "failed" ? "bg-red-100 text-red-700" :
                    "bg-muted text-muted-foreground"
                  )}>
                    {taskStatus}
                  </span>
                )
              )}
              {headerStats.length > 0 && (
                <div className="ml-1 flex flex-wrap items-center gap-1.5 text-[10px] text-muted-foreground">
                {headerStats.map((stat) => (
                  <span
                    key={`${stat.label}-${stat.value}`}
                    className="rounded-full border border-border/70 bg-muted/40 px-2 py-0.5 font-medium tracking-tight"
                  >
                    {stat.label} {'formatter' in stat && stat.formatter ? stat.formatter(stat.value) : formatTokenCount(stat.value)}
                  </span>
                ))}
                </div>
              )}
            </div>
          </div>
        </div>
        <div className="flex items-center gap-1">
          <div className="flex items-center gap-0.5 border-r pr-2 mr-1">
            <TooltipProvider>
              <Tooltip>
                <TooltipTrigger asChild>
                  <Button variant="ghost" size="icon" className="h-7 w-7" onClick={() => scrollToMessage('prev')} disabled={!hasUserMessages}>
                    <ChevronUp className="h-4 w-4" />
                  </Button>
                </TooltipTrigger>
                <TooltipContent>上一条</TooltipContent>
              </Tooltip>
            </TooltipProvider>
            <TooltipProvider>
              <Tooltip>
                <TooltipTrigger asChild>
                  <Button variant="ghost" size="icon" className="h-7 w-7" onClick={() => scrollToMessage('next')} disabled={!hasUserMessages}>
                    <ChevronDown className="h-4 w-4" />
                  </Button>
                </TooltipTrigger>
                <TooltipContent>下一条</TooltipContent>
              </Tooltip>
            </TooltipProvider>
          </div>

          <TooltipProvider>
            <Tooltip>
              <TooltipTrigger asChild>
                <Button
                  variant="ghost"
                  size="icon"
                  className="h-7 w-7"
                  onClick={handleExportSessionTransfer}
                  disabled={isExportingSession}
                >
                  {isExportingSession ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Download className="h-3.5 w-3.5" />}
                </Button>
              </TooltipTrigger>
              <TooltipContent>导出记忆和聊天记录</TooltipContent>
            </Tooltip>
          </TooltipProvider>

          <TooltipProvider>
            <Tooltip>
              <TooltipTrigger asChild>
                <Button
                  variant="ghost"
                  size="icon"
                  className="h-7 w-7"
                  onClick={handlePickImportFile}
                  disabled={isImportingSession || readOnly}
                >
                  {isImportingSession ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Upload className="h-3.5 w-3.5" />}
                </Button>
              </TooltipTrigger>
              <TooltipContent>导入并覆盖当前记忆和聊天记录</TooltipContent>
            </Tooltip>
          </TooltipProvider>

          {headerExtra}
          
          {onClearHistory && (
             <Button variant="ghost" size="icon" className="h-7 w-7" onClick={onClearHistory}>
                <Trash2 className="h-3.5 w-3.5" />
             </Button>
          )}

          {(sessionId || taskId) && (
            <TooltipProvider>
              <Tooltip>
                <TooltipTrigger asChild>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="h-7 w-7"
                    onClick={handleOpenPromptDialog}
                    disabled={loadingPrompt}
                  >
                    {loadingPrompt ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Braces className="h-3.5 w-3.5" />}
                  </Button>
                </TooltipTrigger>
                <TooltipContent>查看当前 Prompt</TooltipContent>
              </Tooltip>
            </TooltipProvider>
          )}

          {isConnected !== undefined && (
            <div className="flex items-center gap-1.5 ml-1">
              <span className={cn(
                "h-2 w-2",
                isConnecting ? "bg-yellow-500 animate-pulse" : isConnected ? "bg-emerald-500" : "bg-muted-foreground"
              )} />
              <span className="text-xs text-muted-foreground">
                {isConnecting ? "连接中..." : isConnected ? "已连接" : "未连接"}
              </span>
            </div>
          )}
        </div>
      </div>
      ) : (
        <>
          {onBack ? (
            <div className="flex items-center border-b px-3 py-1.5">
              <Button
                type="button"
                variant="ghost"
                size="sm"
                className="h-7 gap-1 rounded-md px-2 text-xs text-muted-foreground hover:bg-muted hover:text-foreground"
                onClick={onBack}
                aria-label={backLabel}
              >
                <ArrowLeft className="h-3.5 w-3.5" />
                <span>{backLabel}</span>
              </Button>
            </div>
          ) : null}
          {workbenchTitleToolbarActions && !setWorkbenchChatTitleToolbar ? (
            <div className="flex shrink-0 items-center border-b border-border/60 bg-muted/10 px-2 py-0.5 [-webkit-app-region:no-drag]">
              {workbenchTitleToolbarActions}
            </div>
          ) : null}
        </>
      )}

      {/* Messages List（长会话用虚拟列表；滚动容器用原生 overflow 以便 virtualizer 绑定） */}
      <div className={cn("relative min-h-0 min-w-0 flex-1 flex-col", showAlternateContent ? "hidden" : "flex")}>
        <div
          ref={scrollContainerRef}
          onScroll={handleMessagesScroll}
          className="min-h-0 flex-1 min-w-0 overflow-y-auto overflow-x-hidden scrollbar-thin scrollbar-thumb-muted-foreground/20 scrollbar-track-transparent"
        >
        <div ref={historyLoadSentinelRef} className="h-px w-full shrink-0" aria-hidden="true" />
        <div className="min-w-0 overflow-x-hidden p-4 space-y-4">
          {errorMessage && !hasInlineAssistantError && (
            <div className="flex items-start gap-2 rounded border border-red-200 bg-red-50 px-3 py-2 text-xs text-red-800">
              <AlertCircle className="mt-0.5 h-4 w-4 shrink-0" />
              <div className="min-w-0 flex-1 space-y-1">
                <div className="font-medium">最近一次模型调用错误</div>
                <pre className="max-w-full whitespace-pre-wrap break-words font-mono [overflow-wrap:anywhere]">{errorMessage}</pre>
              </div>
            </div>
          )}

          {shouldRetry && onRetryFromError && (
            <div className="flex items-center gap-2 px-3 py-2 bg-amber-50 border border-amber-200 rounded text-xs text-amber-800 mb-4">
              <AlertCircle className="h-4 w-4 shrink-0" />
              <span className="flex-1">任务执行遇到错误，建议重新执行</span>
              <Button size="sm" variant="outline" className="h-7 text-xs" onClick={onRetryFromError}>重新执行</Button>
            </div>
          )}

          {isHistoryLoading && (
            <div className="sticky top-0 z-10 flex justify-center py-1">
              <div className="flex items-center gap-2 rounded-full border border-border/70 bg-background/95 px-3 py-1 text-[11px] text-muted-foreground shadow-sm backdrop-blur-sm">
                <Loader2 className="h-3.5 w-3.5 animate-spin" />
                正在加载更早消息…
              </div>
            </div>
          )}

          {hasMoreHistory && !isHistoryLoading && (
            <div className="flex justify-center">
              <Button
                type="button"
                variant="outline"
                size="sm"
                className="h-8 text-xs"
                onClick={() => {
                  beginLoadMoreHistory()
                }}
              >
                加载更早消息（100 条）
              </Button>
            </div>
          )}

          {renderMessages.length === 0 ? (
            isHistoryLoading ? (
              <div className="flex flex-col items-center justify-center gap-2 py-10 text-xs text-muted-foreground">
                <Loader2 className="h-5 w-5 animate-spin" />
                正在加载对话记录…
              </div>
            ) : (
              <div className="text-center text-xs text-muted-foreground py-10">暂无消息</div>
            )
          ) : virtualizeMessages ? (
            <div className="relative w-full" style={{ height: `${virtualizer.getTotalSize()}px` }}>
              {virtualizer.getVirtualItems().map((vi) => {
                const msg = renderMessages[vi.index]
                const isLastMessage = vi.index === renderMessages.length - 1
                const previousMsg = vi.index > 0 ? renderMessages[vi.index - 1] : null
                const showAssistantHeader = !(msg.info.role === "assistant" && previousMsg?.info.role === "assistant")
                return (
                  <div
                    key={msg.info.id}
                    data-index={vi.index}
                    ref={virtualizer.measureElement}
                    className="absolute left-0 top-0 w-full pb-4"
                    style={{ transform: `translateY(${vi.start}px)` }}
                  >
                    <ChatV2MessageBlock
                      msg={msg}
                      showAssistantHeader={showAssistantHeader}
                      isLastRetryableUserMessage={isUserAuthoredMessage(msg) && msg.info.id === lastRetryableUserMessageId}
                      isHighlighted={highlightedMessageId === msg.info.id}
                      readOnly={!!readOnly}
                      onCancelTool={onCancelTool}
                      onOpenTask={onOpenTask}
                      onRetryUserMessage={onRetryUserMessage}
                      onViewMemory={handleViewMemory}
                      canViewPrompt={!!(sessionId || taskId)}
                      onMessageLayoutChange={handleVirtualizedMessageLayoutChange}
                      isLastMessage={isLastMessage}
                    />
                  </div>
                )
              })}
            </div>
          ) : (
            <>
              {renderMessages.map((msg, idx) => {
                const previousMsg = idx > 0 ? renderMessages[idx - 1] : null
                const showAssistantHeader = !(msg.info.role === "assistant" && previousMsg?.info.role === "assistant")
                return (
                  <ChatV2MessageBlock
                    key={msg.info.id}
                    msg={msg}
                    showAssistantHeader={showAssistantHeader}
                    isLastRetryableUserMessage={isUserAuthoredMessage(msg) && msg.info.id === lastRetryableUserMessageId}
                    isHighlighted={highlightedMessageId === msg.info.id}
                    readOnly={!!readOnly}
                    onCancelTool={onCancelTool}
                    onOpenTask={onOpenTask}
                    onRetryUserMessage={onRetryUserMessage}
                    onViewMemory={handleViewMemory}
                    canViewPrompt={!!(sessionId || taskId)}
                    isLastMessage={idx === renderMessages.length - 1}
                  />
                )
              })}
              <div ref={bottomRef} />
            </>
          )}
        </div>
        </div>
        {!stickToBottom && renderMessages.length > 0 && (
          <div className="pointer-events-none absolute inset-x-0 bottom-3 z-10 flex justify-center">
            <Button
              type="button"
              variant="secondary"
              size="sm"
              className="pointer-events-auto h-8 gap-1.5 rounded-full border border-border/80 bg-background/95 shadow-md backdrop-blur-sm"
              onClick={() => scrollToBottom("smooth")}
            >
              <ArrowDown className="h-3.5 w-3.5" />
              回到底部
            </Button>
          </div>
        )}
      </div>

      <div className={cn("min-h-0 min-w-0 flex-1 w-full", showAlternateContent ? "flex flex-col" : "hidden")}>
        {alternateContent}
      </div>

      <Dialog open={showErrorDialog} onOpenChange={setShowErrorDialog}>
        <DialogContent className="max-w-lg overflow-hidden">
          <DialogHeader>
            <DialogTitle>任务失败原因</DialogTitle>
            <DialogDescription>
              <pre className="max-w-full whitespace-pre-wrap text-xs text-muted-foreground break-words [overflow-wrap:anywhere]">{errorMessage}</pre>
            </DialogDescription>
          </DialogHeader>
        </DialogContent>
      </Dialog>
        </>
      )}

      {/* Memory Dialog */}
      <MemoryDialog 
        memory={selectedMemory}
        prompt={selectedPrompt}
        rawResponse={selectedRawResponse}
        loading={memoryDialogLoading}
        open={showMemoryDialog}
        onOpenChange={(open) => {
          setShowMemoryDialog(open)
          if (!open) {
            setMemoryDialogLoading(false)
          }
        }}
      />

      <ChatSessionDialogs
        showSessionInfoDialog={showSessionInfoDialog}
        setShowSessionInfoDialog={setShowSessionInfoDialog}
        sessionContext={sessionContext}
        sessionContextLoading={sessionContextLoading}
        composerWorkerLabel={composerWorkerLabel}
        installedSkillsCatalog={installedSkillsCatalog}
        loadedSkillsFromChat={loadedSkillsFromChat}
        handleCopyText={handleCopyText}
        showContextDialog={showContextDialog}
        setShowContextDialog={setShowContextDialog}
        headerStats={headerStats}
        contextLimitTokens={contextLimitTokens}
        formatByteCount={formatByteCount}
        formatTokenCount={formatTokenCount}
        showProjectFilePromptsDialog={showProjectFilePromptsDialog}
        setShowProjectFilePromptsDialog={setShowProjectFilePromptsDialog}
        showPromptDialog={showPromptDialog}
        setShowPromptDialog={setShowPromptDialog}
        currentPrompt={currentPrompt}
        promptDialogTab={promptDialogTab}
        setPromptDialogTab={setPromptDialogTab}
        promptCopied={promptCopied}
        handleCopyPrompt={handleCopyPrompt}
        promptDialogHistoryText={promptDialogHistoryText}
        showImportConfirm={showImportConfirm}
        setShowImportConfirm={setShowImportConfirm}
        pendingImportFilename={pendingImportFilename}
        isImportingSession={isImportingSession}
        clearPendingImport={clearPendingImport}
        handleConfirmImport={handleConfirmImport}
      />
      {!centeredComposer ? (
        <ChatComposer
          readOnly={readOnly}
          isRunning={isRunning}
          taskStatus={taskStatus}
          showSendWhileRunning={showSendWhileRunning}
          hasComposerPayload={hasComposerPayload}
          input={input}
          onInputChange={setInput}
          onSend={handleSend}
          onStop={handleStopClick}
          placeholder={placeholder}
          pendingAttachments={pendingAttachments}
          onRemoveAttachment={removePendingAttachment}
          onAttachmentPickerChange={handleAttachmentPickerChange}
          onComposerPasteCapture={handleComposerPasteCapture}
          attachmentPickerRef={attachmentPickerRef}
          composerWorkerLabel={composerWorkerLabel}
          taskTitle={title}
          taskPlan={taskPlan}
          taskId={taskId}
          sessionId={sessionId}
          sessionContextLoading={sessionContextLoading}
          onOpenSessionInfoDialog={handleOpenSessionInfoDialog}
          onOpenProjectFilePromptsDialog={handleOpenProjectFilePromptsDialog}
          onOpenContextDialog={handleOpenContextDialog}
          contextButtonTitle={contextButtonTitle}
          contextUsagePercent={contextUsagePercent}
          defaultMentionItems={defaultMentionItems}
          onMentionSearch={handleMentionSearch}
          onFileSearch={handleFileSearch}
          onSpecialCommand={handleSpecialCommand}
          pendingMention={pendingMention}
          onInsertMentionComplete={() => setPendingMention(null)}
          composerExtra={composerExtra}
          messageQueue={messageQueue}
          messageQueueAutoSend={messageQueueAutoSend}
          onMessageQueueAutoSendChange={onMessageQueueAutoSendChange}
          onMessageQueueChange={onMessageQueueChange}
          onSendNextQueueItem={onSendNextQueueItem}
          showAlternateContent={showAlternateContent}
        />
      ) : null}

      {/* Review Dialog */}
      <ReviewDialog
        open={showReviewDialog}
        onOpenChange={setShowReviewDialog}
        onConfirm={handleReviewConfirm}
        projectId={projectId}
        taskId={taskId}
      />
    </div>
  )
}
