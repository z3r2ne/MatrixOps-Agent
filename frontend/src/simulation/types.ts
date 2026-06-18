import type { Task } from "@/lib/api"

export interface DeskSlot {
  id: string
  gx: number
  gy: number
  scale: number
  depth: number
  /** 世界像素锚点（脚底中心） */
  x: number
  y: number
}

export interface AgentVisualState {
  taskId: number
  task: Task
  desk: DeskSlot
  screenText: string
  isRoot: boolean
  isRunning: boolean
  workingFrame: 0 | 1
}

export interface SimulationSceneSnapshot {
  agents: AgentVisualState[]
}
