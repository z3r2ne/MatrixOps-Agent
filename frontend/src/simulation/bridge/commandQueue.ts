/**
 * 阶段 2：将 WebSocket / 工具调用事件映射为仿真指令。
 * 当前仅占位，阶段 1 由 React 直接构建 AgentVisualState。
 */
export type SimulationCommand =
  | { type: "WALK_TO"; taskId: number; targetTaskId?: number; deskId?: string }
  | { type: "SAY"; taskId: number; text: string; targetTaskId?: number }
  | { type: "WORK_AT_DESK"; taskId: number }
  | { type: "SCREEN_UPDATE"; taskId: number; text: string }

export interface SimulationCommandQueue {
  commands: SimulationCommand[]
}

export function createSimulationCommandQueue(): SimulationCommandQueue {
  return { commands: [] }
}
