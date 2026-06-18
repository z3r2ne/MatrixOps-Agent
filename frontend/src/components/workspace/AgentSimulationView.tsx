import React, { useCallback, useEffect, useMemo, useRef, useState } from "react"
import { Loader2, Send } from "lucide-react"

import type { Task, WithParts } from "@/lib/api"
import { useGlobalWebSocket } from "@/hooks/useGlobalWebSocket"
import { extractSimulationScreenOutput, taskStatusLabel } from "@/lib/simulationOutput"
import {
  dedupeTasksById,
  extractSubtaskIdsFromMessages,
  patchSubtaskParentLinks,
} from "@/lib/simulationTasks"
import { collectTaskTree } from "@/lib/taskTree"
import { api } from "@/lib/api"
import { isTaskRunning, SCREEN_REGIONS, simulationAsset } from "@/lib/simulationAssets"
import { SimulationCanvas } from "@/simulation/components/SimulationCanvas"
import { buildAgentSnapshots } from "@/simulation/map/officeMap"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { cn } from "@/lib/utils"

interface AgentSimulationViewProps {
  rootTaskId: number | null
  tasks: Task[]
  className?: string
}

interface FocusedMonitorProps {
  task: Task
  messages: WithParts[]
  output: string
  onClose: () => void
  onSend: (content: string) => void
  sending: boolean
}

function FocusedMonitor({ task, messages, output, onClose, onSend, sending }: FocusedMonitorProps) {
  const [draft, setDraft] = useState("")
  const workerName = task.workerName?.trim() || "agent"
  const title = task.name?.trim() || task.content?.trim() || `任务 #${task.id}`
  const region = SCREEN_REGIONS["monitor-focus"]

  const handleSubmit = (event: React.FormEvent) => {
    event.preventDefault()
    const content = draft.trim()
    if (!content || sending) return
    onSend(content)
    setDraft("")
  }

  return (
    <div
      className="absolute inset-0 z-[100] flex items-center justify-center bg-slate-950/65 p-4 backdrop-blur-[4px]"
      onClick={onClose}
    >
      <div className="relative w-[min(920px,96%)]" onClick={(event) => event.stopPropagation()}>
        <img
          src={simulationAsset("monitor-focus-frame.png")}
          alt=""
          className="pointer-events-none w-full select-none"
          draggable={false}
        />

        <div
          className="absolute flex flex-col overflow-hidden bg-[#0b1220]"
          style={{
            left: `${region.left * 100}%`,
            top: `${region.top * 100}%`,
            width: `${region.width * 100}%`,
            height: `${region.height * 100}%`,
          }}
        >
          <div className="flex items-center justify-between border-b border-white/10 px-3 py-2">
            <div className="min-w-0">
              <div className="truncate text-sm font-medium text-slate-100">{title}</div>
              <div className="truncate text-xs text-slate-400">
                {workerName} · {taskStatusLabel(task.status)}
              </div>
            </div>
            <Button type="button" variant="ghost" size="sm" className="text-slate-300 hover:text-white" onClick={onClose}>
              缩小
            </Button>
          </div>

          <div className="min-h-0 flex-1 overflow-y-auto p-3">
            {messages.length === 0 ? (
              <pre className="whitespace-pre-wrap font-mono text-sm leading-6 text-emerald-100/90">{output}</pre>
            ) : (
              <div className="space-y-3">
                {messages.slice(-12).map((message) => (
                  <div key={message.info.id} className="rounded-lg border border-white/10 bg-black/25 p-3">
                    <div className="mb-2 text-xs font-medium text-slate-300">
                      {message.info.role === "user" ? "用户" : message.info.worker || "assistant"}
                    </div>
                    <div className="space-y-2 text-sm text-slate-100">
                      {(message.parts ?? []).map((part) => {
                        if (part.type === "text" && part.text) {
                          return (
                            <pre key={part.id} className="whitespace-pre-wrap font-sans text-sm leading-6 text-slate-100">
                              {part.text}
                            </pre>
                          )
                        }
                        if ((part.type === "tool" || part.type === "tool-delta") && part.tool) {
                          const toolName = part.tool.tool || "tool"
                          const state = part.tool.state
                          return (
                            <div key={part.id} className="rounded border border-white/10 bg-white/5 p-2 text-xs text-slate-200">
                              <div className="font-medium text-slate-300">[{toolName}] {state?.title || state?.status}</div>
                              {state?.output ? (
                                <pre className="mt-1 max-h-40 overflow-auto whitespace-pre-wrap font-mono text-[11px] leading-5 text-emerald-100/90">
                                  {state.output}
                                </pre>
                              ) : null}
                            </div>
                          )
                        }
                        if (part.type === "error") {
                          const messageText = part.error?.message || part.text
                          return messageText ? (
                            <div key={part.id} className="text-xs text-red-300">
                              {messageText}
                            </div>
                          ) : null
                        }
                        return null
                      })}
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>

          <form onSubmit={handleSubmit} className="border-t border-white/10 p-2">
            <div className="flex items-center gap-2">
              <Input
                value={draft}
                onChange={(event) => setDraft(event.target.value)}
                placeholder="向该 Agent 发送消息…"
                className="border-white/10 bg-black/30 text-slate-100 placeholder:text-slate-500"
                disabled={sending}
              />
              <Button type="submit" disabled={sending || !draft.trim()} className="shrink-0">
                {sending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Send className="h-4 w-4" />}
              </Button>
            </div>
          </form>
        </div>
      </div>
    </div>
  )
}

export function AgentSimulationView({ rootTaskId, tasks: seedTasks, className }: AgentSimulationViewProps) {
  const { subscribe, unsubscribe, taskStates, sendMessage } = useGlobalWebSocket()
  const taskStatesRef = useRef(taskStates)
  taskStatesRef.current = taskStates

  const [focusedTaskId, setFocusedTaskId] = useState<number | null>(null)
  const [sendingTaskId, setSendingTaskId] = useState<number | null>(null)
  const [workingFrame, setWorkingFrame] = useState<0 | 1>(0)
  const [loadedSubtasks, setLoadedSubtasks] = useState<Task[]>([])

  const seedTaskIdsKey = useMemo(
    () => seedTasks.map((task) => task.id).sort((a, b) => a - b).join(","),
    [seedTasks],
  )

  useEffect(() => {
    setLoadedSubtasks([])
  }, [rootTaskId, seedTaskIdsKey])

  const subtaskDiscoveryKey = useMemo(() => {
    const chunks: string[] = []
    for (const task of seedTasks) {
      const messages = taskStates.get(task.id)?.messagesV2 ?? []
      const ids = extractSubtaskIdsFromMessages(messages).sort((a, b) => a - b)
      chunks.push(`${task.id}=${ids.join(".")}`)
    }
    return chunks.join("|")
  }, [seedTasks, taskStates])

  const pendingSubtaskIdsKey = useMemo(() => {
    const known = new Set([...seedTasks, ...loadedSubtasks].map((task) => task.id))
    const missing = new Set<number>()

    for (const task of seedTasks) {
      const messages = taskStates.get(task.id)?.messagesV2 ?? []
      for (const subtaskId of extractSubtaskIdsFromMessages(messages)) {
        if (!known.has(subtaskId)) missing.add(subtaskId)
      }
    }

    return [...missing].sort((a, b) => a - b).join(",")
  }, [loadedSubtasks, seedTasks, subtaskDiscoveryKey])

  useEffect(() => {
    if (!pendingSubtaskIdsKey) return

    const ids = pendingSubtaskIdsKey.split(",").map((id) => Number(id))
    let cancelled = false
    void Promise.all(ids.map((id) => api.getTask(id)))
      .then((fetched) => {
        if (cancelled) return
        setLoadedSubtasks((prev) => dedupeTasksById([...prev, ...fetched]))
      })
      .catch((error) => {
        console.error("[simulation] failed to load subtasks:", error)
      })

    return () => {
      cancelled = true
    }
  }, [pendingSubtaskIdsKey])

  const taskIdsKey = useMemo(() => {
    if (!rootTaskId) return ""
    const merged = dedupeTasksById([...seedTasks, ...loadedSubtasks])
    const patched = patchSubtaskParentLinks(merged, taskStatesRef.current)
    return collectTaskTree(patched, rootTaskId)
      .map((task) => task.id)
      .sort((a, b) => a - b)
      .join(",")
  }, [loadedSubtasks, rootTaskId, seedTaskIdsKey])

  const tasks = useMemo(() => {
    if (!rootTaskId) return []
    const merged = dedupeTasksById([...seedTasks, ...loadedSubtasks])
    const patched = patchSubtaskParentLinks(merged, taskStatesRef.current)
    return collectTaskTree(patched, rootTaskId)
  }, [loadedSubtasks, rootTaskId, seedTasks, seedTaskIdsKey])

  const resolvedRootTaskId = rootTaskId ?? tasks[0]?.id ?? null

  const hasRunningTask = useMemo(
    () => tasks.some((task) => isTaskRunning(task.status)),
    [tasks],
  )

  const screenTextByTaskId = useMemo(() => {
    const map = new Map<number, string>()
    for (const task of tasks) {
      const messages = taskStates.get(task.id)?.messagesV2 ?? []
      map.set(task.id, extractSimulationScreenOutput(messages))
    }
    return map
  }, [taskStates, tasks])

  const runningTaskIds = useMemo(() => {
    const set = new Set<number>()
    for (const task of tasks) {
      if (isTaskRunning(task.status)) set.add(task.id)
    }
    return set
  }, [tasks])

  const agents = useMemo(
    () => buildAgentSnapshots(tasks, resolvedRootTaskId, screenTextByTaskId, runningTaskIds, workingFrame),
    [resolvedRootTaskId, runningTaskIds, screenTextByTaskId, tasks, workingFrame],
  )

  useEffect(() => {
    if (!taskIdsKey) return

    const ids = taskIdsKey.split(",").map((id) => Number(id))
    for (const taskId of ids) {
      subscribe(taskId)
    }
    return () => {
      for (const taskId of ids) {
        unsubscribe(taskId)
      }
    }
  }, [taskIdsKey, subscribe, unsubscribe])

  useEffect(() => {
    if (focusedTaskId == null || !taskIdsKey) return
    const ids = taskIdsKey.split(",").map((id) => Number(id))
    if (!ids.includes(focusedTaskId)) {
      setFocusedTaskId(null)
    }
  }, [focusedTaskId, taskIdsKey])

  useEffect(() => {
    if (!hasRunningTask) return
    const timer = window.setInterval(() => {
      setWorkingFrame((frame) => (frame === 0 ? 1 : 0))
    }, 450)
    return () => window.clearInterval(timer)
  }, [hasRunningTask])

  const focusedTask = useMemo(
    () => (focusedTaskId == null ? null : tasks.find((task) => task.id === focusedTaskId) ?? null),
    [focusedTaskId, tasks],
  )

  const handleSend = useCallback(async (taskId: number, content: string) => {
    setSendingTaskId(taskId)
    try {
      sendMessage(taskId, content)
    } finally {
      window.setTimeout(() => setSendingTaskId((current) => (current === taskId ? null : current)), 400)
    }
  }, [sendMessage])

  if (!rootTaskId || !tasks.length) {
    return (
      <div className={cn("flex h-full items-center justify-center text-sm text-muted-foreground", className)}>
        当前没有可仿真的任务
      </div>
    )
  }

  return (
    <div className={cn("relative flex-1 min-h-0 w-full overflow-hidden", className)}>
      <SimulationCanvas
        agents={agents}
        onFocusTask={setFocusedTaskId}
        className="absolute inset-0 h-full w-full"
      />

      {focusedTask ? (
        <FocusedMonitor
          task={focusedTask}
          messages={taskStates.get(focusedTask.id)?.messagesV2 ?? []}
          output={extractSimulationScreenOutput(taskStates.get(focusedTask.id)?.messagesV2)}
          onClose={() => setFocusedTaskId(null)}
          onSend={(content) => void handleSend(focusedTask.id, content)}
          sending={sendingTaskId === focusedTask.id}
        />
      ) : null}
    </div>
  )
}
