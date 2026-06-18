import React, { useEffect, useMemo, useRef } from "react"

import type { AgentVisualState } from "../types"
import type { SimulationRuntime } from "../engine/createSimulationRuntime"
import { createSimulationRuntime } from "../engine/createSimulationRuntime"
import { cn } from "@/lib/utils"

export interface SimulationCanvasProps {
  agents: AgentVisualState[]
  onFocusTask: (taskId: number) => void
  className?: string
}

function buildAgentsSyncKey(agents: AgentVisualState[]): string {
  return agents
    .map((agent) => [
      agent.taskId,
      agent.isRunning ? 1 : 0,
      agent.workingFrame,
      agent.task.status,
      agent.screenText,
      agent.desk.x.toFixed(1),
      agent.desk.y.toFixed(1),
      agent.desk.scale.toFixed(2),
    ].join(":"))
    .join("|")
}

export function SimulationCanvas({ agents, onFocusTask, className }: SimulationCanvasProps) {
  const hostRef = useRef<HTMLDivElement | null>(null)
  const runtimeRef = useRef<SimulationRuntime | null>(null)
  const initGenerationRef = useRef(0)
  const lifecycleRef = useRef({ disposed: false })
  const agentsRef = useRef(agents)
  const onFocusRef = useRef(onFocusTask)
  const pendingSyncKeyRef = useRef<string | null>(null)

  agentsRef.current = agents
  onFocusRef.current = onFocusTask

  const agentsSyncKey = useMemo(() => buildAgentsSyncKey(agents), [agents])

  const syncAgentsIfReady = (syncKey: string) => {
    const runtime = runtimeRef.current
    if (!runtime || runtime.isDestroyed) {
      pendingSyncKeyRef.current = syncKey
      return
    }
    pendingSyncKeyRef.current = null
    void runtime.scene.syncAgents(agentsRef.current)
  }

  useEffect(() => {
    const host = hostRef.current
    if (!host) return

    const initGeneration = ++initGenerationRef.current
    lifecycleRef.current.disposed = false
    let resizeObserver: ResizeObserver | null = null

    void (async () => {
      try {
        const runtime = await createSimulationRuntime(host, (taskId) => onFocusRef.current(taskId))
        if (lifecycleRef.current.disposed || initGeneration !== initGenerationRef.current) {
          runtime.destroy()
          return
        }

        runtimeRef.current = runtime
        const initialKey = buildAgentsSyncKey(agentsRef.current)
        pendingSyncKeyRef.current = null
        await runtime.scene.syncAgents(agentsRef.current)

        resizeObserver = new ResizeObserver(() => {
          if (lifecycleRef.current.disposed || runtime.isDestroyed) return
          runtime.scene.resize(host.clientWidth, host.clientHeight)
        })
        resizeObserver.observe(host)

        if (pendingSyncKeyRef.current && pendingSyncKeyRef.current !== initialKey) {
          void runtime.scene.syncAgents(agentsRef.current)
          pendingSyncKeyRef.current = null
        }
      } catch (error) {
        if (!lifecycleRef.current.disposed) {
          console.error("[simulation] failed to start runtime:", error)
        }
      }
    })()

    return () => {
      lifecycleRef.current.disposed = true
      initGenerationRef.current += 1
      resizeObserver?.disconnect()
      runtimeRef.current?.destroy()
      runtimeRef.current = null
      pendingSyncKeyRef.current = null
      if (host) {
        while (host.firstChild) {
          host.removeChild(host.firstChild)
        }
      }
    }
  }, [])

  useEffect(() => {
    syncAgentsIfReady(agentsSyncKey)
  }, [agentsSyncKey])

  return <div ref={hostRef} className={cn("h-full w-full", className)} />
}
